from fastapi import APIRouter

from app.core.registry import registry
from app.models.schemas import CategoryInfo

router = APIRouter(prefix="/categories", tags=["categories"])


@router.get("", response_model=list[CategoryInfo])
def list_categories() -> list[CategoryInfo]:
    return [
        CategoryInfo(name=c.value, tool_count=len(registry.all(c)))
        for c in registry.categories()
    ]
