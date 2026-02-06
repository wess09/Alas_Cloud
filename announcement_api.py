import os
import json
import logging
import asyncio  # 新增: 用于后台定时任务
from pathlib import Path
from typing import Dict, Any, Optional
from datetime import datetime, timedelta
import re
import secrets
import jwt
from contextlib import asynccontextmanager  # 新增用于 lifespan

from fastapi import FastAPI, HTTPException, Request, Depends, Header
from fastapi.responses import JSONResponse, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel, Field
import uvicorn

# ----------------- 基础配置 -----------------
BASE_DIR = Path(__file__).parent

# 数据目录配置 - 支持通过环境变量自定义
DATA_DIR = Path(os.getenv("DATA_DIR", BASE_DIR))
DATA_DIR.mkdir(exist_ok=True)

BUG_LOG_DIR_PATH = DATA_DIR / "bug_logs"
BLACKLIST_FILE = DATA_DIR / "blacklist.txt"
LOG_FILE = DATA_DIR / "api.log"
FRONTEND_DIR = BASE_DIR / "frontend"

# JWT 配置
# JWT 配置
SECRET_FILE = BASE_DIR / ".jwt_secret"
if SECRET_FILE.exists():
    JWT_SECRET = SECRET_FILE.read_text().strip()
else:
    JWT_SECRET = secrets.token_urlsafe(32)
    SECRET_FILE.write_text(JWT_SECRET)
JWT_ALGORITHM = "HS256"
JWT_EXPIRE_HOURS = 24

# 引入数据库操作
from telemetry_db import (
    init_db, 
    insert_or_update_telemetry, 
    get_aggregate_stats, 
    delete_inactive_instances,
    # 公告相关
    get_latest_announcement,
    create_announcement,
    list_announcements,
    delete_announcement,
    toggle_announcement_active,
    # 管理员相关
    verify_admin_password,
    ensure_default_admin,
    update_admin_password
)


# ----------------- 日志配置 -----------------
# (移除重复定义的 BASE_DIR 等变量)

# 自定义日志过滤器: 过滤掉高频路径的访问日志
class EndpointFilter(logging.Filter):
    """过滤指定路径的 uvicorn access log"""
    
    # 需要过滤的高频路径关键字
    FILTERED_KEYWORDS = {
        "/api/get/announcement",
        "/health",
    }
    
    def filter(self, record: logging.LogRecord) -> bool:
        # 获取格式化后的消息
        message = record.getMessage()
        # 如果消息包含任何过滤关键字，则不记录
        return not any(kw in message for kw in self.FILTERED_KEYWORDS)

def setup_logging():
    """
    配置全局日志：同时输出到控制台和文件，并应用过滤器
    """
    log_format = "%(asctime)s [%(levelname)s] %(name)s: %(message)s"
    date_format = "%Y-%m-%d %H:%M:%S"
    
    # 1. 创建根日志记录器
    root_logger = logging.getLogger()
    root_logger.setLevel(logging.INFO)
    
    # 清理现有的处理器，防止重复
    if root_logger.hasHandlers():
        root_logger.handlers.clear()

    # 2. 创建格式化器
    formatter = logging.Formatter(log_format, datefmt=date_format)

    # 3. 控制台处理器
    console_handler = logging.StreamHandler()
    console_handler.setFormatter(formatter)
    root_logger.addHandler(console_handler)

    # 4. 文件处理器 (自动切分，保持 5 个 10MB 文件)
    from logging.handlers import RotatingFileHandler
    file_handler = RotatingFileHandler(
        LOG_FILE, maxBytes=10*1024*1024, backupCount=5, encoding="utf-8"
    )
    file_handler.setFormatter(formatter)
    root_logger.addHandler(file_handler)

    # 5. 特定日志器配置
    # 获取 uvicorn 相关的日志器并应用过滤器
    for logger_name in ["uvicorn.access"]:
        log_obj = logging.getLogger(logger_name)
        log_obj.addFilter(EndpointFilter())
        # 强制其向上传递给 root 处理器，而不是使用 uvicorn 自己的默认配置
        log_obj.propagate = True

# 初始化日志
setup_logging()
logger = logging.getLogger("AlasAPI")

