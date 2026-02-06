# 公告发布问题诊断

## 需要的信息

为了诊断"无法发布公告"的问题，请提供以下信息：

### 1. 具体错误信息

请打开浏览器的开发者工具（按 F12），查看：

**控制台 (Console)** 标签页：

- 是否有红色错误信息？
- 截图或复制错误文本

**网络 (Network)** 标签页：

- 点击"发布公告"按钮后，找到 `/api/admin/announcement` 请求
- 查看该请求的状态码和响应内容
- 截图或复制响应信息

### 2. 操作步骤

请详细描述您的操作：

1. 是否成功登录管理后台？
2. 填写了哪些字段（标题、内容、URL）？
3. 点击"发布公告"后发生了什么？
4. 是否看到任何提示消息（成功/失败）？

### 3. 可能的问题

根据代码检查，可能的问题包括：

#### 问题 A: 未登录或 Token 过期

- **症状**: 点击按钮后无反应，或提示"未授权"
- **解决**: 刷新页面重新登录

#### 问题 B: 数据库文件不存在或无权限

- **症状**: 服务器返回 500 错误
- **查看**: Docker 容器日志中的错误信息
  ```powershell
  docker logs alas-api
  ```

#### 问题 C: 网络问题

- **症状**: 请求超时或无法连接
- **检查**: 容器是否正常运行，端口映射是否正确

#### 问题 D: 字段验证失败

- **症状**: 提示"请求失败"或字段错误
- **原因**: 标题为空或包含特殊字符

### 4. 快速测试

请尝试以下测试以缩小问题范围:

```powershell
# 测试 1: 检查容器日志
docker logs alas-api --tail 50

# 测试 2: 手动测试 API (在 PowerShell 中)
$headers = @{
    "Authorization" = "Bearer YOUR_TOKEN_HERE"
    "Content-Type" = "application/json"
}
$body = @{
    "title" = "测试公告"
    "content" = "这是一个测试"
    "url" = ""
} | ConvertTo-Json

Invoke-WebRequest -Uri "http://localhost:8000/api/admin/announcement" -Method POST -Headers $headers -Body $body
```

## 下一步

请提供上述信息（特别是浏览器控制台和容器日志），我将帮您快速定位和解决问题。
