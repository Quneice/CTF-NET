"""ERB SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: <%= 7*7 %>→49。
RCE: <%= system(cmd) %>(注入点落到模板源码即 RCE)。
过滤一般只拦 <%: 测 %> 拼接、换行符、<%% 变体。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiErbTool(SstiEngineTool):
    name = "ssti_erb"
    description = "ERB(Ruby) SSTI:模板直接内嵌 Ruby,<%= system %> 即 RCE(占位符)"
    engine_key = "erb"
    engine_name = "ERB"
    default_mode = "exec"
    plan = r"""指纹: <%= 7*7 %>→49。
RCE: <%= system(cmd) %>(注入点落到模板源码即 RCE)。
过滤一般只拦 <%: 测 %> 拼接、换行符、<%% 变体。"""
