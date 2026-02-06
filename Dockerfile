# 使用官方 Python 3.11 运行时作为基础镜像
FROM python:3.11-slim

# 设置工作目录
WORKDIR /app

# 设置环境变量
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    TZ=Asia/Shanghai

# 安装系统依赖（如果需要）
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    tzdata \
    curl && \
    rm -rf /var/lib/apt/lists/*

# 复制依赖文件
COPY requirements_api.txt .

# 安装 Python 依赖
RUN pip install --no-cache-dir -r requirements_api.txt

# 复制应用代码
COPY announcement_api.py .
COPY telemetry_db.py .
COPY migrate_telemetry_db.py .
COPY reset_admin.py .
COPY frontend/ ./frontend/

# 创建必要的目录
RUN mkdir -p /app/bug_logs /app/data

# 暴露端口
EXPOSE 8000

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/health || exit 1

# 启动命令
CMD ["python", "announcement_api.py"]
