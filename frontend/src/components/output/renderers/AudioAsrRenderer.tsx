import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';

import type { RendererProps } from './types';

import { CopyButton, DownloadButton } from '../OutputActions';
import { compileSensitiveRegexes, SENSITIVE_PRESETS } from '@/lib/sensitivePatterns';

/**
 * 语音转文字渲染器（rendererId = 'audio-asr'）。
 * 后端 stdout 为 JSON：{ summary, files[{name, format, lang, duration_sec,
 * chars, text, segments[{start, end, text}]}], skipped_files, notes }。
 *
 * 双层筛选（均为客户端即时生效，无需重跑识别）：
 *   1) 内容关键字：文件级粗筛（原有能力）；
 *   2) 敏感信息筛选：预设正则（手机号/身份证/地址/邮箱/银行卡/IP/URL/
 *      QQ/微信/账号密码/车牌）+ 自定义正则，命中分段高亮，未命中的
 *      文件与分段从视图隐藏，导出（txt/SRT）同步只含命中分段。
 */

interface AsrSegment {
  start: number;
  end: number;
  text: string;
}

interface AsrFileRecord {
  name: string;
  format: string;
  lang: string;
  duration_sec: number;
  chars: number;
  text: string;
  segments: AsrSegment[];
}

interface AsrSummary {
  total: number;
  analyzed: number;
  skipped: number;
  with_text: number;
  empty: number;
  total_chars: number;
  total_duration: number;
  formats: Record<string, number>;
  langs: Record<string, number>;
}

interface AsrPayload {
  summary: AsrSummary;
  files: AsrFileRecord[];
  skipped_files?: { name: string; error: string }[];
  notes?: string[];
}

const inputCls =
  'rounded-md border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200';

const chipCls =
  'rounded-full border px-2.5 py-0.5 text-xs transition-colors cursor-pointer select-none';
const chipOffCls = 'border-slate-300 bg-white text-slate-600 hover:border-indigo-400 hover:text-indigo-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300';
const chipOnCls =
  'border-indigo-500 bg-indigo-500 text-white dark:border-indigo-400 dark:bg-indigo-500';

const markCls = 'rounded-sm bg-amber-200 px-0.5 text-amber-950 dark:bg-amber-500/40 dark:text-amber-100';

function parsePayload(data: unknown): AsrPayload | null {
  if (typeof data !== 'string') return null;
  try {
    const p = JSON.parse(data);
    if (p && Array.isArray(p.files) && p.summary) return p as AsrPayload;
  } catch {
    /* fallthrough */
  }
  return null;
}

/** 秒 → mm:ss（超过 1 小时显示 h:mm:ss） */
function fmtDur(sec: number): string {
  if (!isFinite(sec) || sec <= 0) return '—';
  const s = Math.round(sec);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const r = s % 60;
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}` : `${m}:${String(r).padStart(2, '0')}`;
}

/** 秒 → SRT 时间戳 HH:MM:SS,mmm */
function srtTime(sec: number): string {
  const ms = Math.round(sec * 1000);
  const h = Math.floor(ms / 3600000);
  const m = Math.floor((ms % 3600000) / 60000);
  const s = Math.floor((ms % 60000) / 1000);
  const mm = ms % 1000;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')},${String(mm).padStart(3, '0')}`;
}

/**
 * 文本命中高亮：按正则列表切分文本，命中片段包 <mark>。
 * 每次调用用全新 global 克隆，避免 lastIndex 状态污染。
 */
function highlightText(text: string, regexes: RegExp[]): ReactNode[] {
  if (regexes.length === 0) return [text];
  const global = regexes.map((r) => new RegExp(r.source, r.flags.includes('g') ? r.flags : r.flags + 'g'));
  const parts: ReactNode[] = [];
  let pos = 0;
  let guard = 0;
  while (pos < text.length && guard++ < 1000) {
    let best: { start: number; end: number } | null = null;
    for (const re of global) {
      re.lastIndex = pos;
      const m = re.exec(text);
      if (m && m[0].length > 0 && m.index >= pos && (best === null || m.index < best.start)) {
        best = { start: m.index, end: m.index + m[0].length };
      }
    }
    if (!best) break;
    if (best.start > pos) parts.push(text.slice(pos, best.start));
    parts.push(
      <mark key={`${parts.length}`} className={markCls}>
        {text.slice(best.start, best.end)}
      </mark>,
    );
    pos = best.end;
    if (pos <= best.start) break; // 防御：避免死循环
  }
  if (pos < text.length) parts.push(text.slice(pos));
  return parts;
}

