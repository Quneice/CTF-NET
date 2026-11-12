"""API 请求/响应模型（Pydantic）。"""
from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class ToolSummary(BaseModel):
    id: str
    name: str
    category: str
    version: str
    description: str
    timeout: int


class ParamSchema(BaseModel):
    name: str
    type: str
    required: bool
    default: Any | None = None
    description: str = ""


class ToolDetail(ToolSummary):
    params: list[ParamSchema] = Field(default_factory=list)


class CategoryInfo(BaseModel):
    name: str
    tool_count: int


class RunRequest(BaseModel):
    params: dict[str, Any] = Field(default_factory=dict)


class RunResponse(BaseModel):
    job_id: str


class FileInfo(BaseModel):
    file_id: str
    name: str
    size: int


class ArtifactInfo(BaseModel):
    name: str
    size: int
    content_type: str


class ResultPayload(BaseModel):
    status: str
    exit_code: int
    stdout: str
    stderr: str
    duration: float
    artifacts: list[ArtifactInfo] = Field(default_factory=list)
    data: dict[str, Any] = Field(default_factory=dict)


class JobResponse(BaseModel):
    id: str
    tool_id: str
    status: str
    params: dict[str, Any]
    result: ResultPayload | None = None
    error: str | None = None
    created_at: float
    started_at: float | None = None
    finished_at: float | None = None
