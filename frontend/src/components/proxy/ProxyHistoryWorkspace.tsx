import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertTriangle, RefreshCw, Send, Trash2 } from 'lucide-react';

import { isWails } from '@/lib/backend';
import ProxyMiniStatus, { statusColor } from '@/components/proxy/ProxyMiniStatus';
import { useProxyStore } from '@/stores/useProxyStore';

/**
 * 流量历史工作区(/t/pentest.proxy.history):全宽流量清单(Burp HTTP History 风格),
 * 点行在下方展开请求/响应原始报文,可发送到重放器。
 */
export default function ProxyHistoryWorkspace() {
  const navigate = useNavigate();
  const {
    status,
    history,
    selected,
    busy,
    flash,
    refresh,
    openRecord,
    clearHistory,
    sendToRepeater,
    acquirePolling,
    releasePolling,
  } = useProxyStore();

  useEffect(() => {
    acquirePolling();
    return releasePolling;
  }, [acquirePolling, releasePolling]);

  if (!isWails()) {
    return (
      <div className="mt-6 flex flex-col items-center gap-2 rounded-lg border border-slate-200 p-10 text-slate-500 dark:border-slate-800">
        <AlertTriangle className="h-8 w-8" />
        <p>流量历史需要桌面运行时（Wails），请在打包后的应用内使用。</p>
      </div>
    );
  }

  const running = status?.running ?? false;

  const onSendToRepeater = () => {
    if (!selected) return;
    sendToRepeater(selected.raw_request);
    navigate('/t/pentest.proxy.repeater');
  };

  return (
    <div className="mt-2 space-y-3">
      {/* 顶部:迷你状态 + 操作 */}
      <div className="flex items-center gap-2">
        <ProxyMiniStatus />
        <div className="ml-auto flex items-center gap-1">
          <button
            onClick={() => void clearHistory()}
            disabled={busy}
            className="flex items-center gap-1 rounded-md px-2 py-1.5 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
            title="清空历史"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
          <button
            onClick={() => void refresh()}
            className="flex items-center gap-1 rounded-md px-2 py-1.5 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
            title="刷新"
          >
            <RefreshCw className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {flash && (
        <div
          className={`rounded-md px-3 py-2 text-xs ${
            flash.kind === 'ok'
              ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
              : 'bg-rose-50 text-rose-700 dark:bg-rose-950 dark:text-rose-300'
          }`}
        >
          {flash.text}
        </div>
      )}

      {/* 全宽流量清单 */}
      <div className="overflow-hidden rounded-md border border-slate-200 dark:border-slate-800">
        <div className="max-h-[480px] overflow-auto">
          <table className="w-full text-left text-xs">
            <thead className="sticky top-0 bg-slate-50 text-slate-500 dark:bg-slate-900">
              <tr>
                <th className="px-2 py-1.5 font-medium">#</th>
                <th className="px-2 py-1.5 font-medium">方法</th>
                <th className="px-2 py-1.5 font-medium">状态</th>
                <th className="px-2 py-1.5 font-medium">主机/路径</th>
                <th className="px-2 py-1.5 font-medium">耗时</th>
              </tr>
            </thead>
            <tbody>
              {history.map((item, i) => (
                <tr
                  key={item.id}
                  onClick={() => void openRecord(item.id)}
                  className={`cursor-pointer border-t border-slate-100 dark:border-slate-800/60 ${
                    selected?.id === item.id
                      ? 'bg-brand/10'
                      : 'hover:bg-slate-50 dark:hover:bg-slate-800/60'
                  }`}
                >
                  <td className="px-2 py-1.5 text-slate-400">{i + 1}</td>
                  <td className="px-2 py-1.5">
                    <span className="font-medium">{item.method}</span>
                    {item.intercepted && <span className="ml-1 text-amber-500">⚠</span>}
                  </td>
                  <td className={`px-2 py-1.5 font-medium ${statusColor(item.status)}`}>
                    {item.status || '—'}
                  </td>
                  <td className="max-w-[600px] truncate px-2 py-1.5 text-slate-600 dark:text-slate-300">
                    {item.host}
                    {item.url.replace(/^https?:\/\/[^/]+/, '')}
                  </td>
                  <td className="px-2 py-1.5 text-slate-400">{item.duration_ms}ms</td>
                </tr>
              ))}
              {history.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-2 py-6 text-center text-slate-400">
                    {running ? '暂无流量,设置代理后访问任意网站即可捕获' : '代理未启动'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* 选中详情:元信息 + 发送到重放器 + 请求/响应 */}
      {selected ? (
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2 text-xs text-slate-500">
            <span className="min-w-0 truncate">
              {selected.method} {selected.url} · {selected.status} · {selected.duration_ms}ms
              {selected.note ? ` · ${selected.note}` : ''}
            </span>
            <button
              onClick={onSendToRepeater}
              className="flex shrink-0 items-center gap-1 rounded-md bg-brand px-2.5 py-1 text-xs font-medium text-white hover:opacity-90"
            >
              <Send className="h-3 w-3" /> 发送到重放器
            </button>
          </div>
          <p className="text-[11px] font-medium uppercase text-slate-400">请求</p>
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-slate-200 bg-slate-50 p-2 font-mono text-xs dark:border-slate-800 dark:bg-slate-900">
            {selected.raw_request || '(无)  '}
          </pre>
          <p className="text-[11px] font-medium uppercase text-slate-400">响应</p>
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-slate-200 bg-slate-50 p-2 font-mono text-xs dark:border-slate-800 dark:bg-slate-900">
            {selected.raw_response || '(无)  '}
          </pre>
        </div>
      ) : (
        <p className="rounded-md border border-dashed border-slate-200 p-6 text-center text-xs text-slate-400 dark:border-slate-800">
          点击上方记录查看请求/响应原始报文
        </p>
      )}
    </div>
  );
}
