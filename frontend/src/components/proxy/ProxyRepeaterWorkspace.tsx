import { useEffect } from 'react';
import { AlertTriangle, Plus, Send, X } from 'lucide-react';

import { isWails } from '@/lib/backend';
import ProxyMiniStatus from '@/components/proxy/ProxyMiniStatus';
import { useProxyStore } from '@/stores/useProxyStore';

/**
 * 重放器工作区(/t/pentest.proxy.repeater,Burp Repeater 风格):
 * 数字多标签(1/2/3…),每标签独立请求编辑/发送/响应;从流量历史或代理拦截页
 * 「发送到重放器」即追加标签并跳转至此。
 */
export default function ProxyRepeaterWorkspace() {
  const {
    repeaterTabs,
    activeRepeaterId,
    flash,
    setActiveRepeater,
    updateRepeaterReq,
    closeRepeaterTab,
    replay,
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
        <p>重放器需要桌面运行时（Wails），请在打包后的应用内使用。</p>
      </div>
    );
  }

  const tab = repeaterTabs.find((t) => t.id === activeRepeaterId) ?? null;

  return (
    <div className="mt-2 space-y-3">
      {/* 顶部:迷你状态 */}
      <div className="flex items-center gap-2">
        <ProxyMiniStatus />
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

      {/* 数字标签行(Burp 式 1/2/3…)+ 新建 */}
      <div className="flex items-center gap-1">
        {repeaterTabs.map((t) => (
          <span
            key={t.id}
            className={`group flex items-center rounded-md px-2 py-1 text-xs font-medium transition-colors ${
              t.id === activeRepeaterId
                ? 'bg-brand text-white'
                : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
            }`}
          >
            <button type="button" onClick={() => setActiveRepeater(t.id)} className="px-1">
              {t.id}
            </button>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                closeRepeaterTab(t.id);
              }}
              title="关闭标签"
              aria-label={`关闭标签 ${t.id}`}
              className={`rounded p-0.5 ${
                t.id === activeRepeaterId
                  ? 'hover:bg-white/20'
                  : 'hover:bg-slate-200 dark:hover:bg-slate-700'
              }`}
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}
        <button
          type="button"
          onClick={() => sendToRepeater('')}
          title="新建空白标签"
          className="rounded-md p-1 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <Plus className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* 当前标签主体 */}
      {tab ? (
        <div className="space-y-2">
          <p className="text-[11px] text-slate-400">
            编辑请求报文(源站形式:GET /path HTTP/1.1 + Host 头即可),点击发送。
          </p>
          <textarea
            value={tab.req}
            onChange={(e) => updateRepeaterReq(tab.id, e.target.value)}
            spellCheck={false}
            placeholder={'GET /index.html HTTP/1.1\nHost: example.com\n\n'}
            className="h-64 w-full rounded-md border border-slate-300 p-2 font-mono text-xs dark:border-slate-700 dark:bg-slate-800"
          />
          <div className="flex items-center gap-2">
            <button
              onClick={() => void replay(tab.id)}
              disabled={tab.busy}
              className="flex items-center gap-1 rounded-md bg-brand px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
            >
              <Send className="h-3.5 w-3.5" /> 发送
            </button>
            {tab.meta && <p className="text-xs text-slate-500">{tab.meta}</p>}
          </div>
          <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-md border border-slate-200 bg-slate-50 p-2 font-mono text-xs dark:border-slate-800 dark:bg-slate-900">
            {tab.resp || '(响应将显示在这里)  '}
          </pre>
        </div>
      ) : (
        <p className="rounded-md border border-dashed border-slate-200 p-6 text-center text-xs text-slate-400 dark:border-slate-800">
          暂无重放标签——从「流量历史」或「代理拦截」发送请求,或点 + 新建
        </p>
      )}
    </div>
  );
}
