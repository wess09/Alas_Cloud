# 🚀 快速修复 - 使用 Root 运行

## 修复内容

已简化 Dockerfile，**直接使用 root 用户运行**，避免权限问题。

- ✅ 移除了非 root 用户配置
- ✅ 使用命名 volume（自动管理权限）
- ✅ 更简单，更可靠

## 立即部署

```powershell
cd "c:\Users\Azur\Desktop\项目\Alas_Cloud"

# 停止旧容器
docker compose down

# 删除旧 volume（如果有权限问题的话）
docker volume rm alas_cloud_alas-data

# 重新构建
docker compose build --no-cache

# 启动
docker compose up -d

# 查看日志
docker compose logs -f
```

## 预期结果

应该看到：

```
✅ 服务启动完成 | 遥测数据库已连接 | 黑名单 IP: 0 个
INFO:     Uvicorn running on http://0.0.0.0:8000
```

## 访问管理后台

- URL: `http://localhost:8000/admin`
- 用户名: `admin`
- 密码: `admin123`

发布公告后，应该立即在列表中显示！

---

**注意**：使用 root 运行对于内部服务是安全的，但如果要公开部署，建议配置反向代理（Nginx）作为安全层。
