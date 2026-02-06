from sqlalchemy import Column, String, Integer, Float, DateTime, Boolean, UniqueConstraint, delete, desc
from sqlalchemy.orm import declarative_base
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession
from sqlalchemy.orm import sessionmaker
from datetime import datetime, timedelta
from typing import Dict, Any, Optional, List
from pathlib import Path
import asyncio
import logging
import secrets
import hashlib
import os

# 日志配置
logger = logging.getLogger("TelemetryDB")

# 数据库配置 - 支持通过环境变量自定义数据目录
DATA_DIR = Path(os.getenv("DATA_DIR", "."))
DB_PATH = DATA_DIR / "telemetry.db"
DATABASE_URL = f"sqlite+aiosqlite:///{DB_PATH}"

# 创建异步引擎
engine = create_async_engine(DATABASE_URL, echo=False)

# 创建异步会话工厂
async_session = sessionmaker(
    engine, class_=AsyncSession, expire_on_commit=False
)

Base = declarative_base()

class TelemetryData(Base):
    """遥测数据模型"""
    __tablename__ = "telemetry_data"

    id = Column(Integer, primary_key=True, index=True, autoincrement=True)
    device_id = Column(String, index=True, nullable=False)
    instance_id = Column(String, index=True, nullable=False)
    # IP 地址字段
    ip_address = Column(String, index=True, nullable=True)
    month = Column(String, index=True, nullable=False)
    battle_count = Column(Integer, nullable=False)
    battle_rounds = Column(Integer, nullable=False)
    sortie_cost = Column(Integer, nullable=False)
    akashi_encounters = Column(Integer, nullable=False)
    akashi_probability = Column(Float, nullable=False)
    average_stamina = Column(Float, nullable=False)
    net_stamina_gain = Column(Integer, nullable=False)
    
    # 自动管理的时间字段
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    
    __table_args__ = (
        UniqueConstraint('device_id', 'instance_id', name='uix_device_instance'),
    )


class Announcement(Base):
    """公告数据模型"""
    __tablename__ = "announcements"

    id = Column(Integer, primary_key=True, index=True, autoincrement=True)
    title = Column(String, nullable=False)
    content = Column(String, nullable=True)  # 可为空（当 url 存在时）
    url = Column(String, nullable=True)      # 可选的外链
    created_at = Column(DateTime, default=datetime.utcnow)
    is_active = Column(Boolean, default=True)


class AdminUser(Base):
    """管理员账户模型"""
    __tablename__ = "admin_users"

    id = Column(Integer, primary_key=True, index=True, autoincrement=True)
    username = Column(String, unique=True, nullable=False, index=True)
    password_hash = Column(String, nullable=False)  # SHA256 hash
    created_at = Column(DateTime, default=datetime.utcnow)


async def init_db():
    """初始化数据库"""
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)


async def insert_or_update_telemetry(data: Dict[str, Any]) -> bool:
    """插入或更新遥测数据"""
    async with async_session() as session:
        try:
            from sqlalchemy import select
            stmt = select(TelemetryData).where(
                TelemetryData.device_id == data["device_id"],
                TelemetryData.instance_id == data["instance_id"]
            )
            result = await session.execute(stmt)
            existing = result.scalar_one_or_none()

            if existing:
                existing.month = data["month"]
                existing.battle_count = data["battle_count"]
                existing.battle_rounds = data["battle_rounds"]
                existing.sortie_cost = data["sortie_cost"]
                existing.akashi_encounters = data["akashi_encounters"]
                existing.akashi_probability = data["akashi_probability"]
                existing.average_stamina = data["average_stamina"]
                existing.net_stamina_gain = data["net_stamina_gain"]
                if "ip_address" in data:
                    existing.ip_address = data["ip_address"]
                # 手动更新 updated_at 以确保时间戳最新
                existing.updated_at = datetime.utcnow()
            else:
                new_record = TelemetryData(**data)
                session.add(new_record)

            await session.commit()
            return True
        except Exception as e:
            await session.rollback()
            raise e


