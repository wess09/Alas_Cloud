# 🔧 公告列表不显示问题 - 已修复

## 问题原因

**数据库路径配置错误**：`telemetry_db.py` 中的数据库路径写死为 `./telemetry.db`，没有使用 `DATA_DIR` 环境变量。

这导致：

- 发布公告时，数据被写入 `/app/telemetry.db`（容器根目录）
- 但该文件没有被持久化挂载
- 容器重启后数据丢失

## 修复内容

已修改 `telemetry_db.py` 第 1-19 行：

```python
# 数据库配置 - 支持通过环境变量自定义数据目录
DATA_DIR = Path(os.getenv("DATA_DIR", "."))
DB_PATH = DATA_DIR / "telemetry.db"
DATABASE_URL = f"sqlite+aiosqlite:///{DB_PATH}"
```

现在数据库文件会正确保存到 `/app/data/telemetry.db`（已通过 volume 挂载）。

## 重新部署步骤

### 1️⃣ 停止旧容器

```powershell
cd "c:\Users\Azur\Desktop\项目\Alas_Cloud"
docker compose down
```

### 2️⃣ 重新构建镜像

```powershell
# 重新构建镜像（包含修复后的代码）
docker compose build --no-cache

# 或者如果不想使用缓存
docker compose build --no-cache
```

### 3️⃣ 启动容器

```powershell
docker compose up -d

# 查看日志确认启动成功
docker compose logs -f
```

### 4️⃣ 验证修复

1. **登录管理后台**:
   - 访问 `http://localhost:8000/admin`
   - 使用默认账号登录（用户名：`admin`，密码：`admin123`）

2. **发布测试公告**:
   - 填写标题和内容
   - 点击"发布公告"

3. **检查公告列表**:
   - 应该立即在"公告历史"部分看到刚发布的公告
   - 如果仍然不显示，查看浏览器控制台 (F12) 的错误信息

4. **验证数据持久化**:

   ```powershell
   # 检查数据文件是否存在
   ls data/telemetry.db

   # 重启容器后数据应该保留
   docker compose restart
   # 刷新管理后台，公告应该还在
   ```

## 预期结果

✅ 发布公告后，公告列表立即显示新公告  
✅ 容器重启后，公告数据不会丢失  
✅ 数据文件保存在 `data/telemetry.db`

## 如果仍有问题

查看容器日志：

```powershell
docker compose logs alas-api --tail 100
```

查看浏览器控制台 (F12)：

- Console 标签：查看 JavaScript 错误
- Network 标签：查看 API 请求和响应

---

**准备好了吗？** 按照上述步骤重新部署即可！ 🚀
