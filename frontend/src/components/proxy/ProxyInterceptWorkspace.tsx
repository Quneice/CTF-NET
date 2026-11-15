import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  AlertTriangle,
  BadgeCheck,
  Ban,
  Download,
  Globe,
  KeyRound,
  Play,
  RefreshCw,
  Send,
  ShieldCheck,
  Square,
  Trash2,
} from 'lucide-react';

import { isWails } from '@/lib/backend';
import { proxyCAExport, proxyCAInstall } from '@/lib/proxy';
import { useProxyStore } from '@/stores/useProxyStore';

/**
 * 代理拦截工作区(/t/pentest.proxy.intercept):
 * 代理控制栏(启动/停止/拦截开关/内置浏览器)+ HTTPS 解密(MITM/CA)+ 拦截挂起请求处理。
 * 状态全部来自 useProxyStore(与流量历史/重放器两页共享,keep-alive 下轮询常驻)。
 */
export default function ProxyInterceptWorkspace() {
  const navigate = useNavigate();
  const {
    status,
    busy,
    pendings,
    pendingRaws,
    pendingResult,
    flash,
    browserInfo,
    flashOnce,
    refresh,
    start,
    stop,
    toggleIntercept,
    toggleMitm,
    decide,
    setPendingRaw,
    clearHistory,
    launchBrowser,
    sendToRepeater,
    acquirePolling,
    releasePolling,
    loadBrowserInfo,
  } = useProxyStore();
  const activePending = pendings[0] ?? null;
  const activeRaw = activePending ? (pendingRaws[activePending.id] ?? '') : '';

  // 本地仅三个输入框状态
  const [portInput, setPortInput] = useState('8080');
  const [browserUrl, setBrowserUrl] = useState('http://example.com');
  const [caExportDir, setCaExportDir] = useState('');

  useEffect(() => {
    acquirePolling();
    void loadBrowserInfo();
    return releasePolling;
  }, [acquirePolling, releasePolling, loadBrowserInfo]);

  if (!isWails()) {
    return (
      <div className="mt-6 flex flex-col items-center gap-2 rounded-lg border border-slate-200 p-10 text-slate-500 dark:border-slate-800">
        <AlertTriangle className="h-8 w-8" />
        <p>代理拦截需要桌面运行时（Wails），请在打包后的应用内使用。</p>
      </div>
    );
  }

  const running = status?.running ?? false;

  const onExportCA = async () => {
    const dir = caExportDir.trim();
    if (!dir) {
      flashOnce('err', '请先填写导出目录');
      return;
    }
    try {
      const p = await proxyCAExport(dir);
      flashOnce('ok', `CA 证书已导出:${p}`);
    } catch (e) {
      flashOnce('err', String(e));
    }
  };

  const onInstallCA = async () => {
    try {
      await proxyCAInstall();
      flashOnce('ok', 'CA 已安装到当前用户信任库(无需管理员)');
    } catch (e) {
      flashOnce('err', String(e));
    }
  };

  const onOpenBrowser = () => {
    const port = running ? (status?.port ?? 8080) : Number(portInput) || 8080;
    void launchBrowser(browserUrl, port);
  };

  const onSendToRepeater = () => {
    if (!activePending) return;
    sendToRepeater(activeRaw);
    navigate('/t/pentest.proxy.repeater');
  };

  return (
    <div className="mt-2 space-y-3">
      {/* —— 控制栏 —— */}
      <div className="flex flex-wrap items-center gap-2 rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900/60">
        <span className="flex items-center gap-1.5 text-xs font-medium text-slate-700 dark:text-slate-200">
          <span
            className={`inline-block h-2 w-2 rounded-full ${running ? 'bg-emerald-500' : 'bg-slate-400'}`}
          />
          代理 {running ? `运行中 127.0.0.1:${status?.port}` : '未启动'}
        </span>
        {!running ? (
          <>
            <input
              value={portInput}
              onChange={(e) => setPortInput(e.target.value)}
              className="w-24 rounded-md border border-slate-300 px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
              placeholder="端口"
            />
            <button
              onClick={() => void start(portInput)}
              disabled={busy}
              className="flex items-center gap-1 rounded-md bg-brand px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
            >
              <Play className="h-3.5 w-3.5" /> 启动
            </button>
          </>
        ) : (
          <button
            onClick={() => void stop()}
            disabled={busy}
            className="flex items-center gap-1 rounded-md bg-rose-600 px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
          >
            <Square className="h-3.5 w-3.5" /> 停止
          </button>
        )}
        <button
          onClick={() => void toggleIntercept()}
          disabled={!running || busy}
          className={`flex items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium disabled:opacity-40 ${
            status?.intercept
              ? 'bg-amber-500 text-white'
              : 'bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200'
          }`}
        >
          <ShieldCheck className="h-3.5 w-3.5" /> 拦截{status?.intercept ? '开' : '关'}
        </button>
        <span className="mx-1 h-5 w-px bg-slate-200 dark:bg-slate-700" />
        <input
          value={browserUrl}
          onChange={(e) => setBrowserUrl(e.target.value)}
          className="w-44 rounded-md border border-slate-300 px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
          placeholder="起始页 URL"
          title="内置浏览器起始页"
        />
        <button
          onClick={onOpenBrowser}
          disabled={busy}
          title={
            browserInfo?.available
              ? `用 ${browserInfo.name} 打开(代理自动指向本机,未运行则先启动)`
              : browserInfo?.name || '内置浏览器'
          }
          className="flex items-center gap-1 rounded-md bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50 dark:bg-slate-200 dark:text-slate-800"
        >
          <Globe className="h-3.5 w-3.5" /> 内置浏览器
        </button>
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
        <span className="ml-auto text-[11px] text-slate-400">
          内置浏览器自动走本机代理;其他浏览器/工具手动设代理 127.0.0.1:{status?.port ?? portInput}
        </span>
      </div>

      {/* —— HTTPS 解密(MITM)控制行 —— */}
      <div className="flex flex-wrap items-center gap-2 rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900/60">
        <button
          onClick={() => void toggleMitm()}
          disabled={busy}
          className={`flex items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium disabled:opacity-40 ${
            status?.mitm
              ? 'bg-brand text-white'
              : 'bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200'
          }`}
        >
          <KeyRound className="h-3.5 w-3.5" /> HTTPS 解密{status?.mitm ? '开' : '关'}
        </button>
        <input
          value={caExportDir}
          onChange={(e) => setCaExportDir(e.target.value)}
          className="w-52 rounded-md border border-slate-300 px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
          placeholder="CA 导出目录"
          title="导出的 ctfnet-ca.crt 可导入浏览器/其他机器信任库"
        />
        <button
          onClick={() => void onExportCA()}
          className="flex items-center gap-1 rounded-md px-2.5 py-1.5 text-xs text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          <Download className="h-3.5 w-3.5" /> 导出 CA
        </button>
        <button
          onClick={() => void onInstallCA()}
          className="flex items-center gap-1 rounded-md px-2.5 py-1.5 text-xs text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          title="certutil -user -addstore Root(仅当前用户,无需管理员)"
        >
          <BadgeCheck className="h-3.5 w-3.5" /> 安装到系统信任库
        </button>
        <span className="text-[11px] text-slate-400">
          开启后 HTTPS 流量解密进入拦截/历史;内置浏览器已自动忽略证书错误,其他客户端需信任 CA
        </span>
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

      {/* —— 拦截区 —— */}
      <div className="space-y-2">
        {!running && (
          <p className="rounded-md border border-slate-200 p-3 text-xs text-slate-500 dark:border-slate-800">
            代理未启动,拦截不可用。先在上方控制栏启动代理并打开「拦截」开关。
          </p>
        )}
        {running && !status?.intercept && (
          <p className="rounded-md border border-slate-200 p-3 text-xs text-slate-500 dark:border-slate-800">
            拦截开关当前为「关」,流量将透明转发并记录历史。打开拦截后,每个请求会在此挂起等待处理。
          </p>
        )}
        {activePending ? (
          <>
            <textarea
              value={activeRaw}
              onChange={(e) => setPendingRaw(activePending.id, e.target.value)}
              spellCheck={false}
              className="h-64 w-full rounded-md border border-slate-300 p-2 font-mono text-xs dark:border-slate-700 dark:bg-slate-800"
            />
            <div className="flex flex-wrap items-center gap-2">
              <button
                onClick={() => void decide('forward')}
                disabled={busy}
                className="rounded-md bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
              >
                放行
              </button>
              <button
                onClick={() => void decide('modified')}
                disabled={busy}
                className="rounded-md bg-brand px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
              >
                按修改后报文放行
              </button>
              <button
                onClick={() => void decide('drop')}
                disabled={busy}
                className="flex items-center gap-1 rounded-md bg-rose-600 px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
              >
                <Ban className="h-3.5 w-3.5" /> 丢弃
              </button>
              <button
                onClick={onSendToRepeater}
                className="flex items-center gap-1 rounded-md bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 dark:bg-slate-200 dark:text-slate-800"
                title="把当前(可编辑)报文发到重放器页"
              >
                <Send className="h-3.5 w-3.5" /> 发送到重放器
              </button>
              <span className="text-[11px] text-slate-400">
                拦截中:{activePending.method} {activePending.url}
                {pendings.length > 1 ? ` · 队列中还有 ${pendings.length - 1} 个挂起` : ''}
              </span>
            </div>
          </>
        ) : (
          <p className="rounded-md border border-dashed border-slate-200 p-6 text-center text-xs text-slate-400 dark:border-slate-800">
            暂无挂起的请求。有请求命中拦截时,原始报文会出现在这里供修改/放行/丢弃。
          </p>
        )}
        {pendingResult && (
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-slate-200 bg-slate-50 p-2 font-mono text-xs dark:border-slate-800 dark:bg-slate-900">
            {pendingResult}
          </pre>
        )}
      </div>
    </div>
  );
}
