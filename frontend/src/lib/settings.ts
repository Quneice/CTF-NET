/**
 * 统一应用配置 —— 数据层。
 *
 * 桌面运行时走 wailsjs 生成的 App 绑定（config_api.go）；
 * 纯浏览器 `npm run dev`（isWails() 为 false）回退 localStorage，便于 UI 开发调试。
 * JSON 键与 internal/core/settings.go 的结构体 tag 一致（蛇形）。
 */
import * as rawApp from '../../wailsjs/go/main/App';
import type { FormValues } from '@/types';
import { isWails } from './backend';

// —— 类型（与 core.AppConfig 对应）——

export interface JndiConfig {
  ssh_host: string;
  ssh_port: number;
  ssh_user: string;
  ssh_pass: string;
  lhost: string;
  ldap_port: number;
  rmi_port: number;
  http_port: number;
  lport: number;
  gadget_class: string;
  shell: string;
}

export interface FrpConfig {
  ssh_host: string;
  ssh_port: number;
  ssh_user: string;
  ssh_pass: string;
  remote_dir: string;
  bind_port: number;
  token: string;
  vhost_http_port: number;
  vhost_https_port: number;
  dashboard_port: number;
  dashboard_user: string;
  dashboard_pass: string;
  frps_version: string;
  download_url: string;
}

export interface ToolsConfig {
  python3: string;
  java: string;
  javac: string;
  nmap: string;
  nuclei: string;
  xray: string;
  sqlmap: string;
  putty_dir: string;
  /** fscan exe 路径（端口扫描工具 pentest.scan.port-scan 使用） */
  fscan: string;
}

export interface WordlistsConfig {
  dir_enum: string;
  password: string;
  username: string;
  subdomain: string;
}

export interface ExecutorConfig {
  default_timeout: number;
  max_timeout: number;
  backend: string; // local | docker
  docker_image: string;
  docker_network: string;
  docker_data_volume: string;
}

export interface ProxyConfig {
  /** 全局代理：http://… / https://… / socks5://… / socks5h://…，空=直连 */
  url: string;
}

export interface AiConfig {
  /** LLM API Key（OpenAI 兼容服务） */
  api_key: string;
  /** 如 https://open.bigmodel.cn/api/paas/v4（智谱） */
  base_url: string;
  /** 如 glm-5.3-flash */
  model: string;
  /** 思考强度 low/high/max（智谱 glm-5.x 始终思考；空=不传） */
  reasoning_effort: string;
  /** 单会话最大工具轮数 */
  max_turns: number;
  /** 并发 worker 数（1-8） */
  max_workers: number;
  /** 规划轮次预算 */
  plan_rounds: number;
  /** 任务总时限（秒） */
  timeout_seconds: number;
  /** 是否允许 worker 执行本地命令 */
  enable_exec: boolean;
}

export interface AppConfig {
  jndi: JndiConfig;
  frp: FrpConfig;
  tools: ToolsConfig;
  wordlists: WordlistsConfig;
  executor: ExecutorConfig;
  proxy: ProxyConfig;
  ai: AiConfig;
}

/** 内置默认值（与 core.DefaultAppConfig 保持一致） */
export function defaultAppConfig(): AppConfig {
  return {
    jndi: {
      ssh_host: '',
      ssh_port: 22,
      ssh_user: '',
      ssh_pass: '',
      lhost: '',
      ldap_port: 1389,
      rmi_port: 1099,
      http_port: 8888,
      lport: 31339,
      gadget_class: 'com.sun.rowset.JdbcRowSetImpl',
      shell: '/bin/sh',
    },
    frp: {
      ssh_host: '',
      ssh_port: 0,
      ssh_user: '',
      ssh_pass: '',
      remote_dir: '~/ctfnet-frp',
      bind_port: 7000,
      token: '',
      vhost_http_port: 8080,
      vhost_https_port: 8443,
      dashboard_port: 7500,
      dashboard_user: 'admin',
      dashboard_pass: '',
      frps_version: '0.69.1',
      download_url: '',
    },
    tools: {
      python3: 'python3',
      java: 'java',
      javac: 'javac',
      nmap: 'nmap',
      nuclei: 'nuclei',
      xray: 'xray',
      sqlmap: 'sqlmap',
      putty_dir: '',
      fscan: 'C:\\Users\\HP\\Downloads\\fscan_2.2.2_windows_x64.exe',
    },
    wordlists: { dir_enum: '', password: '', username: '', subdomain: '' },
    executor: {
      default_timeout: 60,
      max_timeout: 600,
      backend: 'local',
      docker_image: 'ctf-toolbox:latest',
      docker_network: 'none',
      docker_data_volume: '',
    },
    proxy: { url: '' },
    ai: {
      api_key: '',
      base_url: 'https://open.bigmodel.cn/api/paas/v4',
      model: 'glm-5.3-flash',
      reasoning_effort: 'low',
      max_turns: 24,
      max_workers: 2,
      plan_rounds: 6,
      timeout_seconds: 3600,
      enable_exec: true,
    },
  };
}

