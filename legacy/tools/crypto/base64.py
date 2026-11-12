"""代码型工具示例：纯 Python 逻辑。"""
import base64

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult


@register_tool
class Base64Tool(Tool):
    name = "base64"
    category = Category.CRYPTO
    version = "1.0.0"
    description = "Base64 编码 / 解码"
    params = [
        ParamSpec("mode", type="str", required=True, description="encode 或 decode"),
        ParamSpec("data", type="text", required=True, description="要处理的数据"),
    ]

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        mode = params["mode"]
        data = params["data"]
        if mode == "encode":
            out = base64.b64encode(data.encode()).decode()
        elif mode == "decode":
            out = base64.b64decode(data).decode(errors="replace")
        else:
            return ToolResult(status=Status.ERROR, stderr=f"未知模式: {mode}")
        return ToolResult(status=Status.SUCCESS, stdout=out)
