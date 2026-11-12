# -*- coding: utf-8 -*-
"""原神·提瓦特通用文字（Teyvat script）识别 / 解码工具。

提瓦特文字是《原神》的一套字形系统：26 个符文与 A-Z 一一对应，本质是
单表替换密码（哥特体风格，把大写拉丁字母上下颠倒再改笔画）。本工具通过
「字体模板匹配」实现识别：用提瓦特字体渲染 A-Z 模板，与图片中的字形
逐像素比对（MAD），切行 / 切字后还原出英文句子。

依赖字体：Teyvat 字体（如 2021 年 Calligraphr 社区版 Teyvat-Regular.ttf）。
字体版本会影响个别字形（K/R、H/X、S/Z、G/C、I/T/V 等镜像/倒置字形易混淆），
请使用与题目同期的字体。未提供字体时回退到 PIL 内置位图字体（仅能识别常规
A-Z，用于验证管线或通用字体文本）。
"""
from __future__ import annotations

from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageFont

from app.core.registry import register_tool
from app.core.tool import Category, ParamSpec, Status, Tool, ToolContext, ToolResult

DEFAULT_CHARSET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
DEFAULT_SIZES = (28, 32, 36, 40, 48)
NORM_HEIGHT = 32
BUNDLED_FONT = Path(__file__).resolve().parent / "fonts" / "Teyvat-Regular.ttf"


# ---------------------------------------------------------------------------
# 图像二值化
# ---------------------------------------------------------------------------

def _otsu_threshold(arr: np.ndarray) -> int:
    hist, _ = np.histogram(arr.ravel(), bins=256, range=(0, 256))
    total = arr.size
    sum_all = int(np.dot(np.arange(256), hist))
    sum_b = 0.0
    w_b = 0
    max_var = 0.0
    thr = 127
    for t in range(256):
        w_b += int(hist[t])
        if w_b == 0:
            continue
        w_f = total - w_b
        if w_f == 0:
            break
        sum_b += t * int(hist[t])
        m_b = sum_b / w_b
        m_f = (sum_all - sum_b) / w_f
        var = w_b * w_f * (m_b - m_f) ** 2
        if var > max_var:
            max_var = var
            thr = t
    return thr


def _to_ink(path: Path) -> np.ndarray:
    """读图转灰度，Otsu 二值化，返回 True=墨迹 的布尔矩阵。"""
    im = Image.open(path).convert("L")
    arr = np.asarray(im)
    thr = _otsu_threshold(arr)
    return arr < thr


# ---------------------------------------------------------------------------
# 行 / 字形切分
# ---------------------------------------------------------------------------

def _line_bands(ink: np.ndarray) -> list[tuple[int, int]]:
    """按行投影找文本带，返回 [(y0, y1), ...]（y1 开区间）。"""
    rows = ink.any(axis=1)
    bands: list[tuple[int, int]] = []
    in_band = False
    y0 = 0
    for y in range(len(rows)):
        if rows[y] and not in_band:
            y0 = y
            in_band = True
        elif not rows[y] and in_band:
            bands.append((y0, y))
            in_band = False
    if in_band:
        bands.append((y0, len(rows)))
    return bands


def _glyph_runs(line_ink: np.ndarray) -> list[tuple[int, int]]:
    """行内按列投影找字形块，返回 [(x0, x1), ...]（x1 开区间）。

    只把「足够宽的全空白列带」当作分隔，忽略字形内部的窄缝——黑体/哥特体
    风格字形（如提瓦特文字）常带断裂笔画，纯零列窄缝会误把单个字拆成多块。
    """
    h = line_ink.shape[0]
    col_ink = line_ink.sum(axis=0)
    min_sep = max(2, int(0.10 * h))
    n = len(col_ink)
    sep = np.zeros(n, dtype=bool)
    x = 0
    while x < n:
        if col_ink[x] == 0:
            z = x
            while z < n and col_ink[z] == 0:
                z += 1
            if z - x >= min_sep:
                sep[x:z] = True
            x = z
        else:
            x += 1
    runs: list[tuple[int, int]] = []
    x = 0
    while x < n:
        if sep[x]:
            x += 1
            continue
        x0 = x
        while x < n and not sep[x]:
            x += 1
        runs.append((x0, x))
    return runs


