"""文件存储：管理上传的文件。"""
from __future__ import annotations

import uuid
from dataclasses import dataclass
from pathlib import Path


@dataclass
class StoredFile:
    id: str
    name: str
    path: Path
    size: int


class FileStore:
    def __init__(self, base_dir: Path) -> None:
        self.base_dir = base_dir
        self.base_dir.mkdir(parents=True, exist_ok=True)

    def save(self, filename: str, data: bytes) -> StoredFile:
        fid = uuid.uuid4().hex[:16]
        path = self.base_dir / fid
        path.write_bytes(data)
        (self.base_dir / f"{fid}.name").write_text(filename, encoding="utf-8")
        return StoredFile(id=fid, name=filename, path=path, size=len(data))

    def get(self, fid: str) -> StoredFile | None:
        path = self.base_dir / fid
        if not path.exists():
            return None
        name_path = self.base_dir / f"{fid}.name"
        name = name_path.read_text(encoding="utf-8") if name_path.exists() else "file"
        return StoredFile(id=fid, name=name, path=path, size=path.stat().st_size)
