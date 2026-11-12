import { useCallback, useEffect, useMemo, useState } from 'react';
import { ArrowLeft, Pause, Play, RefreshCw, RotateCcw, Square, Navigation } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';

import type { AITaskDetailDict } from '@/lib/backend';
import { controlAIIntent, controlAITask, getAITask } from '@/lib/backend';

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
const INTENT_STYLE: Record<string, string> = {
  open: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
  running: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400',
  done: 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400',
  exhausted: 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400',
  blocked: 'bg-rose-50 text-rose-600 dark:bg-rose-500/10 dark:text-rose-400',
  stopped: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
};

const REL_LABEL: Record<string, string> = {
  yields: '产生', spawns: '派生', targets: '目标指向', proves: '证明目标', affects: '影响资产',
};

/** 探索图：按节点类型分列（目标 → 意图 → 事实/资产/漏洞）的 SVG 分层图。
 * 节点色为校验过的品类色（品红/蓝/青/橙，明暗双主题），事实为中性灰；边统一灰线，
 * 关系语义靠 title 提示 + 图例文本（颜色不单独承载语义）。 */
function GraphView({ task }: { task: AITaskDetailDict }) {
  const styles = (
    <style>{`
      .graph-root { --node-finding:#e87ba4; --node-intent:#2a78d6; --node-goal:#1baf7a;
        --node-asset:#eb6834; --node-fact:#898781; --edge-line:#c3c2b7; --edge-text:#52514e; }
      html.dark .graph-root { --node-finding:#d55181; --node-intent:#3987e5; --node-goal:#199e70;
        --node-asset:#d95926; --node-fact:#898781; --edge-line:#898781; --edge-text:#c3c2b7; }
    `}</style>
  );
  const { nodes, edges } = useMemo(() => {
    const nodes = new Map<string, { x: number; y: number; label: string; kind: string }>();
    let y = 0;
    for (const g of task.goals ?? []) {
      nodes.set(`goal:${g.id}`, { x: 70, y: 30 + y * 54, label: `${g.met ? '✓ ' : ''}${g.text}`, kind: 'goal' });
      y++;
    }
    y = 0;
    for (const it of task.intents ?? []) {
      nodes.set(`intent:${it.id}`, { x: 330, y: 20 + y * 44, label: `#${it.id} ${it.summary}`, kind: 'intent' });
      y++;
    }
    y = 0;
    for (const f of task.facts ?? []) {
      nodes.set(`fact:${f.id}`, { x: 600, y: 16 + y * 34, label: f.text, kind: 'fact' });
      y++;
    }
    for (const a of task.assets ?? []) {
      nodes.set(`asset:${a.id}`, { x: 600, y: 16 + y * 34, label: a.url || a.domain || a.ip || `#${a.id}`, kind: 'asset' });
      y++;
    }
    for (const f of task.findings ?? []) {
      nodes.set(`finding:${f.id}`, { x: 600, y: 16 + y * 34, label: `[${f.severity}] ${f.title}`, kind: 'finding' });
      y++;
    }
    return { nodes, edges: task.edges ?? [] };
  }, [task]);

  if (nodes.size === 0) {
    return <div className="p-4 text-sm text-slate-400">暂无探索图数据（意图/事实/资产/漏洞出现后自动成图）。</div>;
  }
  const height = Math.max(120, [...nodes.values()].reduce((m, n) => Math.max(m, n.y), 0) + 40);
  const edgeLines = edges
    .filter((e) => nodes.has(e.from) && nodes.has(e.to))
    .map((e, i) => {
      const a = nodes.get(e.from)!;
      const b = nodes.get(e.to)!;
      return (
        <line key={i} x1={a.x} y1={a.y} x2={b.x} y2={b.y}
          stroke="var(--edge-line)" strokeWidth={1.2} opacity={0.7}>
          <title>{`${a.label} → ${REL_LABEL[e.rel] ?? e.rel} → ${b.label}`}</title>
        </line>
      );
    });
  const fillOf = (kind: string) =>
    kind === 'finding' ? 'var(--node-finding)' : kind === 'intent' ? 'var(--node-intent)'
      : kind === 'goal' ? 'var(--node-goal)' : kind === 'asset' ? 'var(--node-asset)' : 'var(--node-fact)';
  return (
    <div className="graph-root">
      {styles}
      <div className="mb-2 flex flex-wrap gap-x-3 gap-y-1 text-xs" style={{ color: 'var(--edge-text)' }}>
        <span><span className="mr-1 inline-block h-2 w-2 rounded-full align-middle" style={{ background: 'var(--node-goal)' }} />目标</span>
        <span><span className="mr-1 inline-block h-2 w-2 rounded-full align-middle" style={{ background: 'var(--node-intent)' }} />意图</span>
        <span><span className="mr-1 inline-block h-2 w-2 rounded-full align-middle" style={{ background: 'var(--node-asset)' }} />资产</span>
        <span><span className="mr-1 inline-block h-2 w-2 rounded-full align-middle" style={{ background: 'var(--node-fact)' }} />事实</span>
        <span><span className="mr-1 inline-block h-2 w-2 rotate-45 align-middle" style={{ background: 'var(--node-finding)' }} />漏洞</span>
        <span className="text-slate-400">边（灰线）：{Object.values(REL_LABEL).join(' / ')}</span>
      </div>
      <svg viewBox={`0 0 900 ${height}`} className="w-full" style={{ minHeight: 200 }}>
        {edgeLines}
        {[...nodes.entries()].map(([key, n]) => (
          <g key={key}>
            {n.kind === 'finding' ? (
              <rect x={n.x - 4} y={n.y - 4} width={8} height={8} transform={`rotate(45 ${n.x} ${n.y})`} fill={fillOf(n.kind)} />
            ) : (
              <circle cx={n.x} cy={n.y} r={4} fill={fillOf(n.kind)} />
            )}
            <text x={n.x + 10} y={n.y + 4} fontSize={12} className="fill-slate-600 dark:fill-slate-300">
              {n.label.length > 42 ? n.label.slice(0, 42) + '…' : n.label}
            </text>
            <title>{key}</title>
          </g>
        ))}
      </svg>
    </div>
  );
}

