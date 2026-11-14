import { useMemo, useState } from 'react';
import { Binary } from 'lucide-react';

/**
 * 进制 ↔ ASCII 即时查询条(逆向 Wiki 顶部小部件)。
 *
 * 单个模式判定优先级(spec 裁定):0x 前缀 → 十六进制码点;纯 ASCII 数字 → 十进制码点;
 * 其他 → 原文字符串逐字符。批量模式用显式方向开关消除歧义(如 beef 这类全 hex 词)。
 * 码点范围 0–0x10FFFF,越界/非法输入给出提示,不抛错。
 */

const MAX_CODEPOINT = 0x10ffff;

/** 码点合法性:范围 0–0x10FFFF 且不在孤立代理区(0xD800–0xDFFF,无法编码 UTF-8) */
export function isInvalidCodepoint(n: number): boolean {
  return (
    !Number.isFinite(n) ||
    n < 0 ||
    n > MAX_CODEPOINT ||
    (n >= 0xd800 && n <= 0xdfff)
  );
}

export interface ParsedInput {
  kind: 'hex' | 'dec' | 'text';
  codepoints: number[];
}

/** 单框输入解析(纯函数) */
export function parseAsciiInput(input: string): ParsedInput {
  const s = input.trim();
  if (!s) return { kind: 'text', codepoints: [] };
  const lower = s.toLowerCase();
  if (lower.startsWith('0x')) {
    const body = s.slice(2);
    if (/^[0-9a-f]+$/i.test(body)) return { kind: 'hex', codepoints: [Number.parseInt(body, 16)] };
    return { kind: 'text', codepoints: Array.from(s).map((ch) => ch.codePointAt(0) ?? 0) };
  }
  if (/^\d+$/.test(s)) return { kind: 'dec', codepoints: [Number.parseInt(s, 10)] };
  return { kind: 'text', codepoints: Array.from(s).map((ch) => ch.codePointAt(0) ?? 0) };
}

export interface CodepointView {
  hex: string;
  dec: number;
  char: string;
  url: string;
  printable: boolean;
}

/** 手动 UTF-8 百分号编码(encodeURIComponent 对字母数字不编码,且代理对会抛错;此实现对任意字符串安全) */
function percentEncode(char: string): string {
  const bytes = new TextEncoder().encode(char);
  return Array.from(bytes, (b) => '%' + b.toString(16).toUpperCase()).join('');
}

/** 码点四态展示(纯函数):hex / dec / 字符 / URL 编码 */
export function formatCodepoint(n: number): CodepointView {
  const char = String.fromCodePoint(n);
  return {
    hex: '0x' + n.toString(16),
    dec: n,
    char,
    url: percentEncode(char),
    printable: n >= 0x20 && n <= 0x7e,
  };
}

/** 批量:数字串 → 字符串(一律按十六进制解析,0x 前缀可选;返回非法 token) */
export function batchToText(tokens: string[]): { views: CodepointView[]; invalid: string[] } {
  const views: CodepointView[] = [];
  const invalid: string[] = [];
  for (const raw of tokens) {
    const t = raw.trim();
    if (!t) continue;
    const body = /^0[xX]/.test(t) ? t.slice(2) : t;
    if (!/^[0-9a-fA-F]+$/.test(body)) {
      invalid.push(t);
      continue;
    }
    const n = Number.parseInt(body, 16);
    if (isInvalidCodepoint(n)) {
      invalid.push(t);
      continue;
    }
    views.push(formatCodepoint(n));
  }
  return { views, invalid };
}

/** 批量:字符串 → 数字串(hex) */
export function batchToHex(text: string): CodepointView[] {
  return Array.from(text).map((ch) => formatCodepoint(ch.codePointAt(0) ?? 0));
}

