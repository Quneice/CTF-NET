from fastapi import APIRouter, Depends, File, HTTPException, UploadFile
from fastapi.responses import FileResponse

from app.api.deps import get_file_store
from app.models.schemas import FileInfo
from app.storage.files import FileStore

router = APIRouter(prefix="/files", tags=["files"])


@router.post("", response_model=FileInfo)
async def upload_file(
    file: UploadFile = File(...), store: FileStore = Depends(get_file_store)
) -> FileInfo:
    data = await file.read()
    stored = store.save(file.filename or "upload", data)
    return FileInfo(file_id=stored.id, name=stored.name, size=stored.size)


@router.get("/{file_id}")
def download_file(
    file_id: str, store: FileStore = Depends(get_file_store)
) -> FileResponse:
    stored = store.get(file_id)
    if stored is None:
        raise HTTPException(status_code=404, detail=f"文件不存在: {file_id}")
    return FileResponse(stored.path, filename=stored.name)
