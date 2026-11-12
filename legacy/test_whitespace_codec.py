# -*- coding: utf-8 -*-
"""验证 misc.whitespace_codec：
A. hidden snowfall 方案 encode→decode 往返（含中文 utf-8 消息、掩护文本）
B. hidden fixed8 往返
C. ws 算术两个操作数 token（0+55=55 → '7'，写解释器最易踩的坑）
D. ws drop 补丁打印栈顶（Snowfall flag.txt 场景）
E. ws 标签跳转（mark/jz）
F. ws 从中文文本行尾提取程序（step2.txt 场景）
G. ws 输出魔数识别（PK.. → ZIP）
H. hidden snowfall 空白放行首（placement=leading → scope=leading 往返）
I. hidden snowfall 纯空白流（placement=standalone → scope=anywhere 往返）
J. ws 从中文文本行首提取程序（空白不一定在行尾）
K. hidden decode scope=auto 自动探测位置（三种范围并列展示）
对本地后端 127.0.0.1:8000 执行。
"""
import json
import time
import urllib.request

API = "http://127.0.0.1:8000"
TOOL = "misc.whitespace_codec"

SP, TAB, LF = 32, 9, 10


def run_tool(params: dict, max_wait: int = 60) -> dict:
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


def text_section(s: str) -> str:
    """[文本] 与 [base64] 之间的内容（解释器输出正文所在段）。"""
    if "[文本]" not in s:
        return ""
    return s.split("[文本]", 1)[1].split("[base64]", 1)[0]


def check(name: str, cond: bool, stdout: str):
    print(f"[{'PASS' if cond else 'FAIL'}] {name}")
    if not cond:
        print(stdout[-1500:])
        raise SystemExit(1)


def ws_str(tokens: list[int]) -> str:
    return "".join(chr(t) for t in tokens)


def label(n: int) -> list[int]:
    t = [SP if n >= 0 else TAB]
    for c in f"{abs(n):b}":
        t.append(TAB if c == "1" else SP)
    t.append(LF)
    return t


def push(n: int) -> list[int]:
    t = [SP, SP, SP if n >= 0 else TAB]
    for c in f"{abs(n):b}":
        t.append(TAB if c == "1" else SP)
    t.append(LF)
    return t


OUT_C = [TAB, LF, SP, SP]
DROP = [SP, LF, LF]
ADD = [TAB, SP, SP, SP]


def test_hidden_snowfall():
    print("=" * 70)
    print("A. hidden · snowfall：encode → decode 往返（含中文）")
    msg = "OK now you can run whitespace code. By the way, the key is H0wt0Pr1ntAWh17e5p4ceC0de。中文测试"
    # 掩护文本不能含内部空格/Tab（all-scope 算法会混入位流），纯中文文本天然满足
    carrier = "\n".join([
        "雪山崩塌的时候，没有一片雪花是无辜的。",
        "雪花落在掌心，转瞬即逝，像代码里的空白字符。",
        "没有人会在意行尾多出的空格与制表符。",
        "除非你是一个CTF选手。",
    ])
    r = run_tool({
        "mode": "hidden", "action": "encode", "scheme": "snowfall",
        "bit_map": "space0_tab1", "data": msg, "carrier": carrier,
    })
    s = out(r)
    check("encode 任务成功", r.get("status") == "success", s)
    body = s.split("[结果] 隐藏后的文本（空白不可见，请整段复制）:", 1)[1].split("\n\n[可视化]", 1)[0]
    body = body[1:] if body.startswith("\n") else body  # 只去冒号后首换行；保留末尾换行（writeup 算法依赖）
    check("encode 输出含标记行可视化", "␣" in s and "⇥" in s, s)
    # 与 writeup 原样算法交叉验证：空格=0 Tab=1 换行=' '，按 " 1 " 切分
    a = ""
    for ch in body:
        if ch == " ":
            a += "0"
        elif ch == "\t":
            a += "1"
        elif ch == "\n":
            a += " "
    chunks = [c for c in a.split(" 1 ") if c.strip()]
    assert chunks, "writeup 算法没切出任何块"
    raw = bytes(int(c, 2) & 0xFF for c in chunks if all(x in "01" for x in c))
    got = raw.decode("utf-8")  # writeup 原样算法 chr() 每字节 → latin-1 → utf-8
    check("writeup 原样算法可还原", got == msg, s)
    r = run_tool({"mode": "hidden", "action": "decode", "scheme": "snowfall",
                  "scope": "trailing", "bit_map": "space0_tab1", "data": body})
    s = out(r)
    check("decode 任务成功", r.get("status") == "success", s)
    check("decode 还原消息", msg in s, s)
    check("统计含切分段数", "切分" in s, s)


