from fastapi import APIRouter

from app.api.routes import categories, files, sessions, tasks, tools

api_router = APIRouter()
api_router.include_router(categories.router)
api_router.include_router(files.router)
api_router.include_router(sessions.router)
api_router.include_router(tools.router)
api_router.include_router(tasks.router)
