# Alas Cloud API Documentation

本文档详细说明了 Alas Cloud 后端提供的 API 接口。

## 基础信息
- **Base URL**: `http://<server-ip>:8000`
- **默认端口**: `8000`

---

## 认证 (Authentication)

部分管理接口需要 JWT 认证。请在 HTTP Header 中携带：
`Authorization: Bearer <your-token>`

### 管理员登录
- **Endpoint**: `POST /api/admin/login`
- **Request Body**:
  ```json
  {
    "username": "admin",
    "password": "your-password"
  }
  ```
- **Response**: 返回 `token` 和有效期。

---

## 公开 API (Public APIs)

这些接口不需要身份验证。

### 1. 遥测数据上报 (Telemetry)
- **Endpoint**: `POST /api/telemetry`
- **Description**: 收集用户的战斗统计、体力获取等数据。
- **Request Body**: `TelemetryRequest` 模型，包含 `device_id`, `battle_count`, `akashi_encounters` 等。

### 2. 体力大盘 (Stamina)
- **上报数据**: `POST /api/stamina/report`
- **查询 K 线**: `GET /api/stamina/kline?period=1m&range=day`
- **当前汇总**: `GET /api/stamina/latest`
- **实时推送 (SSE)**: `GET /api/stamina/stream`

### 3. AzurStat 统计 (AzurStat)
- **上报掉落**: `POST /api/azurstat` (特定任务如 `opsi_hazard1_leveling`)
- **获取统计**: `GET /api/azurstat/stats`
- **掉落详情**: `GET /api/azurstat/items`
- **历史记录**: `GET /api/azurstat/history`

### 4. 排行榜 (Leaderboard)
- **查看详情**: `GET /api/leaderboard?page=1&size=50&sort=rounds`
- **更新昵称**: `POST /api/user/profile` (需提供 `device_id`)

### 5. 系统与公告
- **获取公告**: `GET /api/get/announcement`
- **更新检查**: `GET /api/updata` (返回自动更新开关状态)
- **Bug 报告**: `POST /api/post/bug`

---

## 管理 API (Admin APIs)

**需 Bearer Token 认证。路径前缀: `/api/admin`**

### 1. 公告管理
- **列表**: `GET /api/admin/announcements`
- **创建**: `POST /api/admin/announcement`
- **删除**: `DELETE /api/admin/announcement/:id`
- **开关**: `PATCH /api/admin/announcement/:id/toggle?is_active=true`

### 2. 用户管理 (封禁与举报)
- **手动封禁**: `POST /api/admin/ban`
- **解封**: `POST /api/admin/unban`
- **驳回举报**: `POST /api/admin/dismiss` (清空针对某设备的所有举报)

### 3. 系统配置
- **获取更新状态**: `GET /api/admin/config/auto_update`
- **修改更新状态**: `PATCH /api/admin/config/auto_update?is_active=true`

---

## 实时数据推送 (SSE)

后端支持 Server-Sent Events 以提供低延迟的数据流。

- **遥测全局统计**: `GET /api/telemetry/stats/stream`
- **体力大盘实时数据**: `GET /api/stamina/stream`

---

## 错误代码
- `200 OK`: 请求成功。
- `400 Bad Request`: 参数校验失败。
- `401 Unauthorized`: 认证失效或未认证。
- `403 Forbidden`: 用户被封禁。
- `404 Not Found`: 资源不存在。
- `500 Internal Server Error`: 服务器内部错误。