def test_hidden_fixed8():
    print("=" * 70)
    print("B. hidden · fixed8：encode → decode 往返")
    msg = "flag{whitespace_8bit}"
    r = run_tool({"mode": "hidden", "action": "encode", "scheme": "fixed8",
                  "bit_map": "space0_tab1", "data": msg})
    s = out(r)
    check("encode 任务成功", r.get("status") == "success", s)
    body = s.split("[结果] 隐藏后的文本（空白不可见，请整段复制）:", 1)[1].split("\n\n[base64]", 1)[0].strip("\n")
    r = run_tool({"mode": "hidden", "action": "decode", "scheme": "fixed8",
                  "scope": "trailing", "bit_map": "space0_tab1", "data": body})
    s = out(r)
    check("decode 还原消息", msg in s, s)


def test_ws_arith_two_tokens():
    print("=" * 70)
    print("C. ws · 算术操作码=两个 token（0+55 → outc '7'）")
    toks = push(0) + push(55) + ADD + OUT_C
    r = run_tool({"mode": "ws", "source": ws_str(toks), "extract": "anywhere", "patch": "none"})
    s = out(r)
    check("任务成功", r.get("status") == "success", s)
    check("输出 '7'", "7" in text_section(s), s)


def test_ws_drop_patch():
    print("=" * 70)
    print("D. ws · drop 补丁打印栈顶（Snowfall flag.txt）")
    toks = push(98) + DROP
    r = run_tool({"mode": "ws", "source": ws_str(toks), "extract": "anywhere", "patch": "none"})
    s = out(r)
    check("无补丁输出为空", "[输出] 0 字节" in s, s)
    r = run_tool({"mode": "ws", "source": ws_str(toks), "extract": "anywhere", "patch": "drop"})
    s = out(r)
    check("drop 补丁输出 b", "b" in text_section(s), s)


def test_ws_label_jump():
    print("=" * 70)
    print("E. ws · mark/jz 标签跳转")
    # push0; jz L; push65 outc（应被跳过）; mark L; push66 outc → "B"
    toks = push(0) + [LF, TAB, SP] + label(0) + push(65) + OUT_C + \
        [LF, SP, SP] + label(0) + push(66) + OUT_C
    r = run_tool({"mode": "ws", "source": ws_str(toks), "extract": "anywhere", "patch": "none"})
    s = out(r)
    check("任务成功", r.get("status") == "success", s)
    check("跳转正确输出 B（非 A）", "B" in text_section(s) and "A" not in text_section(s), s)


def test_ws_extract_trailing():
    print("=" * 70)
    print("F. ws · 从中文文本行尾提取程序（step2 场景）")
    toks = push(72) + OUT_C + push(105) + OUT_C  # "Hi"
    # 程序里的每个 LF token = 文件的一个换行：按 LF 切段，每段藏到一行行尾
    segs, cur = [], []
    for t in toks:
        if t == LF:
            segs.append(cur)
            cur = []
        else:
            cur.append(t)
    segs.append(cur)
    covers = ["雪落无声，万物归于寂静。", "白茫茫一片，覆盖了所有痕迹。", "只有行尾的空白在说话。"]
    text_lines = []
    for k, seg in enumerate(segs):
        ws = "".join(chr(t) for t in seg)
        text_lines.append(covers[k % len(covers)] + ws)
    text = "\n".join(text_lines)
    r = run_tool({"mode": "ws", "source": text, "extract": "trailing", "patch": "none"})
    s = out(r)
    check("任务成功", r.get("status") == "success", s)
    check("输出 Hi", "Hi" in text_section(s), s)


def test_ws_magic_detect():
    print("=" * 70)
    print("G. ws · 输出魔数识别（PK.. → ZIP）")
    toks = []
    for b in b"PK\x03\x04":
        toks += push(b) + OUT_C
    r = run_tool({"mode": "ws", "source": ws_str(toks), "extract": "anywhere", "patch": "none"})
    s = out(r)
    check("任务成功", r.get("status") == "success", s)
    check("识别出 ZIP", "[识别] ZIP 压缩包" in s, s)
    check("给出 base64", "[base64]" in s, s)


def _encode_body(s: str) -> str:
    """从 encode 输出中取出隐藏后文本本体（保留末尾换行）。"""
    body = s.split("[结果] 隐藏后的文本（空白不可见，请整段复制）:", 1)[1].split("\n\n[可视化]", 1)[0]
    return body[1:] if body.startswith("\n") else body


