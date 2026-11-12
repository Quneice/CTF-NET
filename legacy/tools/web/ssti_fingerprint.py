"""SSTI 自动指纹识别 SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
探针表: {{7*7}}→Jinja2/Twig/Nunjucks/AngularJS; ${7*7}→FreeMarker/Velocity/EL/Mako; <%= 7*7 %>→ERB/EJS; #{7*7}→Pug; [[7*7]]→Thymeleaf 3 内联; @(7*7)→Razor; {$smarty.version}→Smarty; {{.}}→Go 模板。
Jinja2 确认: {{7*'7'}}→7777777(特有)、{{config}}→Flask 配置对象。
Smarty 确认: {$smarty.version} 输出版本号、{math equation="7*7"}→49。
实现: 逐探针发送 → 匹配响应 → 报告引擎与版本。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiFingerprintTool(SstiEngineTool):
    name = "ssti_fingerprint"
    description = "多引擎探针指纹识别:识别 Jinja2/Smarty/Twig/FreeMarker/Velocity/Thymeleaf/ERB/Pug 等引擎与版本(占位符)"
    engine_key = "fingerprint"
    engine_name = "SSTI 自动指纹识别"
    default_mode = "detect"
    plan = r"""探针表: {{7*7}}→Jinja2/Twig/Nunjucks/AngularJS; ${7*7}→FreeMarker/Velocity/EL/Mako; <%= 7*7 %>→ERB/EJS; #{7*7}→Pug; [[7*7]]→Thymeleaf 3 内联; @(7*7)→Razor; {$smarty.version}→Smarty; {{.}}→Go 模板。
Jinja2 确认: {{7*'7'}}→7777777(特有)、{{config}}→Flask 配置对象。
Smarty 确认: {$smarty.version} 输出版本号、{math equation="7*7"}→49。
实现: 逐探针发送 → 匹配响应 → 报告引擎与版本。"""
