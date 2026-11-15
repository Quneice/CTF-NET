import { create } from 'zustand';

import {
  browserAvailable,
  browserLaunch,
  onProxyIntercept,
  PROXY_DROP,
  PROXY_MODIFIED,
  proxyDecide,
  proxyGetRecord,
  proxyHistory,
  proxyHistoryClear,
  proxyReplay,
  proxySetIntercept,
  proxySetMitm,
  proxyStart,
  proxyStatus,
  proxyStop,
  type BrowserInfo,
  type ProxyRecord,
  type ProxyStatus,
  type ProxySummary,
} from '@/lib/proxy';

/**
 * 代理工作台共享状态(代理拦截/流量历史/重放器三页共用)。
 *
 * 轮询生命周期 = 引用计数:三个工具页(keep-alive 多开标签)挂载时 acquirePolling、
 * 卸载时 releasePolling,任一页存活即保持一份 2s 轮询 + 拦截事件订阅;全部关闭后
 * 停止,重开即恢复。计数与定时器为模块级变量,不进 state。
 */

/** 重放器数字标签(对应 Burp Repeater 的 1/2/3…) */
export interface RepeaterTab {
  id: number; // 从 1 递增
  req: string;
  resp: string;
  meta: string; // 「状态 · 耗时 · note」一行
  busy: boolean;
}

interface Flash {
  kind: 'ok' | 'err';
  text: string;
}

interface ProxyState {
  status: ProxyStatus | null;
  history: ProxySummary[];
  /** 拦截挂起队列(后端可并发挂起多条;队首为当前处理对象,逐条决策) */
  pendings: ProxyRecord[];
  /** 各挂起请求的可编辑报文副本(按 id;队首的用于修改放行) */
  pendingRaws: Record<string, string>;
  /** 最近一次决策的结果展示(放行后终态/丢弃提示) */
  pendingResult: string;
  /** 流量历史页选中记录详情 */
  selected: ProxyRecord | null;
  repeaterTabs: RepeaterTab[];
  activeRepeaterId: number | null;
  browserInfo: BrowserInfo | null;
  flash: Flash | null;
  busy: boolean;

  acquirePolling(): void;
  releasePolling(): void;
  refresh(): Promise<void>;
  flashOnce(kind: 'ok' | 'err', text: string): void;
  start(portStr: string): Promise<void>;
  stop(): Promise<void>;
  toggleIntercept(): Promise<void>;
  toggleMitm(): Promise<void>;
  decide(action: 'forward' | 'drop' | 'modified'): Promise<void>;
  setPendingRaw(id: string, raw: string): void;
  openRecord(id: string): Promise<void>;
  clearHistory(): Promise<void>;
  sendToRepeater(raw: string): number;
  setActiveRepeater(id: number): void;
  updateRepeaterReq(id: number, raw: string): void;
  closeRepeaterTab(id: number): void;
  replay(id: number): Promise<void>;
  loadBrowserInfo(): Promise<void>;
  launchBrowser(url: string, portHint: number): Promise<void>;
}

/* —— 模块级轮询/编号状态(不进 state) —— */
let refCount = 0;
let timer: number | undefined;
let offIntercept: (() => void) | null = null;
let nextRepeaterId = 0;

