from fastapi import APIRouter, Depends, HTTPException, Query

from app.api.deps import get_task_manager, get_tool_or_404
from app.core.registry import registry
from app.core.task import TaskManager
from app.core.tool import Category
from app.models.schemas import (
    ParamSchema,
    RunRequest,
    RunResponse,
    ToolDetail,
    ToolSummary,
)

router = APIRouter(prefix="/tools", tags=["tools"])


@router.get("", response_model=list[ToolSummary])
def list_tools(category: Category | None = Query(default=None)) -> list[ToolSummary]:
    return [
        ToolSummary(
            id=t.tool_id,
            name=t.name,
            category=t.category.value,
            version=t.version,
            description=t.description,
            timeout=t.timeout,
        )
        for t in registry.all(category)
    ]


@router.get("/{tool_id}", response_model=ToolDetail)
def get_tool(tool_id: str) -> ToolDetail:
    tool = get_tool_or_404(tool_id)
    return ToolDetail(
        id=tool.tool_id,
        name=tool.name,
        category=tool.category.value,
        version=tool.version,
        description=tool.description,
        timeout=tool.timeout,
        params=[
            ParamSchema(
                name=p.name,
                type=p.type,
                required=p.required,
                default=p.default,
                description=p.description,
            )
            for p in tool.params
        ],
    )


@router.post("/{tool_id}/run", response_model=RunResponse)
async def run_tool(
    tool_id: str,
    body: RunRequest,
    tm: TaskManager = Depends(get_task_manager),
) -> RunResponse:
    tool = get_tool_or_404(tool_id)
    try:
        params = tool.validate_params(body.params)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    job = tm.submit(tool, params)
    return RunResponse(job_id=job.id)
