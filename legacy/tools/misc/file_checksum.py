"""代码型文件工具示例：演示 file 类型参数的解析流程。"""
import hashlib

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult


@register_tool
class FileChecksumTool(Tool):
    name = "file_checksum"
    category = Category.MISC
    version = "1.0.0"
    description = "计算上传文件的哈希（md5 / sha1 / sha256）"
    params = [
        ParamSpec("file", type="file", required=True, description="上传文件的 file_id"),
        ParamSpec("algo", type="str", required=False, default="sha256", description="md5 / sha1 / sha256"),
    ]

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        algo = params.get("algo", "sha256").lower()
        if algo not in ("md5", "sha1", "sha256"):
            return ToolResult(status=Status.ERROR, stderr=f"不支持的算法: {algo}")
        filepath = ctx.workdir / params["file"]
        h = hashlib.new(algo)
        h.update(filepath.read_bytes())
        return ToolResult(status=Status.SUCCESS, stdout=h.hexdigest())
