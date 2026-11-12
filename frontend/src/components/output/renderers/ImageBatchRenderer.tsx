import { useMemo, useState } from 'react';

import type { RendererProps } from './types';

import { CopyButton, DownloadButton } from '../OutputActions';

/**
 * 图片批量分析渲染器（rendererId = 'image-batch'）。
 * 后端 stdout 为 JSON：{ summary, images[], skipped_files[] }，
 * 全部筛选在客户端完成 —— 上传一次，随意切换条件，无需重复提交。
 */

interface BatchImage {
  name: string;
  format: string;
  width: number;
  height: number;
  size: number;
  md5: string;
  make: string;
  model: string;
  shot_time: string;
  has_gps: boolean;
  gps_lat?: number;
  gps_lon?: number;
  location: string; // "中国 · 省份" / "国外" / ""（无 GPS）
}

interface BatchSummary {
  total: number;
  analyzed: number;
  skipped: number;
  domestic: number;
  abroad: number;
  no_gps: number;
  total_size: number;
  formats: Record<string, number>;
  provinces: Record<string, number>;
}

interface BatchPayload {
  summary: BatchSummary;
  images: BatchImage[];
  skipped_files?: { name: string; error: string }[];
}

const inputCls =
  'rounded-md border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200';

function fmtSize(n: number): string {
  if (n >= 1 << 20) return `${(n / (1 << 20)).toFixed(1)} MB`;
  if (n >= 1 << 10) return `${(n / 1024).toFixed(0)} KB`;
  return `${n} B`;
}

/** 解析任务 stdout 中的 JSON；失败回退原文展示 */
function parsePayload(data: unknown): BatchPayload | null {
  if (typeof data !== 'string') return null;
  try {
    const p = JSON.parse(data);
    if (p && Array.isArray(p.images) && p.summary) return p as BatchPayload;
  } catch {
    /* fallthrough */
  }
  return null;
}

