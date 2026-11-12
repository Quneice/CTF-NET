import hashlib

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult


@register_tool
class HashTool(Tool):
    name = "hash"
    category = Category.MISC
    version = "1.0.0"
    description = "计算常见哈希（md5/sha1/sha256）"
    params = [
        ParamSpec("algo", type="str", required=True, description="md5 / sha1 / sha256"),
        ParamSpec("data", type="text", required=True, description="要哈希的数据"),
    ]

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        algo = params["algo"].lower()
        if algo not in ("md5", "sha1", "sha256"):
            return ToolResult(status=Status.ERROR, stderr=f"不支持的算法: {algo}")
        h = hashlib.new(algo)
        h.update(params["data"].encode())
        return ToolResult(status=Status.SUCCESS, stdout=h.hexdigest())
