# Alas Cloud Docker 部署指南

本文档介绍如何使用 Docker 部署 Alas Cloud API 服务。

## 快速开始

### 使用 Docker Hub 镜像（推荐）

```bash
# 拉取最新镜像
docker pull <username>/alas-cloud:latest

# 创建数据目录
mkdir -p data

# 运行容器
docker run -d \
  --name alas-api \
  -p 8000:8000 \
  -v $(pwd)/data:/app/data \
  -e TZ=Asia/Shanghai \
  -e DATA_DIR=/app/data \
  --restart unless-stopped \
  <username>/alas-cloud:latest
```

### 使用 docker-compose（推荐）

```bash
# 克隆仓库
git clone <your-repo-url>
cd Alas_Cloud

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

### 本地构建

```bash
# 克隆仓库
git clone <your-repo-url>
cd Alas_Cloud

# 构建镜像
docker build -t alas-cloud:local .

# 运行容器
docker-compose up -d
```

## 环境变量

| 变量名     | 说明             | 默认值          |
| ---------- | ---------------- | --------------- |
| `TZ`       | 时区设置         | `Asia/Shanghai` |
| `DATA_DIR` | 数据文件存储目录 | `/app/data`     |

## 数据持久化

所有数据文件存储在 `data/` 目录中，通过 volume 挂载持久化：

- **data/telemetry.db**: SQLite 数据库文件（存储遥测数据、公告、管理员账户等）
- **data/api.log**: API 访问和错误日志（自动轮转，最多 5 个 10MB 文件）
- **data/blacklist.txt**: IP 黑名单文件
- **data/bug_logs/**: Bug 报告目录

> **⚠️ 注意**:
>
> - 删除容器不会丢失数据，但删除 `data/` 目录会导致数据丢失
> - `.jwt_secret` 文件在容器内生成，容器重启会重新生成（需重新登录管理后台）

## 端口说明

- **8000**: API 服务端口

主要端点：

- `http://localhost:8000/` - 服务状态
- `http://localhost:8000/health` - 健康检查
- `http://localhost:8000/admin` - 管理后台
- `http://localhost:8000/api/get/announcement` - 获取公告
- `http://localhost:8000/api/telemetry` - 提交遥测数据

## GitHub Actions 自动构建

### 配置步骤

1. **在 Docker Hub 创建 Access Token**
   - 登录 Docker Hub
   - 进入 Account Settings → Security
   - 点击 "New Access Token"
   - 保存生成的 token

2. **在 GitHub 仓库添加 Secrets**
   - 进入仓库 Settings → Secrets and variables → Actions
   - 添加以下 secrets：
     - `DOCKERHUB_USERNAME`: 你的 Docker Hub 用户名
     - `DOCKERHUB_TOKEN`: 上一步生成的 Access Token

3. **推送代码触发构建**
   ```bash
   git add .
   git commit -m "Add Docker support"
   git push origin main
   ```

### 触发条件

- 推送到 `main` 或 `master` 分支
- 创建版本标签（如 `v1.0.0`）
- 手动触发（在 Actions 页面）

### 镜像标签

- `latest`: 最新的主分支构建
- `<version>`: Git tag 版本（如 `1.0.0`）
- `<branch>-<sha>`: 分支和 commit SHA

## 常见问题

### 1. 容器无法启动

检查日志：

```bash
docker-compose logs alas-api
```

常见原因：

- 端口 8000 被占用
- 数据库文件损坏
- 权限问题

### 2. 数据库初始化失败

首次启动时，数据库会自动初始化。如果失败，删除 `telemetry.db` 重新启动：

```bash
rm telemetry.db
docker-compose restart
```

### 3. 管理员密码忘记

使用 `reset_admin.py` 脚本重置：

```bash
docker-compose exec alas-api python reset_admin.py
```

或在宿主机上运行：

```bash
python reset_admin.py
```

### 4. 更新镜像

```bash
# 拉取最新镜像
docker-compose pull

# 重启服务
docker-compose up -d
```

### 5. 查看容器状态

```bash
# 查看运行状态
docker-compose ps

# 查看资源使用
docker stats alas-api

# 查看健康检查状态
docker inspect alas-api | grep Health -A 10
```

## 安全建议

1. **修改默认密码**: 首次登录管理后台后，立即修改默认密码（`admin123`）
2. **使用反向代理**: 生产环境建议使用 Nginx 或 Traefik 作为反向代理
3. **启用 HTTPS**: 配置 SSL/TLS 证书（可使用 Let's Encrypt）
4. **限制访问**: 使用防火墙规则限制不必要的访问
5. **定期备份**: 定期备份数据库文件和配置

## 故障排除

### 健康检查失败

```bash
# 手动测试健康检查
curl http://localhost:8000/health

# 进入容器调试
docker-compose exec alas-api /bin/bash
```

### 日志轮转

默认使用 Python 的 RotatingFileHandler，自动轮转日志（最多 5 个 10MB 文件）。

### 性能优化

对于高并发环境，可以考虑：

- 使用 Gunicorn 替代 uvicorn
- 配置多个 worker 进程
- 使用 PostgreSQL 替代 SQLite

## 许可证

请参考项目根目录的 LICENSE 文件。

## 支持

如有问题，请在 GitHub 仓库提交 Issue。
