"""产物存储：落盘工具产出的文件。"""
from __future__ import annotations

from pathlib import Path

from app.core.tool import Artifact


class ArtifactStore:
    def __init__(self, base_dir: Path) -> None:
        self.base_dir = base_dir
        self.base_dir.mkdir(parents=True, exist_ok=True)

    def save_bytes(self, name: str, data: bytes) -> Artifact:
        path = self.base_dir / name
        path.write_bytes(data)
        return Artifact(name=name, path=path, size=len(data))

    def save_text(self, name: str, text: str) -> Artifact:
        return self.save_bytes(name, text.encode("utf-8"))

    def path_for(self, name: str) -> Path:
        return self.base_dir / name
