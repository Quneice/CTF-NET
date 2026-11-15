import { Link } from 'react-router-dom';

import { useProxyStore } from '@/stores/useProxyStore';

/** 状态色(流量历史表格/重放器 meta 共用) */
export function statusColor(status: number): string {
  if (status === 0) return 'text-slate-400';
  if (status < 300) return 'text-emerald-600 dark:text-emerald-400';
  if (status < 400) return 'text-amber-600 dark:text-amber-400';
  return 'text-rose-600 dark:text-rose-400';
}

/**
 * 迷你状态行(流量历史/重放器页右上角):运行状态 + 挂起提醒,
 * 点击跳转代理拦截页(控制栏所在)。
 */
export default function ProxyMiniStatus() {
  const status = useProxyStore((s) => s.status);
  const pendingCount = useProxyStore((s) => s.pendings.length);

  return (
    <Link
      to="/t/pentest.proxy.intercept"
      title="前往代理拦截页控制(启动/停止/拦截开关/MITM)"
      className="flex shrink-0 items-center gap-1.5 text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
    >
      <span
        className={`inline-block h-2 w-2 rounded-full ${status?.running ? 'bg-emerald-500' : 'bg-slate-400'}`}
      />
      {status?.running ? `运行中 127.0.0.1:${status.port}` : '未启动'}
      {pendingCount > 0 && <span className="text-amber-500">⏳ 挂起 {pendingCount}</span>}
    </Link>
  );
}
