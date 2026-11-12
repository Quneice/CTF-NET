"""Groovy SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: ${7*7}→49(Jenkins 场景 ${{7*7}})。
RCE: ${"id".execute().text} —— 字符串 .execute() 直接执行命令。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiGroovyTool(SstiEngineTool):
    name = "ssti_groovy"
    description = "Groovy(Jenkins 等) SSTI:字符串 .execute() 直接跑命令(占位符)"
    engine_key = "groovy"
    engine_name = "Groovy"
    default_mode = "exec"
    plan = r"""指纹: ${7*7}→49(Jenkins 场景 ${{7*7}})。
RCE: ${"id".execute().text} —— 字符串 .execute() 直接执行命令。"""
