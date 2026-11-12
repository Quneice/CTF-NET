"""AngularJS 1.x SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {{7*7}}→49。
1.6+ 无沙箱: {{constructor.constructor('alert(1)')()}}
≤1.5 有沙箱但历史绕过链多;输出型利用(前端 XSS/RCE)。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiAngularjsTool(SstiEngineTool):
    name = "ssti_angularjs"
    description = "AngularJS 1.x 前端模板注入:constructor.constructor 执行 JS(占位符)"
    engine_key = "angularjs"
    engine_name = "AngularJS 1.x"
    default_mode = "info"
    plan = r"""指纹: {{7*7}}→49。
1.6+ 无沙箱: {{constructor.constructor('alert(1)')()}}
≤1.5 有沙箱但历史绕过链多;输出型利用(前端 XSS/RCE)。"""
