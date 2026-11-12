import { useMemo, useState } from 'react';

import type { RendererProps } from './types';

import { CopyButton, DownloadButton } from '../OutputActions';

/**
 * 表格合并渲染器（rendererId = 'table-merge'）。
 * 后端 stdout 为 JSON：{ summary, columns, preview, notes }，
 * 关键字筛选在客户端完成；结果超 20000 行时仅内嵌前 20000 行，
 * 完整数据在任务产物 merged.csv 中。
 */

interface MergeSummary {
  file_a: string;
  file_b: string;
  meta_a: string;
  meta_b: string;
  rows_a: number;
  rows_b: number;
  key_a: string;
  key_b: string;
  how: string;
  columns: number;
  matched_pairs: number;
  left_only: number;
  right_only: number;
  result_rows: number;
  preview_rows: number;
}

interface MergePayload {
  summary: MergeSummary;
  columns: string[];
  preview: string[][];
  notes?: string[];
}

const inputCls =
  'rounded-md border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200';

function parsePayload(data: unknown): MergePayload | null {
  if (typeof data !== 'string') return null;
  try {
    const p = JSON.parse(data);
    if (p && Array.isArray(p.columns) && Array.isArray(p.preview) && p.summary) {
      return p as MergePayload;
    }
  } catch {
    /* fallthrough */
  }
  return null;
}

const HOW_LABELS: Record<string, string> = {
  inner: '内连接',
  left: '左连接',
  right: '右连接',
  outer: '全连接',
};

export default function TableMergeRenderer({ data }: RendererProps) {
  const payload = useMemo(() => parsePayload(data), [data]);
  const [keyword, setKeyword] = useState('');

  if (!payload) {
    return (
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs text-slate-700 dark:bg-slate-900 dark:text-slate-300">
        {String(data)}
      </pre>
    );
  }

  const { summary, columns, preview, notes } = payload;
  const kw = keyword.trim().toLowerCase();
  const filtered = kw
    ? preview.filter((row) => row.some((cell) => String(cell).toLowerCase().includes(kw)))
    : preview;

  // 客户端导出 CSV（带 BOM，Excel 打开中文不乱码）
  const toCsv = () => {
    const esc = (v: string) => JSON.stringify(String(v ?? ''));
    const body = [columns.map(esc).join(',')]
      .concat(filtered.map((row) => row.map(esc).join(',')))
      .join('\n');
    return `﻿${body}`;
  };

  return (
    <div className="space-y-3">
      {/* —— 汇总 —— */}
      <div className="flex flex-wrap gap-2">
        <span className="rounded-full border border-indigo-300 bg-indigo-50 px-3 py-1 text-xs text-indigo-700 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-400">
          {HOW_LABELS[summary.how] ?? summary.how} · 结果 {summary.result_rows} 行 × {summary.columns} 列
        </span>
        <span className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-1">
          左表 {summary.file_a}（{summary.rows_a} 行）
        </span>
        <span className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-1">
          右表 {summary.file_b}（{summary.rows_b} 行）
        </span>
        <span className="rounded-full border border-emerald-300 bg-emerald-50 px-3 py-1 text-xs text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-400">
          匹配对 {summary.matched_pairs}
        </span>
        <span className="rounded-full border border-amber-300 bg-amber-50 px-3 py-1 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400">
          仅左 {summary.left_only}
        </span>
        <span className="rounded-full border border-rose-300 bg-rose-50 px-3 py-1 text-xs text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-400">
          仅右 {summary.right_only}
        </span>
        <span className="rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-3">
          {summary.meta_a} × {summary.meta_b}
        </span>
      </div>

      {/* —— 筛选栏 —— */}
      <div className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 p-3 dark:border-slate-800">
        <label className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          内容关键字
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="任意单元格包含即命中"
            className={`${inputCls} w-56`}
          />
        </label>
        <span className="ml-auto flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          显示 {filtered.length} / {preview.length} 行
          <DownloadButton text={toCsv()} filename="merged.csv" mime="text/csv" />
        </span>
      </div>

      {/* —— 结果表 —— */}
      <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
        <div className="flex items-center justify-end gap-1 border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
          <CopyButton text={JSON.stringify({ summary, columns, preview }, null, 2)} />
        </div>
        <div className="max-h-96 overflow-auto">
          <table className="min-w-full text-sm">
            <thead className="sticky top-0 z-10 bg-slate-50 dark:bg-slate-900">
              <tr>
                {columns.map((c, i) => (
                  <th
                    key={`${c}-${i}`}
                    className="whitespace-nowrap border-b border-slate-200 px-3 py-2 text-left font-medium text-slate-600 dark:border-slate-700 dark:text-slate-300"
                  >
                    {c}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((row, i) => (
                <tr key={i} className="border-t border-slate-100 dark:border-slate-800">
                  {columns.map((_, j) => (
                    <td
                      key={j}
                      className="max-w-64 truncate px-3 py-1.5 text-slate-700 dark:text-slate-300"
                      title={String(row[j] ?? '')}
                    >
                      {row[j] ?? ''}
                    </td>
                  ))}
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={columns.length} className="px-3 py-6 text-center text-slate-400">
                    没有匹配当前筛选条件的行
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* —— 备注 —— */}
      {(notes?.length ?? 0) > 0 && (
        <ul className="space-y-1 rounded-lg border border-slate-200 p-3 text-xs text-slate-500 dark:border-slate-800 dark:text-slate-400">
          {notes!.map((n, i) => (
            <li key={i}>· {n}</li>
          ))}
        </ul>
      )}
    </div>
  );
}
