import { useEffect, useState } from 'react';
import { LayoutDashboard, RefreshCw } from 'lucide-react';
import { Link } from 'react-router-dom';

import type { AIActivityDict, AIStatsDict, AITaskSummaryDict } from '@/lib/backend';
import { getAIStats, listAIActivities, listAITasks } from '@/lib/backend';

const STATUS_LABEL: Record<string, string> = {
  created: '待运行', running: '运行中', done: '已完成', exhausted: '已耗尽',
  failed: '失败', timeout: '超时', stopped: '已停止',
};
const STATUS_STYLE: Record<string, string> = {
  running: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400',
  done: 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400',
  exhausted: 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400',
  failed: 'bg-rose-50 text-rose-600 dark:bg-rose-500/10 dark:text-rose-400',
  timeout: 'bg-rose-50 text-rose-600 dark:bg-rose-500/10 dark:text-rose-400',
  stopped: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
};

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return String(n);
}

function Card({ label, value, hint, color }: { label: string; value: string | number; hint?: string; color: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
      <div className="text-sm text-slate-500 dark:text-slate-400">{label}</div>
      <div className={`mt-1 text-2xl font-semibold ${color}`}>{value}</div>
      {hint && <div className="mt-1 text-xs text-slate-400">{hint}</div>}
    </div>
  );
}

/**
 * AI 仪表盘（/ai-dashboard）：任务总览 / token 消耗 / 活动流。
 * 数据源：后端跨任务聚合（aiagent.GlobalStats / ScanAITasks / RecentActivities），5s 轮询。
 */