# ----------------- 基础配置 & 全局变量 -----------------
# 变量已移至顶部定义

# 内存中的黑名单字典: {IP: 过期时间戳}
BLACKLISTED_IPS = {}

# 后台清理任务句柄 (用于优雅退出)
cleanup_task: asyncio.Task = None

@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    FastAPI 生命周期管理: 代替已弃用的 startup/shutdown 事件
    """
    global cleanup_task
    
    # --- [Startup 逻辑] ---
    await init_db()
    await ensure_default_admin()  # 确保存在默认管理员
    load_blacklist()
    # 启动后台清理任务
    cleanup_task = asyncio.create_task(scheduled_cleanup_task(run_immediately=True))
    logger.info(f"✅ 服务启动完成 | 遥测数据库已连接 | 黑名单 IP: {len(BLACKLISTED_IPS)} 个")
    
    yield  # 这里是应用运行的时间点
    
    # --- [Shutdown 逻辑] ---
    if cleanup_task and not cleanup_task.done():
        cleanup_task.cancel()
        try:
            await cleanup_task
        except asyncio.CancelledError:
            pass
    logger.info("👋 服务已优雅关闭")

app = FastAPI(
    title="Alas API", 
    description="公告数据API + 遥测统计API",
    lifespan=lifespan
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# ----------------- 工具函数: 黑名单管理 -----------------

def load_blacklist():
    """启动时从文件加载黑名单到内存，并清理已过期的记录"""
    if BLACKLIST_FILE.exists():
        try:
            current_time = datetime.now()
            valid_entries = []
            
            with open(BLACKLIST_FILE, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    
                    # 格式: IP,过期时间戳
                    parts = line.split(",")
                    if len(parts) == 2:
                        ip, expire_str = parts
                        try:
                            expire_time = datetime.fromisoformat(expire_str)
                            if expire_time > current_time:
                                # 未过期
                                BLACKLISTED_IPS[ip] = expire_time
                                valid_entries.append(f"{ip},{expire_str}")
                        except ValueError:
                            logger.warning(f"无效的黑名单记录: {line}")
            
            # 更新文件,移除已过期的记录
            if valid_entries:
                with open(BLACKLIST_FILE, "w", encoding="utf-8") as f:
                    f.write("\n".join(valid_entries) + "\n")
            else:
                # 清空文件
                BLACKLIST_FILE.write_text("", encoding="utf-8")
                
        except Exception as e:
            logger.error(f"加载黑名单文件失败: {e}")

def ban_ip(ip: str):
    """将 IP 加入黑名单，拉黑8小时"""
    if not ip:
        return
    
    expire_time = datetime.now() + timedelta(hours=8)
    BLACKLISTED_IPS[ip] = expire_time
    
    try:
        # 读取现有记录
        existing_records = {}
        if BLACKLIST_FILE.exists():
            with open(BLACKLIST_FILE, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if line and "," in line:
                        parts = line.split(",")
                        if len(parts) == 2:
                            existing_records[parts[0]] = parts[1]
        
        # 更新或添加新记录
        existing_records[ip] = expire_time.isoformat()
        
        # 写回文件
        with open(BLACKLIST_FILE, "w", encoding="utf-8") as f:
            for ip_addr, expire_str in existing_records.items():
                f.write(f"{ip_addr},{expire_str}\n")
                
    except Exception as e:
        logger.error(f"写入黑名单文件失败: {e}")

def get_client_ip(request: Request) -> str:
    """统一提取客户端真实 IP"""
    ip = request.headers.get("cf-connecting-ip")
    if not ip:
        forwarded = request.headers.get("x-forwarded-for")
        ip = forwarded.split(",")[0].strip() if forwarded else None
    if not ip:
        ip = request.client.host if request.client else "unknown"
    return ip

# ----------------- 后台任务: 清理过期数据 -----------------

async def scheduled_cleanup_task(run_immediately: bool = True):
    """
    后台循环任务：每小时检查并清理一次过期数据 (超过24小时未更新)
    :param run_immediately: 是否在启动时立即执行一次清理
    """
    logger.info("🕰️ 后台清理任务已初始化，将每小时执行一次")
    
    # 启动时立即执行一次清理 (可选)
    if run_immediately:
        try:
            logger.info("♻️ 启动时执行首次过期实例清理...")
            deleted_count = await delete_inactive_instances(hours_limit=24)
            if deleted_count > 0:
                logger.info(f"🧹 首次清理完成: 已清理 {deleted_count} 个失效实例")
        except Exception as e:
            logger.error(f"⚠️ 首次清理失败: {e}")
    
    while True:
        try:
            # 每小时执行一次 (3600 秒)
            await asyncio.sleep(3600) 
            
            logger.info("♻️ 开始执行过期实例清理...")
            
            # 删除超过 24 小时未更新的数据
            deleted_count = await delete_inactive_instances(hours_limit=24)
            
            if deleted_count > 0:
                logger.info(f"🧹 已自动清理 {deleted_count} 个失效/恶意污染实例 (超过24h未更新)")
                
        except asyncio.CancelledError:
            logger.info("🛑 清理任务被取消，正在停止...")
            break
        except Exception as e:
            logger.error(f"⚠️ 清理任务发生未捕获异常: {e}", exc_info=True)
            # 出错后短暂休眠，防止死循环
            await asyncio.sleep(60)

# ----------------- 数据模型 -----------------

class TelemetryRequest(BaseModel):
    """遥测数据请求模型"""
    device_id: str = Field(..., description="设备唯一标识符 (Hex Hash)")
    instance_id: str = Field(..., description="实例唯一标识符")
    month: str = Field(..., description="统计月份")
    battle_count: int = Field(..., ge=0)
    battle_rounds: int = Field(..., ge=0)
    sortie_cost: int = Field(..., ge=0)
    akashi_encounters: int = Field(..., ge=0)
    akashi_probability: float = Field(..., ge=0, le=1)
    average_stamina: float = Field(..., ge=0)
    net_stamina_gain: int = Field(...)

class BugReportRequest(BaseModel):
    device_id: Optional[str] = Field(None)
    log_type: str = Field(...)
    log_content: str = Field(...)
    timestamp: Optional[str] = Field(None)
    additional_info: Optional[Dict[str, Any]] = Field(None)

# ----------------- 中间件 -----------------


@app.middleware("http")
async def check_blacklist_middleware(request: Request, call_next):
    """
    全局中间件: 拦截黑名单 IP，并自动清理过期记录
    """
    client_ip = get_client_ip(request)
    
    # 检查是否在黑名单中
    if client_ip in BLACKLISTED_IPS:
        expire_time = BLACKLISTED_IPS[client_ip]
        current_time = datetime.now()
        
        # 检查是否已过期
        if expire_time <= current_time:
            # 已过期，从黑名单中移除
            del BLACKLISTED_IPS[client_ip]
            logger.info(f"♻️ IP {client_ip} 黑名单已到期，自动解除")
        else:
            # 未过期，拦截请求
            remaining_hours = (expire_time - current_time).total_seconds() / 3600
            logger.debug(f"🚫 拦截黑名单 IP: {client_ip}，剩余 {remaining_hours:.1f} 小时")
            return JSONResponse(
                status_code=500, 
                content={"detail": "None"}
            )
        
    response = await call_next(request)
    return response

# ----------------- 路由逻辑 -----------------

@app.get("/api/get/announcement")
async def get_announcement_api(id: Optional[int] = None):
    """
    获取最新公告 (支持增量更新)
    
    参数:
        id: 客户端当前缓存的公告 ID (可选)
    
    返回:
        - 如果 id 不存在或与最新 ID 不匹配: 返回完整公告 (HTTP 200)
        - 如果 id 与最新 ID 匹配: 返回空对象 (HTTP 200) 或 304
    """
    latest = await get_latest_announcement()
    
    if not latest:
        return {}
    
    # 增量更新: 如果客户端 ID 与最新 ID 匹配，返回空对象
    if id is not None and str(id) == str(latest.get("announcementId")):
        return {}  # 兼容方式，也可以用 Response(status_code=304)
    
    return latest

@app.get("/")
async def root():
    return {"service": "Alas API", "status": "running"}

@app.get("/health")
async def health_check():
    return {"status": "healthy"}

@app.post("/api/telemetry")
async def submit_telemetry(data: TelemetryRequest, request: Request):
    """
    提交遥测数据
    逻辑:
    1. 校验格式和数值合理性
    2. 如果异常 -> 拉黑IP + 警告日志 + 默默丢弃
    3. 如果正常 -> 入库 (会自动更新 updated_at)
    """
    try:
        ip_address = get_client_ip(request)

        # ----------------- [严格校验区] -----------------
        
        # 1. 校验 Device ID 格式 (32-64位 hex 字符)
        is_valid_hash = bool(re.match(r"^[a-fA-F0-9]{32,64}$", data.device_id))
        
        # 2. 校验关键数值不能为 0
        is_zero_data = (
            data.akashi_encounters == 0 or
            data.akashi_probability == 0 or
            data.average_stamina == 0 or
            data.net_stamina_gain == 0
        )
        
        # 3. 校验战斗逻辑 (battle_count 必须 > battle_rounds)
        is_invalid_battle = data.battle_count <= data.battle_rounds

        # ----------------- [处置异常] -----------------
        if not is_valid_hash or is_zero_data or is_invalid_battle:
            
            reasons = []
            if not is_valid_hash: reasons.append("Invalid Hash")
            if is_zero_data: reasons.append("Zero Values Forbidden")
            if is_invalid_battle: reasons.append("Count <= Rounds")
            
            # --- 拉黑 IP ---
            ban_ip(ip_address)
            
            logger.warning(
                f"🚫 [BAN ACTION] IP: {ip_address} | Device: {data.device_id} | Reasons: {', '.join(reasons)}"
            )
            
            # 欺骗性返回
            return {
                "status": "success",
                "message": "遥测数据已保存",
                "device_id": data.device_id,
                "instance_id": data.instance_id
            }
        
        # ----------------- [正常入库] -----------------
        telemetry_dict = data.model_dump()
        telemetry_dict["ip_address"] = ip_address

        # 数据库层会自动更新 updated_at
        await insert_or_update_telemetry(telemetry_dict)
        
        return {
            "status": "success",
            "message": "遥测数据已保存",
            "device_id": data.device_id,
            "instance_id": data.instance_id
        }

    except Exception as e:
        logger.error(f"处理遥测数据时发生异常: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/telemetry/stats")
async def get_telemetry_stats_route():
    return await get_aggregate_stats()

@app.post("/api/post/bug")
async def submit_bug_log(data: BugReportRequest, request: Request):
    try:
        if not data.timestamp:
            data.timestamp = datetime.now().isoformat()
        
        raw_device_id = data.device_id or "anonymous"
        # 简单的清理，只保留安全字符
        safe_device_id = re.sub(r'[^a-zA-Z0-9_-]', '', raw_device_id)
        if not safe_device_id: 
            safe_device_id = "anonymous"

        device_dir = BUG_LOG_DIR_PATH / safe_device_id
        device_dir.mkdir(parents=True, exist_ok=True)
        log_file = device_dir / f"{safe_device_id}.log"
        
        user_ip = get_client_ip(request)
        
        info_str = json.dumps(data.additional_info, ensure_ascii=False) if data.additional_info else "None"
        log_entry = f"[{data.timestamp}] [{data.log_type.upper()}] {data.log_content}\n  IP: {user_ip}\n  Additional Info: {info_str}\n\n"
        
        with open(log_file, 'a', encoding='utf-8') as f:
            f.write(log_entry)
        
        logger.info(f"收到 Bug 报告: {safe_device_id}")
        return {"status": "success"}
    except Exception as e:
        logger.error(f"保存 Bug 报告失败: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/get/bugs")
async def get_bug_logs():
    return {"message": "Use previous logic if needed"}


# ===================== 管理 API =====================

class LoginRequest(BaseModel):
    """登录请求"""
    username: str = Field(default="admin")
    password: str = Field(...)


class AnnouncementRequest(BaseModel):
    """创建公告请求"""
    title: str = Field(..., min_length=1)
    content: str = Field(default="")
    url: str = Field(default="")


class ChangePasswordRequest(BaseModel):
    """修改密码请求"""
    old_password: str = Field(...)
    new_password: str = Field(..., min_length=6)


def create_jwt_token(username: str) -> str:
    """创建 JWT Token"""
    expire = datetime.utcnow() + timedelta(hours=JWT_EXPIRE_HOURS)
    payload = {
        "sub": username,
        "exp": expire
    }
    return jwt.encode(payload, JWT_SECRET, algorithm=JWT_ALGORITHM)


async def verify_token(authorization: Optional[str] = Header(None)) -> str:
    """验证 JWT Token (依赖注入)"""
    if not authorization:
        raise HTTPException(status_code=401, detail="缺少认证头")
    
    # 支持 "Bearer <token>" 格式
    token = authorization
    if authorization.startswith("Bearer "):
        token = authorization[7:]
    
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=[JWT_ALGORITHM])
        username = payload.get("sub")
        if not username:
            raise HTTPException(status_code=401, detail="无效的 Token")
        return username
    except jwt.ExpiredSignatureError:
        raise HTTPException(status_code=401, detail="Token 已过期")
    except jwt.InvalidTokenError:
        raise HTTPException(status_code=401, detail="无效的 Token")


@app.post("/api/admin/login")
async def admin_login(data: LoginRequest):
    """管理员登录"""
    try:
        if await verify_admin_password(data.username, data.password):
            token = create_jwt_token(data.username)
            return {
                "status": "success",
                "token": token,
                "expires_in": JWT_EXPIRE_HOURS * 3600
            }
        raise HTTPException(status_code=401, detail="用户名或密码错误")
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"登录失败: {e}")
        raise HTTPException(status_code=500, detail="登录服务异常")


@app.post("/api/admin/change-password")
async def admin_change_password(data: ChangePasswordRequest, username: str = Depends(verify_token)):
    """修改管理员密码"""
    # 验证旧密码
    if not await verify_admin_password(username, data.old_password):
        raise HTTPException(status_code=401, detail="旧密码错误")
    
    # 更新密码
    success = await update_admin_password(username, data.new_password)
    if success:
        return {"status": "success", "message": "密码已更新"}
    raise HTTPException(status_code=500, detail="密码更新失败")


@app.post("/api/admin/announcement")
async def create_announcement_api(data: AnnouncementRequest, username: str = Depends(verify_token)):
    """创建新公告"""
    try:
        result = await create_announcement(
            title=data.title,
            content=data.content,
            url=data.url
        )
        logger.info(f"📢 管理员 {username} 创建了新公告: {data.title}")
        return {"status": "success", "announcement": result}
    except Exception as e:
        logger.error(f"创建公告失败: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/api/admin/announcements")
async def list_announcements_api(username: str = Depends(verify_token)):
    """列出所有公告"""
    return await list_announcements()


@app.delete("/api/admin/announcement/{announcement_id}")
async def delete_announcement_api(announcement_id: int, username: str = Depends(verify_token)):
    """删除公告"""
    success = await delete_announcement(announcement_id)
    if success:
        logger.info(f"🗑️ 管理员 {username} 删除了公告 ID: {announcement_id}")
        return {"status": "success"}
    raise HTTPException(status_code=404, detail="公告不存在")


@app.patch("/api/admin/announcement/{announcement_id}/toggle")
async def toggle_announcement_api(announcement_id: int, is_active: bool, username: str = Depends(verify_token)):
    """切换公告激活状态"""
    success = await toggle_announcement_active(announcement_id, is_active)
    if success:
        status_text = "激活" if is_active else "停用"
        logger.info(f"🔄 管理员 {username} {status_text}了公告 ID: {announcement_id}")
        return {"status": "success", "is_active": is_active}
    raise HTTPException(status_code=404, detail="公告不存在")


# 挂载静态文件服务 (管理前端)
if FRONTEND_DIR.exists():
    app.mount("/admin", StaticFiles(directory=str(FRONTEND_DIR), html=True), name="frontend")


if __name__ == "__main__":
    logger.info(f"🚀 启动 Alas API 服务... 日志将保存至: {LOG_FILE}")
    # 设置 log_config=None 让 uvicorn 使用我们上面自定义配置好的日志系统
    uvicorn.run(app, host="0.0.0.0", port=8000, log_config=None)
