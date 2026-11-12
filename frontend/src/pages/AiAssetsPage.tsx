import { useEffect, useMemo, useState } from 'react';
import { Download, Globe, RefreshCw } from 'lucide-react';
import { Link } from 'react-router-dom';

import type { AIAssetDict } from '@/lib/backend';
import { listAIAssets } from '@/lib/backend';

const TYPE_LABEL: Record<string, string> = {
  url: 'URL', domain: '域名', ip: 'IP', service: '服务', host: '主机', other: '其他',
};
const TYPE_STYLE: Record<string, string> = {
  url: 'bg-indigo-100 text-indigo-600 dark:bg-indigo-500/15 dark:text-indigo-400',
  domain: 'bg-sky-100 text-sky-600 dark:bg-sky-500/15 dark:text-sky-400',
  ip: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300',
  service: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400',
  host: 'bg-violet-100 text-violet-600 dark:bg-violet-500/15 dark:text-violet-400',
  other: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
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

/** 资产主标识（URL/域名/IP 按类型取值）。 */
function assetLabel(a: AIAssetDict['asset']): string {
  if (!a) return '';
  return a.url || a.domain || a.ip || `#${a.id ?? ''}`;
}

/**
 * AI 资产（/ai-assets）：跨任务资产聚合，按类型/关键字过滤，JSON 导出。
 */
export default function AiAssetsPage() {
  const [items, setItems] = useState<AIAssetDict[]>([]);
  const [type, setType] = useState('all');
  const [kw, setKw] = useState('');
  const [error, setError] = useState('');

  const refresh = () => {
    listAIAssets()
      .then((r) => {
        setItems(r ?? []);
        setError('');
      })
      .catch((e) => setError(String(e)));
  };

  useEffect(() => {
    document.title = 'CTF-NET — AI 资产';
    refresh();
    const t = setInterval(refresh, 10000);
    return () => clearInterval(t);
  }, []);

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const it of items) {
      const t = it.asset?.type ?? 'other';
      c[t] = (c[t] ?? 0) + 1;
    }
    return c;
  }, [items]);

  const shown = useMemo(() => {
    const k = kw.trim().toLowerCase();
    return items.filter(
      (it) =>
        (type === 'all' || (it.asset?.type ?? 'other') === type) &&
        (!k ||
          assetLabel(it.asset).toLowerCase().includes(k) ||
          (it.task_desc ?? '').toLowerCase().includes(k)),
    );
  }, [items, type, kw]);

  const allTypes = [...new Set(items.map((it) => it.asset?.type ?? 'other'))];

  return (
    <div className="p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <Globe className="h-6 w-6 text-indigo-500" />
            AI 资产
          </h1>
          <p className="mt-1 text-slate-500 dark:text-slate-400">AI 渗透任务识别出的资产（跨任务聚合）</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => downloadJSON(`ctfnet-ai-assets-${new Date().toISOString().slice(0, 10)}.json`, shown)}
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
          onClick={() => setType('all')}
          className={`rounded-full px-3 py-1 text-sm ${type === 'all' ? 'bg-slate-800 text-white dark:bg-slate-200 dark:text-slate-900' : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300'}`}
        >
          全部（{items.length}）
        </button>
        {allTypes.map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setType(t)}
            className={`rounded-full px-3 py-1 text-sm ${type === t ? 'bg-slate-800 text-white dark:bg-slate-200 dark:text-slate-900' : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300'}`}
          >
            {TYPE_LABEL[t] ?? t}（{counts[t] ?? 0}）
          </button>
        ))}
        <input
          value={kw}
          onChange={(e) => setKw(e.target.value)}
          placeholder="搜索资产 / 任务…"
          className="ml-auto w-64 rounded-md border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
        />
      </div>

      {/* 列表 */}
      <div className="mt-3 overflow-hidden rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
        {shown.length === 0 ? (
          <div className="p-6 text-sm text-slate-400">暂无匹配的资产。</div>
        ) : (
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {shown.map((it, i) => {
              const a = it.asset ?? {};
              const t = a.type ?? 'other';
              const props = Object.entries(a.props ?? {});
              return (
                <li key={`${it.task_id}-${a.id}-${i}`} className="px-4 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${TYPE_STYLE[t] ?? TYPE_STYLE.other}`}>
                      {TYPE_LABEL[t] ?? t}
                    </span>
                    <span className="break-all font-mono text-sm text-slate-700 dark:text-slate-200">{assetLabel(a)}</span>
                    <Link to={`/ai-task/${it.task_id}`} className="text-xs text-indigo-500 hover:underline">
                      {it.task_desc || it.task_id}
                    </Link>
                    <span className="ml-auto text-xs text-slate-400">{a.added_by ? `${a.added_by} 发现` : ''}</span>
                  </div>
                  {props.length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-slate-500 dark:text-slate-400">
                      {props.slice(0, 8).map(([k, v]) => (
                        <span key={k}>
                          <span className="text-slate-400">{k}:</span> {v}
                        </span>
                      ))}
                      {props.length > 8 && <span>…</span>}
                    </div>
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
