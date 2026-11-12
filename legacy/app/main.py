from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles

from app.api.routes import api_router
from app.config import get_settings
from app.core.executor import DockerExecutor, Executor, SubprocessExecutor
from app.core.registry import registry
from app.core.sessions import SessionRegistry
from app.core.task import TaskManager
from app.storage.files import FileStore


def build_executor(settings) -> Executor:
    if settings.executor_backend == "docker":
        return DockerExecutor(
            image=settings.docker_image,
            network=settings.docker_network,
            data_volume=settings.docker_data_volume,
            data_root=settings.data_dir,
        )
    return SubprocessExecutor()


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    # 自动发现并注册所有工具插件
    registry.discover("tools")
    # 初始化执行引擎、文件存储与任务管理器，注入 app.state
    executor = build_executor(settings)
    app.state.executor = executor
    app.state.file_store = FileStore(settings.uploads_dir)
    sessions = SessionRegistry()
    app.state.shell_sessions = sessions
    app.state.task_manager = TaskManager(
        executor=executor,
        workdir=settings.work_dir,
        artifacts_dir=settings.artifacts_dir,
        file_store=app.state.file_store,
        sessions=sessions,
    )
    yield


def create_app() -> FastAPI:
    settings = get_settings()
    app = FastAPI(
        title=settings.app_name,
        version="0.1.0",
        description="可扩展的 CTF 工具集成平台（工具箱）",
        lifespan=lifespan,
    )
    app.include_router(api_router, prefix=settings.api_prefix)

    @app.get("/health")
    def health():
        return {"status": "ok", "tool_count": len(registry.all())}

    static_dir = Path(__file__).resolve().parent / "static"
    app.mount("/", StaticFiles(directory=static_dir, html=True), name="static")

    return app


app = create_app()