/**
 * AI 任务详情（/ai-task/:id）：状态控制（暂停/恢复/停止）、意图控制（重跑/舵向）、
 * 探索图、发现与活动流。运行中 3s 轮询刷新。
 */
export default function AiTaskDetailPage() {
  const { id = '' } = useParams();
  const [task, setTask] = useState<AITaskDetailDict | null>(null);
  const [error, setError] = useState('');
  const [msg, setMsg] = useState('');
  const [steerFor, setSteerFor] = useState<number | null>(null);
  const [steerText, setSteerText] = useState('');
  const [busy, setBusy] = useState('');

  const refresh = useCallback(() => {
    getAITask(id)
      .then((t) => {
        if (!t) {
          setError('任务不存在');
          return;
        }
        setTask(t);
        setError('');
      })
      .catch((e) => setError(String(e)));
  }, [id]);

  useEffect(() => {
    document.title = 'CTF-NET — AI 任务详情';
    refresh();
    const t = setInterval(refresh, 3000);
    return () => clearInterval(t);
  }, [refresh]);

  const running = task?.status === 'running';
  const act = (label: string, fn: () => Promise<void>) => {
    setBusy(label);
    fn()
      .then(() => {
        setMsg('');
        refresh();
      })
      .catch((e) => setError(String(e)))
      .finally(() => setBusy(''));
  };

  if (!task && !error) {
    return <div className="p-6 text-sm text-slate-400">加载中…</div>;
  }

  return (
    <div className="p-6">
      <div className="flex flex-wrap items-center gap-2">
        <Link to="/ai-dashboard" className="rounded p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h1 className="min-w-0 flex-1 truncate text-2xl font-semibold">{task?.description || task?.id}</h1>
        {task && (
          <span className={`rounded px-2 py-1 text-sm ${STATUS_STYLE[task.status ?? ''] ?? ''}`}>
            {task.paused ? '已暂停' : STATUS_LABEL[task.status ?? ''] ?? task.status}
          </span>
        )}
        {running && !task?.paused && (
          <button type="button" disabled={!!busy} onClick={() => act('pause', () => controlAITask(id, 'pause'))}
            className="flex items-center gap-1.5 rounded-md border border-slate-200 px-3 py-1.5 text-sm hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:hover:bg-slate-800">
            <Pause className="h-4 w-4" /> 暂停
          </button>
        )}
        {running && task?.paused && (
          <button type="button" disabled={!!busy} onClick={() => act('resume', () => controlAITask(id, 'resume'))}
            className="flex items-center gap-1.5 rounded-md border border-emerald-200 px-3 py-1.5 text-sm text-emerald-600 hover:bg-emerald-50 dark:border-emerald-800 dark:hover:bg-emerald-500/10">
            <Play className="h-4 w-4" /> 恢复
          </button>
        )}
        {running && (
          <button type="button" disabled={!!busy} onClick={() => act('stop', () => controlAITask(id, 'stop'))}
            className="flex items-center gap-1.5 rounded-md border border-rose-200 px-3 py-1.5 text-sm text-rose-600 hover:bg-rose-50 dark:border-rose-800 dark:hover:bg-rose-500/10 disabled:opacity-50">
            <Square className="h-4 w-4" /> 停止
          </button>
        )}
        <button type="button" onClick={refresh} className="flex items-center gap-1.5 rounded-md border border-slate-200 px-3 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800">
          <RefreshCw className="h-4 w-4" /> 刷新
        </button>
      </div>

      {error && <div className="mt-3 rounded-md bg-rose-50 p-3 text-sm text-rose-600">{error}</div>}
      {msg && <div className="mt-3 rounded-md bg-emerald-50 p-3 text-sm text-emerald-600">{msg}</div>}

      {task && (
        <div className="mt-4 space-y-4">
          {/* 目标 + 摘要 */}
          <div className="grid gap-3 md:grid-cols-2">
            <div className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
              <div className="mb-2 text-sm font-medium text-slate-600 dark:text-slate-300">目标</div>
              <ul className="space-y-1.5 text-sm">
                {(task.goals ?? []).map((g) => (
                  <li key={g.id} className="flex items-start gap-2">
                    <span className={g.met ? 'text-emerald-500' : 'text-slate-300'}>●</span>
                    <span className={g.met ? 'text-slate-400 line-through' : 'text-slate-700 dark:text-slate-200'}>{g.text}</span>
                    {g.met && g.met_by && <span className="text-xs text-emerald-600">{g.met_by}</span>}
                  </li>
                ))}
              </ul>
              {task.summary && <div className="mt-3 border-t border-slate-100 pt-2 text-sm text-slate-600 dark:border-slate-800 dark:text-slate-300">{task.summary}</div>}
            </div>
            <div className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
              <div className="mb-2 text-sm font-medium text-slate-600 dark:text-slate-300">统计</div>
              <div className="grid grid-cols-3 gap-2 text-center">
                {[
                  ['漏洞', task.findings?.length ?? 0],
                  ['资产', task.assets?.length ?? 0],
                  ['事实', task.facts?.length ?? 0],
                  ['意图', task.intents?.length ?? 0],
                  ['轮次', `${task.rounds_used ?? 0}/${task.plan_rounds ?? 0}`],
                  ['Token', String(((task.usage?.in ?? 0) + (task.usage?.out ?? 0)))],
                ].map(([k, v]) => (
                  <div key={k} className="rounded bg-slate-50 py-2 dark:bg-slate-800/60">
                    <div className="text-lg font-semibold text-slate-700 dark:text-slate-200">{v}</div>
                    <div className="text-xs text-slate-400">{k}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* 探索图 */}
          <div className="rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900">
            <div className="mb-2 text-sm font-medium text-slate-600 dark:text-slate-300">探索图</div>
            <GraphView task={task} />
          </div>

          {/* 意图列表 + 控制 */}
          <div className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
            <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium dark:border-slate-800">意图（work）</div>
            {(task.intents ?? []).length === 0 ? (
              <div className="p-4 text-sm text-slate-400">规划者尚未派发意图。</div>
            ) : (
              <ul className="divide-y divide-slate-100 dark:divide-slate-800">
                {(task.intents ?? []).map((it) => (
                  <li key={it.id} className="px-4 py-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-slate-700 dark:text-slate-200">#{it.id}</span>
                          <span className={`rounded px-1.5 py-0.5 text-xs ${INTENT_STYLE[it.state] ?? ''}`}>{it.state}</span>
                          {it.steer && <span className="rounded bg-amber-50 px-1.5 py-0.5 text-xs text-amber-600 dark:bg-amber-500/10">舵向待生效</span>}
                          <span className="text-xs text-slate-400">第 {it.round} 轮</span>
                        </div>
                        <div className="mt-1 text-sm text-slate-600 dark:text-slate-300">{it.summary}</div>
                        {it.result && <div className="mt-1 text-xs text-slate-400">{it.result}</div>}
                        {(it.trace?.length ?? 0) > 0 && (
                          <details className="mt-1">
                            <summary className="cursor-pointer text-xs text-indigo-500">执行轨迹（{it.trace?.length} 步）</summary>
                            <ul className="mt-1 space-y-0.5 text-xs text-slate-500">
                              {it.trace?.map((l, i) => <li key={i}>{l}</li>)}
                            </ul>
                          </details>
                        )}
                      </div>
                      <div className="flex shrink-0 gap-1.5">
                        {(it.state === 'blocked' || it.state === 'exhausted' || it.state === 'stopped') && running && (
                          <button type="button" disabled={!!busy}
                            onClick={() => act('rerun', () => controlAIIntent(id, it.id, 'rerun'))}
                            title="重跑该意图"
                            className="rounded border border-slate-200 p-1.5 text-slate-500 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:hover:bg-slate-800">
                            <RotateCcw className="h-3.5 w-3.5" />
                          </button>
                        )}
                        {(it.state === 'running' || it.state === 'open') && running && (
                          <button type="button" disabled={!!busy}
                            onClick={() => { setSteerFor(it.id); setSteerText(''); }}
                            title="注入人工舵向"
                            className="rounded border border-amber-200 p-1.5 text-amber-600 hover:bg-amber-50 disabled:opacity-50 dark:border-amber-800 dark:hover:bg-amber-500/10">
                            <Navigation className="h-3.5 w-3.5" />
                          </button>
                        )}
                      </div>
                    </div>
                    {steerFor === it.id && (
                      <div className="mt-2 flex gap-2">
                        <input
                          value={steerText}
                          onChange={(e) => setSteerText(e.target.value)}
                          placeholder="舵向内容（下一次 worker 回合优先遵循），如：先看 /admin 后台"
                          className="min-w-0 flex-1 rounded border border-slate-200 px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
                        />
                        <button type="button" disabled={!steerText.trim() || !!busy}
                          onClick={() => act('steer', () => controlAIIntent(id, it.id, 'steer', steerText.trim()).then(() => setSteerFor(null)))}
                          className="rounded-md bg-indigo-500 px-3 py-1.5 text-sm text-white hover:bg-indigo-600 disabled:opacity-50">
                          发送
                        </button>
                        <button type="button" onClick={() => setSteerFor(null)} className="rounded-md border border-slate-200 px-3 py-1.5 text-sm hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800">
                          取消
                        </button>
                      </div>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* 发现 */}
          <div className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
            <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium dark:border-slate-800">发现（{task.findings?.length ?? 0}）</div>
            {(task.findings ?? []).length === 0 ? (
              <div className="p-4 text-sm text-slate-400">暂无漏洞。</div>
            ) : (
              <ul className="divide-y divide-slate-100 dark:divide-slate-800">
                {(task.findings ?? []).map((f) => (
                  <li key={f.id} className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="rounded bg-rose-50 px-1.5 py-0.5 text-xs text-rose-600 dark:bg-rose-500/10">{f.severity}</span>
                      <span className="text-sm font-medium text-slate-700 dark:text-slate-200">{f.title}</span>
                      <span className="text-xs text-slate-400">{f.vuln_class}</span>
                    </div>
                    {f.evidence && (
                      <details className="mt-1">
                        <summary className="cursor-pointer text-xs text-indigo-500">证据节选</summary>
                        <pre className="mt-1 whitespace-pre-wrap rounded bg-slate-50 p-2 text-xs text-slate-600 dark:bg-slate-800/60 dark:text-slate-300">{f.evidence}</pre>
                      </details>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* 活动流 */}
          <div className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
            <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium dark:border-slate-800">活动流</div>
            <ul className="max-h-72 divide-y divide-slate-100 overflow-y-auto dark:divide-slate-800">
              {(task.activities ?? []).slice().reverse().map((a, i) => (
                <li key={i} className="px-4 py-1.5 text-xs">
                  <span className="mr-2 text-slate-400">{(a.ts ?? '').replace('T', ' ').slice(5, 19)}</span>
                  <span className="mr-2 text-slate-400">[{a.agent}]</span>
                  <span className="text-slate-600 dark:text-slate-300">{a.summary}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}
