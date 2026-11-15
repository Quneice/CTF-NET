import { isWails } from '@/lib/backend';

import * as rawApp from '../../wailsjs/go/main/App';

/**
 * HTTP 代理拦截与重放(Burp 风格)的绑定门面。
 * 引擎见 internal/proxy(Go,纯标准库);拦截事件 proxy:intercept 载荷为 Summary,
 * 前端按 id 调 proxyGetRecord 拉全量原始报文。
 */

export interface ProxyStatus {
  running: boolean;
  port: number;
  intercept: boolean;
  mitm: boolean;
  ca_file: string;
}

export interface ProxySummary {
  id: string;
  timestamp: number; // Unix 毫秒
  method: string;
  host: string;
  url: string;
  status: number; // 0 = 未完成(被丢弃/转发失败)
  duration_ms: number;
  intercepted: boolean;
  note: string;
}

export interface ProxyRecord extends ProxySummary {
  raw_request: string;
  raw_response: string;
}

export interface ProxyHistoryPage {
  total: number;
  items: ProxySummary[];
}

/** 拦截决策动作(与 Go internal/proxy 常量一致) */
export const PROXY_FORWARD = 'forward';
export const PROXY_DROP = 'drop';
export const PROXY_MODIFIED = 'modified';

export interface BrowserInfo {
  available: boolean;
  name: string;
  path: string;
}

export interface BrowserLaunchResult {
  browser: string;
  url: string;
  proxy_port: number;
}

// 生成绑定 wailsjs/go/main/App 自动更新,勿手改生成文件
const App = rawApp as unknown as {
  ProxyStart(port: number): Promise<void>;
  ProxyStop(): Promise<void>;
  ProxyStatus(): Promise<ProxyStatus>;
  ProxySetIntercept(on: boolean): Promise<void>;
  ProxyHistory(offset: number, limit: number): Promise<ProxyHistoryPage>;
  ProxyGetRecord(id: string): Promise<ProxyRecord>;
  ProxyDecide(id: string, action: string, modified: string): Promise<void>;
  ProxyReplay(raw: string): Promise<ProxyRecord>;
  ProxyHistoryClear(): Promise<void>;
  ProxySetMITM(on: boolean): Promise<void>;
  ProxyCAExport(dir: string): Promise<string>;
  ProxyCAInstall(): Promise<void>;
  BrowserAvailable(): Promise<BrowserInfo>;
  BrowserLaunch(url: string, portHint: number): Promise<BrowserLaunchResult>;
};

function requireWails(): void {
  if (!isWails()) throw new Error('代理拦截需要桌面运行时(请打包后在应用内使用)');
}

export async function proxyStart(port: number): Promise<void> {
  requireWails();
  return App.ProxyStart(port);
}

export async function proxyStop(): Promise<void> {
  requireWails();
  return App.ProxyStop();
}

export async function proxyStatus(): Promise<ProxyStatus> {
  requireWails();
  return App.ProxyStatus();
}

export async function proxySetIntercept(on: boolean): Promise<void> {
  requireWails();
  return App.ProxySetIntercept(on);
}

export async function proxyHistory(offset: number, limit: number): Promise<ProxyHistoryPage> {
  requireWails();
  return App.ProxyHistory(offset, limit);
}

export async function proxyGetRecord(id: string): Promise<ProxyRecord> {
  requireWails();
  return App.ProxyGetRecord(id);
}

export async function proxyDecide(id: string, action: string, modified: string): Promise<void> {
  requireWails();
  return App.ProxyDecide(id, action, modified);
}

export async function proxyReplay(raw: string): Promise<ProxyRecord> {
  requireWails();
  return App.ProxyReplay(raw);
}

export async function proxyHistoryClear(): Promise<void> {
  requireWails();
  return App.ProxyHistoryClear();
}

export async function proxySetMitm(on: boolean): Promise<void> {
  requireWails();
  return App.ProxySetMITM(on);
}

export async function proxyCAExport(dir: string): Promise<string> {
  requireWails();
  return App.ProxyCAExport(dir);
}

export async function proxyCAInstall(): Promise<void> {
  requireWails();
  return App.ProxyCAInstall();
}

export async function browserAvailable(): Promise<BrowserInfo> {
  requireWails();
  return App.BrowserAvailable();
}

export async function browserLaunch(url: string, portHint: number): Promise<BrowserLaunchResult> {
  requireWails();
  return App.BrowserLaunch(url, portHint);
}

/** 订阅拦截事件(Go 在拦截挂起时 EventsEmit);返回退订函数。 */
export function onProxyIntercept(cb: (item: ProxySummary) => void): () => void {
  const rt = (
    window as unknown as {
      runtime?: {
        EventsOn?: (ev: string, cb: (d: ProxySummary) => void) => void;
        EventsOff?: (ev: string) => void;
      };
    }
  ).runtime;
  if (!rt?.EventsOn) return () => {};
  rt.EventsOn('proxy:intercept', cb);
  return () => {
    rt.EventsOff?.('proxy:intercept');
  };
}
