# -*- coding: utf-8 -*-
"""空白编码器：空白隐写（2026 Bugku Snowfall 方案）+ Whitespace 语言解释器。

两种模式：
1. hidden —— 隐藏 / 提取文本（空白不限于行尾，可放行首或全文任意位置）：
   - snowfall 方案（Snowfall step1.txt）：每个字节写 8 位（空格=0、Tab=1），
     紧接一行只含一个「1 位」的标记行；解码按 0-1-0（" 1 "）切分还原字符。
   - fixed8 方案：每 8 个空白位一个字节，无标记行。
   - placement（encode）：trailing=每行行尾 / leading=每行行首 / standalone=纯空白流；
     scope（decode）：trailing=行尾 / leading=行首 / anywhere=全文任意位置 /
     auto=三种全试（不知道空白藏哪时用）。
2. ws —— Whitespace esolang（IMP=空格/Tab/换行）完整解释器（栈/算术/堆/标签跳转/IO），
   支持从行尾 / 行首 / 全文任意位置提取空白当程序（Snowfall step2.txt / flag.txt），
   支持 patch=drop/swap/trace：把被 drop/swap 丢弃的栈值打印出来（Snowfall flag.txt 技巧）。

参考：2026 Bugku Snowfall（MISC）：step1 空白藏 key → step2 空白是 Whitespace
程序并输出加密 7z → key 解 7z 得 flag.txt（Whitespace 程序）→ drop 补丁打印栈顶得 flag。

Whitespace 指令格式要点（写解释器最大的坑）：算术与 IO 指令在 IMP 之后的
操作码是【两个】token；少消费一个 token 会把它当成下一条指令的 IMP，
静默吞掉后续指令 —— 表现为输出字节缺失（如 7z 魔数缺 0x37）。
"""
from __future__ import annotations

import base64

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult

WS_SPACE, WS_TAB, WS_LF = 32, 9, 10

MAX_STEPS = 5_000_000
HEX_LIMIT = 4096
B64_LIMIT = 256 * 1024
TEXT_LIMIT = 4096

MAGICS: list[tuple[str, bytes]] = [
    ("7z 压缩包", b"7z\xbc\xaf\x27\x1c"),
    ("ZIP 压缩包", b"PK\x03\x04"),
    ("ZIP 空包/结尾", b"PK\x05\x06"),
    ("RAR 压缩包", b"Rar!"),
    ("gzip 压缩", b"\x1f\x8b"),
    ("bzip2 压缩", b"BZh"),
    ("PNG 图片", b"\x89PNG\r\n\x1a\n"),
    ("JPEG 图片", b"\xff\xd8\xff"),
    ("GIF 图片", b"GIF8"),
    ("PDF 文档", b"%PDF"),
    ("ELF 可执行", b"\x7fELF"),
    ("SQLite 数据库", b"SQLite format 3"),
    ("WAV 音频", b"RIFF"),
]


# ---------------------------------------------------------------------------
# Whitespace 解释器
# ---------------------------------------------------------------------------

def _parse_num(tokens: list[int], i: int) -> tuple[int, int]:
    """解析数字：sign（空格=+ / Tab=-）+ 二进制位（空格=0 / Tab=1）+ LF 结尾。"""
    sign = 1 if tokens[i] == WS_SPACE else -1
    i += 1
    n = 0
    while i < len(tokens) and tokens[i] != WS_LF:
        n = (n << 1) | (1 if tokens[i] == WS_TAB else 0)
        i += 1
    return sign * n, i + 1  # 跳过结尾 LF


