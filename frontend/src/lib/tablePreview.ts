/**
 * 表格文件列预览嗅探（纯前端，零依赖）。
 *
 * 选中文件后读取其头部即可识别列名，供表单下拉选择合并列：
 * - CSV/TSV/TXT：UTF-8 / UTF-8 BOM / UTF-16LE / GBK 自动识别，
 *   分隔符 , ; \t 自动嗅探，读取前 512KB 即可拿到表头与前几行；
 * - XLSX：解析 zip 中央目录（浏览器原生 DecompressionStream 解压），
 *   读 workbook.xml + 关系文件 + 首个工作表的表头行与前 3 行数据，
 *   sharedStrings / inlineStr / 数值单元格均支持，按 r 属性对齐列位置。
 *
 * 与后端 data.convert.table-merge 的解析口径保持一致（表头 trim、
 * 空表头补 列N、键列去空白匹配等）。
 */

export interface FileTablePreview {
  /** 识别出的列名（与后端 normHeader 口径一致） */
  columns: string[];
  /** 来源描述，如 CSV(GBK, 分隔符 ',') / XLSX(Sheet1) */
  meta: string;
  /** 前 3 行数据样本（用于展示） */
  sample: string[][];
  /** 识别失败原因（此时 columns/sample 为空） */
  error?: string;
}

// ---------------------------------------------------------------------------
// 通用工具
// ---------------------------------------------------------------------------

function readU16(u8: Uint8Array, off: number): number {
  return u8[off] | (u8[off + 1] << 8);
}

function readU32(u8: Uint8Array, off: number): number {
  return (u8[off] | (u8[off + 1] << 8) | (u8[off + 2] << 16) | (u8[off + 3] << 24)) >>> 0;
}

function normHeaderCells(cells: string[]): string[] {
  return cells.map((h, i) => {
    let s = h.trim().replace(/^﻿/, '');
    if (s === '') s = `列${i + 1}`;
    return s;
  });
}

function fail(msg: string): FileTablePreview {
  return { columns: [], meta: '', sample: [], error: msg };
}

// ---------------------------------------------------------------------------
// CSV / TSV / TXT
// ---------------------------------------------------------------------------

/** 解析 CSV 记录（引号转义状态机），最多取 maxRecords 条非空记录 */
function parseCsvRecords(text: string, delim: string, maxRecords: number): string[][] {
  const records: string[][] = [];
  let field = '';
  let row: string[] = [];
  let inQuotes = false;
  const pushRow = () => {
    row.push(field);
    field = '';
    if (row.some((c) => c.trim() !== '')) records.push(row);
    row = [];
  };
  for (let i = 0; i < text.length && records.length < maxRecords; i++) {
    const ch = text[i];
    if (inQuotes) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
    } else if (ch === '"') {
      inQuotes = true;
    } else if (ch === delim) {
      row.push(field);
      field = '';
    } else if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && text[i + 1] === '\n') i++;
      pushRow();
    } else {
      field += ch;
    }
  }
  row.push(field);
  if (row.some((c) => c.trim() !== '')) records.push(row);
  return records;
}

/** 候选分隔符中选能切出最多字段的那个 */
function detectDelimiter(firstLine: string): string {
  let best = ',';
  let bestN = -1;
  for (const d of [',', ';', '\t']) {
    const recs = parseCsvRecords(firstLine, d, 1);
    if (recs.length > 0 && recs[0].length > bestN) {
      best = d;
      bestN = recs[0].length;
    }
  }
  return best;
}

/** 字节流解码：BOM → UTF-8 校验 → GBK 兜底（与后端 decodeTextBytes 对齐） */
function decodeTextBytes(u8: Uint8Array): { text: string; enc: string } {
  if (u8[0] === 0xef && u8[1] === 0xbb && u8[2] === 0xbf) {
    return { text: new TextDecoder('utf-8').decode(u8.subarray(3)), enc: 'UTF-8(BOM)' };
  }
  if (u8[0] === 0xff && u8[1] === 0xfe) {
    return { text: new TextDecoder('utf-16le').decode(u8.subarray(2)), enc: 'UTF-16LE' };
  }
  try {
    return { text: new TextDecoder('utf-8', { fatal: true }).decode(u8), enc: 'UTF-8' };
  } catch {
    /* 非 UTF-8，尝试 GBK */
  }
  try {
    return { text: new TextDecoder('gbk').decode(u8), enc: 'GBK' };
  } catch {
    return { text: new TextDecoder('utf-8').decode(u8), enc: '原始字节' };
  }
}

