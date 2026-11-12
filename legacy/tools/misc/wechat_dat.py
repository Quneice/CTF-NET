"""微信 WeChat FileStorage 加密 .dat 文件还原工具。

微信（PC 版）把聊天中的图片/文件缓存加密后另存为 .dat：整份数据与单个
密钥字节循环异或。密钥可借已知文件头（魔数）逆推：data[i] ^ key == magic[i]。
本工具自动探测常见文件头确定密钥与类型，异或解密后落盘为可下载产物，
也可手动指定已知密钥直接解密（数据恢复场景）。
"""
from __future__ import annotations

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult

# 常见文件头魔数 -> 扩展名（可靠度排序：图片优先）
MAGICS: list[tuple[bytes, str]] = [
    (b"\x89PNG\r\n\x1a\n", "png"),
    (b"\xff\xd8\xff", "jpg"),
    (b"GIF87a", "gif"),
    (b"GIF89a", "gif"),
    (b"BM", "bmp"),
    (b"PK\x03\x04", "zip"),
    (b"Rar!\x1a\x07", "rar"),
    (b"RIFF", "webp"),
]


def _detect_key(data: bytes) -> tuple[int, str] | None:
    """用常见文件头逆推单字节异或密钥，返回 (key, ext) 或 None。"""
    for magic, ext in MAGICS:
        if len(data) < len(magic):
            continue
        key = data[0] ^ magic[0]
        if all(data[i] ^ key == magic[i] for i in range(len(magic))):
            return key, ext
    return None


def _detect_format(data: bytes) -> str:
    """解密后根据文件头识别真实类型，未知返回 bin。"""
    for magic, ext in MAGICS:
        if data.startswith(magic):
            return ext
    return "bin"


@register_tool
class WechatDatTool(Tool):
    name = "wechat_dat"
    category = Category.MISC
    version = "1.0.0"
    description = "还原微信 FileStorage 加密 .dat 缓存（单字节循环异或，文件头逆推密钥）"
    params = [
        ParamSpec("file", type="file", required=True, description="上传的 .dat 文件 file_id（见 POST /api/files）"),
        ParamSpec("key", type="str", required=False, default="",
                  description="可选：手动指定异或密钥（十六进制，如 9E）；留空则按文件头自动探测"),
    ]

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        filepath = ctx.workdir / params["file"]
        try:
            data = filepath.read_bytes()
        except OSError as exc:
            return ToolResult(status=Status.ERROR, stderr=f"读取文件失败: {exc}")

        if not data:
            return ToolResult(status=Status.ERROR, stderr="文件为空")

        manual = (params.get("key") or "").strip()
        if manual:
            try:
                key = int(manual, 16)
            except ValueError:
                return ToolResult(status=Status.ERROR, stderr=f"非法密钥（应为十六进制）: {manual}")
            if not 0 <= key <= 255:
                return ToolResult(status=Status.ERROR, stderr="密钥需为单字节（0x00~0xFF）")
        else:
            hit = _detect_key(data)
            if hit is None:
                return ToolResult(
                    status=Status.ERROR,
                    stderr="未匹配到常见文件头，无法自动推断密钥。请提供 key 参数手动指定异或密钥。",
                )
            key, _ = hit

        out = bytes(b ^ key for b in data)
        ext = _detect_format(out)
        out_name = f"{filepath.stem}_decrypted.{ext}"
        (ctx.artifacts_dir / out_name).write_bytes(out)

        return ToolResult(
            status=Status.SUCCESS,
            stdout=f"[+] 密钥=0x{key:02X}  类型=*.{ext}  大小={len(out)}B -> {out_name}\n",
            data={"key_hex": f"{key:02X}", "format": ext, "size": len(out)},
        )
