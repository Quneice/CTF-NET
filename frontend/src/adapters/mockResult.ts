import type { FormValues, OutputSpec } from '@/types';

/**
 * mock 结果生成器。
 *
 * 后端不存在，这里根据 OutputSpec.type 生成形态匹配的 mock 数据，
 * 让 http / task 等传输型 Adapter 也能返回「可被渲染」的结果。
 * 阶段 5 的 RendererRegistry 会按同样约定消费这些数据。
 */

/** 把任意值转成可读字符串（文件显示文件名） */
function stringifyValue(v: unknown): string {
  if (v === undefined || v === null) return '—';
  if (v instanceof File) return `[文件] ${v.name}`;
  if (Array.isArray(v)) {
    if (v.length > 0 && v.every((x) => x instanceof File)) {
      return `[${v.length} 个文件] ${(v as File[]).map((f) => f.name).join(', ')}`;
    }
    return JSON.stringify(v);
  }
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

/** 生成一段纯文本 mock 结果 */
export function mockText(params: FormValues): string {
  const entries = Object.entries(params);
  if (entries.length === 0) return '（无输入参数）';
  return entries.map(([k, v]) => `${k}: ${stringifyValue(v)}`).join('\n');
}

/** 生成一张 SVG 占位图（data URL），供 image 输出渲染 */
function sampleImageDataUrl(): string {
  const svg =
    '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="200">' +
    '<rect width="100%" height="100%" fill="#6366f1"/>' +
    '<text x="50%" y="50%" fill="#fff" font-family="sans-serif" font-size="20" ' +
    'text-anchor="middle" dominant-baseline="middle">Mock 图片</text></svg>';
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

/** 生成十六进制 mock 结果 */
function mockHex(params: FormValues): string {
  const data = Object.values(params).map(stringifyValue).join(' ');
  const bytes = new TextEncoder().encode(data).slice(0, 64);
  const lines: string[] = [];
  for (let i = 0; i < bytes.length; i += 16) {
    const chunk = Array.from(bytes.slice(i, i + 16));
    const hex = chunk.map((b) => b.toString(16).padStart(2, '0')).join(' ');
    lines.push(`${i.toString(16).padStart(8, '0')}  ${hex}`);
  }
  return lines.join('\n');
}

/** 生成一段代码 mock 结果 */
function mockCode(output: OutputSpec & { type: 'code' }): string {
  const lang = output.language ?? 'text';
  return `// ${lang} 输出（mock）\nfunction demo() {\n  // 阶段 5 将用高亮渲染器展示\n  return 'CTF-NET';\n}\n`;
}

/** 生成 Markdown mock 结果 */
function mockMarkdown(params: FormValues): string {
  const json = JSON.stringify(params, null, 2);
  return `# 结果\n\n已处理以下参数：\n\n\`\`\`json\n${json}\n\`\`\`\n`;
}

/** 生成表格 mock 结果（行对象数组） */
function mockTable(output: OutputSpec & { type: 'table' }): Record<string, string>[] {
  const rows: Record<string, string>[] = [];
  for (let r = 1; r <= 5; r++) {
    const row: Record<string, string> = {};
    for (const col of output.columns) row[col.key] = `${col.title}-${r}`;
    rows.push(row);
  }
  return rows;
}

/** 按 output.type 生成形态匹配的 mock 结果 */
export function generateMockResult(
  output: OutputSpec | undefined,
  params: FormValues,
): unknown {
  switch (output?.type) {
    case 'json':
      return { ok: true, mock: true, echo: params, generatedAt: Date.now() };
    case 'markdown':
      return mockMarkdown(params);
    case 'code':
      return mockCode(output);
    case 'table':
      return mockTable(output);
    case 'hex':
      return mockHex(params);
    case 'image':
      return { type: 'image', name: 'result.png', dataUrl: sampleImageDataUrl() };
    case 'file':
      return {
        type: 'file',
        name: 'result.bin',
        size: 1234,
        contentType: 'application/octet-stream',
      };
    case 'custom':
      return { rendererId: output.rendererId, data: params };
    case 'stream':
    case 'text':
    default:
      return mockText(params);
  }
}
