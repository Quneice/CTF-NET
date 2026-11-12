import { useEffect, useState } from 'react';
import { RotateCcw, Save } from 'lucide-react';

import type { AppConfig } from '@/lib/settings';
import { defaultAppConfig, getAppConfig, updateAppConfig } from '@/lib/settings';

/**
 * 设置页：统一编辑 data/config.json（JNDI 服务器 / 外部工具 / 字典 / 执行器）。
 * 保存即生效（执行器组除外，需重启）；手改 config.json 需重启应用。
 */

// —— 表单结构声明（配置驱动，新增配置项只改这里）——

interface FieldDef {
  key: string;
  label: string;
  type: 'text' | 'password' | 'number' | 'select' | 'switch';
  placeholder?: string;
  help?: string;
  options?: { label: string; value: string }[];
}

interface GroupDef {
  key: keyof AppConfig;
  title: string;
  description: string;
  fields: FieldDef[];
}

const GROUPS: GroupDef[] = [
  {
    key: 'jndi',
    title: 'JNDI 服务器',
    description: 'JNDI 注入工具的攻击服务器连接与监听配置；工具参数留空时自动使用这里的值',
    fields: [
      { key: 'ssh_host', label: '攻击服务器', type: 'text', placeholder: '如 1.2.3.4 或 vps.example.com' },
      { key: 'ssh_port', label: 'SSH 端口', type: 'number' },
      { key: 'ssh_user', label: 'SSH 用户名', type: 'text' },
      { key: 'ssh_pass', label: 'SSH 密码', type: 'password' },
      { key: 'lhost', label: '回连公网 IP', type: 'text', help: '留空默认取攻击服务器地址' },
      { key: 'ldap_port', label: 'LDAP 端口', type: 'number' },
      { key: 'rmi_port', label: 'RMI 端口', type: 'number' },
      { key: 'http_port', label: 'HTTP 端口', type: 'number', help: '恶意类下载服务端口' },
      { key: 'lport', label: '反弹监听端口', type: 'number' },
      { key: 'gadget_class', label: '默认 Gadget', type: 'text' },
      { key: 'shell', label: 'Shell 路径', type: 'text' },
    ],
  },
  {
    key: 'frp',
    title: 'FRP 反向代理',
    description: 'FRP 反向代理工具的服务器部署与监听配置；ssh_* 留空时回退上方 JNDI 服务器（共用一台攻击服务器）',
    fields: [
      { key: 'ssh_host', label: 'FRP 服务器', type: 'text', placeholder: '留空用 JNDI 攻击服务器', help: '部署 frps 的服务器 IP/域名' },
      { key: 'ssh_port', label: 'SSH 端口（0 回退 JNDI）', type: 'number' },
      { key: 'ssh_user', label: 'SSH 用户名', type: 'text' },
      { key: 'ssh_pass', label: 'SSH 密码', type: 'password' },
      { key: 'remote_dir', label: '远程部署目录', type: 'text', placeholder: '如 ~/ctfnet-frp' },
      { key: 'bind_port', label: 'frps 监听端口', type: 'number', help: 'frpc 的 serverPort；需放行入站' },
      { key: 'token', label: '认证 Token', type: 'text', help: '留空则每次运行自动生成随机 token' },
      { key: 'vhost_http_port', label: 'vhost HTTP 端口', type: 'number', help: 'http 型代理走此端口，80 需 root' },
      { key: 'vhost_https_port', label: 'vhost HTTPS 端口', type: 'number', help: 'https 型代理走此端口，443 需 root' },
      { key: 'dashboard_port', label: 'Dashboard 端口', type: 'number', help: '填 0 禁用 Dashboard' },
      { key: 'dashboard_user', label: 'Dashboard 用户名', type: 'text' },
      { key: 'dashboard_pass', label: 'Dashboard 密码', type: 'password', help: '留空自动生成' },
      { key: 'frps_version', label: 'FRP 版本', type: 'text', help: '如 0.69.1（与内嵌本地包一致时自动上传，无需联网）' },
      { key: 'download_url', label: '下载地址模板', type: 'text', help: '占位符 {version} {os} {arch}；留空用 GitHub 官方源 + 镜像回退' },
    ],
  },
  {
    key: 'tools',
    title: '外部工具路径',
    description: '外部命令/二进制的位置；填命令名表示走 PATH，JNDI 场景指远程主机上的命令',
    fields: [
      { key: 'python3', label: 'Python 3', type: 'text' },
      { key: 'java', label: 'Java', type: 'text' },
      { key: 'javac', label: 'Javac', type: 'text' },
      { key: 'nmap', label: 'Nmap', type: 'text' },
      { key: 'nuclei', label: 'Nuclei', type: 'text' },
      { key: 'xray', label: 'Xray', type: 'text' },
      { key: 'sqlmap', label: 'Sqlmap', type: 'text' },
      { key: 'putty_dir', label: 'PuTTY 目录', type: 'text', placeholder: '如 D:\\putty' },
      {
        key: 'fscan',
        label: 'Fscan',
        type: 'text',
        placeholder: '如 C:\\Downloads\\fscan.exe',
        help: '端口扫描工具的外部 exe 路径（pentest.scan.port-scan 使用）',
      },
    ],
  },
  {
    key: 'wordlists',
    title: '字典路径',
    description: '各类字典文件位置，供目录枚举、口令破解等工具使用',
    fields: [
      { key: 'dir_enum', label: '目录枚举字典', type: 'text' },
      { key: 'password', label: '口令破解字典', type: 'text' },
      { key: 'username', label: '用户名字典', type: 'text' },
      { key: 'subdomain', label: '子域名字典', type: 'text' },
    ],
  },
  {
    key: 'proxy',
    title: '全局代理',
    description: '配置后应用外发流量（如漏洞浏览器远程图片）统一走代理；留空 = 直连',
    fields: [
      {
        key: 'url',
        label: '代理地址',
        type: 'text',
        placeholder: '如 socks5://127.0.0.1:7897 或 http://127.0.0.1:7890',
        help: '支持 http / https / socks5 / socks5h；留空为直连，保存即生效',
      },
    ],
  },
  {
    key: 'ai',
    title: 'AI 渗透',
    description: 'AI 渗透任务（ai.pentest.task）与 AI 对话渗透（ai.pentest.chat）的 LLM 接入与运行配置；工具参数留空时自动使用这里的值',
    fields: [
      {
        key: 'api_key',
        label: 'API Key',
        type: 'password',
        placeholder: 'sk-...',
        help: 'OpenAI 兼容服务（智谱 GLM/DeepSeek/OpenAI/Ollama/中转等）的 API Key',
      },
      { key: 'base_url', label: 'API 地址', type: 'text', placeholder: 'https://open.bigmodel.cn/api/paas/v4' },
      { key: 'model', label: '模型', type: 'text', placeholder: 'glm-5.3-flash' },
      {
        key: 'reasoning_effort',
        label: '思考强度',
        type: 'text',
        placeholder: 'low',
        help: '智谱 glm-5.x 始终思考模型的思考档位：low/high/max（low 最省 token）；留空=不传（服务端默认 max）',
      },
      { key: 'max_turns', label: '单会话最大轮数', type: 'number', help: '单个 planner/worker 会话的工具往返上限' },
      { key: 'max_workers', label: '并发 worker 数', type: 'number', help: '1-8，意图并行执行上限' },
      { key: 'plan_rounds', label: '规划轮次', type: 'number', help: '规划者被唤醒的次数预算，超出后意图耗尽即收尾' },
      { key: 'timeout_seconds', label: '任务时限（秒）', type: 'number' },
      {
        key: 'enable_exec',
        label: '允许本地命令执行',
        type: 'switch',
        help: '开放后 worker 可用 exec_command 与配方工具在本机执行命令；仅用于授权目标',
      },
    ],
  },
  {
    key: 'executor',
    title: '执行器与超时',
    description: '工具运行后端与超时上限；本组修改需重启应用后生效',
    fields: [
      { key: 'default_timeout', label: '默认超时（秒）', type: 'number' },
      { key: 'max_timeout', label: '最大超时（秒）', type: 'number' },
      {
        key: 'backend',
        label: '执行器后端',
        type: 'select',
        options: [
          { label: '本地执行 (local)', value: 'local' },
          { label: 'Docker 隔离 (docker)', value: 'docker' },
        ],
      },
      { key: 'docker_image', label: 'Docker 镜像', type: 'text' },
      { key: 'docker_network', label: 'Docker 网络', type: 'text' },
      { key: 'docker_data_volume', label: 'Docker 数据卷', type: 'text', help: '留空表示不挂载' },
    ],
  },
];

