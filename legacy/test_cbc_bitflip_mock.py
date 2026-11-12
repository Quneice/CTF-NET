# -*- coding: utf-8 -*-
"""验证通用化后的 crypto.aes_cbc_bitflip：
A. oracle 模式：Padding Oracle 恢复明文
B. attack 模式：body 来源 + {iv}/{cipher} 占位符 + 无会话 Cookie
C. flip 模式：hex 输入 + iv_in_cipher（iv‖ct 单字段）+ 泄露补偿
对本地 mock 靶机（127.0.0.1:8999）执行。
"""
import base64
import json
import re
import time
import urllib.request

from Crypto.Cipher import AES

API = "http://127.0.0.1:8000"
TOOL = "crypto.aes_cbc_bitflip"
MOCK = "http://127.0.0.1:8999"

KEY = b"M" * 16
IV = b"N" * 16
PLAIN = b"username=aaaaa&role=user"
CT = AES.new(KEY, AES.MODE_CBC, IV).encrypt(PLAIN + bytes([8]) * 8)
CT_B64 = base64.b64encode(CT).decode()
IV_B64 = base64.b64encode(IV).decode()

# 无歧义目标：'role=user' -> 'role=root'（字节 16~24，块 1 偏移 0~8）
ORIG = b"role=user"
TGT = b"role=root"
DELTAS = [a ^ b for a, b in zip(ORIG, TGT)]  # 仅 'user'->'root' 4 字节非 0


def run_tool(params: dict, max_wait: int = 240) -> dict:
    req = urllib.request.Request(
        API + f"/api/tools/{TOOL}/run",
        data=json.dumps({"params": params}).encode(),
        headers={"Content-Type": "application/json"},
    )
    resp = json.load(urllib.request.urlopen(req, timeout=30))
    job_id = resp.get("job_id") or resp.get("id") or resp.get("task_id")
    for _ in range(int(max_wait / 2)):
        r = json.load(urllib.request.urlopen(API + f"/api/tasks/{job_id}", timeout=30))
        if (r.get("status") or "").lower() in ("success", "error", "failed", "done", "complete"):
            return r
        time.sleep(2)
    raise RuntimeError("任务超时")


def out(r: dict) -> str:
    res = r.get("result") or {}
    return res.get("stdout") or r.get("stdout") or ""


def check(name: str, cond: bool, stdout: str):
    print(f"[{'PASS' if cond else 'FAIL'}] {name}")
    if not cond:
        print(stdout[-1200:])
        raise SystemExit(1)


def test_oracle():
    print("=" * 70)
    print("A. oracle 模式：Padding Oracle 恢复明文")
    r = run_tool({
        "mode": "oracle",
        "oracle_url": MOCK + "/oracle",
        "oracle_method": "GET",
        "oracle_params": "token={payload}",
        "oracle_ok_regex": "PADDING OK",
        "target_cipher": CT_B64,
        "iv_b64": IV_B64,
        "input_encoding": "auto",
        "value_encoding": "auto",
        "interval": 0,
        "retries": 3,
    })
    s = out(r)
    print(s)
    check("任务成功", r.get("status") == "success", s)
    check("恢复出 role=user", "username=aaaaa&role=user" in s, s)
    check("去掉填充提示", "去除 PKCS7 填充" in s, s)


def test_attack_body():
    print("=" * 70)
    print("B. attack 模式：body 来源 + 占位符 + 无会话")
    r = run_tool({
        "mode": "attack",
        "plaintext": PLAIN.decode(),
        "orig": ORIG.decode(),
        "target": TGT.decode(),
        "encoding": "ascii",
        "login_url": MOCK + "/login_body",
        "login_method": "GET",
        "login_params": "",
        "replay_url": MOCK + "/check",
        "replay_method": "GET",
        "replay_params": "token={cipher}&iv={iv}",
        "session_cookie": "",
        "source_type": "body",
        "iv_body_regex": '<input[^>]*name="iv"[^>]*value="([^"]+)"',
        "cipher_body_regex": '<input[^>]*name="cipher"[^>]*value="([^"]+)"',
        "value_encoding": "auto",
        "leak_regex": "base64_decode\\('([A-Za-z0-9+/=]+)'\\)",
        "leak_encoding": "base64",
        "success_regex": "flag\\{[^}]+\\}",
        "interval": 0,
        "retries": 3,
    })
    s = out(r)
    print(s)
    check("任务成功", r.get("status") == "success", s)
    check("打到 flag", "flag{mock_cbc_ok}" in s, s)
    check("值走参数而非 Cookie", "token=" in s and "仅 Cookie" not in s, s)


