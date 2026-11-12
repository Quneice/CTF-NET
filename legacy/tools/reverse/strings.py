"""命令型工具示例：包装外部 CLI（strings）。"""
from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Tool


@register_tool
class StringsTool(Tool):
    name = "strings"
    category = Category.REVERSE
    version = "1.0.0"
    description = "提取二进制文件中的可打印字符串（strings）"
    timeout = 60
    params = [
        ParamSpec("file", type="file", required=True, description="上传文件的 file_id（见 POST /api/files）"),
        ParamSpec("min_len", type="int", required=False, default=4, description="最小字符串长度"),
    ]

    def build_command(self, params: dict) -> list[str]:
        return ["strings", "-n", str(params.get("min_len", 4)), params["file"]]