export default function SettingsPage() {
  const [cfg, setCfg] = useState<AppConfig | null>(null);
  const [status, setStatus] = useState<{ ok: boolean; text: string } | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    document.title = '设置 · CTF-NET';
    getAppConfig()
      .then(setCfg)
      .catch((e) => setStatus({ ok: false, text: `加载配置失败: ${e}` }));
  }, []);

  const setField = (group: keyof AppConfig, key: string, value: string | number | boolean) => {
    setCfg((prev) => (prev ? { ...prev, [group]: { ...prev[group], [key]: value } } : prev));
  };

  const handleSave = async () => {
    if (!cfg) return;
    setSaving(true);
    setStatus(null);
    try {
      await updateAppConfig(cfg);
      setStatus({ text: '已保存并生效（执行器组需重启）', ok: true });
    } catch (e) {
      setStatus({ text: `保存失败: ${e instanceof Error ? e.message : e}`, ok: false });
    } finally {
      setSaving(false);
    }
  };

  if (!cfg) {
    return <div className="p-8 text-sm text-slate-400">加载配置中…</div>;
  }

  return (
    <div className="mx-auto max-w-4xl p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-800 dark:text-slate-100">设置</h1>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            统一配置文件 data/config.json；保存即生效（执行器组除外），手改文件需重启应用
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setCfg(defaultAppConfig())}
            className="flex items-center gap-1.5 rounded-md border border-slate-200 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            <RotateCcw className="h-4 w-4" />
            重置为默认
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-1.5 rounded-md bg-indigo-500 px-4 py-1.5 text-sm font-medium text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            <Save className="h-4 w-4" />
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>

      {status && (
        <div
          className={`mb-4 rounded-md px-3 py-2 text-sm ${
            status.ok
              ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-400'
              : 'bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400'
          }`}
        >
          {status.text}
        </div>
      )}

      <div className="space-y-5">
        {GROUPS.map((group) => (
          <section
            key={group.key}
            className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900"
          >
            <header className="border-b border-slate-100 px-5 py-3 dark:border-slate-800">
              <h2 className="font-medium text-slate-800 dark:text-slate-100">{group.title}</h2>
              <p className="mt-0.5 text-xs text-slate-400">{group.description}</p>
            </header>
            <div className="grid grid-cols-1 gap-4 px-5 py-4 sm:grid-cols-2">
              {group.fields.map((field) => {
                const value = (cfg[group.key] as unknown as Record<string, unknown>)[field.key];
                return (
                  <label key={field.key} className="block">
                    <span className="mb-1 block text-sm text-slate-600 dark:text-slate-300">
                      {field.label}
                    </span>
                    {field.type === 'select' ? (
                      <select
                        value={String(value ?? '')}
                        onChange={(e) => setField(group.key, field.key, e.target.value)}
                        className="w-full rounded-md border border-slate-200 bg-white px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                      >
                        {field.options?.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                      </select>
                    ) : field.type === 'switch' ? (
                      <input
                        type="checkbox"
                        checked={Boolean(value)}
                        onChange={(e) => setField(group.key, field.key, e.target.checked)}
                        className="mt-1 h-5 w-5 rounded border-slate-300 text-indigo-500 focus:ring-indigo-500"
                      />
                    ) : (
                      <input
                        type={field.type === 'number' ? 'number' : field.type === 'password' ? 'password' : 'text'}
                        value={field.type === 'number' ? Number(value ?? 0) : String(value ?? '')}
                        placeholder={field.placeholder}
                        onChange={(e) =>
                          setField(
                            group.key,
                            field.key,
                            field.type === 'number' ? Number(e.target.value) : e.target.value,
                          )
                        }
                        className="w-full rounded-md border border-slate-200 bg-white px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                      />
                    )}
                    {field.help && (
                      <span className="mt-1 block text-xs text-slate-400">{field.help}</span>
                    )}
                  </label>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
