"""Go template SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {{.}} 渲染根对象;{{7*7}} 报解析错误(无内建算术)。
无 import、无反射: 只能调用已注册的模板函数和传入对象的公开方法。
RCE 取决于应用暴露了什么: 如 {{.Exec "id"}} 或传入危险对象;否则只能泄露结构体字段。
html/template 自动转义,但历史上仍有上下文逃逸 CVE(如 CVE-2023-24538)。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiGoTool(SstiEngineTool):
    name = "ssti_go"
    description = "Go 模板注入:无 import/反射,依赖已注册函数与暴露对象(占位符)"
    engine_key = "go"
    engine_name = "Go template"
    default_mode = "info"
    plan = r"""指纹: {{.}} 渲染根对象;{{7*7}} 报解析错误(无内建算术)。
无 import、无反射: 只能调用已注册的模板函数和传入对象的公开方法。
RCE 取决于应用暴露了什么: 如 {{.Exec "id"}} 或传入危险对象;否则只能泄露结构体字段。
html/template 自动转义,但历史上仍有上下文逃逸 CVE(如 CVE-2023-24538)。"""
