# 🔧 Docker Volume 挂载问题修复

## 问题说明

之前的配置使用文件挂载方式（如 `./telemetry.db:/app/telemetry.db`），当宿主机文件不存在时，Docker 会自动创建**目录**而不是文件，导致应用启动失败并报错：

```
IsADirectoryError: [Errno 21] Is a directory: '/app/api.log'
```

## 解决方案

✅ **已修改为目录挂载方式**，所有数据文件统一存放在 `data/` 目录中。

### 修改的文件

1. **[docker-compose.yml](file:///c:/Users/Azur/Desktop/项目/Alas_Cloud/docker-compose.yml)**
   - 改为挂载 `./data:/app/data` 目录
   - 添加 `DATA_DIR=/app/data` 环境变量

2. **[announcement_api.py](file:///c:/Users/Azur/Desktop/项目/Alas_Cloud/announcement_api.py)**
   - 支持通过 `DATA_DIR` 环境变量自定义数据目录
   - 默认值为当前目录（本地运行）或 `/app/data`（容器运行）

3. **[README_DOCKER.md](file:///c:/Users/Azur/Desktop/项目/Alas_Cloud/README_DOCKER.md)**
   - 更新使用说明和数据持久化说明

## 立即操作步骤

### 1️⃣ 停止并删除旧容器

```powershell
cd "c:\Users\Azur\Desktop\项目\Alas_Cloud"

# 停止和删除旧容器
docker compose down

# 如果存在旧的挂载目录问题，清理它们
Remove-Item .jwt_secret -Force -Recurse -ErrorAction SilentlyContinue
Remove-Item api.log -Force -Recurse -ErrorAction SilentlyContinue
Remove-Item blacklist.txt -Force -Recurse -ErrorAction SilentlyContinue
```

### 2️⃣ 创建数据目录

```powershell
# 创建数据目录
New-Item -ItemType Directory -Path data -Force

# 如果有旧的数据文件，迁移到 data 目录
if (Test-Path telemetry.db -PathType Leaf) {
    Move-Item telemetry.db data\telemetry.db -Force
}
```

### 3️⃣ 重新构建并启动容器

```powershell
# 重新构建镜像（包含最新代码）
docker compose build

# 启动服务
docker compose up -d

# 查看日志确认启动成功
docker compose logs -f
```

### 4️⃣ 验证服务

```powershell
# 测试健康检查
curl http://localhost:8000/health

# 测试 API
curl http://localhost:8000/api/get/announcement

# 访问管理后台
# 浏览器打开: http://localhost:8000/admin
```

## 数据文件位置

现在所有数据文件都在 `data/` 目录下：

```
Alas_Cloud/
├── data/                    # 🔹 数据目录（挂载到容器）
│   ├── telemetry.db        # SQLite 数据库
│   ├── api.log             # API 日志
│   ├── blacklist.txt       # IP 黑名单
│   └── bug_logs/           # Bug 报告
├── docker-compose.yml
├── Dockerfile
└── announcement_api.py
```

## 预期输出

正常启动后应该看到：

```
✅ 服务启动完成 | 遥测数据库已连接 | 黑名单 IP: 0 个
INFO:     Started server process [1]
INFO:     Waiting for application startup.
INFO:     Application startup complete.
INFO:     Uvicorn running on http://0.0.0.0:8000 (Press CTRL+C to quit)
```

## 常见问题

### Q: 容器一直重启怎么办？

查看日志找出错误原因：

```powershell
docker compose logs alas-api
```

### Q: 旧数据会丢失吗？

不会，只要你在步骤 2 中迁移了 `telemetry.db` 文件。

### Q: JWT secret 会重新生成吗？

是的，每次容器重启都会重新生成（存储在容器内），这意味着需要重新登录管理后台。这是安全的设计。

---

**准备好了吗？** 按照上述步骤操作即可！ 🚀
