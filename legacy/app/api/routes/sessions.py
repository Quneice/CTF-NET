"""交互式 shell 会话 API：下发命令 / 查询状态 / 关闭会话。"""
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field

from app.api.deps import get_sessions
from app.core.sessions import SessionRegistry

router = APIRouter(prefix="/sessions", tags=["sessions"])


class ExecRequest(BaseModel):
    command: str = Field(min_length=1, max_length=4096, description="下发给反弹 shell 的命令")
    timeout: float = Field(default=15, ge=2, le=120, description="等待命令完成的超时（秒）")


class ExecResponse(BaseModel):
    output: str
    timed_out: bool = False
    closed: bool = False


@router.get("/{sid}")
def session_status(sid: str, sessions: SessionRegistry = Depends(get_sessions)) -> dict:
    return {"alive": sessions.get(sid) is not None}


@router.post("/{sid}/exec", response_model=ExecResponse)
async def exec_command(
    sid: str, body: ExecRequest, sessions: SessionRegistry = Depends(get_sessions)
) -> ExecResponse:
    session = sessions.get(sid)
    if session is None:
        raise HTTPException(status_code=404, detail="会话不存在或已关闭")
    output, timed_out, closed = await session.exec_command(body.command, body.timeout)
    if closed:
        sessions.remove(sid)
        await session.close()  # 远程清理（shell 已死，仅回收 fifo 保活进程与工作目录）
    return ExecResponse(output=output, timed_out=timed_out, closed=closed)


@router.post("/{sid}/close")
async def close_session(sid: str, sessions: SessionRegistry = Depends(get_sessions)) -> dict:
    session = sessions.get(sid)
    if session is None:
        raise HTTPException(status_code=404, detail="会话不存在或已关闭")
    sessions.remove(sid)
    await session.close()
    return {"ok": True}