async function sniffCsv(file: File): Promise<FileTablePreview> {
  const buf = await file.slice(0, 512 * 1024).arrayBuffer(); // 头部足够拿到表头
  const { text, enc } = decodeTextBytes(new Uint8Array(buf));
  const first = text.split(/\r?\n/).find((l) => l.trim() !== '');
  if (!first) return fail('文件内容为空');
  const delim = detectDelimiter(first);
  const recs = parseCsvRecords(text, delim, 4);
  if (recs.length === 0) return fail('文件内容为空');
  const columns = normHeaderCells(recs[0]);
  const sample = recs.slice(1, 4).map((r) => columns.map((_, i) => r[i] ?? ''));
  return { columns, sample, meta: `CSV(${enc}, 分隔符 '${delim}')` };
}

// ---------------------------------------------------------------------------
// XLSX（zip 中央目录 + DecompressionStream）
// ---------------------------------------------------------------------------

/** 从文件尾部反向找 End of Central Directory 签名 */
function findEOCD(u8: Uint8Array): number {
  const min = Math.max(0, u8.length - 22 - 65535);
  for (let i = u8.length - 22; i >= min; i--) {
    if (u8[i] === 0x50 && u8[i + 1] === 0x4b && u8[i + 2] === 0x05 && u8[i + 3] === 0x06) {
      return i;
    }
  }
  return -1;
}

async function inflateRaw(data: Uint8Array): Promise<Uint8Array> {
  const ds = new DecompressionStream('deflate-raw');
  const buf = data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) as ArrayBuffer;
  const stream = new Blob([buf]).stream().pipeThrough(ds);
  return new Uint8Array(await new Response(stream).arrayBuffer());
}

/** 列引用（如 "AA12"）→ 0 基列号 */
function colIndex(ref: string): number {
  let n = 0;
  for (const ch of ref) {
    if (ch >= 'A' && ch <= 'Z') n = n * 26 + (ch.charCodeAt(0) - 64);
  }
  return n - 1;
}

/** 读取 zip 内文件内容（仅支持 store / deflate，与 xlsx 实际使用一致） */
async function readZipEntry(
  u8: Uint8Array,
  entries: Map<string, { off: number; csize: number; method: number }>,
  name: string,
): Promise<Uint8Array> {
  const e = entries.get(name);
  if (!e) throw new Error(`zip 内缺少 ${name}`);
  const nameLen = readU16(u8, e.off + 26);
  const extraLen = readU16(u8, e.off + 28);
  const dataStart = e.off + 30 + nameLen + extraLen;
  const data = u8.subarray(dataStart, dataStart + e.csize);
  if (e.method === 0) return data;
  if (e.method === 8) return inflateRaw(data);
  throw new Error(`不支持的 zip 压缩方式 ${e.method}`);
}

function parseXml(text: string): Document {
  const doc = new DOMParser().parseFromString(text, 'application/xml');
  if (doc.getElementsByTagName('parsererror').length > 0) {
    throw new Error('XML 解析失败');
  }
  return doc;
}