// —— Wails 绑定 ——

const App = rawApp as unknown as {
  GetAppConfig(): Promise<AppConfig>;
  UpdateAppConfig(cfg: AppConfig): Promise<void>;
};

// —— localStorage 回退（浏览器开发模式）——

const LS_KEY = 'ctfnet.settings';

export async function getAppConfig(): Promise<AppConfig> {
  if (isWails()) return App.GetAppConfig();
  try {
    const saved = JSON.parse(localStorage.getItem(LS_KEY) || '{}') as Partial<AppConfig>;
    const def = defaultAppConfig();
    return {
      jndi: { ...def.jndi, ...saved.jndi },
      frp: { ...def.frp, ...saved.frp },
      tools: { ...def.tools, ...saved.tools },
      wordlists: { ...def.wordlists, ...saved.wordlists },
      executor: { ...def.executor, ...saved.executor },
      proxy: { ...def.proxy, ...saved.proxy },
      ai: { ...def.ai, ...saved.ai },
    };
  } catch {
    return defaultAppConfig();
  }
}

export async function updateAppConfig(cfg: AppConfig): Promise<void> {
  if (isWails()) return App.UpdateAppConfig(cfg);
  localStorage.setItem(LS_KEY, JSON.stringify(cfg));
}

// —— 工具表单默认值（配置文件 → 表单预填，与后端 ApplyDefaults 同源）——

/**
 * FRP 工具表单默认值：来自统一配置 frp 组（ssh_* 留空回退 jndi 组，
 * server_addr 默认取 SSH 服务器地址，与 core.settings.go frpDefaults 一致）。
 */
export function frpToolFormDefaults(cfg: AppConfig): FormValues {
  const f = cfg.frp;
  const j = cfg.jndi;
  const host = f.ssh_host || j.ssh_host;
  return {
    ssh_host: host,
    ssh_port: f.ssh_port || j.ssh_port,
    ssh_user: f.ssh_user || j.ssh_user,
    ssh_pass: f.ssh_pass || j.ssh_pass,
    server_addr: host,
    remote_dir: f.remote_dir,
    bind_port: f.bind_port,
    token: f.token,
    vhost_http_port: f.vhost_http_port,
    vhost_https_port: f.vhost_https_port,
    dashboard_port: f.dashboard_port,
    dashboard_user: f.dashboard_user,
    dashboard_pass: f.dashboard_pass,
    frps_version: f.frps_version,
    download_url: f.download_url,
  };
}

/** JNDI 工具表单默认值：来自统一配置 jndi 组（与后端 jndiDefaults 一致）。 */
export function jndiToolFormDefaults(cfg: AppConfig): FormValues {
  const j = cfg.jndi;
  return {
    ssh_host: j.ssh_host,
    ssh_port: j.ssh_port,
    ssh_user: j.ssh_user,
    ssh_pass: j.ssh_pass,
    lhost: j.lhost,
    ldap_port: j.ldap_port,
    rmi_port: j.rmi_port,
    http_port: j.http_port,
    lport: j.lport,
    gadget_class: j.gadget_class,
    shell: j.shell,
    putty_dir: cfg.tools.putty_dir,
  };
}
