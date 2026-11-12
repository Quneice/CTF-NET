import { useMemo, useState } from 'react';

import type { RendererProps } from './types';

import { CopyButton, DownloadButton } from '../OutputActions';

/**
 * 图片二维码识别渲染器（rendererId = 'image-qrcode'）。
 * 后端 stdout 为 JSON：{ summary, images[{name, format, codes[{text, format}]}], skipped_files }，
 * 筛选（仅含有二维码、内容关键字）在客户端完成，上传一次随意切换。
 */

interface QrCodeOut {
  text: string;
  format: string; // "QR Code" / "Data Matrix" / "Aztec"
}

interface QrcodeFileRecord {
  name: string;
  format: string;
  codes: QrCodeOut[];
}

interface QrcodeSummary {
  total: number;
  analyzed: number;
  skipped: number;
  with_qr: number;
  no_qr: number;
  total_codes: number;
  formats: Record<string, number>;
}

interface QrcodePayload {
  summary: QrcodeSummary;
  images: QrcodeFileRecord[];
  skipped_files?: { name: string; error: string }[];
}

const inputCls =
  'rounded-md border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200';

function parsePayload(data: unknown): QrcodePayload | null {
  if (typeof data !== 'string') return null;
  try {
    const p = JSON.parse(data);
    if (p && Array.isArray(p.images) && p.summary) return p as QrcodePayload;
  } catch {
    /* fallthrough */
  }
  return null;
}

/** 展开成扁平行：一条记录一枚码 */
interface FlatRow {
  name: string;
  imgFormat: string;
  codeIndex: number;
  codeFormat: string;
  text: string;
}

export default function ImageQrcodeRenderer({ data }: RendererProps) {
  const payload = useMemo(() => parsePayload(data), [data]);
  const [onlyWithQR, setOnlyWithQR] = useState(false);
  const [keyword, setKeyword] = useState('');

  if (!payload) {
    return (
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs text-slate-700 dark:bg-slate-900 dark:text-slate-300">
        {String(data)}
      </pre>
    );
  }

  const { summary, images, skipped_files } = payload;

  const rows: FlatRow[] = images
    .filter((img) => !onlyWithQR || img.codes.length > 0)
    .flatMap((img) =>
      (img.codes ?? []).map((c, i) => ({
        name: img.name,
        imgFormat: img.format,
        codeIndex: i,
        codeFormat: c.format,
        text: c.text,
      })),
    );
  const kw = keyword.trim().toLowerCase();
  const filtered = kw ? rows.filter((r) => r.text.toLowerCase().includes(kw)) : rows;

  const toCsv = () => {
    const header = '文件名,图片格式,序号,二维码格式,内容';
    const body = filtered
      .map((r) => [r.name, r.imgFormat, r.codeIndex + 1, r.codeFormat, r.text].map((v) => JSON.stringify(String(v))).join(','))
      .join('\n');
    return `${header}\n${body}`;
  };

  const visibleFiles = onlyWithQR ? images.filter((i) => (i.codes?.length ?? 0) > 0) : images;
  const matchedCount = kw
    ? visibleFiles.reduce((n, i) => n + (i.codes ?? []).filter((c) => c.text.toLowerCase().includes(kw)).length, 0)
    : summary.total_codes;

  return (
    <div className="space-y-3">
      {/* —— 汇总 —— */}
      <div className="flex flex-wrap gap-2">
        <span className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-1">
          共 {summary.analyzed} 张<span className="ml-1 text-ink-3">（跳过 {summary.skipped}）</span>
        </span>
        <span className="rounded-full border border-emerald-300 bg-emerald-50 px-3 py-1 text-xs text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-400">
          含二维码 {summary.with_qr}
        </span>
        <span className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
          无二维码 {summary.no_qr}
        </span>
        <span className="rounded-full border border-indigo-300 bg-indigo-50 px-3 py-1 text-xs text-indigo-700 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-400">
          二维码总数 {summary.total_codes}
        </span>
        {Object.entries(summary.formats ?? {})
          .sort((a, b) => b[1] - a[1])
          .map(([f, n]) => (
            <span key={f} className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-2">
              {f} × {n}
            </span>
          ))}
      </div>

      {/* —— 筛选栏 —— */}
      <div className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 p-3 dark:border-slate-800">
        <label className="flex cursor-pointer items-center gap-1.5 text-xs text-slate-600 dark:text-slate-300">
          <input type="checkbox" checked={onlyWithQR} onChange={(e) => setOnlyWithQR(e.target.checked)} className="h-3.5 w-3.5" />
          仅显示含二维码的图片
        </label>
        <label className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          内容关键字
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="如 flag{ / http://"
            className={`${inputCls} w-56`}
          />
        </label>
        <span className="ml-auto flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          匹配 {matchedCount} / {summary.total_codes} 个码
          <DownloadButton text={toCsv()} filename="qrcode_contents.csv" mime="text/csv" />
        </span>
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
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">图片格式</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">码格式</th>
                <th className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300">内容</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((r, i) => (
                <tr key={`${r.name}-${r.codeIndex}-${i}`} className="border-t border-slate-100 dark:border-slate-800">
                  <td className="max-w-56 truncate px-3 py-1.5 text-slate-700 dark:text-slate-300" title={r.name}>
                    {r.name}
                  </td>
                  <td className="whitespace-nowrap px-3 py-1.5 text-slate-500 dark:text-slate-400">{r.imgFormat}</td>
                  <td className="whitespace-nowrap px-3 py-1.5">
                    <span className="rounded-full border border-indigo-200 bg-indigo-50 px-2 py-0.5 text-xs text-indigo-700 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-400">
                      {r.codeFormat}
                    </span>
                  </td>
                  <td className="max-w-xl px-3 py-1.5">
                    <span className="block max-w-full break-all font-mono text-xs text-slate-700 dark:text-slate-300">{r.text}</span>
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-3 py-6 text-center text-slate-400">
                    没有匹配当前筛选条件的二维码
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
