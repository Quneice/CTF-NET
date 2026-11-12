"""交互式 shell 会话管理（内存版，MVP）。

工具（如 web.jndi）在反弹 shell 建立后，把「下发命令 / 清理环境」两个
回调注册进来，前端通过 /api/sessions/{id}/exec 持续交互。
进程重启后会话丢失（与 TaskManager 一致）；关闭会话时工具负责清理远程环境。
"""
from __future__ import annotations

import asyncio
import time
import uuid
from dataclasses import dataclass, field
from typing import Awaitable, Callable

# exec 回调: (command, timeout) -> (output, timed_out, closed)
ExecCallback = Callable[[str, float], Awaitable[tuple[str, bool, bool]]]
CloseCallback = Callable[[], Awaitable[None]]


@dataclass
class ShellSession:
    id: str
    exec_cb: ExecCallback
    close_cb: CloseCallback
    created_at: float = field(default_factory=time.time)
    _lock: asyncio.Lock = field(default_factory=asyncio.Lock)

    async def exec_command(self, command: str, timeout: float = 15.0) -> tuple[str, bool, bool]:
        """下发命令并返回 (增量输出, 是否超时, 会话是否已断开)。串行化保证
        fifo 写入与偏移读取不交错。"""
        async with self._lock:
            return await self.exec_cb(command, timeout)

    async def close(self) -> None:
        async with self._lock:
            await self.close_cb()


class SessionRegistry:
    def __init__(self) -> None:
        self._sessions: dict[str, ShellSession] = {}

    def create(self, exec_cb: ExecCallback, close_cb: CloseCallback) -> ShellSession:
        session = ShellSession(id=uuid.uuid4().hex[:12], exec_cb=exec_cb, close_cb=close_cb)
        self._sessions[session.id] = session
        return session

    def get(self, sid: str) -> ShellSession | None:
        return self._sessions.get(sid)

    def remove(self, sid: str) -> None:
        self._sessions.pop(sid, None)
