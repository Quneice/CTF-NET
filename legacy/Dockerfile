# CTF 工具箱专用沙箱镜像：内置各类赛题解题工具，供 DockerExecutor 调用。
# 运行方式：docker run --rm --network none -v <工作目录>:/work -w /work ctf-toolbox <命令>
FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    PIP_NO_CACHE_DIR=1 \
    PYTHONUNBUFFERED=1

# ---- 系统与构建依赖 ----
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        wget \
        git \
        passwd \
        locales \
        build-essential \
        libssl-dev \
        libffi-dev \
        zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

# ---- 分类解题工具 ----
RUN apt-get update && apt-get install -y --no-install-recommends \
        # 逆向 / 二进制（reverse / pwn）
        binutils \
        gdb \
        gdb-multiarch \
        strace \
        ltrace \
        patchelf \
        qemu-user \
        file \
        # 密码学（crypto）
        openssl \
        libgmp-dev \
        libmpfr-dev \
        libmpc-dev \
        # 网络 / web
        netcat-openbsd \
        nmap \
        dnsutils \
        iputils-ping \
        socat \
        # 取证 / 杂项（forensics / misc）
        xxd \
        binwalk \
        foremost \
        steghide \
        libimage-exiftool-perl \
        imagemagick \
        ffmpeg \
        p7zip-full \
        unzip \
        zstd \
        lz4 \
        jq \
    && rm -rf /var/lib/apt/lists/*

# ---- Python 解题库 ----
COPY requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt \
    && rm -f /tmp/requirements.txt

# ---- 非 root 运行（安全加固）----
RUN useradd --create-home --shell /bin/bash ctf \
    && mkdir -p /work \
    && chown ctf:ctf /work

WORKDIR /work
USER ctf