export default function AiDashboardPage() {
  const [stats, setStats] = useState<AIStatsDict>({});
  const [tasks, setTasks] = useState<AITaskSummaryDict[]>([]);
  const [acts, setActs] = useState<AIActivityDict[]>([]);
  const [error, setError] = useState('');

  const refresh = () => {
    Promise.all([getAIStats(), listAITasks(), listAIActivities(30)])
      .then(([s, t, a]) => {
        setStats(s);
        setTasks(t ?? []); // Wails 会把 Go 的 nil 切片序列化成 null，兜底空数组
        setActs(a ?? []);
        setError('');
      })
      .catch((e) => setError(String(e)));
  };

  useEffect(() => {
    document.title = 'CTF-NET — AI 仪表盘';
    refresh();
    const t = setInterval(refresh, 5000);
    return () => clearInterval(t);
  }, []);

  const usageIn = stats.usage?.in ?? 0;
  const usageOut = stats.usage?.out ?? 0;
  const maxDay = Math.max(1, ...(stats.by_day ?? []).map((d) => d.usage.in + d.usage.out));

  return (
    <div className="p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <LayoutDashboard className="h-6 w-6 text-indigo-500" />
            AI 仪表盘
          </h1>
          <p className="mt-1 text-slate-500 dark:text-slate-400">
            AI 渗透任务总览、token 消耗与跨任务活动流
          </p>
        </div>
        <button
          type="button"
          onClick={refresh}
          className="flex items-center gap-1.5 rounded-md border border-slate-200 px-3 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          刷新
        </button>
      </div>

      {error && <div className="mt-4 rounded-md bg-rose-50 p-3 text-sm text-rose-600">{error}</div>}

      {/* 总览卡 */}
      <div className="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7">
        <Card label="任务总数" value={stats.tasks ?? 0} color="text-slate-800 dark:text-slate-100"
          hint={`运行中 ${stats.running ?? 0} · 暂停 ${stats.paused ?? 0}`} />
        <Card label="已完成" value={stats.done ?? 0} color="text-emerald-600" hint={`耗尽 ${stats.exhausted ?? 0} · 失败 ${stats.failed ?? 0}`} />
        <Card label="漏洞发现" value={stats.findings ?? 0} color="text-rose-600" />
        <Card label="资产" value={stats.assets ?? 0} color="text-indigo-600" />
        <Card label="事实" value={stats.facts ?? 0} color="text-slate-600" />
        <Card label="意图" value={stats.intents ?? 0} color="text-amber-600" />
        <Card label="Token 消耗" value={fmtTokens(usageIn + usageOut)} color="text-violet-600"
          hint={`输入 ${fmtTokens(usageIn)} / 输出 ${fmtTokens(usageOut)}`} />
      </div>

      {/* 按天 token 用量（纯 CSS 条形，零图表依赖；蓝=输入 / 橙=输出，校验配色） */}
      {(stats.by_day?.length ?? 0) > 0 && (
        <div className="mt-4 rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
          <div className="mb-3 flex items-center gap-3 text-sm font-medium text-slate-600 dark:text-slate-300">
            <span>Token 消耗（按天）</span>
            <span className="flex items-center gap-1.5 text-xs font-normal text-slate-500">
              <span className="inline-block h-2 w-2 rounded-sm bg-[#2a78d6] dark:bg-[#3987e5]" />输入
              <span className="ml-2 inline-block h-2 w-2 rounded-sm bg-[#eb6834] dark:bg-[#d95926]" />输出
            </span>
          </div>
          <div className="space-y-1.5">
            {(stats.by_day ?? []).map((d) => {
              const total = d.usage.in + d.usage.out;
              return (
                <div key={d.day} className="flex items-center gap-2 text-xs">
                  <div className="w-24 shrink-0 text-slate-500">{d.day.slice(5)}</div>
                  <div className="flex h-4 flex-1 gap-0.5 overflow-hidden rounded bg-slate-100 dark:bg-slate-800">
                    <div className="rounded-l bg-[#2a78d6] dark:bg-[#3987e5]" style={{ width: `${(d.usage.in / maxDay) * 100}%` }} title={`输入 ${fmtTokens(d.usage.in)}`} />
                    <div className="rounded-r bg-[#eb6834] dark:bg-[#d95926]" style={{ width: `${(d.usage.out / maxDay) * 100}%` }} title={`输出 ${fmtTokens(d.usage.out)}`} />
                  </div>
                  <div className="w-20 shrink-0 text-right text-slate-500">{fmtTokens(total)}</div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        {/* 任务列表 */}
        <div className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium dark:border-slate-800">任务</div>
          {tasks.length === 0 ? (
            <div className="p-4 text-sm text-slate-400">
              暂无任务。在「AI 对话」中让助手发起自动化任务，或运行工具 ai.pentest.task。
            </div>
          ) : (
            <ul className="divide-y divide-slate-100 dark:divide-slate-800">
              {tasks.slice(0, 8).map((t) => (
                <li key={t.id}>
                  <Link to={`/ai-task/${t.id}`} className="block px-4 py-2.5 hover:bg-slate-50 dark:hover:bg-slate-800/50">
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-sm text-slate-700 dark:text-slate-200">
                        {t.description || t.goal_text || t.id}
                      </span>
                      <span className={`shrink-0 rounded px-1.5 py-0.5 text-xs ${STATUS_STYLE[t.status ?? ''] ?? ''}`}>
                        {t.paused ? '已暂停' : STATUS_LABEL[t.status ?? ''] ?? t.status}
                      </span>
                    </div>
                    <div className="mt-0.5 text-xs text-slate-400">
                      漏洞 {t.findings ?? 0} · 资产 {t.assets ?? 0} · 意图 {t.intents ?? 0}
                      {(t.usage?.in ?? 0) + (t.usage?.out ?? 0) > 0 && ` · ${fmtTokens((t.usage?.in ?? 0) + (t.usage?.out ?? 0))} token`}
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* 活动流 */}
        <div className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium dark:border-slate-800">活动流</div>
          {acts.length === 0 ? (
            <div className="p-4 text-sm text-slate-400">暂无活动。</div>
          ) : (
            <ul className="max-h-[480px] divide-y divide-slate-100 overflow-y-auto dark:divide-slate-800">
              {acts.map((a, i) => (
                <li key={`${a.task_id}-${a.activity?.seq}-${i}`} className="px-4 py-2 text-xs">
                  <div className="flex items-center gap-2">
                    <span className="shrink-0 text-slate-400">
                      {(a.activity?.ts ?? '').replace('T', ' ').slice(5, 16)}
                    </span>
                    <span className="shrink-0 rounded bg-slate-100 px-1 text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                      {a.activity?.agent ?? 'system'}
                    </span>
                    <Link to={`/ai-task/${a.task_id}`} className="shrink-0 text-indigo-500 hover:underline">
                      {a.task_desc || a.task_id}
                    </Link>
                  </div>
                  <div className="mt-0.5 text-slate-600 dark:text-slate-300">{a.activity?.summary}</div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}
