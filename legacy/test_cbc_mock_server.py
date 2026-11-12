# -*- coding: utf-8 -*-
"""本地 mock 靶机：验证 CBC 工具的通用能力（oracle / body 捕获 / 占位符 / hex / iv‖ct）。
端口 8999，固定密钥。明文 = username=aaaaa&role=user（24B + 8B PKCS7 = 32B，2 块）。
"""
import base64

from Crypto.Cipher import AES
from fastapi import FastAPI
from fastapi.responses import HTMLResponse, PlainTextResponse

KEY = b"M" * 16
IV = b"N" * 16
PLAIN = b"username=aaaaa&role=user"

app = FastAPI()


def enc() -> bytes:
    return AES.new(KEY, AES.MODE_CBC, IV).encrypt(PLAIN + bytes([8]) * 8)


def pkcs7_ok(data: bytes) -> bool:
    if not data or len(data) % 16:
        return False
    p = data[-1]
    return 1 <= p <= 16 and data[-p:] == bytes([p]) * p


@app.post("/login")
def login():
    """cookie 来源：PHPSESSID + iv + cipher 三个 Set-Cookie。"""
    ct = enc()
    resp = PlainTextResponse("logged in")
    resp.set_cookie("PHPSESSID", "mocksess123")
    resp.set_cookie("iv", base64.b64encode(IV).decode())
    resp.set_cookie("cipher", base64.b64encode(ct).decode())
    return resp


@app.get("/login_body")
def login_body():
    """body 来源：HTML 隐藏表单里给 iv/cipher。"""
    ct = enc()
    html = (
        '<html><form>'
        f'<input type="hidden" name="iv" value="{base64.b64encode(IV).decode()}" />'
        f'<input type="hidden" name="cipher" value="{base64.b64encode(ct).decode()}" />'
        '</form></html>'
    )
    return HTMLResponse(html)


@app.get("/check")
def check(token: str = "", iv: str = ""):
    """attack 重放端点：解密后 role=root 给 flag，否则 die 式泄露解密明文。"""
    try:
        ct = base64.b64decode(token)
        ivb = base64.b64decode(iv)
    except Exception:
        return PlainTextResponse("BAD B64", status_code=400)
    plain = AES.new(KEY, AES.MODE_CBC, ivb).decrypt(ct)
    if not pkcs7_ok(plain):
        return PlainTextResponse("BAD PAD", status_code=500)
    if b"role=root" in plain:
        return PlainTextResponse("flag{mock_cbc_ok}")
    return PlainTextResponse(
        f"base64_decode('{base64.b64encode(plain).decode()}') can't unserialize")


@app.get("/oracle")
def oracle(token: str = ""):
    """Padding Oracle：token = iv′‖C（32B），填充有效 200 VALID / 无效 500 INVALID。"""
    try:
        payload = base64.b64decode(token)
    except Exception:
        return PlainTextResponse("BAD B64", status_code=400)
    if len(payload) < 32:
        return PlainTextResponse("SHORT", status_code=400)
    ivb, ct = payload[:16], payload[16:]
    plain = AES.new(KEY, AES.MODE_CBC, ivb).decrypt(ct)
    if pkcs7_ok(plain):
        return PlainTextResponse("PADDING OK")
    return PlainTextResponse("PADDING BAD", status_code=500)
