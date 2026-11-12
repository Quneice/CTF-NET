from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import FileResponse

from app.api.deps import get_task_manager
from app.core.task import TaskManager
from app.models.schemas import JobResponse

router = APIRouter(prefix="/tasks", tags=["tasks"])


@router.get("/{job_id}", response_model=JobResponse)
def get_job(job_id: str, tm: TaskManager = Depends(get_task_manager)) -> JobResponse:
    job = tm.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail=f"任务不存在: {job_id}")
    return JobResponse(**job.to_dict())


@router.get("/{job_id}/artifacts/{name}")
def download_artifact(
    job_id: str, name: str, tm: TaskManager = Depends(get_task_manager)
) -> FileResponse:
    job = tm.get(job_id)
    if job is None or job.result is None:
        raise HTTPException(status_code=404, detail=f"任务或结果不存在: {job_id}")
    for artifact in job.result.artifacts:
        if artifact.name == name:
            return FileResponse(artifact.path, filename=artifact.name)
    raise HTTPException(status_code=404, detail=f"产物不存在: {name}")
