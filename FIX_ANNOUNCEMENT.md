# 🔧 修复公告系统问题

## 修改内容

1. **添加公告哈希值**：每个公告生成唯一的 MD5 哈希值，用于增量更新判断
2. **数据库模型更新**：添加 `announcement_hash` 字段
3. **API 返回值更新**：`announcementId` 现在返回哈希值而非数字ID

## 部署步骤

### 方法 1：删除旧数据库重新开始（推荐，如果数据不重要）

```powershell
cd "c:\Users\Azur\Desktop\项目\Alas_Cloud"

# 停止容器
docker compose down

# 删除旧数据（注意：会丢失所有数据！）
docker volume rm alas_cloud_alas-data

# 重新构建镜像
docker compose build --no-cache

# 启动
docker compose up -d

# 查看日志
docker compose logs -f
```

### 方法 2：保留数据并迁移

```powershell
cd "c:\Users\Azur\Desktop\项目\Alas_Cloud"

# 1. 停止容器
docker compose down

# 2. 重新构建镜像
docker compose build --no-cache

# 3. 启动容器
docker compose up -d

# 4. 运行迁移脚本
docker exec alas-api python migrate_announcement_hash.py

# 5. 查看日志
docker compose logs -f
```

## 验证

1. 访问管理后台：`http://localhost:8000/admin`
2. 登录（admin / admin123）
3. 发布测试公告
4. 确认公告列表显示正常
5. 测试公告 API：`curl http://localhost:8000/api/get/announcement`

## API 变化

### `/api/get/announcement` 响应格式

```json
{
  "announcementId": "a1b2c3d4e5f6...", // 32位MD5哈希
  "title": "公告标题",
  "content": "公告内容",
  "url": ""
}
```

客户端可以缓存 `announcementId`，下次请求时带上 `?id=<hash>` 参数，如果公告未变化则返回空对象。

## 需要更新 Dockerfile

确保新文件被复制到容器中：

```dockerfile
COPY migrate_announcement_hash.py .
```

---

执行完毕后访问管理后台测试！