def test_flip_hex_iv_in_cipher():
    print("=" * 70)
    print("C. flip 模式：hex 输入 + iv‖ct 单字段 + 泄露补偿")
    # 本地模拟翻转后的泄露：与工具相同的定位逻辑（role=user 在字节 15~23，跨块；
    # 非零 Δ 落在块 1 偏移 4~7 → 翻转 C0[4..7]）→ 解密得垃圾块 0 + ole=root 块 1
    pos = PLAIN.index(ORIG)
    cb = bytearray(CT)
    for i, d in enumerate(DELTAS):
        if d:
            off = pos + i
            j, o = off // 16, off % 16
            assert j == 1, f"偏移 {off} 不在块 1"
            cb[o] ^= d
    garbage = AES.new(KEY, AES.MODE_CBC, IV).decrypt(bytes(cb))
    assert garbage[16:24] == b"ole=root"
    leaked_hex = garbage.hex()

    r = run_tool({
        "mode": "flip",
        "plaintext": PLAIN.decode(),
        "orig": ORIG.decode(),
        "target": TGT.decode(),
        "encoding": "ascii",
        "cipher_b64": (IV + CT).hex(),
        "input_encoding": "hex",
        "iv_in_cipher": True,
        "leaked_b64": leaked_hex,
    })
    s = out(r)
    print(s)
    check("任务成功", r.get("status") == "success", s)
    m = re.search(r"\[密文\] 翻转后 iv‖ct（hex）: ([0-9a-f]+)", s)
    check("输出 hex 的 iv‖ct", bool(m), s)
    m = re.search(r"└─ cipher=([0-9a-f]+)", s)  # 补偿后的最终 iv‖ct 在重放行
    check("输出重放 cipher", bool(m), s)
    combined = bytes.fromhex(m.group(1))
    iv_new, ct_new = combined[:16], combined[16:]
    final = AES.new(KEY, AES.MODE_CBC, iv_new).decrypt(ct_new)
    check("补偿后块 0 恢复", final[:16] == b"username=aaaaa&r", s)
    check("块 1 为 role=root", final[16:24] == b"ole=root", s)


def test_attack_manual():
    print("=" * 70)
    print("D. attack 模式：手动来源（不登录，直接给 iv/cipher）")
    r = run_tool({
        "mode": "attack",
        "plaintext": PLAIN.decode(),
        "orig": ORIG.decode(),
        "target": TGT.decode(),
        "encoding": "ascii",
        "source_type": "manual",
        "manual_iv": IV_B64,
        "manual_cipher": CT_B64,
        "value_encoding": "auto",
        "replay_url": MOCK + "/check",
        "replay_method": "GET",
        "replay_params": "token={cipher}&iv={iv}",
        "session_cookie": "",
        "leak_regex": "base64_decode\\('([A-Za-z0-9+/=]+)'\\)",
        "leak_encoding": "base64",
        "success_regex": "flag\\{[^}]+\\}",
        "interval": 0,
        "retries": 3,
    })
    s = out(r)
    print(s)
    check("任务成功", r.get("status") == "success", s)
    check("打到 flag", "flag{mock_cbc_ok}" in s, s)
    check("跳过登录", "跳过登录" in s and "[登录]" not in s, s)


if __name__ == "__main__":
    test_oracle()
    test_attack_body()
    test_flip_hex_iv_in_cipher()
    test_attack_manual()
    print("\n✅ 通用化 mock 验证全部通过：oracle / body 捕获 / 占位符 / hex / iv‖ct / 手动来源")
