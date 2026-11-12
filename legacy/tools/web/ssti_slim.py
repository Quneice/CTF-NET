"""Slim SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: 行首 = 7*7→49。
RCE: 行首 = 输出命令结果 = %x{cmd};行首 - 静默执行 - system(cmd)。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiSlimTool(SstiEngineTool):
    name = "ssti_slim"
    description = "Slim(Ruby) SSTI:行首 = / - 注入 Ruby 语句(占位符)"
    engine_key = "slim"
    engine_name = "Slim"
    default_mode = "exec"
    plan = r"""指纹: 行首 = 7*7→49。
RCE: 行首 = 输出命令结果 = %x{cmd};行首 - 静默执行 - system(cmd)。"""
