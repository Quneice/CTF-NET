"""Handlebars SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {{this}}(logic-less,无算术,{{7*7}} 报错/原样)。
无直接 RCE: BlackHat 2019(Mahmoud Gamal)用 lookup/with/split/pop/push 摸到 constructor 构造 Function 执行。
CTF 中更常见当 XSS/信息泄露打,RCE 链题少见。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiHandlebarsTool(SstiEngineTool):
    name = "ssti_handlebars"
    description = "Handlebars SSTI:无直接 RCE,lookup/with/split 构造 Function 链(占位符)"
    engine_key = "handlebars"
    engine_name = "Handlebars"
    default_mode = "info"
    plan = r"""指纹: {{this}}(logic-less,无算术,{{7*7}} 报错/原样)。
无直接 RCE: BlackHat 2019(Mahmoud Gamal)用 lookup/with/split/pop/push 摸到 constructor 构造 Function 执行。
CTF 中更常见当 XSS/信息泄露打,RCE 链题少见。"""