def _scan_labels(tokens: list[int]) -> dict[int, int]:
    """第一遍：按指令边界完整走一遍，记录所有 mark 标签位置。"""
    labels: dict[int, int] = {}
    i = 0
    n = len(tokens)
    while i < n:
        imp = tokens[i]
        i += 1
        if imp == WS_SPACE:                      # 栈操作
            if i >= n:
                break
            c = tokens[i]
            i += 1
            if c == WS_SPACE:                    # push n
                _, i = _parse_num(tokens, i)
            elif c == WS_TAB:                    # copy n
                _, i = _parse_num(tokens, i)
            elif c == WS_LF:
                if i < n and tokens[i] == WS_TAB:  # slide n（dup/drop 无数字）
                    _, i = _parse_num(tokens, i)
                else:
                    i += 1
        elif imp == WS_TAB:                      # 算术/堆/IO
            if i >= n:
                break
            c = tokens[i]
            i += 1
            if c in (WS_SPACE, WS_LF):           # 算术、IO：操作码 = 两个 token
                i += 2
            else:                                # 堆：操作码 = 一个 token
                i += 1
        else:                                    # 流程控制
            if i >= n:
                break
            c = tokens[i]
            i += 1
            if c == WS_LF:                       # END
                break
            if c == WS_SPACE:                    # mark / call / jump
                if i >= n:
                    break
                c2 = tokens[i]
                i += 1
                lbl, i = _parse_num(tokens, i)
                if c2 == WS_SPACE:               # mark
                    labels[lbl] = i
            else:                                # jz / jn / ret
                if i < n and tokens[i] != WS_LF:
                    _, i = _parse_num(tokens, i)  # jz/jn 带标签；ret 无
                else:
                    i += 1
    return labels


