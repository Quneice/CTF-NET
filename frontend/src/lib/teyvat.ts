/**
 * 提瓦特文字（Teyvat script）识别引擎 —— 浏览器端字体模板匹配 OCR。
 *
 * 与后端 tools/misc/teyvat.py 算法一致：
 *   Otsu 二值化 → 行投影切行 → 列投影切字（墨迹极少列作分隔）→
 *   字体多字号渲染 A-Z 模板 → 中心对齐 MAD 匹配。
 *
 * 依赖浏览器 Canvas + FontFace：上传 Teyvat 字体（.ttf/.otf）后即可识别
 * 题图；未上传字体时回退系统字体（仅能识别常规 A-Z，用于验证/演示）。
 */

export interface TeyvatOcrResult {
  text: string;
  lowConfidence: string[];
  fontUsed: string;
}

interface Bitmap {
  w: number;
  h: number;
  data: Uint8Array; // 0/1
}

const DEFAULT_CHARSET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
const DEFAULT_SIZES = [28, 32, 36, 40, 48];
const NORM_HEIGHT = 32;

// ---------------------------------------------------------------------------
// 图像加载 / 二值化
// ---------------------------------------------------------------------------

function loadImageData(file: File): Promise<ImageData> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = img.naturalWidth;
      canvas.height = img.naturalHeight;
      const ctx = canvas.getContext('2d');
      if (!ctx) {
        URL.revokeObjectURL(url);
        reject(new Error('无法创建 canvas 上下文'));
        return;
      }
      ctx.drawImage(img, 0, 0);
      resolve(ctx.getImageData(0, 0, canvas.width, canvas.height));
      URL.revokeObjectURL(url);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('图片加载失败'));
    };
    img.src = url;
  });
}

function otsuThreshold(gray: Uint8Array): number {
  const hist = new Float64Array(256);
  for (let i = 0; i < gray.length; i++) hist[gray[i]]++;
  const total = gray.length;
  let sum = 0;
  for (let t = 0; t < 256; t++) sum += t * hist[t];
  let sumB = 0;
  let wB = 0;
  let maxVar = 0;
  let thr = 127;
  for (let t = 0; t < 256; t++) {
    wB += hist[t];
    if (wB === 0) continue;
    const wF = total - wB;
    if (wF === 0) break;
    sumB += t * hist[t];
    const mB = sumB / wB;
    const mF = (sum - sumB) / wF;
    const v = wB * wF * (mB - mF) * (mB - mF);
    if (v > maxVar) {
      maxVar = v;
      thr = t;
    }
  }
  return thr;
}

function binarize(img: ImageData): { w: number; h: number; data: Uint8Array } {
  const w = img.width;
  const h = img.height;
  const gray = new Uint8Array(w * h);
  for (let i = 0, p = 0; i < w * h; i++, p += 4) {
    const r = img.data[p];
    const g = img.data[p + 1];
    const b = img.data[p + 2];
    const a = img.data[p + 3];
    gray[i] = a < 128 ? 255 : ((r + g + b) / 3) | 0;
  }
  const thr = otsuThreshold(gray);
  const data = new Uint8Array(w * h);
  for (let i = 0; i < w * h; i++) data[i] = gray[i] < thr ? 1 : 0;
  return { w, h, data };
}

// ---------------------------------------------------------------------------
// 行 / 字形切分
// ---------------------------------------------------------------------------

function lineBands(ink: Uint8Array, w: number, h: number): Array<[number, number]> {
  const rowSum = new Uint32Array(h);
  for (let y = 0; y < h; y++) {
    let s = 0;
    for (let x = 0; x < w; x++) s += ink[y * w + x];
    rowSum[y] = s;
  }
  const bands: Array<[number, number]> = [];
  let inBand = false;
  let y0 = 0;
  for (let y = 0; y < h; y++) {
    if (rowSum[y] > 0 && !inBand) {
      y0 = y;
      inBand = true;
    } else if (rowSum[y] === 0 && inBand) {
      bands.push([y0, y]);
      inBand = false;
    }
  }
  if (inBand) bands.push([y0, h]);
  return bands;
}