# ---------------------------------------------------------------------------
# 模板生成与匹配
# ---------------------------------------------------------------------------

def _tight_crop(arr: np.ndarray) -> np.ndarray:
    ys, xs = np.where(arr)
    if len(xs) == 0:
        return arr
    return arr[ys.min():ys.max() + 1, xs.min():xs.max() + 1]


def _normalize(arr: np.ndarray, height: int = NORM_HEIGHT) -> np.ndarray:
    h, w = arr.shape
    nw = max(1, int(round(w * height / h)))
    im = Image.fromarray((~arr).astype(np.uint8) * 255)
    im = im.resize((nw, height), Image.Resampling.BILINEAR)
    return np.asarray(im) < 128


def _render_templates(font, charset: str, sizes) -> dict[str, list[np.ndarray]]:
    """渲染每个字符的多字号二值模板，归一化到统一高度。"""
    templates: dict[str, list[np.ndarray]] = {ch: [] for ch in charset}
    for size in sizes:
        f = ImageFont.truetype(str(font), size) if font else ImageFont.load_default()
        for ch in charset:
            bbox = f.getbbox(ch)
            w = max(1, bbox[2] - bbox[0])
            h = max(1, bbox[3] - bbox[1])
            pad = 4
            img = Image.new("L", (w + pad * 2, h + pad * 2), 255)
            d = ImageDraw.Draw(img)
            d.text((pad - bbox[0], pad - bbox[1]), ch, font=f, fill=0)
            arr = _tight_crop(np.asarray(img) < 128)
            templates[ch].append(_normalize(arr))
    return templates


def _mad(a: np.ndarray, b: np.ndarray) -> float:
    """中心对齐后的平均绝对像素差（两矩阵等高）。"""
    h = a.shape[0]
    w = max(a.shape[1], b.shape[1])
    ca = np.zeros((h, w), dtype=bool)
    cb = np.zeros((h, w), dtype=bool)
    xa = (w - a.shape[1]) // 2
    xb = (w - b.shape[1]) // 2
    ca[:, xa:xa + a.shape[1]] = a
    cb[:, xb:xb + b.shape[1]] = b
    return float(np.mean(ca != cb))


def _match(glyph: np.ndarray, templates: dict[str, list[np.ndarray]]) -> tuple[str, float]:
    a = _normalize(glyph)
    best_ch = "?"
    best_score = 1.0
    for ch, arrs in templates.items():
        for t in arrs:
            s = _mad(a, t)
            if s < best_score:
                best_score = s
                best_ch = ch
    return best_ch, best_score


# ---------------------------------------------------------------------------
# 主识别流程
# ---------------------------------------------------------------------------

def recognize(
    image_path: Path,
    font_path: Path | None,
    charset: str,
    threshold: float,
    sizes=DEFAULT_SIZES,
) -> tuple[str, list[tuple[str, float]]]:
    """识别图片中的文字，返回 (拼接文本, 逐字形(字符,置信度)列表)。"""
    ink = _to_ink(image_path)
    font = font_path if (font_path and font_path.exists()) else None
    templates = _render_templates(font, charset, sizes)

    lines: list[str] = []
    confidences: list[tuple[str, float]] = []
    for y0, y1 in _line_bands(ink):
        line_ink = ink[y0:y1, :]
        runs = _glyph_runs(line_ink)
        if not runs:
            continue
        gaps = [runs[i][0] - runs[i - 1][1] for i in range(1, len(runs))]
        median_gap = float(np.median(gaps)) if gaps else 0.0
        chars: list[str] = []
        for i, (x0, x1) in enumerate(runs):
            if i > 0:
                gap = runs[i][0] - runs[i - 1][1]
                if gap > max(4, 2.0 * median_gap):
                    chars.append(" ")
            glyph = _tight_crop(line_ink[:, x0:x1])
            ch, score = _match(glyph, templates)
            chars.append(ch if score < threshold else "?")
            confidences.append((ch, score))
        lines.append("".join(chars))

    return "\n".join(lines), confidences


