# -*- coding: utf-8 -*-
"""兽音译者（beast_js）编码 / 解码工具。

兽音是「兽音译者」的文本编码：字符集 {嗷,呜,啊,~}（索引 0~3）。每个明文字符
取 4 位十六进制 Unicode 码，第 n 位 hex digit x 按 k=(x+n)%16 映射成 2 个符号
beast[k//4]+beast[k%4]（n 逐位递增，相当于位置偏移）。原版带固定包装
"~呜嗷"+body+"啊"。

CTF 中常把 4 个符号按自然顺序替换成 B/U/G/K（Bugku 主题），且可能抄漏 1 个
符号（body 长度不是 8 的倍数），解码时需枚举恢复。
"""
from __future__ import annotations

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult

BEAST = ["嗷", "呜", "啊", "~"]
WRAP_PREFIX = "~呜嗷"
WRAP_SUFFIX = "啊"
HEX_DIGITS = "0123456789abcdef"

BEAST_MAP = {"嗷": 0, "呜": 1, "啊": 2, "~": 3}
BUGK_MAP = {"B": 0, "U": 1, "G": 2, "K": 3}


def _encode(text: str) -> str:
    out: list[str] = []
    n = 0
    for ch in text:
        hx = f"{ord(ch):04x}"
        for c in hx:
            x = int(c, 16)
            k = (x + n) % 16
            out.append(BEAST[k // 4] + BEAST[k % 4])
            n += 1
    return WRAP_PREFIX + "".join(out) + WRAP_SUFFIX


def _decode_body(body: str, m: dict[str, int]) -> str:
    hx: list[str] = []
    n = 0
    for i in range(0, len(body) - 1, 2):
        k = m[body[i]] * 4 + m[body[i + 1]]
        x = (k - n) % 16
        hx.append(HEX_DIGITS[x])
        n += 1
    hx_str = "".join(hx)
    return "".join(chr(int(hx_str[i:i + 4], 16)) for i in range(0, len(hx_str) - 3, 4))


def _score(text: str) -> float:
    printable = sum(1 for c in text if 32 <= ord(c) < 127)
    ratio = printable / len(text) if text else 0.0
    bonus = sum(1 for kw in ("flag", "bugku", "{", "}") if kw in text)
    return ratio + bonus * 0.2


def _recover(body: str, m: dict[str, int]) -> str:
    """body 长度为奇数（少抄 1 个符号）时，枚举插入 1 个符号恢复。"""
    best, best_score = "", -1.0
    for pos in range(len(body) + 1):
        for c in m:
            candidate = body[:pos] + c + body[pos:]
            decoded = _decode_body(candidate, m)
            sc = _score(decoded)
            if sc > best_score:
                best, best_score = decoded, sc
    return best


def _decode(text: str) -> str:
    body = text.strip()
    wrapped = False
    if body.startswith(WRAP_PREFIX):
        body = body[len(WRAP_PREFIX):]
        wrapped = True
    elif body.startswith("KUB"):
        body = body[3:]
        wrapped = True
    if wrapped:
        if body.endswith(WRAP_SUFFIX):
            body = body[:-1]
        elif body.endswith("G"):
            body = body[:-1]

    if any(c in BEAST_MAP for c in body):
        m = BEAST_MAP
    elif any(c in BUGK_MAP for c in body):
        m = BUGK_MAP
    else:
        return ""

    body = "".join(c for c in body if c in m)
    if not body:
        return ""

    if len(body) % 2 == 1:
        return _recover(body, m)
    return _decode_body(body, m)


@register_tool
class BeastTool(Tool):
    name = "beast"
    category = Category.CRYPTO
    version = "1.0.0"
    description = "兽音译者（beast_js）编码/解码：文本 ↔ 兽音符号（嗷呜啊~/BUGK），解码含缺失符号恢复"
    params = [
        ParamSpec("mode", type="str", required=True, description="encode 或 decode"),
        ParamSpec("data", type="text", required=True, description="要编码/解码的文本"),
    ]

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        mode = params["mode"].lower()
        data = params["data"]
        if mode == "encode":
            return ToolResult(status=Status.SUCCESS, stdout=_encode(data))
        if mode == "decode":
            return ToolResult(status=Status.SUCCESS, stdout=_decode(data))
        return ToolResult(status=Status.ERROR, stderr=f"未知模式: {mode}")