function glyphRuns(ink: Uint8Array, w: number, y0: number, y1: number): Array<[number, number]> {
  const h = y1 - y0;
  const colSum = new Uint32Array(w);
  for (let y = y0; y < y1; y++) {
    for (let x = 0; x < w; x++) colSum[x] += ink[y * w + x];
  }
  // 只把「足够宽的全空白列带」当作分隔，忽略字形内部的窄缝（哥特体断裂笔画）
  const minSep = Math.max(2, Math.floor(0.1 * h));
  const sep = new Uint8Array(w);
  let x = 0;
  while (x < w) {
    if (colSum[x] === 0) {
      let z = x;
      while (z < w && colSum[z] === 0) z++;
      if (z - x >= minSep) {
        for (let k = x; k < z; k++) sep[k] = 1;
      }
      x = z;
    } else {
      x++;
    }
  }
  const runs: Array<[number, number]> = [];
  x = 0;
  while (x < w) {
    if (sep[x]) {
      x++;
      continue;
    }
    const x0 = x;
    while (x < w && !sep[x]) x++;
    runs.push([x0, x]);
  }
  return runs;
}

// ---------------------------------------------------------------------------
// 字形裁剪 / 归一化 / 模板渲染 / 匹配
// ---------------------------------------------------------------------------

function cropGlyph(ink: Uint8Array, w: number, y0: number, y1: number, x0: number, x1: number): Bitmap {
  let minX = x1;
  let maxX = x0;
  let minY = y1;
  let maxY = y0;
  for (let y = y0; y < y1; y++) {
    for (let x = x0; x < x1; x++) {
      if (ink[y * w + x]) {
        if (x < minX) minX = x;
        if (x > maxX) maxX = x;
        if (y < minY) minY = y;
        if (y > maxY) maxY = y;
      }
    }
  }
  if (minX > maxX) return { w: 1, h: 1, data: new Uint8Array([0]) };
  const cw = maxX - minX + 1;
  const ch = maxY - minY + 1;
  const data = new Uint8Array(cw * ch);
  for (let y = minY; y <= maxY; y++) {
    for (let x = minX; x <= maxX; x++) {
      data[(y - minY) * cw + (x - minX)] = ink[y * w + x];
    }
  }
  return { w: cw, h: ch, data };
}

function normalize(bmp: Bitmap, height: number): Bitmap {
  const scale = height / bmp.h;
  const nw = Math.max(1, Math.round(bmp.w * scale));

  const src = document.createElement('canvas');
  src.width = bmp.w;
  src.height = bmp.h;
  const sctx = src.getContext('2d')!;
  const sid = sctx.createImageData(bmp.w, bmp.h);
  for (let i = 0; i < bmp.data.length; i++) {
    const v = bmp.data[i] ? 0 : 255;
    sid.data[i * 4] = v;
    sid.data[i * 4 + 1] = v;
    sid.data[i * 4 + 2] = v;
    sid.data[i * 4 + 3] = 255;
  }
  sctx.putImageData(sid, 0, 0);

  const out = document.createElement('canvas');
  out.width = nw;
  out.height = height;
  const octx = out.getContext('2d')!;
  octx.drawImage(src, 0, 0, nw, height);
  const od = octx.getImageData(0, 0, nw, height);

  const data = new Uint8Array(nw * height);
  for (let i = 0; i < nw * height; i++) data[i] = od.data[i * 4] < 128 ? 1 : 0;
  return { w: nw, h: height, data };
}

function renderGlyph(fontFamily: string, char: string, size: number): Bitmap {
  const pad = size;
  const canvas = document.createElement('canvas');
  canvas.width = size * 2 + pad * 2;
  canvas.height = size * 2 + pad * 2;
  const ctx = canvas.getContext('2d')!;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.font = `${size}px ${fontFamily}`;
  ctx.fillStyle = '#000';
  ctx.textBaseline = 'alphabetic';
  ctx.fillText(char, pad, pad + size);

  const id = ctx.getImageData(0, 0, canvas.width, canvas.height);
  const W = canvas.width;
  const H = canvas.height;
  const tmp = new Uint8Array(W * H);
  let minX = W;
  let maxX = -1;
  let minY = H;
  let maxY = -1;
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      const a = id.data[(y * W + x) * 4 + 3];
      const on = a > 64 ? 1 : 0;
      tmp[y * W + x] = on;
      if (on) {
        if (x < minX) minX = x;
        if (x > maxX) maxX = x;
        if (y < minY) minY = y;
        if (y > maxY) maxY = y;
      }
    }
  }
  if (maxX < minX) return { w: 1, h: 1, data: new Uint8Array([0]) };
  const cw = maxX - minX + 1;
  const ch = maxY - minY + 1;
  const data = new Uint8Array(cw * ch);
  for (let y = minY; y <= maxY; y++) {
    for (let x = minX; x <= maxX; x++) {
      data[(y - minY) * cw + (x - minX)] = tmp[y * W + x];
    }
  }
  return normalize({ w: cw, h: ch, data }, NORM_HEIGHT);
}

