# -*- coding: utf-8 -*-
"""离线验证 crypto.aes_cbc_bitflip：随机密钥加密 14607 明文，
经真实 API 走 flip 模式（翻转 → 泄露补偿），再用本地解密核对结果。"""
import base64
import json
import time
import urllib.request

from Crypto.Cipher import AES

API = "http://127.0.0.1:8000"
TOOL = "crypto.aes_cbc_bitflip"

PLAIN = 'a:2:{s:8:"username";s:5:"aaaaa";s:8:"password";s:5:"ppppp";}'
WANT_P0 = b'a:2:{s:8:"userna'
WANT_P1 = b'me";s:5:"admin";'
KEY = b'K' * 16
IV = b'I' * 16

CIPHER = AES.new(KEY, AES.MODE_CBC, IV).encrypt(PLAIN.encode() + b"\x04" * 4)  # 60B + PKCS7 pad
CIPHER_B64 = base64.b64encode(CIPHER).decode()
IV_B64 = base64.b64encode(IV).decode()


def _out(r: dict) -> str:
    res = r.get("result") or {}
    return res.get("stdout") or r.get("stdout") or ""


def run_tool(params: dict) -> dict:
    req = urllib.request.Request(
        API + f"/api/tools/{TOOL}/run",
        data=json.dumps({"params": params}).encode(),
        headers={"Content-Type": "application/json"},
    )
    resp = json.load(urllib.request.urlopen(req, timeout=30))
    job_id = resp.get("job_id") or resp.get("id") or resp.get("task_id")
    for _ in range(40):
        r = json.load(urllib.request.urlopen(API + f"/api/tasks/{job_id}", timeout=30))
        status = (r.get("status") or "").lower()
        if status in ("success", "error", "failed", "done", "complete"):
            return r
        time.sleep(0.25)
    raise RuntimeError("任务未结束: " + json.dumps(r, ensure_ascii=False)[:200])


def decrypt(cipher_b64: str, iv: bytes) -> bytes:
    ct = base64.b64decode(cipher_b64)
    return AES.new(KEY, AES.MODE_CBC, iv).decrypt(ct)


def main():
    # —— 第 1 步：flip 模式，不带泄露 ——
    r1 = run_tool({
        "mode": "flip",
        "plaintext": PLAIN,
        "orig": "aaaaa",
        "target": "admin",
        "encoding": "ascii",
        "cipher_b64": CIPHER_B64,
        "iv_b64": IV_B64,
    })
    print("=" * 70)
    print("【flip 无泄露】status =", r1.get("status"))
    print(_out(r1))
    assert r1.get("status") == "success", "flip 模式失败"
    m = None
    import re as _re
    m = _re.search(r"\[密文\] 翻转后 cipher（base64）: (\S+)", _out(r1))
    assert m, "输出里没有翻转后密文"
    flipped_b64 = m.group(1)

    garbage = decrypt(flipped_b64, IV)
    print("\n【本地解密核对（原 IV）】")
    print("  P0(垃圾):", garbage[:16], "期望!=", WANT_P0)
    print("  P1      :", garbage[16:32], "期望 ==", WANT_P1)
    assert garbage[16:32] == WANT_P1, "翻转后 P1 不是 admin 块"
    assert garbage[:16] != WANT_P0, "P0 居然没被破坏？"
    assert garbage[32:] == b's:8:"password";s:5:"ppppp";}\x04\x04\x04\x04', "P2/P3 不应被改动"
    print("  P2/P3 未被改动: OK")

    # —— 第 2 步：flip 模式，带泄露（模拟服务器 die 泄露） ——
    leaked_b64 = base64.b64encode(garbage).decode()
    r2 = run_tool({
        "mode": "flip",
        "plaintext": PLAIN,
        "orig": "aaaaa",
        "target": "admin",
        "encoding": "ascii",
        "cipher_b64": CIPHER_B64,
        "iv_b64": IV_B64,
        "leaked_b64": leaked_b64,
    })
    print("\n" + "=" * 70)
    print("【flip 带泄露】status =", r2.get("status"))
    print(_out(r2))
    assert r2.get("status") == "success", "补偿模式失败"
    m = _re.search(r"\[补偿\] IV_new = IV \^ 垃圾 \^ 期望前缀 = ([A-Za-z0-9+/=]+)", _out(r2))
    assert m, "输出里没有补偿 IV"
    iv_new = base64.b64decode(m.group(1))

    final = decrypt(flipped_b64, iv_new)
    print("\n【补偿后本地解密核对】")
    print("  P0:", final[:16], "期望 ==", WANT_P0)
    print("  P1:", final[16:32], "期望 ==", WANT_P1)
    assert final[:16] == WANT_P0, "补偿 IV 后 P0 未恢复"
    assert final[16:32] == WANT_P1, "补偿后 P1 不对"
    assert final[:60] == PLAIN.replace("aaaaa", "admin").encode(), "整体明文不符"
    print("\n✅ 离线验证全部通过：翻转位置、Δ 值、泄露补偿 IV 均正确")


if __name__ == "__main__":
    main()
