"""Django SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {{7*7}} 原样输出(模板语言刻意阉割,无算术)。
模板层无法直接 RCE,只能泄露上下文: {% debug %} 输出全部变量、{{ settings.SECRET_KEY }} 视上下文是否传入。
思路: 拿泄露信息配合其他漏洞(伪造 session、进后台),不要指望模板层拿 shell。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiDjangoTool(SstiEngineTool):
    name = "ssti_django"
    description = "Django 模板注入:无直接 RCE,仅上下文泄露(% debug / settings),配合伪造 session(占位符)"
    engine_key = "django"
    engine_name = "Django"
    default_mode = "info"
    plan = r"""指纹: {{7*7}} 原样输出(模板语言刻意阉割,无算术)。
模板层无法直接 RCE,只能泄露上下文: {% debug %} 输出全部变量、{{ settings.SECRET_KEY }} 视上下文是否传入。
思路: 拿泄露信息配合其他漏洞(伪造 session、进后台),不要指望模板层拿 shell。"""