function buildTemplates(fontFamily: string, charset: string, sizes: number[]): Record<string, Bitmap[]> {
  const templates: Record<string, Bitmap[]> = {};
  for (const ch of charset) templates[ch] = [];
  for (const size of sizes) {
    for (const ch of charset) {
      templates[ch].push(renderGlyph(fontFamily, ch, size));
    }
  }
  return templates;
}

function mad(a: Bitmap, b: Bitmap): number {
  const h = a.h;
  const w = Math.max(a.w, b.w);
  const xa = (w - a.w) >> 1;
  const xb = (w - b.w) >> 1;
  let diff = 0;
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const av = x >= xa && x < xa + a.w ? a.data[y * a.w + (x - xa)] : 0;
      const bv = x >= xb && x < xb + b.w ? b.data[y * b.w + (x - xb)] : 0;
      if (av !== bv) diff++;
    }
  }
  return diff / (h * w);
}

function matchGlyph(glyph: Bitmap, templates: Record<string, Bitmap[]>): { ch: string; score: number } {
  const g = normalize(glyph, NORM_HEIGHT);
  let bestCh = '?';
  let bestScore = 1;
  for (const ch of Object.keys(templates)) {
    for (const t of templates[ch]) {
      const s = mad(g, t);
      if (s < bestScore) {
        bestScore = s;
        bestCh = ch;
      }
    }
  }
  return { ch: bestCh, score: bestScore };
}

function median(arr: number[]): number {
  if (!arr.length) return 0;
  const s = [...arr].sort((a, b) => a - b);
  const m = s.length >> 1;
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
}

// ---------------------------------------------------------------------------
// 字体加载
// ---------------------------------------------------------------------------

async function loadFontFace(file: File): Promise<string> {
  const family = `teyvat-${Date.now()}`;
  const buf = await file.arrayBuffer();
  const face = new FontFace(family, buf);
  await face.load();
  document.fonts.add(face);
  await document.fonts.ready;
  return family;
}

async function loadBundledFont(): Promise<string | null> {
  try {
    const res = await fetch('/fonts/Teyvat-Regular.ttf');
    if (!res.ok) return null;
    const buf = await res.arrayBuffer();
    const family = 'teyvat-bundled';
    const face = new FontFace(family, buf);
    await face.load();
    document.fonts.add(face);
    await document.fonts.ready;
    return family;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// 主识别流程
// ---------------------------------------------------------------------------

export async function recognizeTeyvat(
  imageFile: File,
  fontFile: File | undefined,
  charset: string,
  threshold: number,
): Promise<TeyvatOcrResult> {
  const img = await loadImageData(imageFile);
  const { w, h, data: ink } = binarize(img);

  let fontFamily: string;
  let fontUsed: string;
  if (fontFile) {
    fontFamily = await loadFontFace(fontFile);
    fontUsed = fontFile.name;
  } else {
    const bundled = await loadBundledFont();
    if (bundled) {
      fontFamily = bundled;
      fontUsed = '内置 Teyvat 字体（2021）';
    } else {
      fontFamily = 'Arial, sans-serif';
      fontUsed = '系统字体（仅常规 A-Z）';
    }
  }
  const templates = buildTemplates(fontFamily, charset, DEFAULT_SIZES);

  const lines: string[] = [];
  const lowConfidence: string[] = [];
  for (const [y0, y1] of lineBands(ink, w, h)) {
    const runs = glyphRuns(ink, w, y0, y1);
    if (!runs.length) continue;
    const gaps: number[] = [];
    for (let i = 1; i < runs.length; i++) gaps.push(runs[i][0] - runs[i - 1][1]);
    const medianGap = median(gaps);
    let line = '';
    for (let i = 0; i < runs.length; i++) {
      if (i > 0) {
        const gap = runs[i][0] - runs[i - 1][1];
        if (gap > Math.max(4, 2 * medianGap)) line += ' ';
      }
      const glyph = cropGlyph(ink, w, y0, y1, runs[i][0], runs[i][1]);
      const { ch, score } = matchGlyph(glyph, templates);
      if (score < threshold) {
        line += ch;
      } else {
        line += '?';
        lowConfidence.push(`${ch}(${score.toFixed(3)})`);
      }
    }
    lines.push(line);
  }

  return { text: lines.join('\n'), lowConfidence, fontUsed };
}