async def get_aggregate_stats() -> Dict[str, Any]:
    """获取聚合统计数据"""
    async with async_session() as session:
        from sqlalchemy import select, func
        
        stmt = select(
            func.count(TelemetryData.id).label("total_devices"),
            func.sum(TelemetryData.battle_count).label("total_battle_count"),
            func.sum(TelemetryData.battle_rounds).label("total_battle_rounds"),
            func.sum(TelemetryData.sortie_cost).label("total_sortie_cost"),
            func.sum(TelemetryData.akashi_encounters).label("total_akashi_encounters"),
            func.avg(TelemetryData.average_stamina).label("avg_stamina"),
            func.sum(TelemetryData.net_stamina_gain).label("total_net_stamina_gain")
        )
        
        result = await session.execute(stmt)
        row = result.first()
        
        if row and row.total_devices:
            avg_akashi_probability = 0
            if row.total_battle_rounds and row.total_battle_rounds > 0:
                avg_akashi_probability = (row.total_akashi_encounters or 0) / row.total_battle_rounds
            
            return {
                "total_devices": row.total_devices or 0,
                "total_battle_count": row.total_battle_count or 0,
                "total_battle_rounds": row.total_battle_rounds or 0,
                "total_sortie_cost": row.total_sortie_cost or 0,
                "total_akashi_encounters": row.total_akashi_encounters or 0,
                "avg_akashi_probability": round(avg_akashi_probability, 4),
                "avg_stamina": round(row.avg_stamina, 2) if row.avg_stamina else 0,
                "total_net_stamina_gain": row.total_net_stamina_gain or 0
            }
        else:
            return {
                "total_devices": 0,
                "total_battle_count": 0,
                "total_battle_rounds": 0,
                "total_sortie_cost": 0,
                "total_akashi_encounters": 0,
                "avg_akashi_probability": 0,
                "avg_stamina": 0,
                "total_net_stamina_gain": 0
            }

async def delete_inactive_instances(hours_limit: int = 24) -> int:
    """
    删除超过指定小时数未更新的实例数据
    :param hours_limit: 默认为24小时
    :return: 删除的行数
    """
    async with async_session() as session:
        try:
            # 计算截止时间 (当前 UTC 时间 - 小时数)
            cutoff_time = datetime.utcnow() - timedelta(hours=hours_limit)
            
            # 构建删除语句
            stmt = delete(TelemetryData).where(TelemetryData.updated_at < cutoff_time)
            
            # 执行删除
            result = await session.execute(stmt)
            deleted_count = result.rowcount
            
            await session.commit()
            return deleted_count
        except Exception as e:
            await session.rollback()
            # 记录错误但不一定抛出，以免打断主进程，或者根据需要抛出
            logger.error(f"Cleanup error: {e}") 
            return 0


# ===================== 公告管理函数 =====================

async def get_latest_announcement() -> Optional[Dict[str, Any]]:
    """获取最新的激活公告"""
    async with async_session() as session:
        from sqlalchemy import select
        stmt = select(Announcement).where(
            Announcement.is_active == True
        ).order_by(desc(Announcement.id)).limit(1)
        
        result = await session.execute(stmt)
        announcement = result.scalar_one_or_none()
        
        if announcement:
            return {
                "announcementId": announcement.id,
                "title": announcement.title,
                "content": announcement.content or "",
                "url": announcement.url or ""
            }
        return None


async def create_announcement(title: str, content: str = "", url: str = "") -> Dict[str, Any]:
    """创建新公告"""
    async with async_session() as session:
        try:
            new_announcement = Announcement(
                title=title,
                content=content if content else None,
                url=url if url else None,
                is_active=True
            )
            session.add(new_announcement)
            await session.commit()
            await session.refresh(new_announcement)
            
            return {
                "id": new_announcement.id,
                "title": new_announcement.title,
                "content": new_announcement.content or "",
                "url": new_announcement.url or "",
                "created_at": new_announcement.created_at.isoformat()
            }
        except Exception as e:
            await session.rollback()
            raise e


