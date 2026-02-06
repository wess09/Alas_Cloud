# 🔧 权限错误修复指南

## 问题原因

**Docker Volume 权限问题**：

- 挂载的 `./data` 目录在宿主机上可能不存在或权限不正确
- Docker 会以 root 创建该目录
- 容器内的 `appuser` (UID 1000) 无法写入

## 立即修复步骤

### 方案 1: 修改宿主机目录权限（推荐）

```powershell
cd "c:\Users\Azur\Desktop\项目\Alas_Cloud"

# 停止容器
docker compose down

# 在 Windows 上，确保 data 目录存在
if (!(Test-Path data)) { New-Item -ItemType Directory -Path data }

# 重新构建镜像
docker compose build

# 启动容器
docker compose up -d

# 查看日志
docker compose logs -f
```

**注意**：Windows 上的 Docker Desktop 通常会自动处理权限映射。

### 方案 2: 以 Root 用户运行（快速但不推荐）

修改 `docker-compose.yml`，添加 `user: root`:

```yaml
services:
  alas-api:
    build: .
    image: hajiming/alas-cloud:latest
    container_name: alas-api
    restart: unless-stopped
    user: "0:0"  # 添加这行，以 root 运行
    ports:
      - "8000:8000"
    ...
```

然后重启：

```powershell
docker compose down
docker compose up -d
```

### 方案 3: 使用命名 Volume（最佳实践）

修改 `docker-compose.yml`:

```yaml
services:
  alas-api:
    build: .
    image: hajiming/alas-cloud:latest
    container_name: alas-api
    restart: unless-stopped
    ports:
      - "8000:8000"
    volumes:
      # 使用命名 volume 而不是绑定挂载
      - alas-data:/app/data
    environment:
      - TZ=Asia/Shanghai
      - DATA_DIR=/app/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 5s

# 定义命名 volume
volumes:
  alas-data:
    driver: local
```

优点：

- Docker 自动管理权限
- 数据持久化更可靠
- 跨平台兼容性更好

缺点：

- 数据不直接在项目目录中，需要用 `docker volume inspect alas-data` 查看位置

## 验证修复

启动后检查：

```powershell
# 查看容器日志
docker compose logs alas-api

# 应该看到成功启动的消息：
# ✅ 服务启动完成 | 遥测数据库已连接 | 黑名单 IP: 0 个
```

## 检查数据文件

```powershell
# 如果使用绑定挂载（方案 1）
ls data/

# 如果使用命名 volume（方案 3）
docker exec alas-api ls -la /app/data/
```

## 常见问题

### Q: Windows 上仍然有权限问题怎么办？

使用方案 3（命名 volume）或方案 2（root 用户）。

### Q: 如何访问命名 volume 中的数据？

```powershell
# 查看 volume 信息
docker volume inspect alas-data

# 进入容器查看文件
docker exec -it alas-api ls -la /app/data/

# 复制数据库文件到宿主机
docker cp alas-api:/app/data/telemetry.db ./telemetry_backup.db
```

### Q: 如何备份数据？

```powershell
# 如果使用绑定挂载
Copy-Item -Recurse data data_backup_$(Get-Date -Format "yyyyMMdd_HHmmss")

# 如果使用命名 volume
docker run --rm -v alas-data:/data -v ${PWD}:/backup alpine tar czf /backup/alas-data-backup.tar.gz -C /data .
```

---

**推荐使用方案 3（命名 volume）** - 最简单且兼容性最好！
