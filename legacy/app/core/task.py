"""任务管理：异步任务队列 + 任务状态追踪。"""
from __future__ import annotations

import asyncio
import shutil
import time
import uuid
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from app.core.executor import Executor
from app.core.tool import Artifact, Status, Tool, ToolContext, ToolResult


@dataclass
class Job:
    id: str
    tool_id: str
    params: dict[str, Any]
    status: Status = Status.PENDING
    result: ToolResult | None = None
    error: str | None = None
    created_at: float = field(default_factory=time.time)
    started_at: float | None = None
    finished_at: float | None = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "tool_id": self.tool_id,
            "params": self.params,
            "status": self.status.value,
            "result": self.result.to_dict() if self.result else None,
            "error": self.error,
            "created_at": self.created_at,
            "started_at": self.started_at,
            "finished_at": self.finished_at,
        }


class TaskManager:
    """内存任务队列（MVP）。可替换为 Redis + 分布式队列。"""

    def __init__(
        self,
        executor: Executor,
        workdir: Path,
        artifacts_dir: Path,
        file_store: Any = None,
        sessions: Any = None,
    ) -> None:
        self.executor = executor
        self.workdir = workdir
        self.artifacts_dir = artifacts_dir
        self.file_store = file_store
        self.sessions = sessions
        self.workdir.mkdir(parents=True, exist_ok=True)
        self.artifacts_dir.mkdir(parents=True, exist_ok=True)
        self._jobs: dict[str, Job] = {}
        self._tasks: set[asyncio.Task] = set()

    def submit(self, tool: Tool, params: dict[str, Any]) -> Job:
        job = Job(id=uuid.uuid4().hex[:12], tool_id=tool.tool_id, params=params)
        self._jobs[job.id] = job
        task = asyncio.create_task(self._run(job, tool))
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)
        return job

    def get(self, job_id: str) -> Job | None:
        return self._jobs.get(job_id)

    async def _run(self, job: Job, tool: Tool) -> None:
        job.status = Status.RUNNING
        job.started_at = time.time()
        job_workdir = self.workdir / job.id
        job_artifacts = self.artifacts_dir / job.id
        job_workdir.mkdir(parents=True, exist_ok=True)
        job_artifacts.mkdir(parents=True, exist_ok=True)
        try:
            params = await asyncio.to_thread(
                self._resolve_file_params, tool, job.params, job_workdir
            )
            ctx = ToolContext(self.executor, job_workdir, job_artifacts, self.sessions)
            result = await tool.run(params, ctx)
            self._collect_artifacts(result, job_artifacts)
            job.result = result
            job.status = result.status
        except Exception as exc:  # noqa: BLE001
            job.status = Status.ERROR
            job.error = f"{type(exc).__name__}: {exc}"
        finally:
            job.finished_at = time.time()

    def _resolve_file_params(
        self, tool: Tool, params: dict[str, Any], workdir: Path
    ) -> dict[str, Any]:
        """把 file 类型参数（file_id）落盘到工作目录，替换为相对文件名。"""
        if not self.file_store:
            return dict(params)
        resolved = dict(params)
        for spec in tool.params:
            if spec.type != "file":
                continue
            fid = resolved.get(spec.name)
            if not fid:
                continue
            stored = self.file_store.get(fid)
            if stored is None:
                raise FileNotFoundError(f"文件不存在: {fid}")
            safe_name = self._safe_filename(stored.name)
            shutil.copyfile(stored.path, workdir / safe_name)
            resolved[spec.name] = safe_name
        return resolved

    @staticmethod
    def _safe_filename(name: str) -> str:
        name = name.replace("\\", "/").split("/")[-1]
        return name or "upload"

    @staticmethod
    def _collect_artifacts(result: ToolResult, artifacts_dir: Path) -> None:
        """自动收集产物目录下工具写出的文件。"""
        existing = {a.path for a in result.artifacts}
        for f in sorted(artifacts_dir.iterdir()):
            if f.is_file() and f not in existing:
                result.artifacts.append(Artifact(name=f.name, path=f, size=f.stat().st_size))