async def list_announcements(limit: int = 20) -> List[Dict[str, Any]]:
    """列出所有公告（按 ID 降序）"""
    async with async_session() as session:
        from sqlalchemy import select
        stmt = select(Announcement).order_by(desc(Announcement.id)).limit(limit)
        
        result = await session.execute(stmt)
        announcements = result.scalars().all()
        
        return [
            {
                "id": a.id,
                "title": a.title,
                "content": a.content or "",
                "url": a.url or "",
                "is_active": a.is_active,
                "created_at": a.created_at.isoformat()
            }
            for a in announcements
        ]


async def delete_announcement(announcement_id: int) -> bool:
    """删除公告"""
    async with async_session() as session:
        try:
            from sqlalchemy import select
            stmt = select(Announcement).where(Announcement.id == announcement_id)
            result = await session.execute(stmt)
            announcement = result.scalar_one_or_none()
            
            if announcement:
                await session.delete(announcement)
                await session.commit()
                return True
            return False
        except Exception as e:
            await session.rollback()
            raise e


async def toggle_announcement_active(announcement_id: int, is_active: bool) -> bool:
    """切换公告激活状态"""
    async with async_session() as session:
        try:
            from sqlalchemy import select
            stmt = select(Announcement).where(Announcement.id == announcement_id)
            result = await session.execute(stmt)
            announcement = result.scalar_one_or_none()
            
            if announcement:
                announcement.is_active = is_active
                await session.commit()
                return True
            return False
        except Exception as e:
            await session.rollback()
            raise e


# ===================== 管理员账户函数 =====================

def hash_password(password: str) -> str:
    """使用 SHA256 哈希密码"""
    return hashlib.sha256(password.encode()).hexdigest()


async def get_admin_user(username: str) -> Optional[Dict[str, Any]]:
    """获取管理员账户"""
    async with async_session() as session:
        from sqlalchemy import select
        stmt = select(AdminUser).where(AdminUser.username == username)
        result = await session.execute(stmt)
        user = result.scalar_one_or_none()
        
        if user:
            return {
                "id": user.id,
                "username": user.username,
                "password_hash": user.password_hash
            }
        return None


async def create_admin_user(username: str, password: str) -> Dict[str, Any]:
    """创建管理员账户"""
    async with async_session() as session:
        try:
            new_user = AdminUser(
                username=username,
                password_hash=hash_password(password)
            )
            session.add(new_user)
            await session.commit()
            await session.refresh(new_user)
            
            return {
                "id": new_user.id,
                "username": new_user.username
            }
        except Exception as e:
            await session.rollback()
            raise e


async def verify_admin_password(username: str, password: str) -> bool:
    """验证管理员密码"""
    user = await get_admin_user(username)
    if not user:
        return False
    return user["password_hash"] == hash_password(password)


async def update_admin_password(username: str, new_password: str) -> bool:
    """更新管理员密码"""
    async with async_session() as session:
        try:
            from sqlalchemy import select
            stmt = select(AdminUser).where(AdminUser.username == username)
            result = await session.execute(stmt)
            user = result.scalar_one_or_none()
            
            if user:
                user.password_hash = hash_password(new_password)
                await session.commit()
                return True
            return False
        except Exception as e:
            await session.rollback()
            raise e


async def ensure_default_admin():
    """确保存在默认管理员账户，如果不存在则创建"""
    user = await get_admin_user("admin")
    if not user:
        # 默认密码设置为 admin123
        default_password = "admin123"
        await create_admin_user("admin", default_password)
        logger.warning(f"🔐 已创建默认管理员账户 | 用户名: admin | 密码: {default_password}")
        logger.warning("⚠️ 请立即登录并修改默认密码!")
        return default_password
    return None