/** 等轮询拿到决策后的最终记录(放行后响应回写历史),最多 ~5s */
async function waitRecord(id: string, tries = 10): Promise<ProxyRecord | null> {
  for (let i = 0; i < tries; i++) {
    try {
      const rec = await proxyGetRecord(id);
      if (rec.status !== 0 || rec.note) return rec;
    } catch {
      // 记录尚未入历史,继续等
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  return null;
}

export const useProxyStore = create<ProxyState>()((set, get) => {
  const flashOnce = (kind: 'ok' | 'err', text: string) => {
    set({ flash: { kind, text } });
    window.setTimeout(() => set({ flash: null }), 4000);
  };

  const patchTab = (id: number, patch: Partial<RepeaterTab>) => {
    set((s) => ({
      repeaterTabs: s.repeaterTabs.map((t) => (t.id === id ? { ...t, ...patch } : t)),
    }));
  };

  return {
    status: null,
    history: [],
    pendings: [],
    pendingRaws: {},
    pendingResult: '',
    selected: null,
    repeaterTabs: [],
    activeRepeaterId: null,
    browserInfo: null,
    flash: null,
    busy: false,

    acquirePolling: () => {
      refCount++;
      if (refCount === 1) {
        void get().refresh();
        timer = window.setInterval(() => void get().refresh(), 2000);
        offIntercept = onProxyIntercept((item) => {
          void (async () => {
            try {
              const rec = await proxyGetRecord(item.id);
              set((s) => ({
                pendings: [...s.pendings, rec],
                pendingRaws: { ...s.pendingRaws, [rec.id]: rec.raw_request },
              }));
            } catch {
              // 记录竞态未入历史时忽略(轮询会兜底)
            }
          })();
        });
      }
    },

    releasePolling: () => {
      refCount = Math.max(0, refCount - 1);
      if (refCount === 0) {
        if (timer !== undefined) {
          window.clearInterval(timer);
          timer = undefined;
        }
        offIntercept?.();
        offIntercept = null;
      }
    },

    refresh: async () => {
      try {
        const st = await proxyStatus();
        set({ status: st });
        if (st.running) {
          const page = await proxyHistory(0, 100);
          set({ history: page.items });
        }
      } catch {
        set({ status: null });
      }
    },

    flashOnce,

    start: async (portStr) => {
      const port = Number(portStr);
      if (!Number.isInteger(port) || port < 1 || port > 65535) {
        flashOnce('err', '端口需为 1-65535 的整数');
        return;
      }
      set({ busy: true });
      try {
        await proxyStart(port);
        flashOnce('ok', `代理已启动:127.0.0.1:${port}`);
        await get().refresh();
      } catch (e) {
        flashOnce('err', String(e));
      } finally {
        set({ busy: false });
      }
    },

    stop: async () => {
      set({ busy: true });
      try {
        await proxyStop();
        flashOnce('ok', '代理已停止');
        set({ pendings: [], pendingRaws: {} });
        await get().refresh();
      } catch (e) {
        flashOnce('err', String(e));
      } finally {
        set({ busy: false });
      }
    },

    toggleIntercept: async () => {
      const next = !(get().status?.intercept ?? false);
      try {
        await proxySetIntercept(next);
        await get().refresh();
      } catch (e) {
        flashOnce('err', String(e));
      }
    },

    toggleMitm: async () => {
      const next = !(get().status?.mitm ?? false);
      try {
        await proxySetMitm(next);
        flashOnce(
          'ok',
          next
            ? 'HTTPS 解密已开启(内置浏览器自动忽略证书错误,其他客户端需信任 CA)'
            : 'HTTPS 解密已关闭(CONNECT 直通)',
        );
        await get().refresh();
      } catch (e) {
        flashOnce('err', String(e));
      }
    },

    decide: async (action) => {
      const cur = get().pendings[0];
      if (!cur) return;
      const raw = get().pendingRaws[cur.id] ?? cur.raw_request;
      set({ busy: true });
      try {
        await proxyDecide(cur.id, action, action === PROXY_MODIFIED ? raw : '');
        set({
          pendingResult:
            action === PROXY_DROP ? '已丢弃(目标不会收到该请求)' : '已放行,等待响应…',
        });
        const rec = await waitRecord(cur.id);
        // 按 id 移除(而非清空队列):等待期间可能有新挂起入队,不能误伤
        set((s) => {
          const pendingRaws = { ...s.pendingRaws };
          delete pendingRaws[cur.id];
          return { pendings: s.pendings.filter((p) => p.id !== cur.id), pendingRaws };
        });
        if (rec) {
          set({
            pendingResult:
              rec.status > 0
                ? `已完成:${rec.status}(${rec.duration_ms}ms)\n\n${rec.raw_response}`
                : `${rec.note ?? '未完成'}`,
          });
        }
        void get().refresh();
      } catch (e) {
        flashOnce('err', String(e));
      } finally {
        set({ busy: false });
      }
    },

    setPendingRaw: (id, raw) =>
      set((s) => ({ pendingRaws: { ...s.pendingRaws, [id]: raw } })),

    openRecord: async (id) => {
      try {
        const rec = await proxyGetRecord(id);
        set({ selected: rec });
      } catch (e) {
        flashOnce('err', String(e));
      }
    },

    clearHistory: async () => {
      try {
        await proxyHistoryClear();
        set({ history: [], selected: null });
        flashOnce('ok', '历史已清空');
      } catch (e) {
        flashOnce('err', String(e));
      }
    },

    sendToRepeater: (raw) => {
      nextRepeaterId += 1;
      const tab: RepeaterTab = { id: nextRepeaterId, req: raw, resp: '', meta: '', busy: false };
      set((s) => ({ repeaterTabs: [...s.repeaterTabs, tab], activeRepeaterId: tab.id }));
      return tab.id;
    },

    setActiveRepeater: (id) => set({ activeRepeaterId: id }),

    updateRepeaterReq: (id, raw) => patchTab(id, { req: raw }),

    closeRepeaterTab: (id) =>
      set((s) => {
        const idx = s.repeaterTabs.findIndex((t) => t.id === id);
        if (idx < 0) return {};
        const tabs = s.repeaterTabs.filter((t) => t.id !== id);
        const active =
          s.activeRepeaterId === id
            ? tabs.length
              ? tabs[Math.min(idx, tabs.length - 1)].id
              : null
            : s.activeRepeaterId;
        return { repeaterTabs: tabs, activeRepeaterId: active };
      }),

    replay: async (id) => {
      const tab = get().repeaterTabs.find((t) => t.id === id);
      if (!tab) return;
      if (!tab.req.trim()) {
        flashOnce('err', '请先输入要重放的请求报文');
        return;
      }
      patchTab(id, { busy: true });
      try {
        const rec = await proxyReplay(tab.req);
        patchTab(id, {
          resp: rec.raw_response,
          meta: `${rec.status > 0 ? rec.status : '未完成'} · ${rec.duration_ms}ms${
            rec.note ? ' · ' + rec.note : ''
          }`,
        });
        void get().refresh();
      } catch (e) {
        flashOnce('err', String(e));
      } finally {
        patchTab(id, { busy: false });
      }
    },

    loadBrowserInfo: async () => {
      try {
        set({ browserInfo: await browserAvailable() });
      } catch {
        // 探测失败保持 null,按钮报错兜底
      }
    },

    launchBrowser: async (url, portHint) => {
      set({ busy: true });
      try {
        const res = await browserLaunch(url.trim() || 'http://example.com', portHint);
        flashOnce('ok', `已启动 ${res.browser}(代理 127.0.0.1:${res.proxy_port} → ${res.url})`);
        await get().refresh();
      } catch (e) {
        flashOnce('err', String(e));
      } finally {
        set({ busy: false });
      }
    },
  };
});
