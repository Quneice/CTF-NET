"""执行引擎：统一 Executor 接口，支持本机子进程与 Docker 沙箱两种后端。"""
from __future__ import annotations

import asyncio
import time
from abc import ABC, abstractmethod
from pathlib import Path

from app.core.tool import Status, ToolResult


class Executor(ABC):
    """执行器抽象接口。新增沙箱后端（如 Kubernetes Pod、远程沙箱）只需实现 run()。"""

    @abstractmethod
    async def run(
        self,
        cmd: list[str],
        cwd: Path | None = None,
        timeout: int = 60,
        env: dict | None = None,
    ) -> ToolResult:
        ...

    async def _exec(
        self,
        cmd: list[str],
        cwd: Path | None = None,
        timeout: int = 60,
        env: dict | None = None,
    ) -> ToolResult:
        """共享的 asyncio 子进程执行 + 超时控制。"""
        start = time.perf_counter()
        try:
            proc = await asyncio.create_subprocess_exec(
                *cmd,
                cwd=str(cwd) if cwd else None,
                env=env,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
        except FileNotFoundError as exc:
            return ToolResult(
                status=Status.ERROR,
                exit_code=127,
                stderr=f"命令不存在: {exc}",
                duration=time.perf_counter() - start,
            )

        try:
            stdout_b, stderr_b = await asyncio.wait_for(
                proc.communicate(), timeout=timeout
            )
        except asyncio.TimeoutError:
            proc.kill()
            await proc.communicate()
            return ToolResult(
                status=Status.TIMEOUT,
                exit_code=-1,
                stderr=f"执行超时（>{timeout}s）: {' '.join(cmd)}",
                duration=time.perf_counter() - start,
            )

        status = Status.SUCCESS if proc.returncode == 0 else Status.ERROR
        return ToolResult(
            status=status,
            exit_code=proc.returncode,
            stdout=stdout_b.decode(errors="replace"),
            stderr=stderr_b.decode(errors="replace"),
            duration=time.perf_counter() - start,
        )


class SubprocessExecutor(Executor):
    """本机子进程执行：无隔离，最快，适合可信工具。"""

    async def run(
        self,
        cmd: list[str],
        cwd: Path | None = None,
        timeout: int = 60,
        env: dict | None = None,
    ) -> ToolResult:
        return await self._exec(cmd, cwd=cwd, timeout=timeout, env=env)


class DockerExecutor(Executor):
    """Docker 沙箱执行：隔离文件系统与网络，适合运行不可信二进制（pwn/reverse）。

    两种挂载模式：
    - bind 模式（默认）：cwd 挂载到容器 /work，适合本机运行；
    - volume 模式：传入 data_volume + data_root 后改用命名卷挂载，适合 App 运行在容器内（DooD），
      跨容器共享同一数据卷，规避宿主机路径不一致问题。

    - 默认 --network none 完全断网（web 类工具需配置 docker_network=bridge）；
    - 通过 extra_mounts 可追加额外卷（只读数据、字典等）。
    """

    def __init__(
        self,
        image: str,
        network: str = "none",
        extra_mounts: list[str] | None = None,
        data_volume: str | None = None,
        data_root: Path | None = None,
    ) -> None:
        self.image = image
        self.network = network
        self.extra_mounts = extra_mounts or []
        self.data_volume = data_volume
        self.data_root = data_root

    async def run(
        self,
        cmd: list[str],
        cwd: Path | None = None,
        timeout: int = 60,
        env: dict | None = None,
    ) -> ToolResult:
        if self.data_volume and self.data_root:
            rel = (
                Path(cwd).resolve().relative_to(self.data_root.resolve()).as_posix()
                if cwd else ""
            )
            if rel == ".":
                rel = ""
            container_cwd = f"/data/{rel}" if rel else "/data"
            docker_cmd = [
                "docker", "run", "--rm",
                "--network", self.network,
                "-v", f"{self.data_volume}:/data",
                "-w", container_cwd,
            ]
        else:
            host_src = str(Path(cwd).resolve() if cwd else Path.cwd().resolve())
            host_src = host_src.replace("\\", "/")
            docker_cmd = [
                "docker", "run", "--rm",
                "--network", self.network,
                "-v", f"{host_src}:/work",
                "-w", "/work",
            ]
        for mount in self.extra_mounts:
            docker_cmd += ["-v", mount]
        if env:
            for key, value in env.items():
                docker_cmd += ["-e", f"{key}={value}"]
        docker_cmd.append(self.image)
        docker_cmd.extend(cmd)

        return await self._exec(docker_cmd, timeout=timeout)
