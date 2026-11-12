from fastapi import HTTPException, Request

from app.core.registry import registry
from app.core.sessions import SessionRegistry
from app.core.task import TaskManager
from app.core.tool import Tool
from app.storage.files import FileStore


def get_tool_or_404(tool_id: str) -> Tool:
    tool = registry.get(tool_id)
    if tool is None:
        raise HTTPException(status_code=404, detail=f"工具不存在: {tool_id}")
    return tool


def get_task_manager(request: Request) -> TaskManager:
    return request.app.state.task_manager


def get_sessions(request: Request) -> SessionRegistry:
    return request.app.state.shell_sessions


def get_file_store(request: Request) -> FileStore:
    return request.app.state.file_store
