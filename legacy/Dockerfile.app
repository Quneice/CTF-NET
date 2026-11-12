# FastAPI 应用镜像：容器内运行工具箱服务，通过挂载 docker.sock 调用宿主 Docker 启动沙箱容器（DooD）。
FROM python:3.12-slim-bookworm

ENV PIP_NO_CACHE_DIR=1 \
    PYTHONUNBUFFERED=1 \
    DEBIAN_FRONTEND=noninteractive

# docker CLI：DockerExecutor 依赖 `docker run` 操作宿主守护进程（靠挂载 /var/run/docker.sock）
RUN apt-get update && apt-get install -y --no-install-recommends \
        docker.io \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 依赖层（利用构建缓存）
RUN pip install --no-cache-dir \
        "fastapi>=0.110" \
        "uvicorn[standard]>=0.29" \
        "pydantic>=2.6" \
        "pydantic-settings>=2.2" \
        "python-multipart>=0.0.9"

# 源码
COPY app ./app
COPY tools ./tools

EXPOSE 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
