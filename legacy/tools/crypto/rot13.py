import codecs

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult


@register_tool
class Rot13Tool(Tool):
    name = "rot13"
    category = Category.CRYPTO
    version = "1.0.0"
    description = "ROT13 凯撒移位（位移 13）"
    params = [
        ParamSpec("data", type="text", required=True, description="要处理的数据"),
    ]

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        return ToolResult(status=Status.SUCCESS, stdout=codecs.encode(params["data"], "rot_13"))