# ---------------------------------------------------------------------------
# 工具注册
# ---------------------------------------------------------------------------

@register_tool
class TeyvatTool(Tool):
    name = "teyvat"
    category = Category.MISC
    version = "1.0.0"
    description = "识别/解码原神提瓦特文字（Teyvat script）图片：字体模板匹配 OCR，切行切字还原英文"
    params = [
        ParamSpec("image", type="file", required=True, description="提瓦特文字图片（PNG/JPG），file_id 见 POST /api/files"),
        ParamSpec("font_file", type="file", required=False, default="",
                  description="可选：Teyvat 字体（.ttf/.otf）。留空回退 PIL 内置字体（仅能识别常规 A-Z）"),
        ParamSpec("charset", type="str", required=False, default=DEFAULT_CHARSET,
                  description="参与匹配的字符集，默认 A-Z 与数字"),
        ParamSpec("threshold", type="float", required=False, default=0.30,
                  description="MAD 匹配阈值，低于该值才接受为对应字符（0~1，越小越严格）"),
    ]

    async def run(self, params: dict, ctx: ToolContext) -> ToolResult:
        image_path = ctx.workdir / params["image"]
        if not image_path.exists():
            return ToolResult(status=Status.ERROR, stderr=f"图片不存在: {params['image']}")

        font_path: Path | None = None
        font_arg = (params.get("font_file") or "").strip()
        if font_arg:
            font_path = ctx.workdir / font_arg
            if not font_path.exists():
                return ToolResult(status=Status.ERROR, stderr=f"字体不存在: {font_arg}")
        elif BUNDLED_FONT.exists():
            font_path = BUNDLED_FONT

        try:
            threshold = float(params.get("threshold", 0.30))
        except (TypeError, ValueError):
            return ToolResult(status=Status.ERROR, stderr=f"非法阈值: {params.get('threshold')!r}")
        if not 0.0 <= threshold <= 1.0:
            return ToolResult(status=Status.ERROR, stderr="阈值需在 0~1 之间")

        charset = params.get("charset", DEFAULT_CHARSET) or DEFAULT_CHARSET

        try:
            text, confidences = recognize(image_path, font_path, charset, threshold)
        except Exception as exc:  # noqa: BLE001
            return ToolResult(status=Status.ERROR, stderr=f"识别失败: {type(exc).__name__}: {exc}")

        if not text.strip():
            return ToolResult(status=Status.SUCCESS, stdout="[识别结果为空] 未在图片中检测到文字带，请确认图片方向/对比度。")

        font_note = f"字体: {font_path.name}" if font_path else "字体: PIL 内置（仅常规 A-Z，建议上传 Teyvat 字体）"
        header = f"[+] 提瓦特文字识别（字体模板匹配，阈值 {threshold}）\n[+] {font_note}\n" + "-" * 40 + "\n"
        low_conf = [f"{ch}={score:.3f}" for ch, score in confidences if score >= threshold]
        tail = ""
        if low_conf:
            tail = "\n" + "-" * 40 + "\n[!] 低置信度字形（输出为 ?，建议核对）: " + ", ".join(low_conf)

        return ToolResult(
            status=Status.SUCCESS,
            stdout=header + text + tail + "\n",
            data={
                "text": text,
                "font": font_path.name if font_path else None,
                "low_confidence": low_conf,
            },
        )