def test_hidden_leading():
    print("=" * 70)
    print("H. hidden · snowfall：空白放行首（leading）encode → decode 往返")
    msg = "snow_leading_位置无关_通用"
    carrier = "\n".join([
        "雪山崩塌的时候，没有一片雪花是无辜的。",
        "雪花落在掌心，转瞬即逝，像代码里的空白字符。",
    ])
    r = run_tool({
        "mode": "hidden", "action": "encode", "scheme": "snowfall",
        "placement": "leading", "bit_map": "space0_tab1",
        "data": msg, "carrier": carrier,
    })
    s = out(r)
    check("encode 任务成功", r.get("status") == "success", s)
    check("统计标注放置位置=行首", "放置位置：行首" in s, s)
    r = run_tool({"mode": "hidden", "action": "decode", "scheme": "snowfall",
                  "scope": "leading", "bit_map": "space0_tab1", "data": _encode_body(s)})
    s = out(r)
    check("decode 任务成功", r.get("status") == "success", s)
    check("scope=leading 还原消息", msg in s, s)


def test_hidden_standalone():
    print("=" * 70)
    print("I. hidden · snowfall：纯空白流（standalone）encode → decode（anywhere）往返")
    msg = "standalone_pure_ws"
    r = run_tool({
        "mode": "hidden", "action": "encode", "scheme": "snowfall",
        "placement": "standalone", "bit_map": "space0_tab1", "data": msg,
    })
    s = out(r)
    check("encode 任务成功", r.get("status") == "success", s)
    check("统计标注纯空白流", "纯空白流" in s, s)
    r = run_tool({"mode": "hidden", "action": "decode", "scheme": "snowfall",
                  "scope": "anywhere", "bit_map": "space0_tab1", "data": _encode_body(s)})
    s = out(r)
    check("decode 任务成功", r.get("status") == "success", s)
    check("scope=anywhere 还原消息", msg in s, s)


def test_ws_extract_leading():
    print("=" * 70)
    print("J. ws · 空白在行首：leading 提取（空白不一定在行尾）")
    toks = push(72) + OUT_C + push(105) + OUT_C  # "Hi"
    # 程序里的每个 LF token = 文件的一个换行：按 LF 切段，每段藏到一行行首
    segs, cur = [], []
    for t in toks:
        if t == LF:
            segs.append(cur)
            cur = []
        else:
            cur.append(t)
    segs.append(cur)
    covers = ["雪落无声，万物归于寂静。", "白茫茫一片，覆盖了所有痕迹。", "只有行首的空白在说话。"]
    text_lines = []
    for k, seg in enumerate(segs):
        ws = "".join(chr(t) for t in seg)
        text_lines.append(ws + covers[k % len(covers)])
    text = "\n".join(text_lines)
    r = run_tool({"mode": "ws", "source": text, "extract": "leading", "patch": "none"})
    s = out(r)
    check("任务成功", r.get("status") == "success", s)
    check("输出 Hi", "Hi" in text_section(s), s)


def test_hidden_auto():
    print("=" * 70)
    print("K. hidden · decode scope=auto：不知道空白藏哪，自动试三种")
    # 用 leading 编码的文本，auto 应能还原，且并列展示三种范围结果
    msg = "auto_detect_me"
    r = run_tool({
        "mode": "hidden", "action": "encode", "scheme": "snowfall",
        "placement": "leading", "bit_map": "space0_tab1", "data": msg,
        "carrier": "雪落无声，万物归于寂静。",
    })
    s = out(r)
    check("encode 任务成功", r.get("status") == "success", s)
    r = run_tool({"mode": "hidden", "action": "decode", "scheme": "snowfall",
                  "scope": "auto", "bit_map": "space0_tab1", "data": _encode_body(s)})
    s = out(r)
    check("decode 任务成功", r.get("status") == "success", s)
    check("auto 还原消息", msg in s, s)
    check("auto 并列展示三种范围", "—— 范围 trailing ——" in s and "—— 范围 leading ——" in s
          and "—— 范围 anywhere ——" in s, s)


if __name__ == "__main__":
    test_hidden_snowfall()
    test_hidden_fixed8()
    test_ws_arith_two_tokens()
    test_ws_drop_patch()
    test_ws_label_jump()
    test_ws_extract_trailing()
    test_ws_magic_detect()
    test_hidden_leading()
    test_hidden_standalone()
    test_ws_extract_leading()
    test_hidden_auto()
    print("\n✅ 空白编码器验证全部通过：snowfall/fixed8 隐写往返、Whitespace 解释器"
          "（算术双 token、drop 补丁、标签跳转、行尾/行首提取、魔数识别）"
          "+ 通用化位置（行首/纯空白流/auto 自动探测）")
