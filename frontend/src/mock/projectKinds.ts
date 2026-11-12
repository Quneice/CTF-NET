/**
 * 渗透测试项目条目类别配置（单一数据源）。
 *
 * 每种 kind 声明：中文名、图标、表格列、表单字段。
 * EntryTable / EntryDialog 由本配置驱动，新增类别只改本文件。
 * 与后端 internal/projects/store.go 的 Kind* 常量一一对应。
 */

/** 表单字段类型（刻意保持简单，不复用工具系统的 InputField） */
export interface EntryField {
  /** content JSON 中的键 */
  key: string;
  label: string;
  type: 'text' | 'password' | 'textarea' | 'select' | 'file';
  options?: { label: string; value: string }[];
  placeholder?: string;
  required?: boolean;
  rows?: number;
}

export interface EntryKindInfo {
  kind: string;
  label: string;
  /** lucide-react 图标名 */
  icon: string;
  /** 列表表格列：key 为 content JSON 键（'title' 为条目标题） */
  columns: { key: string; title: string }[];
  /** 新增/编辑表单字段 */
  fields: EntryField[];
}

export const ENTRY_KINDS: EntryKindInfo[] = [
  {
    kind: 'asset',
    label: '资产/资源',
    icon: 'Server',
    columns: [
      { key: 'type', title: '类型' },
      { key: 'value', title: '值' },
      { key: 'note', title: '备注' },
    ],
    fields: [
      {
        key: 'type',
        label: '类型',
        type: 'select',
        required: true,
        options: [
          { label: '域名', value: 'domain' },
          { label: 'IP', value: 'ip' },
          { label: '子域名', value: 'subdomain' },
          { label: 'URL', value: 'url' },
          { label: '服务', value: 'service' },
        ],
      },
      { key: 'value', label: '值', type: 'text', required: true, placeholder: 'example.com / 1.1.1.1' },
      { key: 'note', label: '备注', type: 'text' },
    ],
  },
  {
    kind: 'attack',
    label: '攻击面',
    icon: 'Crosshair',
    columns: [
      { key: 'surface', title: '面类型' },
      { key: 'entry_point', title: '入口点' },
      { key: 'risk', title: '风险等级' },
      { key: 'description', title: '描述' },
    ],
    fields: [
      {
        key: 'surface',
        label: '面类型',
        type: 'select',
        required: true,
        options: [
          { label: 'Web 应用', value: 'web' },
          { label: 'API 接口', value: 'api' },
          { label: '网络服务', value: 'network' },
          { label: '移动应用', value: 'mobile' },
          { label: '其他', value: 'other' },
        ],
      },
      { key: 'entry_point', label: '入口点', type: 'text', required: true, placeholder: 'https://…/login' },
      {
        key: 'risk',
        label: '风险等级',
        type: 'select',
        options: [
          { label: '高', value: 'high' },
          { label: '中', value: 'medium' },
          { label: '低', value: 'low' },
          { label: '未知', value: 'unknown' },
        ],
      },
      { key: 'description', label: '描述', type: 'textarea', rows: 3 },
    ],
  },
  {
    kind: 'port',
    label: '端口扫描',
    icon: 'Radar',
    columns: [
      { key: 'host', title: '主机' },
      { key: 'port', title: '端口' },
      { key: 'protocol', title: '协议' },
      { key: 'service', title: '服务' },
      { key: 'state', title: '状态' },
    ],
    fields: [
      { key: 'host', label: '主机', type: 'text', required: true, placeholder: '1.1.1.1' },
      { key: 'port', label: '端口', type: 'text', required: true, placeholder: '443' },
      {
        key: 'protocol',
        label: '协议',
        type: 'select',
        options: [
          { label: 'TCP', value: 'tcp' },
          { label: 'UDP', value: 'udp' },
        ],
      },
      { key: 'service', label: '服务', type: 'text', placeholder: 'nginx / mysql / ssh' },
      {
        key: 'state',
        label: '状态',
        type: 'select',
        options: [
          { label: '开放', value: 'open' },
          { label: '关闭', value: 'closed' },
          { label: '过滤', value: 'filtered' },
        ],
      },
    ],
  },
  {
    kind: 'fingerprint',
    label: '指纹识别',
    icon: 'Fingerprint',
    columns: [
      { key: 'target', title: '目标' },
      { key: 'product', title: '产品' },
      { key: 'version', title: '版本' },
      { key: 'note', title: '备注' },
    ],
    fields: [
      { key: 'target', label: '目标', type: 'text', required: true, placeholder: 'https://example.com' },
      { key: 'product', label: '产品', type: 'text', required: true, placeholder: 'ThinkPHP / nginx / Shiro' },
      { key: 'version', label: '版本', type: 'text' },
      { key: 'note', label: '备注', type: 'text' },
    ],
  },
  {
    kind: 'poc',
    label: 'PoC/EXP',
    icon: 'Bomb',
    columns: [
      { key: 'vuln', title: '漏洞类型' },
      { key: 'note', title: '备注' },
    ],
    fields: [
      { key: 'vuln', label: '漏洞类型', type: 'text', placeholder: 'SQL 注入 / RCE / 反序列化' },
      { key: 'code', label: 'PoC 代码', type: 'textarea', rows: 8, placeholder: '粘贴 PoC / EXP 脚本内容' },
      { key: 'note', label: '备注', type: 'text' },
    ],
  },
  {
    kind: 'credential',
    label: '凭据',
    icon: 'KeyRound',
    columns: [
      { key: 'username', title: '用户名' },
      { key: 'password', title: '密码' },
      { key: 'source', title: '来源' },
    ],
    fields: [
      { key: 'username', label: '用户名', type: 'text', required: true },
      { key: 'password', label: '密码/密钥', type: 'password', required: true },
      { key: 'source', label: '来源', type: 'text', placeholder: '弱口令 / 数据库泄漏 / 配置文件' },
    ],
  },
  {
    kind: 'sensitive',
    label: '敏感信息',
    icon: 'Eye',
    columns: [
      { key: 'type', title: '类型' },
      { key: 'value', title: '值' },
      { key: 'source', title: '来源' },
    ],
    fields: [
      {
        key: 'type',
        label: '类型',
        type: 'select',
        required: true,
        options: [
          { label: '密钥/Token', value: 'key' },
          { label: '敏感路径', value: 'path' },
          { label: '邮箱', value: 'email' },
          { label: '手机号', value: 'phone' },
          { label: '身份证号', value: 'idcard' },
          { label: '其他', value: 'other' },
        ],
      },
      { key: 'value', label: '值', type: 'textarea', rows: 2, required: true },
      { key: 'source', label: '来源', type: 'text', placeholder: 'JS 文件 / GitHub / 接口响应' },
    ],
  },
  {
    kind: 'note',
    label: '笔记',
    icon: 'NotebookPen',
    columns: [{ key: 'content', title: '内容' }],
    fields: [{ key: 'content', label: '内容', type: 'textarea', rows: 8, required: true, placeholder: '支持 Markdown 文本' }],
  },
  {
    kind: 'attachment',
    label: '附件',
    icon: 'Paperclip',
    columns: [
      { key: 'name', title: '文件名' },
      { key: 'size', title: '大小' },
    ],
    fields: [{ key: 'file', label: '文件', type: 'file', required: true }],
  },
];

export const entryKindMap = new Map(ENTRY_KINDS.map((k) => [k.kind, k]));

export function kindLabel(kind: string): string {
  return entryKindMap.get(kind)?.label ?? kind;
}
