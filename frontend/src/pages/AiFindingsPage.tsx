import { useEffect, useMemo, useState } from 'react';
import { Download, RefreshCw, ShieldAlert } from 'lucide-react';
import { Link } from 'react-router-dom';

import type { AIFindingDict } from '@/lib/backend';
import { listAIFindings } from '@/lib/backend';

const SEV_ORDER = ['critical', 'high', 'medium', 'low', 'info'];
const SEV_LABEL: Record<string, string> = {
  critical: '严重', high: '高危', medium: '中危', low: '低危', info: '提示',
};
const SEV_STYLE: Record<string, string> = {
  critical: 'bg-rose-600 text-white',
  high: 'bg-rose-100 text-rose-600 dark:bg-rose-500/15 dark:text-rose-400',
  medium: 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400',
  low: 'bg-sky-100 text-sky-600 dark:bg-sky-500/15 dark:text-sky-400',
  info: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
};

function downloadJSON(name: string, data: unknown) {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

/**
 * AI 发现（/ai-findings）：跨任务漏洞聚合，按严重度/关键字过滤，
 * 证据可展开，支持 JSON 导出。
 */
export default function AiFindingsPage() {
  const [items, setItems] = useState<AIFindingDict[]>([]);
  const [sev, setSev] = useState('all');
  const [kw, setKw] = useState('');
  const [error, setError] = useState('');

  const refresh = () => {
    listAIFindings()
      .then((r) => {
        setItems(r ?? []);
        setError('');
      })
      .catch((e) => setError(String(e)));
  };

  useEffect(() => {
    document.title = 'CTF-NET — AI 发现';
    refresh();
    const t = setInterval(refresh, 10000);
    return () => clearInterval(t);
  }, []);

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const it of items) {
      const s = it.finding?.severity ?? 'unknown';
      c[s] = (c[s] ?? 0) + 1;
    }
    return c;
  }, [items]);

  const shown = useMemo(() => {
    const k = kw.trim().toLowerCase();
    return items.filter(
      (it) =>
        (sev === 'all' || (it.finding?.severity ?? 'unknown') === sev) &&
        (!k ||
          (it.finding?.title ?? '').toLowerCase().includes(k) ||
          (it.finding?.vuln_class ?? '').toLowerCase().includes(k) ||
          (it.task_desc ?? '').toLowerCase().includes(k)),
    );
  }, [items, sev, kw]);

  return (
    <div className="p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <ShieldAlert className="h-6 w-6 text-rose-500" />
            AI 发现
          </h1>
          <p className="mt-1 text-slate-500 dark:text-slate-400">AI 渗透任务上报的漏洞（跨任务聚合）</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => downloadJSON(`ctfnet-ai-findings-${new Date().toISOString().slice(0, 10)}.json`, shown)}
            className="flex items-center gap-1.5 rounded-md border border-slate-200 px-3 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          >
            <Download className="h-3.5 w-3.5" /> 导出 JSON
          </button>
          <button
            type="button"
            onClick={refresh}
            className="flex items-center gap-1.5 rounded-md border border-slate-200 px-3 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          >
            <RefreshCw className="h-3.5 w-3.5" /> 刷新
          </button>
        </div>
      </div>

      {error && <div className="mt-3 rounded-md bg-rose-50 p-3 text-sm text-rose-600">{error}</div>}

      {/* 过滤行 */}
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => setSev('all')}
          className={`rounded-full px-3 py-1 text-sm ${sev === 'all' ? 'bg-slate-800 text-white dark:bg-slate-200 dark:text-slate-900' : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300'}`}
        >
          全部（{items.length}）
        </button>
        {SEV_ORDER.map((s) =>
          (counts[s] ?? 0) > 0 ? (
            <button
              key={s}
              type="button"
              onClick={() => setSev(s)}
              className={`rounded-full px-3 py-1 text-sm ${sev === s ? 'bg-slate-800 text-white dark:bg-slate-200 dark:text-slate-900' : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300'}`}
            >
              {SEV_LABEL[s]}（{counts[s]}）
            </button>
          ) : null,
        )}
        <input
          value={kw}
          onChange={(e) => setKw(e.target.value)}
          placeholder="搜索标题 / 漏洞类型 / 任务…"
          className="ml-auto w-64 rounded-md border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
        />
      </div>

      {/* 列表 */}
      <div className="mt-3 overflow-hidden rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
        {shown.length === 0 ? (
          <div className="p-6 text-sm text-slate-400">暂无匹配的发现。</div>
        ) : (
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {shown.map((it, i) => {
              const f = it.finding ?? {};
              const sevKey = SEV_LABEL[f.severity ?? ''] ? (f.severity ?? 'info') : 'info';
              return (
                <li key={`${it.task_id}-${f.id}-${i}`} className="px-4 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${SEV_STYLE[sevKey] ?? SEV_STYLE.info}`}>
                      {SEV_LABEL[sevKey]}
                    </span>
                    <span className="text-sm font-medium text-slate-700 dark:text-slate-200">{f.title}</span>
                    {f.vuln_class && <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">{f.vuln_class}</span>}
                    <Link to={`/ai-task/${it.task_id}`} className="text-xs text-indigo-500 hover:underline">
                      {it.task_desc || it.task_id}
                    </Link>
                    <span className="ml-auto text-xs text-slate-400">
                      {f.confirmed && '✔ 已确认 · '}
                      {(f.created_at ?? '').replace('T', ' ').slice(0, 19)}
                    </span>
                  </div>
                  {f.summary && <div className="mt-1 text-sm text-slate-600 dark:text-slate-300">{f.summary}</div>}
                  {f.evidence && (
                    <details className="mt-1">
                      <summary className="cursor-pointer text-xs text-indigo-500">证据</summary>
                      <pre className="mt-1 max-h-64 overflow-y-auto whitespace-pre-wrap rounded bg-slate-50 p-2 text-xs text-slate-600 dark:bg-slate-800/60 dark:text-slate-300">{f.evidence}</pre>
                    </details>
                  )}
                  {f.poc && (
                    <details className="mt-1">
                      <summary className="cursor-pointer text-xs text-indigo-500">POC</summary>
                      <pre className="mt-1 max-h-64 overflow-y-auto whitespace-pre-wrap rounded bg-slate-50 p-2 text-xs text-slate-600 dark:bg-slate-800/60 dark:text-slate-300">{f.poc}</pre>
                    </details>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}