async function sniffXlsx(buf: ArrayBuffer): Promise<FileTablePreview> {
  const u8 = new Uint8Array(buf);
  const eocd = findEOCD(u8);
  if (eocd < 0) return fail('不是有效的 xlsx 文件');
  // 中央目录
  const entries = new Map<string, { off: number; csize: number; method: number }>();
  let p = readU32(u8, eocd + 16);
  const end = p + readU32(u8, eocd + 12);
  while (p + 46 <= u8.length && p < end) {
    if (readU32(u8, p) !== 0x02014b50) break;
    const method = readU16(u8, p + 10);
    const csize = readU32(u8, p + 20);
    const nameLen = readU16(u8, p + 28);
    const extraLen = readU16(u8, p + 30);
    const commentLen = readU16(u8, p + 32);
    const lho = readU32(u8, p + 42);
    const name = new TextDecoder().decode(u8.subarray(p + 46, p + 46 + nameLen));
    entries.set(name, { off: lho, csize, method });
    p += 46 + nameLen + extraLen + commentLen;
  }
  // 首个工作表：workbook.xml → r:id → rels → target
  const wb = parseXml(new TextDecoder().decode(await readZipEntry(u8, entries, 'xl/workbook.xml')));
  const firstSheet = wb.getElementsByTagName('sheet')[0];
  if (!firstSheet) return fail('工作簿不含任何工作表');
  const sheetName = firstSheet.getAttribute('name') ?? 'Sheet1';
  const relNs = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships';
  const rid = firstSheet.getAttributeNS(relNs, 'id') ?? firstSheet.getAttribute('r:id') ?? '';
  const rels = parseXml(new TextDecoder().decode(await readZipEntry(u8, entries, 'xl/_rels/workbook.xml.rels')));
  let target = '';
  for (const rel of Array.from(rels.getElementsByTagName('Relationship'))) {
    if (rel.getAttribute('Id') === rid) {
      target = rel.getAttribute('Target') ?? '';
      break;
    }
  }
  if (!target) return fail('无法定位首个工作表');
  const sheetPath = target.startsWith('/') ? target.slice(1) : `xl/${target}`;
  // 共享字符串表（可能不存在）
  const shared: string[] = [];
  if (entries.has('xl/sharedStrings.xml')) {
    const ss = parseXml(new TextDecoder().decode(await readZipEntry(u8, entries, 'xl/sharedStrings.xml')));
    for (const si of Array.from(ss.getElementsByTagName('si'))) {
      shared.push(si.textContent ?? '');
    }
  }
  // 表头行 + 前 3 行数据
  const sheet = parseXml(new TextDecoder().decode(await readZipEntry(u8, entries, sheetPath)));
  const rows = Array.from(sheet.getElementsByTagName('row')).slice(0, 4);
  if (rows.length === 0) return fail(`工作表 ${sheetName} 为空`);
  const parsed: string[][] = rows.map((row) => {
    const cells: string[] = [];
    for (const c of Array.from(row.getElementsByTagName('c'))) {
      const t = c.getAttribute('t');
      const ref = c.getAttribute('r') ?? '';
      const v = c.getElementsByTagName('v')[0]?.textContent ?? '';
      let val = '';
      if (t === 's') {
        val = shared[Number(v)] ?? '';
      } else if (t === 'inlineStr') {
        val = c.getElementsByTagName('is')[0]?.textContent ?? '';
      } else if (t === 'str') {
        val = v;
      } else if (t === 'b') {
        val = v === '1' ? 'TRUE' : 'FALSE';
      } else if (t === 'e') {
        val = '#ERR';
      } else {
        val = v; // 数值原样
      }
      const idx = colIndex(ref);
      if (idx >= 0) cells[idx] = val;
    }
    return cells;
  });
  const columns = normHeaderCells(parsed[0]);
  const sample = parsed.slice(1, 4).map((r) => columns.map((_, i) => r[i] ?? ''));
  return { columns, sample, meta: `XLSX(${sheetName})` };
}

// ---------------------------------------------------------------------------
// 入口
// ---------------------------------------------------------------------------

/** 选中表格文件后识别其列名与数据样本 */
export async function sniffTableFile(file: File): Promise<FileTablePreview> {
  const ext = (file.name.split('.').pop() ?? '').toLowerCase();
  try {
    if (ext === 'csv' || ext === 'tsv' || ext === 'txt') {
      return await sniffCsv(file);
    }
    if (ext === 'xlsx') {
      if (file.size > 50 * 1024 * 1024) return fail('文件超过 50MB，跳过预览');
      return await sniffXlsx(await file.arrayBuffer());
    }
    if (ext === 'xls') {
      return fail('旧版 .xls 暂不支持，请在 Excel 中另存为 .xlsx');
    }
    return fail('不支持的文件类型（支持 .csv/.tsv/.txt/.xlsx）');
  } catch (e) {
    return fail(e instanceof Error ? e.message : String(e));
  }
}
