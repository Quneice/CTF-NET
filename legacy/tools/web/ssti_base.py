"""SSTI 按引擎拆分的工具基类 —— 占位符骨架。

每个模板引擎一个独立工具文件（ssti_<engine>.py），共享本基类的
参数契约与占位符 run()；逐引擎实现时覆盖 run() 即可。
"""
from __future__ import annotations

import json
import re
import urllib.parse
from typing import ClassVar

from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult

MODES = ["exec", "read_file", "info", "detect"]


def parse_raw_http_request(raw: str) -> dict:
    """解析原始 HTTP 请求包（Burp/浏览器复制格式）。

    结构：请求行（METHOD 目标 HTTP/1.1）+ 头部 + 空行 + Body。
    返回 dict：method / url（完整 scheme://host[:port]/path[?query]）/
    headers（小写键）/ params（[(name, value), ...]，查询串在前、表单在后）。

    仅支持 urlencoded 表单与查询参数作为注入载体；multipart/json 抛 ValueError。
    """
    text = raw.replace("\r\n", "\n").replace("\r", "\n").lstrip("\n")
    lines = text.split("\n")
    m = re.match(r"^(\w+)\s+(\S+)(?:\s+HTTP/\d(?:\.\d)?)?\s*$", lines[0])
    if not m:
        raise ValueError(f"无法解析请求行: {lines[0]!r}")
    method, target = m.group(1).upper(), m.group(2)

    headers: dict[str, str] = {}
    idx = 1
    for i in range(1, len(lines)):
        line = lines[i]
        if not line.strip():
            idx = i + 1
            break
        if ":" in line:
            k, v = line.split(":", 1)
            headers[k.strip().lower()] = v.strip()
    body = "\n".join(lines[idx:]).strip()

    if target.startswith(("http://", "https://")):
        url = target
    else:
        host = headers.get("host", "")
        if not host:
            raise ValueError("请求行是相对路径时必须带 Host 头")
        scheme = "https" if host.rsplit(":", 1)[-1] == "443" else "http"
        url = f"{scheme}://{host}{target}"

    params: list[tuple[str, str]] = list(urllib.parse.parse_qsl(
        urllib.parse.urlsplit(url).query, keep_blank_values=True))
    ctype = headers.get("content-type", "")
    if body.strip():
        if "multipart" in ctype:
            raise ValueError("暂不支持 multipart/form-data 原始包")
        params += urllib.parse.parse_qsl(body, keep_blank_values=True)
    return {"method": method, "url": url, "headers": headers, "params": params}


class SstiEngineTool(Tool):
    category = Category.WEB
    version = "0.1.0"
    timeout = 60

    engine_key: ClassVar[str] = ""      # 引擎标识（tool_id 后缀）
    engine_name: ClassVar[str] = ""     # 引擎显示名
    default_mode: ClassVar[str] = "exec"
    plan: ClassVar[str] = ""            # 实现要点（占位输出展示）

    params = [
        ParamSpec("target_url", type="str", required=False,
                  description="注入点 URL（模板回显所在的请求端点）；粘贴原始数据包时可留空"),
        ParamSpec("param", type="str", required=False,
                  description="注入参数名（表单字段 / URL 参数）；原始数据包解析后可选择，留空自动选第一个"),
        ParamSpec("method", type="str", required=False, default="POST",
                  description="请求方法：GET / POST"),
        ParamSpec("raw_request", type="text", required=False, default="",
                  description="原始 HTTP 请求包（请求行 + Host/头 + 空行 + Body）。粘贴后自动解析"
                              "URL/方法/请求头/参数，其余参数按原值固定附带；仅支持 urlencoded 表单"),
        ParamSpec("mode", type="str", required=False,
                  description="exec=命令执行 / read_file=任意文件读 / "
                              "info=信息收集 / detect=指纹确认"),
        ParamSpec("command", type="str", required=False, default="id",
                  description="exec 模式要执行的命令"),
        ParamSpec("file_path", type="str", required=False, default="/etc/passwd",
                  description="read_file 模式要读取的路径"),
        ParamSpec("probe", type="text", required=False, default="",
                  description="自定义指纹探针（留空使用内置探针）"),
        ParamSpec("payload", type="text", required=False, default="",
                  description="自定义 payload（留空按模式自动构造）"),
        ParamSpec("extra_params", type="text", required=False, default="",
                  description="额外固定参数，格式 a=1&b=2（如 team_name=x），每个请求都附带；"
                              "POST 并入表单，GET 并入查询串"),
    ]

    def validate_params(self, params: dict) -> dict:
        """mode 缺省时用各引擎自己的默认模式；target_url 与 raw_request 二选一。"""
        validated = super().validate_params(params)
        validated.setdefault("mode", self.default_mode)
        if not (validated.get("target_url") or validated.get("raw_request")):
            raise ValueError("需要 target_url（靶机端点）或 raw_request（原始数据包）之一")
        if not validated.get("raw_request") and not validated.get("param"):
            raise ValueError("缺少注入参数名 param（粘贴原始数据包时留空则自动选第一个参数）")
        return validated

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        # —— 占位符：功能未实现，仅校验契约并回显本次调用 ——
        mode = params.get("mode", self.default_mode)
        if mode not in MODES:
            return ToolResult(
                status=Status.ERROR,
                stderr=f"未知模式: {mode}（可选: {', '.join(MODES)}）",
            )
        lines = [
            f"[占位符] {self.engine_name} SSTI 利用 —— 功能开发中",
            "",
            "本次调用参数:",
            json.dumps(params, ensure_ascii=False, indent=2),
            "",
            "实现要点:",
            self.plan or "(未填写)",
            "",
            f"当前调用: mode={mode}",
            f"  注入点: {params.get('method', 'POST')} {params.get('target_url', '')}",
            f"  参数名: {params.get('param', '')}",
        ]
        if mode == "exec":
            lines.append(f"  目标命令: {params.get('command', 'id')}")
        if mode == "read_file":
            lines.append(f"  目标路径: {params.get('file_path', '/etc/passwd')}")
        return ToolResult(
            status=Status.SUCCESS,
            stdout="\n".join(lines) + "\n",
            data={"engine": self.engine_key, "mode": mode, "placeholder": True},
        )
