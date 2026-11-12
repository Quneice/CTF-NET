"""命令型工具示例：包装外部 CLI（curl）。"""
from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Tool


@register_tool
class HttpHeadersTool(Tool):
    name = "http_headers"
    category = Category.WEB
    version = "1.0.0"
    description = "获取 HTTP 响应头（curl -I）"
    timeout = 30
    params = [
        ParamSpec("url", type="str", required=True, description="目标 URL"),
    ]

    def build_command(self, params: dict) -> list[str]:
        return ["curl", "-sI", params["url"]]