def run_ws(tokens: list[int], patch: str = "none") -> bytes:
    """执行 Whitespace 程序，返回输出字节。

    patch：none 原样执行；drop=drop 弹栈前打印栈顶；swap=swap 时打印两值；
    trace=drop+swap 都打印（把「被丢弃/换位的栈值」可视化，Snowfall flag.txt 场景）。
    """
    labels = _scan_labels(tokens)
    stack: list[int] = []
    heap: dict[int, int] = {}
    call: list[int] = []
    out = bytearray()

    def emit(vals: list[int]) -> None:
        for v in vals:
            out.append(v & 0xFF)

    i = 0
    n = len(tokens)
    steps = 0
    while i < n:
        steps += 1
        if steps > MAX_STEPS:
            out.extend(f"\n[中止] 超过最大执行步数 {MAX_STEPS}".encode())
            break
        imp = tokens[i]
        i += 1
        if imp == WS_SPACE:                      # 栈操作
            c = tokens[i]
            i += 1
            if c == WS_SPACE:                    # push n
                v, i = _parse_num(tokens, i)
                stack.append(v)
            elif c == WS_LF:
                c2 = tokens[i]
                i += 1
                if c2 == WS_SPACE:               # dup
                    if stack:
                        stack.append(stack[-1])
                elif c2 == WS_TAB:               # swap
                    if len(stack) >= 2:
                        if patch in ("swap", "trace"):
                            emit([stack[-1], stack[-2]])
                        stack[-1], stack[-2] = stack[-2], stack[-1]
                else:                            # drop —— 补丁：弹栈前打印栈顶
                    if stack:
                        if patch in ("drop", "trace"):
                            emit([stack[-1]])
                        stack.pop()
            else:                                # copy n
                v, i = _parse_num(tokens, i)
                if 0 <= v < len(stack):
                    stack.append(stack[-1 - v])
        elif imp == WS_TAB:
            c = tokens[i]
            i += 1
            if c == WS_SPACE:                    # 算术：操作码 = 两个 token
                c2, c3 = tokens[i], tokens[i + 1]
                i += 2
                if len(stack) >= 2:
                    b, a = stack.pop(), stack.pop()
                    if c2 == WS_SPACE and c3 == WS_SPACE:
                        stack.append(a + b)
                    elif c2 == WS_SPACE and c3 == WS_TAB:
                        stack.append(a - b)
                    elif c2 == WS_SPACE and c3 == WS_LF:
                        stack.append(a * b)
                    elif c2 == WS_TAB and c3 == WS_SPACE:
                        stack.append(a // b if b else 0)
                    else:
                        stack.append(a % b if b else 0)
            elif c == WS_TAB:                    # 堆：操作码 = 一个 token
                c2 = tokens[i]
                i += 1
                if c2 == WS_SPACE:               # store：pop v, pop addr
                    if len(stack) >= 2:
                        v, addr = stack.pop(), stack.pop()
                        heap[addr] = v
                else:                            # retrieve
                    if stack:
                        addr = stack.pop()
                        stack.append(heap.get(addr, 0))
            else:                                # IO：操作码 = 两个 token
                c2, c3 = tokens[i], tokens[i + 1]
                i += 2
                if c2 == WS_SPACE and c3 == WS_SPACE:      # outc
                    if stack:
                        emit([stack.pop()])
                elif c2 == WS_SPACE and c3 == WS_TAB:      # outn
                    if stack:
                        out.extend(str(stack.pop()).encode())
                else:                                      # readc/readn 不读输入
                    if stack:
                        stack.pop()
        else:                                    # 流程控制
            if i >= n:
                break
            c = tokens[i]
            i += 1
            if c == WS_LF:                       # END
                break
            if c == WS_SPACE:                    # mark / call / jump（都带标签）
                if i >= n:
                    break
                c2 = tokens[i]
                i += 1
                lbl, i = _parse_num(tokens, i)
                if c2 == WS_TAB:                 # call
                    call.append(i)
                    i = labels.get(lbl, i)
                elif c2 == WS_LF:                # jump
                    i = labels.get(lbl, i)
                # mark：执行期 no-op
            else:                                # jz / jn / ret
                if i >= n:
                    break
                c2 = tokens[i]
                i += 1
                if c2 == WS_SPACE:               # jz
                    lbl, i = _parse_num(tokens, i)
                    v = stack.pop() if stack else 0
                    if v == 0:
                        i = labels.get(lbl, i)
                elif c2 == WS_TAB:               # jn
                    lbl, i = _parse_num(tokens, i)
                    v = stack.pop() if stack else 0
                    if v < 0:
                        i = labels.get(lbl, i)
                else:                            # ret
                    if call:
                        i = call.pop()
    return bytes(out)


# ---------------------------------------------------------------------------
# 空白提取 / 隐写
# ---------------------------------------------------------------------------

def _extract_ws(text: str, where: str) -> list[int]:
    """按位置提取空白 token：trailing=每行行尾 / leading=每行行首，换行补 LF。

    （空白不一定在行尾：行首用 leading，任意位置用 anywhere=全文过滤。）
    """
    if where == "anywhere":
        return filter_ws(text)
    toks: list[int] = []
    for ln in text.split("\n"):
        ln = ln.rstrip("\r")
        if where == "trailing":
            ws = ln[len(ln.rstrip(" \t")):]
        else:
            ws = ln[:len(ln) - len(ln.lstrip(" \t"))]
        for ch in ws:
            toks.append(WS_SPACE if ch == " " else WS_TAB)
        toks.append(WS_LF)
    while toks and toks[-1] == WS_LF:
        toks.pop()
    return toks


def filter_ws(text: str) -> list[int]:
    """全文只保留空格/Tab/换行（纯 Whitespace 源码）。"""
    toks: list[int] = []
    for ch in text.replace("\r", ""):
        if ch == " ":
            toks.append(WS_SPACE)
        elif ch == "\t":
            toks.append(WS_TAB)
        elif ch == "\n":
            toks.append(WS_LF)
    return toks


def _bit_chars(bit_map: str) -> tuple[str, str]:
    """返回 (0 位字符, 1 位字符)。"""
    return (" ", "\t") if bit_map == "space0_tab1" else ("\t", " ")


def _stream_of(text: str, scope: str, bit_map: str) -> str:
    """把文本转成 '0'/'1'/' ' 位流（' ' 来自换行，作分隔用；Snowfall 原样算法）。

    scope：trailing=每行行尾 / leading=每行行首 / anywhere=全文任意位置的空格与 Tab。
    """
    z, o = _bit_chars(bit_map)
    s = ""
    if scope == "anywhere":
        for ch in text.replace("\r", ""):
            if ch == z:
                s += "0"
            elif ch == o:
                s += "1"
            elif ch == "\n":
                s += " "
    else:
        for ln in text.split("\n"):
            ln = ln.rstrip("\r")
            if scope == "trailing":
                ws = ln[len(ln.rstrip(" \t")):]
            else:
                ws = ln[:len(ln) - len(ln.lstrip(" \t"))]
            for ch in ws:
                s += "0" if ch == z else "1"
            s += " "
    return s


def encode_hidden(message: str, carrier: str, scheme: str, bit_map: str,
                  placement: str = "trailing") -> tuple[str, str]:
    """把消息藏进空白。返回 (结果文本, 统计行)。

    placement：trailing=每行行尾（Snowfall 原样）/ leading=每行行首 /
    standalone=纯空白流（忽略掩护文本）。
    """
    z, o = _bit_chars(bit_map)
    data = message.encode("utf-8")
    use_carrier = placement in ("trailing", "leading")
    lines = carrier.split("\n") if (carrier.strip() and use_carrier) else []
    out_lines: list[str] = []
    idx = 0

    def next_line() -> str:
        nonlocal idx
        if idx < len(lines):
            ln = lines[idx].rstrip("\r")
        else:
            ln = ""
        idx += 1
        return ln

    for b in data:
        bits = "".join(z if c == "0" else o for c in f"{b:08b}")
        if scheme == "snowfall":
            if placement == "leading":
                out_lines.append(bits + next_line())
                out_lines.append(o + next_line())   # 标记行：单个「1 位」
            elif placement == "standalone":
                out_lines.append(bits)
                out_lines.append(o)                 # 标记行：单个「1 位」
            else:
                out_lines.append(next_line() + bits)
                out_lines.append(next_line() + o)   # 标记行：单个「1 位」
        else:  # fixed8
            if placement == "leading":
                out_lines.append(bits + next_line())
            elif placement == "standalone":
                out_lines.append(bits)
            else:
                out_lines.append(next_line() + bits)
    # 以换行结尾：保证最后一个字节的 0-1-0 分隔完整（writeup 原样算法依赖末尾换行）
    text = "\n".join(out_lines) + "\n"
    place = {"trailing": "行尾", "leading": "行首", "standalone": "纯空白流（无掩护文本）"}[placement]
    stats = (f"[统计] 消息 {len(data)} 字节（utf-8），每字节 "
             f"{'2 行（8 位行 + 标记行）' if scheme == 'snowfall' else '1 行（8 位）'}，"
             f"共占用 {len(out_lines)} 行（末尾补换行保证最后字节分隔完整）；"
             f"放置位置：{place}；"
             f"位映射：{'空格=0 Tab=1' if bit_map == 'space0_tab1' else '空格=1 Tab=0'}")
    return text, stats


def decode_hidden(text: str, scope: str, scheme: str, bit_map: str) -> tuple[bytes, str]:
    """从行尾空白提取隐藏文本。返回 (字节, 统计行)。"""
    stream = _stream_of(text, scope, bit_map)
    # 分隔符 = 换行(位流里的空格) + 标记位(1) + 换行(空格)，即 writeup 的 split(" 1 ")
    delim = " " + "1" + " "
    bits = stream.replace(" ", "")
    out = bytearray()
    n_chunks = 0
    if scheme == "snowfall":
        for chunk in stream.split(delim):
            chunk = chunk.strip()
            if not chunk:
                continue
            try:
                out.append(int(chunk, 2) & 0xFF)
                n_chunks += 1
            except ValueError:
                pass
    else:  # fixed8
        for k in range(0, len(bits) - 7, 8):
            try:
                out.append(int(bits[k:k + 8], 2))
                n_chunks += 1
            except ValueError:
                break
    stats = (f"[提取] 位流 {len(stream)} 位（含换行分隔），纯数据位 {len(bits)}；"
             f"{'按 0-1-0 切分' if scheme == 'snowfall' else '每 8 位一组'}得 {n_chunks} 字节")
    return bytes(out), stats


# ---------------------------------------------------------------------------
# 输出辅助
# ---------------------------------------------------------------------------

def _hexdump(b: bytes, limit: int = HEX_LIMIT) -> str:
    body = b[:limit]
    lines = [body[k:k + 32].hex() for k in range(0, len(body), 32)]
    if len(b) > limit:
        lines.append(f"…（截断，共 {len(b)} 字节）")
    return "\n".join(lines) if lines else "（空）"


def _text_preview(b: bytes, limit: int = TEXT_LIMIT) -> str:
    body = b[:limit]
    try:
        s = body.decode("utf-8")
        enc = "utf-8"
    except UnicodeDecodeError:
        s = body.decode("latin-1")
        enc = "latin-1"
    vis = "".join(ch if ch.isprintable() or ch in "\n\t" else f"\\x{ord(ch):02x}" for ch in s)
    if len(b) > limit:
        vis += f"\n…（截断，共 {len(b)} 字节）"
    return f"[文本] （{enc}，不可见字符已转义）:\n{vis}" if vis else "[文本] （空）"


def _detect(b: bytes) -> list[str]:
    hits = [name for name, magic in MAGICS if b.startswith(magic)]
    return hits


def _b64(b: bytes) -> str:
    if len(b) <= B64_LIMIT:
        return base64.b64encode(b).decode()
    return base64.b64encode(b[:B64_LIMIT]).decode() + f"…（截断，共 {len(b)} 字节）"


# ---------------------------------------------------------------------------
# 工具
# ---------------------------------------------------------------------------

@register_tool
class WhitespaceCodecTool(Tool):
    name = "whitespace_codec"
    category = Category.MISC
    version = "1.1.0"
    timeout = 60
    description = ("空白隐写编解码（Snowfall 方案；空白可放行尾/行首/全文任意位置，"
                   "解码支持 auto 自动探测位置）+ Whitespace 语言解释器"
                   "（drop/swap 补丁打印被丢弃栈值，输出含魔数识别/base64）")
    params = [
        ParamSpec("mode", type="str", required=False, default="hidden",
                  description="hidden=空白隐写 / ws=Whitespace 解释器"),
        # —— hidden ——
        ParamSpec("action", type="str", required=False, default="decode",
                  description="decode=提取隐藏文本 / encode=隐藏消息"),
        ParamSpec("data", type="text", required=False, default="",
                  description="decode：含隐藏空白的文本；encode：要隐藏的消息"),
        ParamSpec("carrier", type="text", required=False, default="",
                  description="encode 掩护文本（可选，行数不够自动补空行；placement=standalone 时忽略）"),
        ParamSpec("scheme", type="str", required=False, default="snowfall",
                  description="snowfall=8 位+标记行 / fixed8=每 8 位一字符"),
        ParamSpec("scope", type="str", required=False, default="trailing",
                  description="decode 取空白范围：trailing=每行行尾 / leading=每行行首 / "
                              "anywhere=全文任意位置 / auto=三种全试（不知道空白藏哪时用）"),
        ParamSpec("placement", type="str", required=False, default="trailing",
                  description="encode 空白放置位置：trailing=每行行尾 / leading=每行行首 / "
                              "standalone=纯空白流（无掩护文本）"),
        ParamSpec("bit_map", type="str", required=False, default="space0_tab1",
                  description="空格=0 Tab=1 / 空格=1 Tab=0"),
        # —— ws ——
        ParamSpec("source", type="text", required=False, default="",
                  description="Whitespace 程序源码（可直接贴整篇文本，按 extract 方式提取空白）"),
        ParamSpec("extract", type="str", required=False, default="anywhere",
                  description="提取方式：anywhere=全文所有空白（任意位置，通用）/ "
                              "trailing=每行行尾 / leading=每行行首"),
        ParamSpec("patch", type="str", required=False, default="none",
                  description="none / drop（弹栈前打印栈顶）/ swap / trace"),
    ]

    def _run_hidden(self, p: dict) -> str:
        action = p.get("action") or "decode"
        scheme = p.get("scheme") or "snowfall"
        scope = p.get("scope") or "trailing"
        bit_map = p.get("bit_map") or "space0_tab1"
        data = p.get("data") or ""
        if scheme not in ("snowfall", "fixed8"):
            raise ValueError("scheme 必须是 snowfall / fixed8")
        if bit_map not in ("space0_tab1", "space1_tab0"):
            raise ValueError("bit_map 必须是 space0_tab1 / space1_tab0")
        if scope not in ("trailing", "leading", "anywhere", "auto"):
            raise ValueError("scope 必须是 trailing / leading / anywhere / auto")
        bm = "空格=0 Tab=1" if bit_map == "space0_tab1" else "空格=1 Tab=0"
        if action == "encode":
            if not data:
                raise ValueError("encode 必须提供要隐藏的消息 data")
            placement = p.get("placement") or "trailing"
            if placement not in ("trailing", "leading", "standalone"):
                raise ValueError("placement 必须是 trailing / leading / standalone")
            text, stats = encode_hidden(data, p.get("carrier") or "", scheme, bit_map, placement)
            lines = [f"[模式] hidden · encode（{scheme}，{placement}）", stats,
                     "", "[结果] 隐藏后的文本（空白不可见，请整段复制）:", text]
            if scheme == "snowfall":
                def vis_line(ln: str) -> str:
                    if placement == "leading":
                        head = ln[:len(ln) - len(ln.lstrip(" \t"))]
                        return head.replace(" ", "␣").replace("\t", "⇥") + ln[len(head):]
                    tail = ln[len(ln.rstrip(" \t")):]
                    return ln[:len(ln) - len(tail)] + tail.replace(" ", "␣").replace("\t", "⇥")
                vis = "\n".join(vis_line(ln) for ln in text.split("\n"))
                lines += ["", "[可视化] ␣=空格 ⇥=Tab（每字节：8 位行 + 1 个 ⇥/␣ 标记行）:", vis]
            lines += ["", f"[base64] （复制到本地还原成文件/文本用）:\n{_b64(text.encode('utf-8'))}"]
            return "\n".join(lines)
        # decode
        if not data:
            raise ValueError("decode 必须提供含隐藏空白的文本 data")
        scopes = ["trailing", "leading", "anywhere"] if scope == "auto" else [scope]
        lines = []
        if scope == "auto":
            lines.append(f"[模式] hidden · decode（{scheme}，auto：依次尝试 trailing / leading / anywhere，{bm}）")
        for sc in scopes:
            out, stats = decode_hidden(data, sc, scheme, bit_map)
            try:
                msg = out.decode("utf-8")
                enc = "utf-8"
            except UnicodeDecodeError:
                msg = out.decode("latin-1")
                enc = "latin-1"
            if len(scopes) == 1:
                lines.append(f"[模式] hidden · decode（{scheme}，{sc}，{bm}）")
                lines.append(stats)
                lines.append(f"[结果] 隐藏文本（{enc}）:\n{msg}")
                if not out:
                    lines.append("[提示] 未解析出字节：可换 scope=auto（自动尝试行尾/行首/任意位置）/ scheme=fixed8 / 位映射重试")
                elif enc == "latin-1":
                    lines.append("[提示] 非合法 utf-8，按 latin-1 展示（可能本身是二进制或换用 fixed8 重试）")
            else:
                lines += ["", f"—— 范围 {sc} ——", stats,
                          f"[结果] 隐藏文本（{enc}）:\n{msg}" if out else "[结果] （空）"]
        return "\n".join(lines)

    def _run_ws(self, p: dict) -> str:
        source = p.get("source") or ""
        if not source:
            raise ValueError("ws 模式必须提供程序源码 source")
        patch = p.get("patch") or "none"
        if patch not in ("none", "drop", "swap", "trace"):
            raise ValueError("patch 必须是 none / drop / swap / trace")
        extract = p.get("extract") or "anywhere"
        if extract not in ("anywhere", "trailing", "leading"):
            raise ValueError("extract 必须是 anywhere / trailing / leading")
        n_lines = source.count(chr(10)) + 1
        toks = _extract_ws(source, extract)
        how = {
            "anywhere": "全文过滤（任意位置的空格/Tab/换行，其余字符忽略）",
            "trailing": f"行尾空白提取，共 {n_lines} 行",
            "leading": f"行首空白提取，共 {n_lines} 行",
        }[extract]
        out = run_ws(toks, patch)
        lines = [f"[模式] ws · Whitespace 解释器", f"[程序] {how} → {len(toks)} 个 token",
                 f"[补丁] {patch}（{'原样执行' if patch == 'none' else patch + '：打印被丢弃/换位的栈值'}）",
                 f"[输出] {len(out)} 字节", "", "[hex]\n" + _hexdump(out), "", _text_preview(out)]
        hits = _detect(out)
        if hits:
            lines += ["", "[识别] " + "、".join(hits)]
            if "7z 压缩包" in hits:
                lines.append("[提示] 若 7z 带密码：密码可能藏在别处（如行尾空白隐藏文本）；"
                             "本地可用 py7zr / 7-Zip 解开后再喂回本工具")
        lines += ["", f"[base64] （复制后用 Base64 工具还原二进制）:\n{_b64(out)}"]
        return "\n".join(lines)

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        try:
            mode = params.get("mode") or "hidden"
            if mode == "hidden":
                out = self._run_hidden(params)
            elif mode == "ws":
                out = self._run_ws(params)
            else:
                raise ValueError(f"未知模式: {mode}（hidden / ws）")
            return ToolResult(status=Status.SUCCESS, stdout=out)
        except ValueError as e:
            return ToolResult(status=Status.ERROR, stdout=f"[中止] {e}")
