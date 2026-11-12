import type { RendererProps } from './types';

/** 自定义渲染器示例：交易追踪关系图（rendererId = 'tx-graph'） */
export default function TxGraphRenderer({ data }: RendererProps) {
  const payload = (data as { data?: Record<string, unknown> } | null)?.data ?? data;
  const address = typeof payload === 'object' && payload ? String((payload as Record<string, unknown>).address ?? '?') : '?';

  return (
    <div className="rounded-lg border border-slate-200 p-4 dark:border-slate-800">
      <div className="mb-3 text-sm font-medium text-slate-700 dark:text-slate-200">交易关系图（mock）</div>
      <div className="flex items-center gap-3 text-xs">
        <div className="rounded-md bg-indigo-500 px-3 py-2 text-white">from: {address}</div>
        <div className="text-slate-400">→</div>
        <div className="rounded-md bg-emerald-500 px-3 py-2 text-white">to: 0x…mock</div>
      </div>
      <p className="mt-3 text-xs text-slate-400">自定义渲染器 rendererId = tx-graph（阶段 5 演示）</p>
    </div>
  );
}
