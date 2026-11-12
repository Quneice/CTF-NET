"""核心抽象：赛题类型、工具基类、参数声明、结果模型。

这是整个工具箱的"契约层"——所有工具插件都必须遵守这里的接口约定。
"""
from __future__ import annotations

from abc import ABC
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
from typing import Any, ClassVar


class Category(str, Enum):
    """CTF 赛题类型。新增类型时在此扩展即可。"""

    WEB = "web"
    PWN = "pwn"
    REVERSE = "reverse"
    CRYPTO = "crypto"
    MISC = "misc"
    FORENSICS = "forensics"
    MOBILE = "mobile"
    BLOCKCHAIN = "blockchain"
    OSINT = "osint"
    AI = "ai"


@dataclass(frozen=True)
class ParamSpec:
    """工具参数的声明式描述，用于自动校验 + 生成 API 文档。"""

    name: str
    type: str = "str"  # str | int | float | bool | text | file
    required: bool = True
    default: Any = None
    description: str = ""


class Status(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    SUCCESS = "success"
    ERROR = "error"
    TIMEOUT = "timeout"


@dataclass
class Artifact:
    """工具产生的文件产物。"""

    name: str
    path: Path
    size: int = 0
    content_type: str = "application/octet-stream"


@dataclass
class ToolResult:
    """统一的工具执行结果。"""

    status: Status
    exit_code: int = 0
    stdout: str = ""
    stderr: str = ""
    duration: float = 0.0
    artifacts: list[Artifact] = field(default_factory=list)
    data: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "status": self.status.value,
            "exit_code": self.exit_code,
            "stdout": self.stdout,
            "stderr": self.stderr,
            "duration": round(self.duration, 3),
            "artifacts": [
                {"name": a.name, "size": a.size, "content_type": a.content_type}
                for a in self.artifacts
            ],
            "data": self.data,
        }


class ToolContext:
    """工具运行时上下文：提供执行器、工作目录、产物目录、交互会话注册表。"""

    def __init__(
        self,
        executor: Any,
        workdir: Path,
        artifacts_dir: Path,
        sessions: Any = None,
    ) -> None:
        self.executor = executor
        self.workdir = workdir
        self.artifacts_dir = artifacts_dir
        self.sessions = sessions


class Tool(ABC):
    """工具基类。

    两种实现方式：
    1. 命令型：重写 build_command()，返回要执行的命令，由 Executor 跑子进程；
    2. 代码型：重写 run()，在 Python 内直接完成逻辑。
    """

    name: ClassVar[str] = ""
    category: ClassVar[Category] = Category.MISC
    version: ClassVar[str] = "1.0.0"
    description: ClassVar[str] = ""
    params: ClassVar[list[ParamSpec]] = []
    timeout: ClassVar[int] = 60

    @property
    def tool_id(self) -> str:
        return f"{self.category.value}.{self.name}"

    def validate_params(self, params: dict[str, Any]) -> dict[str, Any]:
        """基于 ParamSpec 做必填校验与默认值填充。"""
        validated: dict[str, Any] = {}
        for spec in self.params:
            value = params.get(spec.name, spec.default)
            if spec.required and value in (None, ""):
                raise ValueError(f"缺少必需参数: {spec.name}")
            if value is not None:
                validated[spec.name] = value
        return validated

    def build_command(self, params: dict[str, Any]) -> list[str]:
        """命令型工具：返回要在子进程中执行的命令（含参数）。"""
        raise NotImplementedError(
            f"工具 {self.tool_id} 既未实现 build_command 也未覆盖 run"
        )

    async def run(self, params: dict[str, Any], ctx: ToolContext) -> ToolResult:
        """默认执行流程：命令型。代码型工具覆盖此方法。"""
        cmd = self.build_command(params)
        return await ctx.executor.run(cmd, cwd=ctx.workdir, timeout=self.timeout)
