# Alas Cloud Backend

Alas Cloud Backend 是为 [AzurLaneAutoScript (Alas)](https://github.com/LCHRC/AzurLaneAutoScript) 开发的云端配套后端服务，使用 Go 语言编写，基于 Gin 框架，提供遥测数据收集、体力大盘、统计信息、公告管理等功能。

## 技术栈

- **语言**: Go (1.21+)
- **框架**: [Gin Web Framework](https://github.com/gin-gonic/gin)
- **数据库**: SQLite (通过 [GORM](https://gorm.io/))
- **认证**: JWT (jsonwebtoken)
- **实时通信**: Server-Sent Events (SSE)

## 目录结构

- `cmd/server/`: 项目入口，包含 `main.go`。
- `internal/handlers/`: API 业务逻辑处理。
- `internal/models/`: 数据库模型定义及请求/响应结构。
- `internal/database/`: 数据库连接与初始化。
- `internal/middleware/`: 中间件（如 JWT 鉴权）。
- `internal/tasks/`: 后台定时任务（数据清理、统计聚合等）。
- `internal/utils/`: 工具函数（JWT、哈希等）。
- `dashboard/`: 前端展示页面（Vue/React 等）。

## 快速开始

### 运行环境
确保本地已安装 Go 环境。或者使用 Docker 部署。

### 本地启动
```bash
cd go_backend
go mod tidy
go run cmd/server/main.go
```
服务器默认启动在 `http://localhost:8000`。

### Docker 部署
使用根目录下的 `docker-compose.yml`:
```bash
docker-compose up -d
```

## 默认账户
- **管理员**: `admin`
- **默认密码**: `admin123` (建议首次登录后立即修改)

## 详细文档
- [API 接口文档](./API.md)