export default function AudioAsrRenderer({ data }: RendererProps) {
  const payload = useMemo(() => parsePayload(data), [data]);
  const [keyword, setKeyword] = useState('');
  const [activePresets, setActivePresets] = useState<string[]>([]);
  const [customRegex, setCustomRegex] = useState('');

  const { regexes, errors } = useMemo(
    () => compileSensitiveRegexes(activePresets, customRegex),
    [activePresets, customRegex],
  );

  if (!payload) {
    return (
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs text-slate-700 dark:bg-slate-900 dark:text-slate-300">
        {String(data)}
      </pre>
    );
  }

  const { summary, files, skipped_files, notes } = payload;
  const filterActive = regexes.length > 0;

  const togglePreset = (id: string) =>
    setActivePresets((prev) => (prev.includes(id) ? prev.filter((p) => p !== id) : [...prev, id]));

  const kw = keyword.trim().toLowerCase();
  const kwHit = (f: AsrFileRecord) =>
    !kw || f.text.toLowerCase().includes(kw) || f.segments.some((s) => s.text.toLowerCase().includes(kw));

  const matchedSegsOf = (f: AsrFileRecord) =>
    filterActive ? f.segments.filter((s) => regexes.some((r) => r.test(s.text))) : f.segments;

  const filtered = filterActive
    ? files.filter((f) => kwHit(f) && matchedSegsOf(f).length > 0)
    : files.filter(kwHit);

  const totalMatchedSegs = filtered.reduce((n, f) => n + matchedSegsOf(f).length, 0);

  const toText = () =>
    filtered
      .filter((f) => (filterActive ? matchedSegsOf(f).length > 0 : f.text))
      .map((f) => {
        const body = filterActive
          ? matchedSegsOf(f)
              .map((s) => s.text)
              .join('\n')
          : f.text;
        return `===== ${f.name} (${f.lang || '?'}) =====\n${body}`;
      })
      .join('\n\n');

  const toSrt = () => {
    let idx = 1;
    const blocks: string[] = [];
    for (const f of filtered) {
      for (const seg of matchedSegsOf(f)) {
        blocks.push(`${idx++}\n${srtTime(seg.start)} --> ${srtTime(seg.end)}\n${seg.text}`);
      }
    }
    return blocks.join('\n\n');
  };

  return (
    <div className="space-y-3">
      {/* —— 汇总 —— */}
      <div className="flex flex-wrap gap-2">
        <span className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-1">
          识别 {summary.analyzed} 个<span className="ml-1 text-ink-3">（跳过 {summary.skipped}）</span>
        </span>
        <span className="rounded-full border border-emerald-300 bg-emerald-50 px-3 py-1 text-xs text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-400">
          含文字 {summary.with_text}
        </span>
        <span className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
          总字符 {summary.total_chars}
        </span>
        <span className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
          音频总时长 {fmtDur(summary.total_duration)}
        </span>
        {Object.entries(summary.langs ?? {})
          .sort((a, b) => b[1] - a[1])
          .map(([l, n]) => (
            <span key={l} className="rounded-full border border-indigo-300 bg-indigo-50 px-3 py-1 text-xs text-indigo-700 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-400">
              语种 {l} × {n}
            </span>
          ))}
        {Object.entries(summary.formats ?? {})
          .sort((a, b) => b[1] - a[1])
          .map(([f, n]) => (
            <span key={f} className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-2">
              .{f} × {n}
            </span>
          ))}
      </div>

      {/* —— 关键字筛选与导出 —— */}
      <div className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 p-3 dark:border-slate-800">
        <label className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          内容关键字
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="如 flag{ / 密码"
            className={`${inputCls} w-56`}
          />
        </label>
        <span className="ml-auto flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          显示 {filtered.length} / {files.length} 个文件
          <DownloadButton text={toText()} filename="asr_texts.txt" mime="text/plain" />
          <DownloadButton text={toSrt()} filename="asr_all.srt" mime="application/x-subrip" />
        </span>
      </div>

      {/* —— 敏感信息筛选（正则预设 + 自定义） —— */}
      <div className="rounded-lg border border-slate-200 p-3 dark:border-slate-800">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-xs font-medium text-slate-600 dark:text-slate-300">敏感信息筛选</span>
          {SENSITIVE_PRESETS.map((p) => {
            const on = activePresets.includes(p.id);
            return (
              <button
                key={p.id}
                type="button"
                title={p.hint}
                onClick={() => togglePreset(p.id)}
                className={`${chipCls} ${on ? chipOnCls : chipOffCls}`}
              >
                {p.label}
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => {
              setActivePresets([]);
              setCustomRegex('');
            }}
            className="ml-1 text-xs text-slate-400 underline decoration-dotted hover:text-slate-600 dark:hover:text-slate-300"
          >
            清空
          </button>
        </div>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
            自定义正则
            <input
              value={customRegex}
              onChange={(e) => setCustomRegex(e.target.value)}
              placeholder="如 flag\{[^}]+\} 或 \d{4}年"
              className={`${inputCls} w-64 font-mono`}
            />
          </label>
          {filterActive && (
            <span className="rounded-full border border-amber-300 bg-amber-50 px-2.5 py-0.5 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400">
              命中 {filtered.length} 个文件 · {totalMatchedSegs} 个分段（未命中已隐藏，导出同步过滤）
            </span>
          )}
        </div>
        {errors.length > 0 && (
          <ul className="mt-2 space-y-0.5 text-xs text-amber-600 dark:text-amber-500">
            {errors.map((e, i) => (
              <li key={i}>· {e}</li>
            ))}
          </ul>
        )}
      </div>

      {/* —— 每文件结果 —— */}
      <div className="space-y-3">
        {filtered.map((f, i) => {
          const segs = matchedSegsOf(f);
          return (
            <div key={i} className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
              <div className="flex flex-wrap items-center gap-2 border-b border-slate-200 bg-slate-50 px-3 py-2 dark:border-slate-800 dark:bg-slate-900">
                <span className="truncate text-sm font-medium text-slate-700 dark:text-slate-200" title={f.name}>
                  {f.name}
                </span>
                <span className="rounded-full border border-line bg-surface-raised px-2 py-0.5 text-xs text-ink-2">
                  .{f.format}
                </span>
                <span className="rounded-full border border-indigo-200 bg-indigo-50 px-2 py-0.5 text-xs text-indigo-700 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-400">
                  {f.lang || '未知语言'}
                </span>
                <span className="rounded-full border border-line bg-surface-raised px-2 py-0.5 text-xs text-ink-3">
                  时长 {fmtDur(f.duration_sec)}
                </span>
                {filterActive && (
                  <span className="rounded-full border border-amber-300 bg-amber-50 px-2 py-0.5 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400">
                    命中 {segs.length} 段
                  </span>
                )}
                <span className="ml-auto flex items-center gap-1">
                  <CopyButton text={f.text} />
                  <DownloadButton text={f.text} filename={`${f.name.replace(/\.[^.]+$/, '')}.txt`} mime="text/plain" />
                </span>
              </div>
              {segs.length > 0 ? (
                <div className="max-h-72 overflow-auto">
                  <table className="min-w-full text-sm">
                    <tbody>
                      {segs.map((seg, j) => (
                        <tr key={j} className="border-t border-slate-100 dark:border-slate-800">
                          <td className="whitespace-nowrap px-3 py-1.5 align-top font-mono text-xs text-slate-400">
                            {fmtDur(seg.start)} – {fmtDur(seg.end)}
                          </td>
                          <td className="px-3 py-1.5 text-slate-700 dark:text-slate-300">
                            {filterActive ? highlightText(seg.text, regexes) : seg.text}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="px-3 py-4 text-center text-xs text-slate-400">
                  {f.text ? f.text : '未识别到语音内容'}
                </div>
              )}
            </div>
          );
        })}
        {filtered.length === 0 && (
          <div className="rounded-lg border border-slate-200 px-3 py-6 text-center text-sm text-slate-400 dark:border-slate-800">
            {filterActive ? '没有命中当前敏感信息筛选条件的文件（可调整预设或清空筛选）' : '没有匹配当前筛选条件的文件'}
          </div>
        )}
      </div>

      {/* —— 备注 —— */}
      {(notes?.length ?? 0) > 0 && (
        <ul className="space-y-1 rounded-lg border border-slate-200 p-3 text-xs text-slate-500 dark:border-slate-800 dark:text-slate-400">
          {notes!.map((n, i) => (
            <li key={i}>· {n}</li>
          ))}
        </ul>
      )}

      {/* —— 跳过的文件 —— */}
      {(skipped_files?.length ?? 0) > 0 && (
        <details className="rounded-lg border border-slate-200 p-3 text-xs dark:border-slate-800">
          <summary className="cursor-pointer text-slate-500 dark:text-slate-400">
            跳过 {skipped_files!.length} 个文件（点击展开）
          </summary>
          <ul className="mt-2 space-y-1 text-slate-500 dark:text-slate-400">
            {skipped_files!.map((s, i) => (
              <li key={i} className="truncate">
                {s.name} — {s.error}
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}