export default function AsciiConverter() {
  const [mode, setMode] = useState<'single' | 'batch'>('single');
  const [input, setInput] = useState('');
  const [batchDir, setBatchDir] = useState<'toText' | 'toHex'>('toText');

  const single = useMemo(() => {
    const parsed = parseAsciiInput(input);
    const outOfRange =
      parsed.codepoints.length === 1 && isInvalidCodepoint(parsed.codepoints[0]);
    return { parsed, outOfRange };
  }, [input]);

  const batch = useMemo(() => {
    if (batchDir === 'toText') {
      return batchToText(input.split(/[\s,]+/));
    }
    return { views: batchToHex(input), invalid: [] as string[] };
  }, [input, batchDir]);

  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex flex-wrap items-center gap-2">
        <Binary className="h-4 w-4 shrink-0 text-indigo-500" />
        <span className="text-xs font-medium text-slate-600 dark:text-slate-300">进制 ↔ ASCII</span>
        {/* 模式切换 */}
        <div className="flex overflow-hidden rounded-md border border-slate-300 text-xs dark:border-slate-700">
          {(['single', 'batch'] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={`px-2 py-1 ${
                mode === m
                  ? 'bg-indigo-500 text-white'
                  : 'bg-white text-slate-600 hover:bg-slate-100 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700'
              }`}
            >
              {m === 'single' ? '单个' : '批量'}
            </button>
          ))}
        </div>
        {mode === 'batch' && (
          /* 批量方向开关(显式消除 beef 类歧义) */
          <div className="flex overflow-hidden rounded-md border border-slate-300 text-xs dark:border-slate-700">
            {(['toText', 'toHex'] as const).map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => setBatchDir(d)}
                className={`px-2 py-1 ${
                  batchDir === d
                    ? 'bg-slate-700 text-white dark:bg-slate-600'
                    : 'bg-white text-slate-600 hover:bg-slate-100 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700'
                }`}
              >
                {d === 'toText' ? '数字串 → 字符串' : '字符串 → 数字串'}
              </button>
            ))}
          </div>
        )}
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={
            mode === 'single'
              ? '如 0x64 / 100 / d'
              : batchDir === 'toText'
                ? '如 64 65 66 或 0x64,0x65,0x66'
                : '如 flag'
          }
          className="min-w-[10rem] flex-1 rounded-md border border-slate-300 bg-white px-2 py-1.5 font-mono text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
        />
      </div>

      {/* 结果区 */}
      <div className="mt-2 min-h-[1.75rem] text-xs">
        {mode === 'single' ? (
          single.outOfRange ? (
            <span className="text-amber-600 dark:text-amber-400">
              超出 Unicode 范围(0–0x10FFFF)或位于代理区(0xD800–0xDFFF,无法编码)
            </span>
          ) : single.parsed.codepoints.length === 0 ? (
            <span className="text-slate-400">输入即转:0x 前缀十六进制 · 纯数字十进制 · 其他按字符</span>
          ) : single.parsed.codepoints.length === 1 ? (
            (() => {
              const v = formatCodepoint(single.parsed.codepoints[0]);
              return (
                <span className="font-mono text-slate-700 dark:text-slate-200">
                  {v.printable ? (
                    <span className="mr-2">
                      字符 <b className="text-indigo-600 dark:text-indigo-400">{v.char}</b>
                    </span>
                  ) : (
                    <span className="mr-2">
                      字符 <b className="text-indigo-600 dark:text-indigo-400">{v.char}</b>(控制/不可打印)
                    </span>
                  )}
                  <span className="mr-2">十六进制 {v.hex}</span>
                  <span className="mr-2">十进制 {v.dec}</span>
                  <span>URL {v.url}</span>
                </span>
              );
            })()
          ) : (
            <span className="font-mono text-slate-700 dark:text-slate-200">
              按字符串逐字符:
              {single.parsed.codepoints.map((n, i) => {
                const v = formatCodepoint(n);
                return (
                  <span key={i} className="mr-2">
                    {v.char}={v.hex}
                  </span>
                );
              })}
            </span>
          )
        ) : batchDir === 'toText' ? (
          batch.views.length === 0 && batch.invalid.length === 0 ? (
            <span className="text-slate-400">空格/逗号分隔的十六进制数字串(0x 前缀可选,如 64 65 66 或 0x64,0x65,0x66)</span>
          ) : (
            <span className="font-mono text-slate-700 dark:text-slate-200">
              {batch.views.length > 0 && (
                <span className="mr-2">
                  字符串 <b className="text-indigo-600 dark:text-indigo-400">{batch.views.map((v) => v.char).join('')}</b>
                </span>
              )}
              <span className="mr-2">hex {batch.views.map((v) => v.hex.replace('0x', '')).join(' ')}</span>
              <span>dec {batch.views.map((v) => v.dec).join(' ')}</span>
              {batch.invalid.length > 0 && (
                <span className="ml-2 text-amber-600 dark:text-amber-400">无法解析: {batch.invalid.join(' ')}</span>
              )}
            </span>
          )
        ) : batch.views.length === 0 ? (
          <span className="text-slate-400">输入任意字符串,逐字符转码</span>
        ) : (
          <span className="font-mono text-slate-700 dark:text-slate-200">
            <span className="mr-2">
              hex <b className="text-indigo-600 dark:text-indigo-400">{batch.views.map((v) => v.hex.replace('0x', '')).join(' ')}</b>
            </span>
            <span>dec {batch.views.map((v) => v.dec).join(' ')}</span>
          </span>
        )}
      </div>

      {/* 常驻规则说明 */}
      <p className="mt-1 text-[11px] leading-relaxed text-slate-400">
        规则:单个模式 0x 前缀按十六进制码点、纯数字按十进制码点、其他按原文字符串逐字符;批量「数字串 → 字符串」方向一律按十六进制(0x 可选)。要把纯数字/单词按字符转码,请用「批量 · 字符串 → 数字串」方向。
      </p>
    </div>
  );
}