export default function ImageBatchRenderer({ data }: RendererProps) {
  const payload = useMemo(() => parsePayload(data), [data]);

  const [locFilter, setLocFilter] = useState('all'); // all|domestic|abroad|nogps
  const [province, setProvince] = useState('');
  const [format, setFormat] = useState('all');
  const [camera, setCamera] = useState('');
  const [minW, setMinW] = useState('');
  const [maxW, setMaxW] = useState('');
  const [minH, setMinH] = useState('');
  const [maxH, setMaxH] = useState('');

  if (!payload) {
    return (
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs text-slate-700 dark:bg-slate-900 dark:text-slate-300">
        {String(data)}
      </pre>
    );
  }

  const { summary, images, skipped_files } = payload;
  const formats = Array.from(new Set(images.map((i) => i.format))).sort();
  const provinces = Array.from(
    new Set(
      images
        .map((i) => i.location)
        .filter((l) => l.startsWith('中国 · '))
        .map((l) => l.slice('中国 · '.length)),
    ),
  ).sort();

  const num = (s: string): number | null => {
    if (s.trim() === '') return null;
    const n = Number(s);
    return Number.isFinite(n) ? n : null;
  };

  const filtered = images.filter((img) => {
    switch (locFilter) {
      case 'domestic':
        if (!img.location.startsWith('中国 · ')) return false;
        break;
      case 'abroad':
        if (img.location !== '国外') return false;
        break;
      case 'nogps':
        if (img.has_gps) return false;
        break;
    }
    if (province && !img.location.includes(province)) return false;
    if (format !== 'all' && img.format !== format) return false;
    if (camera) {
      const q = camera.trim().toLowerCase();
      if (q && !`${img.make} ${img.model}`.toLowerCase().includes(q)) return false;
    }
    const minw = num(minW);
    const maxw = num(maxW);
    const minh = num(minH);
    const maxh = num(maxH);
    if (minw !== null && img.width < minw) return false;
    if (maxw !== null && img.width > maxw) return false;
    if (minh !== null && img.height < minh) return false;
    if (maxh !== null && img.height > maxh) return false;
    return true;
  });

  const toCsv = () => {
    const header = '文件名,格式,宽度,高度,大小(字节),制造商,型号,拍摄时间,GPS纬度,GPS经度,拍摄位置';
    const body = filtered
      .map((r) =>
        [
          r.name,
          r.format,
          r.width,
          r.height,
          r.size,
          r.make,
          r.model,
          r.shot_time,
          r.gps_lat ?? '',
          r.gps_lon ?? '',
          r.location,
        ]
          .map((v) => JSON.stringify(String(v)))
          .join(','),
      )
      .join('\n');
    return `${header}\n${body}`;
  };

  const reset = () => {
    setLocFilter('all');
    setProvince('');
    setFormat('all');
    setCamera('');
    setMinW('');
    setMaxW('');
    setMinH('');
    setMaxH('');
  };

  return (
    <div className="space-y-3">
      {/* —— 汇总 —— */}
      <div className="flex flex-wrap gap-2">
        <span className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-1">
          共 {summary.analyzed} 张<span className="ml-1 text-ink-3">（跳过 {summary.skipped}）</span>
        </span>
        <span className="rounded-full border border-emerald-300 bg-emerald-50 px-3 py-1 text-xs text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-400">
          国内 {summary.domestic}
        </span>
        <span className="rounded-full border border-amber-300 bg-amber-50 px-3 py-1 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400">
          国外 {summary.abroad}
        </span>
        <span className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
          无 GPS {summary.no_gps}
        </span>
        <span className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-2">
          总大小 {fmtSize(summary.total_size)}
        </span>
        {Object.entries(summary.formats ?? {})
          .sort((a, b) => b[1] - a[1])
          .map(([f, n]) => (
            <span
              key={f}
              className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-2"
            >
              {f} × {n}
            </span>
          ))}
      </div>

      {/* —— 筛选栏 —— */}
      <div className="rounded-lg border border-slate-200 p-3 dark:border-slate-800">
        <div className="flex flex-wrap items-end gap-2">
          <label className="flex flex-col gap-0.5 text-xs text-slate-500 dark:text-slate-400">
            位置
            <select
              value={locFilter}
              onChange={(e) => setLocFilter(e.target.value)}
              className={inputCls}
            >
              <option value="all">全部</option>
              <option value="domestic">国内</option>
              <option value="abroad">国外</option>
              <option value="nogps">无 GPS</option>
            </select>
          </label>
          <label className="flex flex-col gap-0.5 text-xs text-slate-500 dark:text-slate-400">
            省份
            <select
              value={province}
              onChange={(e) => setProvince(e.target.value)}
              className={inputCls}
            >
              <option value="">全部</option>
              {provinces.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-0.5 text-xs text-slate-500 dark:text-slate-400">
            格式
            <select
              value={format}
              onChange={(e) => setFormat(e.target.value)}
              className={inputCls}
            >
              <option value="all">全部</option>
              {formats.map((f) => (
                <option key={f} value={f}>
                  {f}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-0.5 text-xs text-slate-500 dark:text-slate-400">
            机型（关键字）
            <input
              value={camera}
              onChange={(e) => setCamera(e.target.value)}
              placeholder="如 Canon / iPhone"
              className={`${inputCls} w-32`}
            />
          </label>
          <span className="flex items-end gap-2 text-xs text-slate-500 dark:text-slate-400">
            <label className="flex flex-col gap-0.5">
              最小宽
              <input value={minW} onChange={(e) => setMinW(e.target.value)} placeholder="不限" className={`${inputCls} w-20`} />
            </label>
            <label className="flex flex-col gap-0.5">
              最大宽
              <input value={maxW} onChange={(e) => setMaxW(e.target.value)} placeholder="不限" className={`${inputCls} w-20`} />
            </label>
            <label className="flex flex-col gap-0.5">
              最小高
              <input value={minH} onChange={(e) => setMinH(e.target.value)} placeholder="不限" className={`${inputCls} w-20`} />
            </label>
            <label className="flex flex-col gap-0.5">
              最大高
              <input value={maxH} onChange={(e) => setMaxH(e.target.value)} placeholder="不限" className={`${inputCls} w-20`} />
            </label>
          </span>
          <button
            type="button"
            onClick={reset}
            className="rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          >
            重置
          </button>
          <span className="ml-auto flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
            匹配 {filtered.length} / {images.length}
            <DownloadButton text={toCsv()} filename="filtered_images.csv" mime="text/csv" />
          </span>
        </div>
      </div>

      {/* —— 结果表 —— */}
      <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
        <div className="flex items-center justify-end gap-1 border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
          <CopyButton text={JSON.stringify(filtered, null, 2)} />
        </div>
        <div className="max-h-96 overflow-auto">
          <table className="min-w-full text-sm">
            <thead className="sticky top-0 bg-slate-50 dark:bg-slate-900">
              <tr>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">文件名</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">格式</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">尺寸</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">大小</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">机型</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">拍摄时间</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">拍摄位置</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">GPS</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((r, i) => (
                <tr key={`${r.name}-${i}`} className="border-t border-slate-100 dark:border-slate-800">
                  <td className="max-w-56 truncate px-3 py-1.5 text-slate-700 dark:text-slate-300" title={r.name}>
                    {r.name}
                  </td>
                  <td className="whitespace-nowrap px-3 py-1.5 text-slate-700 dark:text-slate-300">{r.format}</td>
                  <td className="whitespace-nowrap px-3 py-1.5 text-slate-700 dark:text-slate-300">
                    {r.width > 0 ? `${r.width} × ${r.height}` : '-'}
                  </td>
                  <td className="whitespace-nowrap px-3 py-1.5 text-slate-700 dark:text-slate-300">{fmtSize(r.size)}</td>
                  <td className="whitespace-nowrap px-3 py-1.5 text-slate-700 dark:text-slate-300">
                    {[r.make, r.model].filter(Boolean).join(' ') || '-'}
                  </td>
                  <td className="whitespace-nowrap px-3 py-1.5 text-slate-700 dark:text-slate-300">{r.shot_time || '-'}</td>
                  <td className="whitespace-nowrap px-3 py-1.5">
                    {r.location === '' ? (
                      <span className="text-slate-400">-</span>
                    ) : r.location === '国外' ? (
                      <span className="text-amber-600 dark:text-amber-400">{r.location}</span>
                    ) : (
                      <span className="text-emerald-600 dark:text-emerald-400">{r.location}</span>
                    )}
                  </td>
                  <td className="whitespace-nowrap px-3 py-1.5 font-mono text-xs text-slate-500 dark:text-slate-400">
                    {r.has_gps ? `${r.gps_lat?.toFixed(4)}, ${r.gps_lon?.toFixed(4)}` : '-'}
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-3 py-6 text-center text-slate-400">
                    没有匹配当前筛选条件的图片
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

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
