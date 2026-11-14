import type { InputField, InvokeSpec, OutputSpec, ToolDefinition } from '@/types';

/**
 * 工具 mock 数据。
 *
 * 设计原则：
 * 1. 前 15 个「完整工具」逐字手写，覆盖所有 InputField 类型、所有 InvokeSpec
 *    类型、所有 OutputSpec 类型，作为「新增工具只改配置」的范式示例。
 * 2. 其余 90+ 个工具通过 `presets`（预设）+ `shorthandRows`（简写行）批量生成，
 *    用最少代码撑起 100+ 的规模，同时保持类型严格符合 ToolDefinition。
 * 3. 所有 URL / handlerId 均为 mock 占位，阶段 4 的适配层会拦截并模拟返回。
 */

// ---------------------------------------------------------------------------
// 一、完整工具（范式示例）
// ---------------------------------------------------------------------------

/** Base 家族编码清单：每个编码一个独立工具，挂载到 codec.base 子分类(批 2 起) */
const BASE_ENCODINGS: { key: string; name: string; description: string }[] = [
  { key: 'base2', name: 'Base2 编解码', description: 'Base2（二进制）编解码。' },
  { key: 'base8', name: 'Base8 编解码', description: 'Base8（八进制）编解码。' },
  { key: 'base16', name: 'Base16 编解码', description: 'Base16（十六进制）编解码。' },
  { key: 'base32', name: 'Base32 编解码', description: 'Base32（RFC 4648）编解码。' },
  { key: 'base45', name: 'Base45 编解码', description: 'Base45（RFC 9285）编解码。' },
  { key: 'base58', name: 'Base58 编解码', description: 'Base58（Bitcoin）编解码。' },
  { key: 'base62', name: 'Base62 编解码', description: 'Base62 编解码。' },
  { key: 'base64', name: 'Base64 编解码', description: 'Base64 编解码。' },
  { key: 'base64url', name: 'Base64 URL-safe 编解码', description: 'Base64 URL-safe 编解码。' },
  { key: 'base85', name: 'Base85 编解码', description: 'Base85（Ascii85）编解码。' },
  { key: 'base91', name: 'Base91 编解码', description: 'Base91（basE91）编解码。' },
  { key: 'base92', name: 'Base92 编解码', description: 'Base92 编解码。' },
];

/** Base 编码工具的可选项：哪些支持自定义字符集 / 填充字符 */
const BASE_CODEC_OPTIONS: Record<string, ToolDefinition['codec']> = {
  base2: true, base8: true, base16: true, base85: true,
  base32: { alphabet: true, padding: '=' },
  base45: { alphabet: true },
  base58: { alphabet: true },
  base62: { alphabet: true },
  base64: { alphabet: true, padding: '=' },
  base64url: { alphabet: true, padding: '' },
  base91: { alphabet: true },
  base92: { alphabet: true },
};

const baseTools: ToolDefinition[] = BASE_ENCODINGS.map((e) => ({
  id: `crypto.encoding.base.${e.key}`,
  name: e.name,
  // 批 2:Base 家族整体挂载 codec.base(批 1 时仅 base64 先行迁入,现全部迁入)
  categoryId: 'codec.base',
  description: e.description,
  tags: [e.key.toUpperCase(), '编码'],
  version: '1.0.0',
  inputSchema: [
    {
      type: 'radio',
      name: 'mode',
      label: '模式',
      defaultValue: 'decode',
      options: [
        { label: '编码', value: 'encode' },
        { label: '解码', value: 'decode' },
      ],
    },
    { type: 'textarea', name: 'data', label: '数据', required: true, placeholder: '输入要编码/解码的文本…' },
  ],
  invoke: { type: 'local', handlerId: `local.base.${e.key}` },
  output: { type: 'text' },
  codec: BASE_CODEC_OPTIONS[e.key],
}));

/** 常用双向编解码工具（URL / Hex / 摩斯 / ROT47），使用「编码前/编码后」实时互转 UI */
const codecTools: ToolDefinition[] = [
  {
    id: 'crypto.encoding.url-decode',
    name: 'URL 编解码',
    categoryId: 'codec',
    description: 'URL 编码与解码。',
    tags: ['URL', '编码'],
    version: '1.0.0',
    inputSchema: [],
    invoke: { type: 'local', handlerId: 'local.url' },
    output: { type: 'text' },
    codec: true,
  },
  {
    id: 'crypto.encoding.hex-decode',
    name: 'Hex 编解码',
    categoryId: 'codec',
    description: '十六进制与字符串互转。',
    tags: ['Hex', '编码'],
    version: '1.0.0',
    inputSchema: [],
    invoke: { type: 'local', handlerId: 'local.hex' },
    output: { type: 'text' },
    codec: true,
  },
  {
    id: 'crypto.encoding.morse',
    name: '摩斯电码',
    categoryId: 'codec',
    description: '摩斯电码编解码。',
    tags: ['摩斯', '编码'],
    version: '1.0.0',
    inputSchema: [],
    invoke: { type: 'local', handlerId: 'local.morse' },
    output: { type: 'text' },
    codec: true,
  },
  {
    id: 'crypto.encoding.rot47',
    name: 'ROT47',
    categoryId: 'codec',
    description: 'ROT47 可打印 ASCII 旋转编解码（自逆）。',
    tags: ['ROT47', '编码'],
    version: '1.0.0',
    inputSchema: [],
    invoke: { type: 'local', handlerId: 'local.rot47' },
    output: { type: 'text' },
    codec: true,
  },
];

const fullTools: ToolDefinition[] = [
  // —— 本地计算 + 纯文本输出 + select/textarea ——
  {
    id: 'crypto.classical.rot13',
    name: 'ROT13',
    categoryId: 'crypto.classical',
    description: 'ROT13 凯撒移位（位移 13），快速加解密。',
    tags: ['古典密码', '凯撒', '加密', '解密'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decode',
        options: [
          { label: '加密', value: 'encode' },
          { label: '解密', value: 'decode' },
        ],
      },
      { type: 'textarea', name: 'data', label: '数据', required: true, placeholder: '输入要处理的文本…' },
    ],
    invoke: { type: 'local', handlerId: 'local.rot13' },
    output: { type: 'text' },
  },

  // —— 异步任务 + JSON 输出 + group(可折叠)/number/radio/visibleWhen ——
  {
    id: 'crypto.modern.rsa-decrypt',
    name: 'RSA 解密',
    categoryId: 'crypto.modern.rsa',
    description: 'RSA 参数分解与解密，支持常见攻击模式。',
    tags: ['RSA', '现代密码', '解密', '攻击'],
    version: '1.1.0',
    docsUrl: 'https://example.com/rsa',
    inputSchema: [
      {
        type: 'radio',
        name: 'mode',
        label: '攻击模式',
        defaultValue: 'factor',
        options: [
          { label: '已知 p/q', value: 'known-pq' },
          { label: '大数分解', value: 'factor' },
          { label: '低加密指数', value: 'low-e' },
          { label: '共模攻击', value: 'common-modulus' },
        ],
      },
      {
        type: 'group',
        name: 'rsa-params',
        label: 'RSA 参数',
        collapsible: true,
        children: [
          { type: 'number', name: 'p', label: 'p（可选）', placeholder: '大素数 p' },
          { type: 'number', name: 'q', label: 'q（可选）', placeholder: '大素数 q' },
          { type: 'number', name: 'e', label: 'e', defaultValue: 65537, placeholder: '公钥指数' },
          { type: 'number', name: 'n', label: 'n', placeholder: '模数 n' },
        ],
      },
      {
        type: 'textarea',
        name: 'ciphertext',
        label: '密文',
        required: true,
        visibleWhen: { field: 'mode', op: 'in', value: ['factor', 'known-pq', 'low-e', 'common-modulus'] },
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/mock/rsa/submit',
      pollUrl: '/api/mock/rsa/poll',
      intervalMs: 800,
      timeoutMs: 30000,
    },
    output: { type: 'json' },
  },

  // —— HTTP POST + 代码高亮输出 + text/select/switch/disabledWhen ——
  {
    id: 'crypto.modern.aes',
    name: 'AES 加解密',
    categoryId: 'crypto.modern.aes',
    description: 'AES 对称加密，支持 CBC/ECB/GCM 与多种填充。',
    tags: ['AES', '对称加密'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'key', label: '密钥', required: true, placeholder: '密钥' },
      {
        type: 'select',
        name: 'mode',
        label: '加密模式',
        defaultValue: 'CBC',
        options: [
          { label: 'CBC', value: 'CBC' },
          { label: 'ECB', value: 'ECB' },
          { label: 'GCM', value: 'GCM' },
        ],
      },
      {
        type: 'select',
        name: 'padding',
        label: '填充',
        defaultValue: 'PKCS7',
        options: [
          { label: 'PKCS7', value: 'PKCS7' },
          { label: 'Zero', value: 'ZERO' },
          { label: 'No', value: 'NONE' },
        ],
        // GCM 不需要填充，条件禁用演示
        disabledWhen: { field: 'mode', op: 'eq', value: 'GCM' },
      },
      { type: 'switch', name: 'hexOutput', label: '十六进制输出', defaultValue: true },
      { type: 'textarea', name: 'data', label: '数据', required: true },
    ],
    invoke: { type: 'http', method: 'POST', url: '/api/mock/aes', contentType: 'json' },
    output: { type: 'code', language: 'text' },
  },

  // —— CBC 字节翻转攻击（真实后端 crypto.aes_cbc_bitflip）——
  {
    id: 'crypto.modern.aes.cbc-bitflip',
    name: 'CBC 字节翻转攻击',
    categoryId: 'crypto.modern.aes',
    description:
      'AES-CBC 通用攻击：本地计算翻转密文/IV 与泄露补偿；在线翻转+泄露级联补偿攻击（iv/cipher 可来自 Cookie 或响应体，重放走参数占位符或 Cookie）；PKCS7 Padding Oracle 全明文恢复。',
    tags: ['AES', 'CBC', '字节翻转', 'Padding Oracle', '现代密码'],
    version: '1.1.0',
    formColumns: 2,
    inputSchema: [
      {
        type: 'note',
        name: 'note_four_data',
        label: '无密钥修改密文：四类数据',
        spanFull: true,
        content:
          '① 有效明文（模板）—— 定位修改位置、得到期望的块内容；可来自源码泄露 / 报错回显\n'
          + '② 原片段 → 目标片段（等长替换）—— 自动算逐字节差分 Δ，推出要翻转的密文字节\n'
          + '③ IV —— 目标在块 0 时直接翻 IV（IV′ = IV ⊕ Δ）；块 0 被破坏后用它做最终补偿\n'
          + '④ 翻转后解密出的明文 —— 靶机报错泄露（base64 / hex / raw），提供被破坏块的「垃圾值」用于修复\n'
          + '\n'
          + '按目标所在块，哪些数据可缺：\n'
          + '· 目标全在块 0（前 16 字节）→ 只需 ①②③，不需要 ④（翻 IV 不破坏任何密文块）\n'
          + '· 目标在块 1 → 需要 ④ 泄露一次（修复被破坏的块 0）\n'
          + '· 目标在块 ≥2 → 每修复一层都需再泄露一次（级联补偿）\n'
          + '· 没有明文模板 → 先切 oracle 模式恢复整段明文，再回来做翻转',
      },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'flip',
        options: [
          { label: '本地计算（flip）', value: 'flip' },
          { label: '在线攻击（attack）', value: 'attack' },
          { label: 'Padding Oracle 恢复明文（oracle）', value: 'oracle' },
        ],
      },
      {
        type: 'group',
        name: 'g_plain',
        label: '明文与目标（flip / attack）',
        collapsible: true,
        spanFull: true,
        columns: 2,
        visibleWhen: { field: 'mode', op: 'in', value: ['flip', 'attack'] },
        children: [
          {
            type: 'textarea',
            name: 'plaintext',
            label: '已知明文（模板）',
            required: true,
            spanFull: true,
            rows: 2,
            placeholder: 'a:2:{s:8:"username";s:5:"aaaaa";s:8:"password";s:5:"ppppp";}',
            help:
              '加密前的完整明文（如 PHP serialize / JSON 串），工具在明文里定位原片段并计算翻转位置；'
              + '没有模板时可先用 oracle 模式恢复明文',
          },
          { type: 'text', name: 'orig', label: '原片段', required: true, placeholder: 'aaaaa' },
          {
            type: 'text',
            name: 'target',
            label: '目标片段（与原片段等长）',
            required: true,
            placeholder: 'admin',
            help: 'CBC 翻转不能改变长度字段，只能等长替换（如 aaaaa → admin）',
          },
          {
            type: 'select',
            name: 'encoding',
            label: '明文编码',
            defaultValue: 'ascii',
            options: [
              { label: 'ascii', value: 'ascii' },
              { label: 'latin-1', value: 'latin-1' },
              { label: 'utf-8', value: 'utf-8' },
            ],
          },
        ],
      },
      {
        type: 'group',
        name: 'g_local',
        label: '密文材料（flip / oracle）',
        collapsible: true,
        columns: 2,
        visibleWhen: { field: 'mode', op: 'in', value: ['flip', 'oracle'] },
        children: [
          {
            type: 'text',
            name: 'cipher_b64',
            label: '密文',
            spanFull: true,
            placeholder: '完整密文（可选）；oracle 模式留空则用 target_cipher',
            help: 'base64 / hex / raw 按输入编码解析；勾选「密文含 IV」时填 iv‖ct',
          },
          {
            type: 'text',
            name: 'iv_b64',
            label: 'IV（独立，可选）',
            placeholder: '16 字节',
            help: 'flip：与泄露明文搭配算补偿 IV；oracle：恢复块 0 需要它',
          },
          {
            type: 'textarea',
            name: 'leaked_b64',
            label: '泄露的解密明文（flip 补偿用）',
            rows: 2,
            placeholder: '第一次翻转后服务器泄露的解密结果（可选）',
            help:
              '翻转非 0 块会连带破坏前一整块，把泄露值填进来即可算出补偿 IV；'
              + '被破坏块多于 1 块时建议用 attack 模式在线完成',
          },
          {
            type: 'select',
            name: 'input_encoding',
            label: '输入编码',
            defaultValue: 'auto',
            options: [
              { label: 'auto（先 base64 后 hex）', value: 'auto' },
              { label: 'base64', value: 'base64' },
              { label: 'hex', value: 'hex' },
              { label: 'raw（latin-1 原文）', value: 'raw' },
            ],
            help: '作用于密文 / IV / 泄露明文 / target_cipher',
          },
          {
            type: 'switch',
            name: 'iv_in_cipher',
            label: '密文含 IV（iv‖ct）',
            help: '密文值前 16 字节即 IV，无需独立 IV 字段（如 token=iv‖ct 单字段存储）',
          },
        ],
      },
      {
        type: 'group',
        name: 'g_target',
        label: '在线攻击目标（attack 模式）',
        collapsible: true,
        spanFull: true,
        columns: 2,
        visibleWhen: { field: 'mode', op: 'eq', value: 'attack' },
        children: [
          {
            type: 'text',
            name: 'login_url',
            label: '登录端点',
            spanFull: true,
            placeholder: 'http://靶机:端口/login',
            help: '获取 iv/cipher 的请求端点；manual 来源可留空（填写则登录以抓取会话 Cookie）',
          },
          {
            type: 'select',
            name: 'login_method',
            label: '登录方法',
            defaultValue: 'POST',
            options: [
              { label: 'POST', value: 'POST' },
              { label: 'GET', value: 'GET' },
            ],
          },
          {
            type: 'text',
            name: 'login_params',
            label: '登录参数',
            placeholder: 'username=aaaaa&password=x&submit=Login',
            help: 'POST 为表单、GET 为查询串',
          },
          {
            type: 'text',
            name: 'replay_url',
            label: '重放端点（留空=登录端点）',
            placeholder: 'http://靶机:端口/check',
            help: '解密校验发生在别的端点时填写',
          },
          {
            type: 'select',
            name: 'replay_method',
            label: '重放方法',
            defaultValue: 'GET',
            options: [
              { label: 'GET', value: 'GET' },
              { label: 'POST', value: 'POST' },
            ],
          },
          {
            type: 'text',
            name: 'replay_params',
            label: '重放参数',
            placeholder: '留空则值走 Cookie 名；或 token={cipher}&iv={iv}',
            help: '支持 {iv}/{cipher} 占位符（走表单/查询串）；无占位符时按 Cookie 名发送',
          },
          {
            type: 'select',
            name: 'source_type',
            label: 'iv/cipher 来源',
            defaultValue: 'cookie',
            options: [
              { label: 'Set-Cookie', value: 'cookie' },
              { label: '登录响应体（正则捕获）', value: 'body' },
              { label: '手动输入（manual）', value: 'manual' },
            ],
            help: 'cookie：按下方 Cookie 名取；body：按正则从响应体捕获；manual：直接用下方手动填写的 iv/cipher 值',
          },
          {
            type: 'text',
            name: 'session_cookie',
            label: '会话 Cookie 名（留空=不带）',
            defaultValue: 'PHPSESSID',
            help: '部分题目不带会话就不触发解密校验，留空则不携带会话',
          },
          {
            type: 'text',
            name: 'iv_cookie',
            label: 'IV Cookie 名',
            defaultValue: 'iv',
            visibleWhen: { field: 'source_type', op: 'eq', value: 'cookie' },
          },
          {
            type: 'text',
            name: 'cipher_cookie',
            label: '密文 Cookie 名',
            defaultValue: 'cipher',
            visibleWhen: { field: 'source_type', op: 'eq', value: 'cookie' },
          },
          {
            type: 'textarea',
            name: 'iv_body_regex',
            label: 'IV 捕获正则（捕获组 1）',
            rows: 2,
            spanFull: true,
            visibleWhen: { field: 'source_type', op: 'eq', value: 'body' },
            placeholder: '<input[^>]*name="iv"[^>]*value="([^"]+)"',
            help: '勾选「密文含 IV」时可留空',
          },
          {
            type: 'textarea',
            name: 'cipher_body_regex',
            label: '密文捕获正则（捕获组 1）',
            rows: 2,
            spanFull: true,
            visibleWhen: { field: 'source_type', op: 'eq', value: 'body' },
            placeholder: '<input[^>]*name="cipher"[^>]*value="([^"]+)"',
          },
          {
            type: 'text',
            name: 'manual_iv',
            label: '手动 IV 值',
            spanFull: true,
            visibleWhen: { field: 'source_type', op: 'eq', value: 'manual' },
            placeholder: '16 字节，base64/hex/raw 按「值编码」解析',
            help: 'source_type=manual 时直接粘贴 IV（如浏览器 DevTools 里复制的值）；勾选「密文含 IV」时忽略此项',
          },
          {
            type: 'text',
            name: 'manual_cipher',
            label: '手动密文值',
            required: true,
            spanFull: true,
            visibleWhen: { field: 'source_type', op: 'eq', value: 'manual' },
            placeholder: 'base64/hex/raw 按「值编码」解析；勾选「密文含 IV」时填 iv‖ct',
            help: 'source_type=manual 时直接粘贴密文值；会话可用「附加 Cookie」携带（如 PHPSESSID=xxx）',
          },
          {
            type: 'select',
            name: 'value_encoding',
            label: '值编码（捕获 & 重放）',
            defaultValue: 'auto',
            options: [
              { label: 'auto（先 base64 后 hex）', value: 'auto' },
              { label: 'base64', value: 'base64' },
              { label: 'hex', value: 'hex' },
              { label: 'raw（latin-1 原文）', value: 'raw' },
            ],
          },
          {
            type: 'text',
            name: 'extra_cookies',
            label: '附加 Cookie',
            spanFull: true,
            placeholder: 'auth=xxx&token2={cipher}（可选）',
            help: 'a=1&b=2 格式，每次请求都带上；支持 {iv}/{cipher} 占位符',
          },
          {
            type: 'switch',
            name: 'iv_in_cipher',
            label: '密文含 IV（iv‖ct）',
            help: '单字段存 iv‖ct（前 16 字节为 IV）；此时只需捕获一个密文字段',
          },
        ],
      },
      {
        type: 'group',
        name: 'g_adv',
        label: '攻击进阶（attack 模式）',
        collapsible: true,
        columns: 2,
        visibleWhen: { field: 'mode', op: 'eq', value: 'attack' },
        children: [
          {
            type: 'textarea',
            name: 'leak_regex',
            label: '泄露正则',
            rows: 2,
            defaultValue: "base64_decode\\('([A-Za-z0-9+/=]+)'\\)",
            help: '匹配响应里泄露的解密明文（捕获组 1），默认适配 14607 的 die(base64_encode) 格式',
          },
          {
            type: 'select',
            name: 'leak_encoding',
            label: '泄露编码',
            defaultValue: 'base64',
            options: [
              { label: 'base64', value: 'base64' },
              { label: 'hex', value: 'hex' },
              { label: 'raw', value: 'raw' },
            ],
          },
          {
            type: 'text',
            name: 'success_regex',
            label: '成功标志正则',
            defaultValue: 'flag\\{[^}]+\\}',
            help: '重放后在此匹配 flag 等成功标志',
          },
          { type: 'number', name: 'interval', label: '请求间隔（秒）', defaultValue: 0 },
          {
            type: 'number',
            name: 'retries',
            label: '瞬时错误重试次数',
            defaultValue: 3,
            help: '连接失败/未解析到泄露时指数退避重试',
          },
        ],
      },
      {
        type: 'group',
        name: 'g_oracle',
        label: 'Padding Oracle 配置（oracle 模式）',
        collapsible: true,
        spanFull: true,
        columns: 2,
        visibleWhen: { field: 'mode', op: 'eq', value: 'oracle' },
        children: [
          {
            type: 'text',
            name: 'oracle_url',
            label: 'Oracle 端点',
            required: true,
            spanFull: true,
            placeholder: 'http://靶机:端口/decrypt',
            help: '提交 iv′‖C 后有效/无效填充响应有差异的端点',
          },
          {
            type: 'select',
            name: 'oracle_method',
            label: 'Oracle 方法',
            defaultValue: 'GET',
            options: [
              { label: 'GET', value: 'GET' },
              { label: 'POST', value: 'POST' },
            ],
          },
          {
            type: 'text',
            name: 'oracle_params',
            label: 'Oracle 参数（{payload} 占位）',
            spanFull: true,
            placeholder: 'token={payload}',
            help: '{payload} 替换为构造的 iv′‖C 值；留空则只发空请求（配合 oracle_cookie）',
          },
          {
            type: 'text',
            name: 'oracle_cookie',
            label: 'payload 放 Cookie 名（可选）',
            placeholder: 'token',
            help: '与 oracle_params 可同时用',
          },
          {
            type: 'text',
            name: 'target_cipher',
            label: '要恢复的密文（留空用上方「密文」）',
            spanFull: true,
            placeholder: 'base64/hex 密文',
          },
          {
            type: 'text',
            name: 'oracle_ok_regex',
            label: '填充有效正则',
            placeholder: 'padding ok|success|解密成功',
            help:
              '响应体匹配 = 填充有效（与状态码为 AND 关系，两者可只填一个）；'
              + '注意子串陷阱：正则 VALID 会匹配 INVALID，用更精确的词或加锚点',
          },
          {
            type: 'number',
            name: 'oracle_ok_status',
            label: '填充有效状态码（0=不用）',
            defaultValue: 0,
            help: '如服务端以 200/500 区分填充对错，填 200',
          },
          {
            type: 'switch',
            name: 'oracle_invert',
            label: '反转判定',
            help: '匹配/命中 = 填充无效（服务端反着返回时勾选）',
          },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.aes_cbc_bitflip/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 2000,
      timeoutMs: 900000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 异步任务 + 纯文本输出 + file(单文件)/range ——
  // 批 1 F1 修复:接线 Go 真实实现 reverse.strings(参数名与 Go ParamSpec 一致:file / min_len)
  {
    id: 'reverse.static.strings',
    name: 'strings 字符串提取',
    categoryId: 'reverse.static',
    description: '提取二进制文件中的可打印字符串。',
    tags: ['逆向', 'strings', '二进制'],
    version: '1.0.0',
    inputSchema: [
      { type: 'file', name: 'file', label: '文件', multiple: false, accept: '*/*' },
      { type: 'range', name: 'min_len', label: '最小长度', min: 2, max: 16, step: 1, defaultValue: 4 },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/reverse.strings/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 500,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 异步任务 + 纯文本输出(curl -I 可读格式) ——
  // 批 1 F1 修复:接线 Go 真实实现 web.http_headers(仅 url 参数;Go 端不接受自定义头)
  {
    id: 'web.basic.http-headers',
    name: 'HTTP 响应头',
    categoryId: 'web.basic',
    description: '获取目标 URL 的 HTTP 响应头。',
    tags: ['Web', 'HTTP', '响应头'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'url', label: '目标 URL', required: true, placeholder: 'https://example.com' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/web.http_headers/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 目录枚举（接线 Go 真实实现 pentest.scan.dir-enum，参考 gobuster dir） ——
  {
    id: 'pentest.scan.dir-enum',
    name: '目录枚举',
    categoryId: 'pentest.scan',
    description: '字典枚举网站目录与文件（参考 gobuster：并发字典爆破 + 通配符检测）。',
    tags: ['目录', '枚举', 'task:扫描'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'target_url', label: '目标 URL', required: true, placeholder: 'http://192.168.1.10/', spanFull: true },
      { type: 'select', name: 'dict_name', label: '内置字典', optionsSource: 'dirWordlists', help: '从项目字典库 wordlists/dir 选择；留空则按「上传文件 → 设置页默认字典 → 内置小字典」优先级自动选择' },
      { type: 'file', name: 'wordlist', label: '自定义字典文件', multiple: false, accept: '.txt', help: '上传自定义字典，优先级高于库内字典' },
      { type: 'text', name: 'extensions', label: '扩展名', placeholder: 'php,html,txt', help: '逗号分隔，不含扩展的词会追加这些后缀再探测' },
      { type: 'number', name: 'threads', label: '并发线程', defaultValue: 25 },
      { type: 'text', name: 'status_codes', label: '有效状态码', defaultValue: '200,204,301,302,307,401,403' },
      { type: 'number', name: 'timeout', label: '单请求超时（秒）', defaultValue: 10 },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/pentest.scan.dir-enum/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 端口扫描（接线 Go 真实实现 pentest.scan.port-scan：外部 fscan 二进制）——
  {
    id: 'pentest.scan.port-scan',
    name: '端口扫描',
    categoryId: 'pentest.scan',
    description:
      '端口/服务扫描（外部 fscan 二进制：存活探测 + 端口扫描 + Web POC + 弱口令检测；' +
      'exe 路径在设置页「外部工具路径 → Fscan」配置）。',
    tags: ['端口', 'fscan', 'task:扫描'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'text',
        name: 'target',
        label: '目标',
        required: true,
        spanFull: true,
        placeholder: '192.168.1.1 / 192.168.1.1-255 / 192.168.1.0/24 / 域名',
        help: '也支持主机文件路径（fscan -h 参数）',
      },
      { type: 'text', name: 'ports', label: '端口', placeholder: '80,443,8080', help: '逗号分隔；留空用 fscan 内置常用端口' },
      { type: 'number', name: 'threads', label: '扫描线程', defaultValue: 600 },
      { type: 'number', name: 'timeout', label: '端口超时（秒）', defaultValue: 3 },
      {
        type: 'select',
        name: 'mode',
        label: '扫描模式',
        defaultValue: 'all',
        options: [
          { label: '完整扫描（端口 + Web POC + 弱口令）', value: 'all' },
          { label: '仅存活探测', value: 'icmp' },
        ],
      },
      {
        type: 'switch',
        name: 'no_ping',
        label: '跳过存活探测 (-np)',
        defaultValue: true,
        help: '默认开启：目标禁 ping 时仍会扫描（很多服务器禁 ICMP，不跳过反而会漏掉整个主机）；扫大网段想提速可关闭',
      },
      { type: 'switch', name: 'no_poc', label: '不做 Web POC (-nopoc)' },
      { type: 'switch', name: 'no_brute', label: '不做弱口令爆破 (-nobr)' },
      {
        type: 'text',
        name: 'extra_args',
        label: '附加参数',
        spanFull: true,
        placeholder: '如 -full -pa 9090 -lang en',
        help: '按空格切分后原样追加到命令末尾；全局代理已自动映射 -socks5/-proxy',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/pentest.scan.port-scan/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— Web 指纹识别（Go 原生实现 pentest.scan.web-fingerprint：finger-plus 引擎移植）——
  {
    id: 'pentest.scan.web-fingerprint',
    name: 'Web 指纹',
    categoryId: 'pentest.scan',
    description:
      'finger-plus 指纹引擎的 Go 原生移植：静态字符串 + 文件 Hash(murmur3/FOFA) + 路径探测，' +
      '内置 1.3 万+ CMS 指纹库，自动关联已知漏洞。',
    tags: ['指纹', '技术栈', 'CMS', 'task:识别'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'text',
        name: 'target_url',
        label: '目标 URL',
        required: true,
        spanFull: true,
        placeholder: 'http://192.168.1.10/',
      },
      {
        type: 'number',
        name: 'threshold',
        label: '识别阈值',
        defaultValue: 0.3,
        step: 0.05,
        help: '累计得分 ≥ 该值的 CMS 判定为识别成功（与 Python 版默认一致）',
      },
      {
        type: 'switch',
        name: 'enable_path_probe',
        label: '路径探测',
        defaultValue: false,
        help: '默认关闭（开启后会对指纹库路径并发探测、产生大量发包）。开启时：带返回内容校验条件的路径指纹严格探测；其余路径指纹按状态码探测且仅认 403 命中（200 响应在"任意路径都返 200"的目标上全是误报）。内置库 199 条路径指纹均走状态码探测。',
      },
      {
        type: 'number',
        name: 'max_resources',
        label: '资源下载上限',
        defaultValue: 30,
        help: '页面引用的 JS/CSS 最多下载数量（图标最多 3 个）',
      },
      {
        type: 'number',
        name: 'timeout',
        label: '单请求超时（秒）',
        defaultValue: 15,
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/pentest.scan.web-fingerprint/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— JS 接口提取（Go 真实实现 pentest.scan.jsfinder：JSFinder 移植，LinkFinder 正则 + 页面 JS 抓取 + 深度爬取 + 批量文件模式）——
  {
    id: 'pentest.scan.jsfinder',
    name: 'JsFinder',
    categoryId: 'pentest.scan',
    description:
      'JSFinder 的 Go 移植：抓取页面 <script>（含内联脚本）后用 LinkFinder 正则提取 URL 与子域名，' +
      '支持深度爬取一层（<a href> 逐链接）与 URL/JS 清单文件批量模式（Burp 导出）。',
    tags: ['JS', '接口', 'URL', '子域名', 'task:提取'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'text',
        name: 'target_url',
        label: '目标 URL',
        spanFull: true,
        placeholder: 'http://www.example.com/',
        help: '与「URL/JS 文件」二选一（需带 http:// 或 https://）',
      },
      {
        type: 'file',
        name: 'url_file',
        label: 'URL/JS 文件',
        multiple: false,
        accept: '.txt',
        help: '一行一个 URL 或 JS 链接（Burp 抓取导出）；提供后忽略目标 URL',
      },
      {
        type: 'switch',
        name: 'deep',
        label: '深度爬取（-d）',
        defaultValue: false,
        help: '提取页面所有 <a href> 后逐链接再提取一层（仅对目标 URL 生效）',
      },
      {
        type: 'switch',
        name: 'js_mode',
        label: 'JS 模式（-j）',
        defaultValue: false,
        help: 'URL 文件每行视为 JS 文件直接提取（返回相对 URL）',
      },
      {
        type: 'text',
        name: 'cookie',
        label: 'Cookie',
        spanFull: true,
        placeholder: 'session=xxx',
        help: '访问目标时的 Cookie（如需要登录态）',
      },
      { type: 'number', name: 'threads', label: '并发线程', defaultValue: 10, help: '脚本抓取/深度爬取/文件批量的并发度' },
      { type: 'number', name: 'timeout', label: '单请求超时（秒）', defaultValue: 3, help: '与 JSFinder 默认一致，慢站点可调大' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/pentest.scan.jsfinder/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— Packer-Fuzzer（Go 真实实现 pentest.scan.packer_fuzzer：被动 JS 分析，脚本收集 + webpack 指纹 + 精确/暴力接口提取 + BaseUrl 拼接 + GET/POST 探测 + 敏感信息扫描；不含节点反混淆与主动漏洞测试）——
  {
    id: 'pentest.scan.packer_fuzzer',
    name: 'Packer-Fuzzer',
    categoryId: 'pentest.scan',
    description:
      'Packer-Fuzzer 被动 JS 分析链的 Go 移植：收集页面 <script>/<link>/内联脚本与动态引用，' +
      'webpack 打包指纹检测，8 类精确正则 + 暴力提取接口路径并按 BaseUrl 目录拼接完整 API，' +
      '逐个 GET 探测（404/401/405 分类）与可选 POST 回退探测（表单→JSON→XML），' +
      '并扫描 JS 中的 access token/密码/邮箱等敏感信息。',
    tags: ['JS', '接口提取', 'webpack', '接口探测', '敏感信息', 'task:扫描'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'text',
        name: 'target_url',
        label: '目标 URL',
        required: true,
        spanFull: true,
        placeholder: 'http://www.example.com/index.html',
        help: '需带 http:// 或 https://',
      },
      {
        type: 'text',
        name: 'cookie',
        label: 'Cookie',
        spanFull: true,
        placeholder: 'session=xxx',
        help: '访问目标时的 Cookie（如需要登录态）',
      },
      {
        type: 'text',
        name: 'header',
        label: '自定义请求头',
        spanFull: true,
        placeholder: 'Cache-Control:no-cache',
        help: '单个 "Key:Value"，探测与抓取均携带',
      },
      {
        type: 'text',
        name: 'base_url',
        label: 'BaseUrl 目录（可选）',
        spanFull: true,
        placeholder: '/api/v1,/api/v2',
        help: '逗号分隔的接口目录；留空则从 JS 中自动提取（含默认 /）',
      },
      {
        type: 'text',
        name: 'api_host',
        label: '接口主机（可选）',
        spanFull: true,
        placeholder: 'https://api.example.com',
        help: '拼接接口的主机，默认使用目标 URL 的主机',
      },
      {
        type: 'text',
        name: 'file_ext',
        label: '文件扩展名（可选）',
        placeholder: '.json',
        help: '拼接时追加/替换接口路径的扩展名，留空不处理',
      },
      { type: 'switch', name: 'violent', label: '强制暴力提取', defaultValue: false, help: '默认仅当精确提取不足 30 条时自动暴力；开启后始终暴力提取全部引号字符串' },
      { type: 'switch', name: 'probe', label: 'GET 探测接口', defaultValue: true, help: '对拼接后的完整 API 逐个发 GET，按状态码分类' },
      { type: 'switch', name: 'post_probe', label: 'POST 探测接口', defaultValue: false, help: '表单→JSON→XML 回退链探测（默认关闭；原版默认开启，探测为授权测试场景）' },
      {
        type: 'text',
        name: 'post_data',
        label: 'POST 数据（可选）',
        placeholder: 'a=1',
        help: '留空使用内置固定值（表单 a=1 / JSON {"a": "1"} / XML <a>1</a>）',
      },
      { type: 'number', name: 'threads', label: '并发线程', defaultValue: 20, help: 'JS 下载与接口探测的并发度' },
      { type: 'number', name: 'timeout', label: '单请求超时（秒）', defaultValue: 6, help: '与 Packer-Fuzzer 默认一致' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/pentest.scan.packer_fuzzer/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— AI 渗透（一级分类 ai.pentest；Go 原生引擎 internal/aiagent：
  // 自动化任务 = planner/worker 双角色 LLM agent + 意图队列编排 + 内置工具调用 + YAML 配方；
  // 对话渗透 = ChatAgent 直接侦察/验证 + 可发起后台自动化任务；DeepSeek/OpenAI 兼容 API）——
  {
    id: 'ai.pentest.task',
    name: 'AI 渗透任务',
    categoryId: 'ai.pentest',
    description:
      'AI 自动化渗透：LLM 规划者 + 执行者双角色自主侦察与漏洞验证。规划者读态势批量派发意图，' +
      '执行者用工具集（HTTP 请求 / 本地命令 / 配方工具）验证并把漏洞、事实、资产写回黑板，' +
      '循环推进直到目标达成或意图耗尽。产出带证据的漏洞清单与完整执行轨迹。',
    tags: ['AI', 'LLM', 'Agent', '自动化', 'task:渗透'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'text',
        name: 'goal',
        label: '渗透目标',
        spanFull: true,
        placeholder: 'http://192.168.1.10:8080 的 Web 服务，找出可利用漏洞并证明',
        help: '目标地址 + 期望成果，必填',
      },
      {
        type: 'text',
        name: 'description',
        label: '任务描述',
        spanFull: true,
        placeholder: '（可选，默认取目标）',
      },
      {
        type: 'text',
        name: 'target_hint',
        label: '已知线索',
        spanFull: true,
        placeholder: '如：8080 端口 Nginx，后台路径 /admin 已发现',
        help: '资产/端口/凭据等线索，供规划者起步参考（可选）',
      },
      { type: 'number', name: 'timeout_seconds', label: '任务时限（秒）', defaultValue: 3600, help: '留空取设置页 AI 配置' },
      { type: 'number', name: 'max_workers', label: '并发 worker 数', defaultValue: 2, help: '1-8；留空取设置页 AI 配置' },
      {
        type: 'password',
        name: 'api_key',
        label: 'API Key（可选）',
        spanFull: true,
        placeholder: '留空使用设置页 AI 配置',
      },
      {
        type: 'text',
        name: 'model',
        label: '模型（可选）',
        spanFull: true,
        placeholder: '如 glm-5.3-flash；留空使用设置页 AI 配置',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/ai.pentest.task/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 2000,
      timeoutMs: 3600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 对话式渗透测试（Go 原生 ChatAgent：每轮消息 = 一次工具运行，会话历史持久化；
  // 助手可直接侦察/验证，也可发起后台自动化任务；入口推荐侧边栏「AI 对话」页）——
  {
    id: 'ai.pentest.chat',
    name: 'AI 对话渗透',
    categoryId: 'ai.pentest',
    description:
      '对话式渗透测试：与 AI 助手对话推进渗透。助手可直接执行侦察验证工具（HTTP 请求/本地命令/配方），' +
      '把事实与漏洞沉淀到会话黑榜，并可发起后台自动化渗透任务（start_task）后随时查询进展。' +
      '会话历史持久化，传 session_id 续聊；推荐在侧边栏「AI 对话」页使用完整聊天界面。',
    tags: ['AI', '对话', 'Agent', 'task:利用'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'text',
        name: 'message',
        label: '消息',
        spanFull: true,
        placeholder: '如：帮我侦察 http://192.168.1.10:8080，先看响应头与后台路径',
        help: '目标描述/指令/追问均可，必填',
      },
      {
        type: 'text',
        name: 'session_id',
        label: '会话 ID（可选）',
        spanFull: true,
        placeholder: '空 = 新会话；续聊填上次返回的 session_id',
        help: '结果 data.session_id 返回新会话 ID',
      },
      {
        type: 'switch',
        name: 'enable_exec',
        label: '允许本地命令执行',
        defaultValue: true,
        help: '关闭后助手只能用 HTTP 与配方工具',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/ai.pentest.chat/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1500,
      timeoutMs: 1800000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 域名枚举（OneForAll v0.4.5 数据修复版 Go 移植：38 被动源 + 9.5 万词爆破 + SRV + 泛解析过滤 + HTTP 探测 + finder/altdns + CDN/归属地）——
  {
    id: 'pentest.scan.subdomain-enum',
    name: '域名枚举',
    categoryId: 'pentest.scan',
    description:
      'OneForAll v0.4.5 数据修复版 Go 移植：证书透明/搜索引擎/数据集等 38 个被动源 + ' +
      '9.5 万词字典爆破 + SRV 枚举 + 泛解析过滤 + HTTP 探测 + finder/altdns 扩展 + CDN/归属地信息丰富，' +
      '导出 CSV/TXT/JSON 报告。',
    tags: ['子域名', '枚举', 'OneForAll', 'task:扫描'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'text',
        name: 'target',
        label: '目标域名',
        required: true,
        spanFull: true,
        placeholder: 'example.com',
      },
      {
        type: 'switch',
        name: 'enable_collect',
        label: '被动收集',
        defaultValue: true,
        help: '证书透明（crt.sh/CertSpotter/GoogleCT）、搜索引擎（Bing/百度/Google/Yahoo…）、数据集（NetCraft/DNSDumpster/Robtex/AlienVault…）等 38 个无 key 模块',
      },
      {
        type: 'switch',
        name: 'enable_brute',
        label: '字典爆破',
        defaultValue: true,
        help: '内置 95266 词修复版字典 + SRV 前缀枚举（brute_concurrent 并发解析，默认 2000）',
      },
      {
        type: 'radio',
        name: 'http_port',
        label: 'HTTP 探测端口',
        defaultValue: 'small',
        options: [
          { label: 'small（80,443）', value: 'small' },
          { label: 'medium（5 端口）', value: 'medium' },
          { label: 'large（149 端口）', value: 'large' },
        ],
        help: '对解析成功的子域做 HTTP 存活探测（标题/状态码/banner），默认 small',
      },
      {
        type: 'switch',
        name: 'only_alive',
        label: '仅导出存活',
        defaultValue: false,
        help: '只导出 alive=1 的子域到 CSV/TXT/JSON',
      },
      {
        type: 'number',
        name: 'brute_concurrent',
        label: '爆破并发数',
        defaultValue: 2000,
        help: '字典爆破的 DNS 解析并发（对齐 OneForAll 默认）',
      },
      {
        type: 'switch',
        name: 'enable_dns',
        label: 'DNS 解析',
        defaultValue: true,
        help: '关闭则收集/爆破后直接导出（等价 --dns False）',
      },
      {
        type: 'switch',
        name: 'enable_req',
        label: 'HTTP 探测',
        defaultValue: true,
        help: '关闭则解析后直接导出（等价 --req False）',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/pentest.scan.subdomain-enum/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 3600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— FRP 反向代理（接线 Go 真实实现 pentest.tunnel.frp：SSH 自动部署 frps + 生成 frpc 配置）——
  {
    id: 'pentest.tunnel.frp',
    name: 'FRP 反向代理',
    categoryId: 'pentest.tunnel',
    description:
      'SSH 自动在服务器部署 frps 并生成 frpc 客户端配置（tcp/udp/http/https/socks5），' +
      '把 frpc 与配置上传到目标服务器运行即可实现内网反向代理。',
    tags: ['内网穿透', '隧道', 'frp', '反向代理'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'radio',
        name: 'mode',
        label: '模式',
        defaultValue: 'full',
        options: [
          { label: '完整流程：部署 frps + 生成 frpc 配置', value: 'full' },
          { label: '仅部署 frps', value: 'deploy_frps' },
          { label: '仅生成 frpc 配置（不连服务器）', value: 'gen_frpc' },
          { label: '停止远端 frps', value: 'stop_frps' },
        ],
      },
      {
        type: 'group',
        name: 'g_ssh',
        label: '服务器（SSH）',
        collapsible: true,
        visibleWhen: { field: 'mode', op: 'in', value: ['full', 'deploy_frps', 'stop_frps'] },
        children: [
          {
            type: 'text',
            name: 'ssh_host',
            label: 'SSH 主机',
            placeholder: '留空用「设置 → FRP 反向代理」配置',
            help: '部署 frps 的服务器 IP/域名',
          },
          { type: 'number', name: 'ssh_port', label: 'SSH 端口', placeholder: '留空用设置页配置' },
          { type: 'text', name: 'ssh_user', label: 'SSH 用户名', placeholder: '留空用设置页配置' },
          { type: 'password', name: 'ssh_pass', label: 'SSH 密码', placeholder: '留空用设置页配置' },
          { type: 'text', name: 'remote_dir', label: '远程部署目录', defaultValue: '~/ctfnet-frp' },
        ],
      },
      {
        type: 'group',
        name: 'g_frps',
        label: 'frps 部署与监听',
        collapsible: true,
        visibleWhen: { field: 'mode', op: 'in', value: ['full', 'deploy_frps'] },
        children: [
          {
            type: 'file',
            name: 'frps_binary',
            label: '本地 frps 二进制（可选）',
            accept: '*/*',
            help: '服务器无法联网时上传本地 frps 可执行文件，跳过下载',
          },
          { type: 'text', name: 'frps_version', label: 'FRP 版本', defaultValue: '0.69.1', help: '与内嵌本地包一致时自动上传，无需联网下载' },
          {
            type: 'text',
            name: 'download_url',
            label: '下载地址模板（可选）',
            spanFull: true,
            placeholder: 'https://…/frp_{version}_{os}_{arch}.tar.gz',
            help: '占位符 {version} {os} {arch}；留空用 GitHub 官方源 + 镜像回退',
          },
          { type: 'number', name: 'vhost_http_port', label: 'vhost HTTP 端口', defaultValue: 8080 },
          { type: 'number', name: 'vhost_https_port', label: 'vhost HTTPS 端口', defaultValue: 8443 },
          { type: 'number', name: 'dashboard_port', label: 'Dashboard 端口', defaultValue: 7500, help: '填 0 禁用' },
          { type: 'text', name: 'dashboard_user', label: 'Dashboard 用户名', defaultValue: 'admin' },
          { type: 'password', name: 'dashboard_pass', label: 'Dashboard 密码', placeholder: '留空自动生成' },
        ],
      },
      {
        type: 'group',
        name: 'g_frpc',
        label: 'frpc 客户端配置（连接与代理）',
        collapsible: true,
        visibleWhen: { field: 'mode', op: 'in', value: ['full', 'gen_frpc'] },
        children: [
          { type: 'number', name: 'bind_port', label: 'frps 监听端口', defaultValue: 7000, help: '即 frpc 的 serverPort' },
          { type: 'password', name: 'token', label: '认证 Token', placeholder: '留空自动生成随机 token' },
          {
            type: 'text',
            name: 'server_addr',
            label: 'frpc 连接地址',
            placeholder: '留空默认取 SSH 服务器地址',
            help: 'frps 服务器公网地址（IP 或域名）',
          },
          {
            type: 'radio',
            name: 'proxy_type',
            label: '代理类型',
            defaultValue: 'tcp',
            options: [
              { label: 'tcp（端口映射）', value: 'tcp' },
              { label: 'udp（端口映射）', value: 'udp' },
              { label: 'http（按域名）', value: 'http' },
              { label: 'https（按域名）', value: 'https' },
              { label: 'socks5（代理出口，扫内网用这个）', value: 'socks5' },
            ],
          },
          { type: 'text', name: 'proxy_name', label: '代理名称', placeholder: '留空自动生成' },
          {
            type: 'select',
            name: 'client_platform',
            label: 'frpc 客户端平台',
            defaultValue: 'linux_amd64',
            options: [
              { label: 'linux_amd64', value: 'linux_amd64' },
              { label: 'linux_arm64', value: 'linux_arm64' },
              { label: 'windows_amd64', value: 'windows_amd64' },
            ],
            help: '从内嵌包解出对应平台的 frpc 作为产物下载',
          },
          { type: 'text', name: 'local_ip', label: '本地服务 IP', defaultValue: '127.0.0.1' },
          { type: 'number', name: 'local_port', label: '本地端口', required: true, help: '目标服务器上要转发的本地服务端口' },
          {
            type: 'number',
            name: 'remote_port',
            label: '远端端口',
            visibleWhen: { field: 'proxy_type', op: 'in', value: ['tcp', 'udp'] },
            help: '映射到 frps 的对外端口；留空自动分配（full 模式探测服务器空闲端口，优先取本地端口）',
          },
          {
            type: 'text',
            name: 'custom_domain',
            label: '自定义域名',
            visibleWhen: { field: 'proxy_type', op: 'in', value: ['http', 'https'] },
            placeholder: '如 app.example.com（需解析到服务器）',
          },
        ],
      },
      {
        type: 'switch',
        name: 'remove_files',
        label: '停止时删除部署目录',
        defaultValue: false,
        visibleWhen: { field: 'mode', op: 'eq', value: 'stop_frps' },
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/pentest.tunnel.frp/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 2000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 本地计算 + 十六进制输出 + checkboxGroup ——
  {
    id: 'crypto.hash.hash',
    name: '哈希计算',
    categoryId: 'crypto.hash',
    description: '计算 MD5 / SHA1 / SHA256 等常见哈希。',
    tags: ['哈希', 'md5', 'sha256'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'checkboxGroup',
        name: 'algos',
        label: '算法',
        defaultValue: ['md5', 'sha256'],
        options: [
          { label: 'MD5', value: 'md5' },
          { label: 'SHA1', value: 'sha1' },
          { label: 'SHA256', value: 'sha256' },
          { label: 'SHA512', value: 'sha512' },
        ],
      },
      { type: 'textarea', name: 'data', label: '数据', required: true },
    ],
    invoke: { type: 'local', handlerId: 'local.hash' },
    output: { type: 'hex' },
  },

  // —— HTTP form-data + 图片输出 + file/range/switch ——
  {
    id: 'stego.image.lsb',
    name: 'LSB 隐写提取',
    categoryId: 'stego.image',
    description: '从图片最低有效位提取隐藏数据。',
    tags: ['隐写', 'LSB', '图片'],
    version: '1.0.0',
    inputSchema: [
      { type: 'file', name: 'image', label: '图片文件', accept: 'image/*' },
      { type: 'range', name: 'bit', label: '提取位', min: 0, max: 7, step: 1, defaultValue: 0 },
      { type: 'switch', name: 'extract', label: '提取模式（关闭则为嵌入）', defaultValue: true },
    ],
    invoke: { type: 'http', method: 'POST', url: '/api/mock/lsb', contentType: 'form-data' },
    output: { type: 'image' },
  },

  // —— SSE 流式 + 流式输出 + number/text ——
  {
    id: 'misc.demo.sse-stream',
    name: 'SSE 流式输出示例',
    categoryId: 'misc.demo',
    description: '演示服务端推送（SSE）流式返回。',
    tags: ['演示', 'SSE', '流式', 'task:演示'],
    version: '1.0.0',
    inputSchema: [
      { type: 'number', name: 'chunks', label: '分块数量', defaultValue: 10, placeholder: '1-100' },
      { type: 'text', name: 'topic', label: '主题', placeholder: '可选' },
    ],
    invoke: { type: 'sse', url: '/api/mock/sse' },
    output: { type: 'stream' },
  },

  // —— WebSocket + 流式输出 ——
  {
    id: 'misc.demo.ws-echo',
    name: 'WebSocket 回显',
    categoryId: 'misc.demo',
    description: '演示 WebSocket 双向通信。',
    tags: ['演示', 'WebSocket', '流式', 'task:演示'],
    version: '1.0.0',
    inputSchema: [{ type: 'textarea', name: 'message', label: '消息', required: true }],
    invoke: { type: 'ws', url: 'ws://localhost:5173/api/mock/ws' },
    output: { type: 'stream' },
  },

  // —— HTTP form-data + 文件下载输出 + file/checkboxGroup/code ——
  {
    id: 'forensics.disk.carve',
    name: '文件雕刻',
    categoryId: 'forensics.disk',
    description: '从磁盘镜像中雕刻（恢复）指定类型的文件。',
    tags: ['取证', '磁盘', '雕刻'],
    version: '1.0.0',
    inputSchema: [
      { type: 'file', name: 'image', label: '磁盘镜像', accept: '.img,.dd,.raw' },
      {
        type: 'checkboxGroup',
        name: 'types',
        label: '目标类型',
        defaultValue: ['jpg'],
        options: [
          { label: 'JPG', value: 'jpg' },
          { label: 'PNG', value: 'png' },
          { label: 'ZIP', value: 'zip' },
          { label: 'PDF', value: 'pdf' },
        ],
      },
      { type: 'code', name: 'signature', label: '自定义文件头（十六进制）', language: 'hex', rows: 3 },
    ],
    invoke: { type: 'http', method: 'POST', url: '/api/mock/carve', contentType: 'form-data' },
    output: { type: 'file' },
  },

  // —— 本地计算 + 代码高亮输出 + file/select ——
  {
    id: 'reverse.static.disasm',
    name: '反汇编',
    categoryId: 'reverse.static',
    description: '对二进制进行反汇编分析。',
    tags: ['逆向', '反汇编', '静态分析'],
    version: '1.0.0',
    inputSchema: [
      { type: 'file', name: 'binary', label: '二进制文件', accept: '*/*' },
      {
        type: 'select',
        name: 'arch',
        label: '架构',
        defaultValue: 'x86-64',
        options: [
          { label: 'x86-64', value: 'x86-64' },
          { label: 'x86', value: 'x86' },
          { label: 'ARM', value: 'arm' },
          { label: 'ARM64', value: 'arm64' },
        ],
      },
    ],
    invoke: { type: 'local', handlerId: 'local.disasm' },
    output: { type: 'code', language: 'asm' },
  },

  // —— 本地计算 + JSON 输出 + radio/password/visibleWhen ——
  {
    id: 'web.basic.jwt-decode',
    name: 'JWT 解码/伪造',
    categoryId: 'web.basic',
    description: '解码 JWT，或使用密钥伪造签名。',
    tags: ['JWT', 'Web', '签名'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'radio',
        name: 'action',
        label: '操作',
        defaultValue: 'decode',
        options: [
          { label: '解码', value: 'decode' },
          { label: '伪造签名', value: 'forge' },
        ],
      },
      {
        type: 'password',
        name: 'secret',
        label: '签名密钥（HS256）',
        visibleWhen: { field: 'action', op: 'eq', value: 'forge' },
      },
      { type: 'textarea', name: 'token', label: 'JWT Token', required: true },
    ],
    invoke: { type: 'local', handlerId: 'local.jwt' },
    output: { type: 'json' },
  },

  // —— 本地计算 + Markdown 输出 ——
  {
    id: 'misc.report.writeup',
    name: 'Writeup 报告生成',
    categoryId: 'misc.report',
    description: '根据解题过程生成 Markdown Writeup。',
    tags: ['报告', 'Markdown', 'Writeup', 'task:生成'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'title', label: '题目名称', required: true },
      { type: 'textarea', name: 'steps', label: '解题步骤', required: true },
      { type: 'textarea', name: 'flag', label: 'Flag' },
    ],
    invoke: { type: 'local', handlerId: 'local.writeup' },
    output: { type: 'markdown' },
  },

  // —— HTTP PUT + 自定义渲染输出（custom rendererId） ——
  {
    id: 'blockchain.chain.trace',
    name: '交易追踪',
    categoryId: 'blockchain.chain',
    description: '追踪链上交易流向，生成关系图。',
    tags: ['区块链', '交易', '追踪'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'address', label: '地址 / TxHash', required: true },
      {
        type: 'select',
        name: 'chain',
        label: '链',
        defaultValue: 'eth',
        options: [
          { label: 'Ethereum', value: 'eth' },
          { label: 'BSC', value: 'bsc' },
          { label: 'Polygon', value: 'polygon' },
        ],
      },
    ],
    invoke: { type: 'http', method: 'PUT', url: '/api/mock/trace', contentType: 'json' },
    output: { type: 'custom', rendererId: 'tx-graph' },
  },

  // —— 本地计算 + 纯文本输出 + radio/number/textarea（栅栏密码，含栏数爆破） ——
  {
    id: 'crypto.classical.rail-fence',
    name: '栅栏密码',
    categoryId: 'crypto.classical',
    description: '栅栏（Rail Fence）转置密码，支持编解码与按栏数自动爆破。',
    tags: ['古典密码', '栅栏'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'radio',
        name: 'mode',
        label: '模式',
        defaultValue: 'decode',
        options: [
          { label: '编码', value: 'encode' },
          { label: '解码', value: 'decode' },
          { label: '栏数爆破', value: 'brute' },
        ],
      },
      {
        type: 'select',
        name: 'variant',
        label: '变体',
        defaultValue: 'zigzag',
        options: [
          { label: '之字形（Zigzag）', value: 'zigzag' },
          { label: '循环（Cyclic）', value: 'cyclic' },
        ],
        visibleWhen: { field: 'mode', op: 'in', value: ['encode', 'decode'] },
      },
      {
        type: 'number',
        name: 'rails',
        label: '栏数',
        defaultValue: 3,
        placeholder: '栅栏栏数（≥2）',
        visibleWhen: { field: 'mode', op: 'in', value: ['encode', 'decode'] },
      },
      { type: 'textarea', name: 'data', label: '数据', required: true, placeholder: '输入要编码/解码的文本…' },
    ],
    invoke: { type: 'local', handlerId: 'local.rail-fence' },
    output: { type: 'text' },
  },

  // —— 本地计算 + 纯文本输出 + textarea（自动暴力解码） ——
  {
    id: 'crypto.auto-decode',
    name: '自动解码（暴力解码）',
    categoryId: 'crypto.auto-decode',
    description: '对目标自动尝试所有常见解码方式（Base 家族 / Hex / URL / ROT / ROT-N / 摩斯 / 栅栏 / 兽音 / 倒序等），支持多轮解码，暴力找出可读明文与 flag。',
    tags: ['解码', '暴力', '自动'],
    version: '1.0.0',
    inputSchema: [
      { type: 'textarea', name: 'data', label: '数据', required: true, placeholder: '粘贴要解码的密文…' },
    ],
    invoke: { type: 'local', handlerId: 'local.auto-decode' },
    output: { type: 'text' },
  },

  // —— RSA · Fermat 分解（|p-q| 较小时） ——
  {
    id: 'crypto.modern.rsa.fermat-family.close',
    name: '|p-q| 小 · Fermat 分解',
    categoryId: 'crypto.modern.rsa',
    description: '当 p、q 相距很近时，用 n = a² - b² = (a-b)(a+b) 分解 n。',
    tags: ['RSA', 'Fermat', '数值接近'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, placeholder: '支持十进制或 0x 前缀十六进制' },
      { type: 'number', name: 'maxIter', label: '最大迭代次数', defaultValue: 100000, placeholder: '默认 100000' },
    ],
    invoke: { type: 'local', handlerId: 'local.rsa-fermat' },
    output: { type: 'text' },
  },

  // —— RSA · 高次关联开根（n ≈ r^(2k)，p、q 为同一 r 的 k 次多项式） ——
  {
    id: 'crypto.modern.rsa.shared-structure.nth-root',
    name: '同一 r 高次关联 · 开根',
    categoryId: 'crypto.modern.rsa',
    description: 'p、q 为同一 r 的 k 次多项式时 n ≈ r^(2k)，开 2k 次方根恢复 r。',
    tags: ['RSA', '开根', '同一生成结构'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, placeholder: '支持十进制或 0x 前缀十六进制' },
      { type: 'number', name: 'k', label: '次数 k（n ≈ r^(2k)）', defaultValue: 5, placeholder: '默认 5' },
      { type: 'number', name: 'delta', label: '搜索窗口 ±delta', defaultValue: 5, placeholder: '默认 5' },
    ],
    invoke: { type: 'local', handlerId: 'local.rsa-nth-root' },
    output: { type: 'text' },
  },

  // —— RSA · 共模攻击（同一 n，两组互素指数加密同一明文） ——
  {
    id: 'crypto.modern.rsa-common-modulus',
    name: 'RSA 共模攻击',
    categoryId: 'crypto.modern.rsa',
    description: '同一模数 n 下，两组公钥 (e1, c1)、(e2, c2) 加密同一明文 m；gcd(e1,e2)=1 时用扩展欧几里得 s1·e1 + s2·e2 = 1 直接恢复明文，无需分解 n。',
    tags: ['RSA', '共模攻击', '扩展欧几里得'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, placeholder: '支持十进制或 0x 前缀十六进制' },
      { type: 'text', name: 'e1', label: '公钥指数 e1', required: true, placeholder: '十进制或 0x' },
      { type: 'text', name: 'c1', label: '密文 c1', required: true, placeholder: 'm^e1 mod n' },
      { type: 'text', name: 'e2', label: '公钥指数 e2', required: true, placeholder: '十进制或 0x' },
      { type: 'text', name: 'c2', label: '密文 c2', required: true, placeholder: 'm^e2 mod n' },
    ],
    invoke: { type: 'local', handlerId: 'local.rsa-common-modulus' },
    output: { type: 'text' },
  },

  // —— RSA · Wiener 攻击（e 大 d 小，低解密指数攻击） ——
  {
    id: 'crypto.modern.rsa-wiener',
    name: 'RSA Wiener 攻击',
    categoryId: 'crypto.modern.rsa',
    description: '低解密指数攻击：e 很大、d 很小（d < n^(1/4)/3）时，连分数展开 e/n 的各渐近分数 k/d 逐个验证，由 φ 反解 p、q 并恢复私钥 d；提供密文 c 时直接解出明文。',
    tags: ['RSA', 'Wiener', '连分数', '低解密指数'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, placeholder: '支持十进制或 0x 前缀十六进制' },
      { type: 'text', name: 'e', label: '公钥指数 e', required: true, placeholder: '十进制或 0x（Wiener 场景下 e 通常很大）' },
      { type: 'text', name: 'c', label: '密文 c（可选）', placeholder: '提供则同时解出明文 m = c^d mod n' },
    ],
    invoke: { type: 'local', handlerId: 'local.rsa-wiener' },
    output: { type: 'text' },
  },

  // —— 古典密码套件（真实后端 crypto.classical_*，算法移植自 CryptoDen 并对拍一致） ——
  {
    id: 'crypto.classical.caesar-decrypt',
    name: '凯撒密码',
    categoryId: 'crypto.classical.caesar',
    description: '凯撒移位加解密（大小写保留、非字母原样），支持一键穷举全部 25 个位移。',
    tags: ['古典密码', '凯撒', '穷举'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true, placeholder: '密文 / 明文' },
      { type: 'number', name: 'shift', label: '位移', defaultValue: 3, help: '0-25' },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
      { type: 'switch', name: 'brute', label: '穷举 25 个位移', defaultValue: false, help: '输出全部位移的解密结果' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.caesar/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.vigenere-decrypt',
    name: '维吉尼亚密码',
    categoryId: 'crypto.classical.vigenere',
    description: '维吉尼亚加解密：printable 字母表（CryptoDen 兼容，100 可见字符全移位）或经典 A-Z 字母模式（已知密钥）。',
    tags: ['古典密码', '维吉尼亚'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true },
      { type: 'text', name: 'key', label: '密钥', required: true, placeholder: '关键词' },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
      {
        type: 'select',
        name: 'alphabet',
        label: '字母表',
        defaultValue: 'printable',
        options: [
          { label: 'printable（CryptoDen 兼容）', value: 'printable' },
          { label: 'alpha（经典 A-Z）', value: 'alpha' },
        ],
        help: 'printable：数字/符号/空格全参与移位（CryptoDen 行为）；alpha：仅字母移位',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.vigenere/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.affine-decrypt',
    name: '仿射密码',
    categoryId: 'crypto.classical.affine',
    description: '仿射密码 y=(a·x+b) mod 26 加解密（a 与 26 互素），支持穷举全部 12×26 组密钥。',
    tags: ['古典密码', '仿射', '穷举'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true },
      { type: 'number', name: 'a', label: '乘数 a', defaultValue: 5, help: '须与 26 互素' },
      { type: 'number', name: 'b', label: '偏移 b', defaultValue: 8 },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
      { type: 'switch', name: 'brute', label: '穷举 312 组密钥', defaultValue: false, help: 'a∈12 个互素值 × b∈0..25' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.affine/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.atbash',
    name: '埃特巴什 Atbash',
    categoryId: 'crypto.classical',
    description: '埃特巴什码（A↔Z / B↔Y 字母表反向映射，自逆变换，大小写保留）。',
    tags: ['古典密码', 'Atbash'],
    version: '1.0.0',
    inputSchema: [{ type: 'textarea', name: 'data', label: '文本', required: true, placeholder: '加密解密同一函数' }],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.atbash/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.reverse',
    name: '倒序密码',
    categoryId: 'crypto.classical',
    description: '字符串反转（Unicode 安全，中英文均可）。',
    tags: ['古典密码', '倒序'],
    version: '1.0.0',
    inputSchema: [{ type: 'textarea', name: 'data', label: '文本', required: true }],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.reverse/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.polybius',
    name: '波利比奥斯 Polybius',
    categoryId: 'crypto.classical',
    description: '波利比奥斯 5×5 方阵坐标（I/J 合并为 24，空格=0，坐标空格分隔）。',
    tags: ['古典密码', 'Polybius', '棋盘密码'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本 / 坐标串', required: true, spanFull: true, placeholder: '加密：字母文本；解密：如 23 15 31 31 34' },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.polybius/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.keyed.fenham',
    name: '费娜姆 XOR',
    categoryId: 'crypto.classical.keyed',
    description: '单字节循环 XOR（Fenham 密码）：密钥 0-255，输出十六进制 / 十进制 / 文本，加解密同函数。',
    tags: ['古典密码', 'XOR', 'Fenham'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true },
      { type: 'number', name: 'key', label: '密钥', defaultValue: 42, help: '0-255 单字节' },
      {
        type: 'select',
        name: 'out',
        label: '输出格式',
        defaultValue: 'hex',
        options: [
          { label: '十六进制', value: 'hex' },
          { label: '十进制', value: 'dec' },
          { label: '文本', value: 'text' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.xor/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.keyed.hill',
    name: '希尔密码 Hill 2×2',
    categoryId: 'crypto.classical.keyed',
    description: '希尔密码（2×2 密钥矩阵，det 与 26 互素方可逆；明文奇数长度补 x，解密默认去尾部补位）。',
    tags: ['古典密码', 'Hill', '矩阵'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true, placeholder: '仅字母参与，输出小写' },
      { type: 'number', name: 'k1', label: 'k1', defaultValue: 3 },
      { type: 'number', name: 'k2', label: 'k2', defaultValue: 3 },
      { type: 'number', name: 'k3', label: 'k3', defaultValue: 2 },
      { type: 'number', name: 'k4', label: 'k4', defaultValue: 5 },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
      { type: 'switch', name: 'strip_x', label: '解密去补位 x', defaultValue: true, help: '明文本以 x 结尾时关闭' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.hill/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.keyed.coltrans',
    name: '列移位 ColTrans',
    categoryId: 'crypto.classical.keyed',
    description: '列移位转置（行写列读）：密钥为关键词（按字母序 rank 读列）、宽度数字或列序数字（如 3512），CryptoDen 兼容。',
    tags: ['古典密码', '转置', '列移位'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true, placeholder: '空白自动去除' },
      { type: 'text', name: 'key', label: '列密钥', required: true, placeholder: '如 HACK（关键词）/ 3（宽度）/ 3512（列序）' },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.columnar/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.keyed.column-permutation',
    name: '列置换',
    categoryId: 'crypto.classical.keyed',
    description: '列置换加密（数字列序版）：密钥如 3512 表示按 3,5,1,2 的相对顺序读列；也支持关键词与宽度形式。',
    tags: ['古典密码', '置换', '列置换'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true },
      { type: 'text', name: 'key', label: '列序 / 密钥', required: true, placeholder: '如 3512（列序）/ 4（宽度）/ ZEBRA（关键词）' },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.columnar/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.keyed.bifid',
    name: '双密码 Bifid',
    categoryId: 'crypto.classical.keyed',
    description: '双密码 Bifid：关键词生成 5×5 方阵（无 J），字符坐标行串与列串拼接后两两重组，打散坐标结构。',
    tags: ['古典密码', 'Bifid', 'Polybius'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true, placeholder: '仅字母参与（J→I），输出大写' },
      { type: 'text', name: 'key', label: '方阵关键词', required: true },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.bifid/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.keyed.foursquare',
    name: '四方密码',
    categoryId: 'crypto.classical.keyed',
    description: '四方密码：两个关键词方阵（右上 / 左下）+ 两个明文方阵，双字母组对角替换；重复字母插 X、末尾补 X（CryptoDen 同款）。',
    tags: ['古典密码', '四方', 'FourSquare'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '文本', required: true, spanFull: true, placeholder: '仅字母（J→I），输出大写' },
      { type: 'text', name: 'key1', label: '关键词 1（右上）', required: true },
      { type: 'text', name: 'key2', label: '关键词 2（左下）', required: true },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.foursquare/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.adfgx',
    name: 'ADFGX',
    categoryId: 'crypto.classical',
    description: 'ADFGX：key1 生成 5×5 Polybius 方阵（无 J）→ 坐标映射为 ADFGX 字母 → key2 列置换重排（二战德军密码）。',
    tags: ['古典密码', 'ADFGX', 'Polybius', '列置换'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '明文 / 密文', required: true, spanFull: true, placeholder: '加密：字母文本；解密：ADFGX 字母串' },
      { type: 'text', name: 'key1', label: '方阵关键词', required: true },
      { type: 'text', name: 'key2', label: '列置换关键词', required: true },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.adfgx/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.classical.keyed.adfgvx',
    name: 'ADFGVX',
    categoryId: 'crypto.classical.keyed',
    description: 'ADFGVX：key1 生成 6×6 方阵（A-Z + 0-9，支持数字明文）→ 坐标映射 ADFGVX 字母 → key2 列置换（ADFGX 的增强版）。',
    tags: ['古典密码', 'ADFGVX', 'Polybius', '列置换'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'textarea', name: 'data', label: '明文 / 密文', required: true, spanFull: true, placeholder: '加密：字母数字；解密：ADFGVX 字母串' },
      { type: 'text', name: 'key1', label: '方阵关键词', required: true },
      { type: 'text', name: 'key2', label: '列置换关键词', required: true },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'decrypt',
        options: [
          { label: '加密', value: 'encrypt' },
          { label: '解密', value: 'decrypt' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.adfgvx/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 30000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— RSA 攻击套件（真实后端 crypto.rsa_*，算法参考 RsaCtfTool 纯 Go 实现） ——
  {
    id: 'crypto.modern.rsa.low-exponent',
    name: 'RSA 低加密指数',
    categoryId: 'crypto.modern.rsa',
    description: '小 e 小明文攻击（e=3/5/7）：m^e < n 时直接开 e 次根；带截断时扫描 c + k·n。',
    tags: ['RSA', '低指数', '开根'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, placeholder: '十进制或 0x' },
      { type: 'text', name: 'e', label: '公钥指数 e', required: true, placeholder: '如 3' },
      { type: 'text', name: 'c', label: '密文 c', required: true, spanFull: true },
      { type: 'number', name: 'k_limit', label: 'k 扫描上限', defaultValue: 100000, help: 'm^e = c + k·n 的 k 上限' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_low_e/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.small-e',
    name: 'RSA 小指数攻击',
    categoryId: 'crypto.modern.rsa',
    description: '小指数场景汇总入口：m^e 未截断直接开根 / 轻微截断 c + k·n 扫描（与低加密指数同引擎）。',
    tags: ['RSA', '小指数'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true },
      { type: 'text', name: 'e', label: '公钥指数 e', required: true },
      { type: 'text', name: 'c', label: '密文 c', required: true, spanFull: true },
      { type: 'number', name: 'k_limit', label: 'k 扫描上限', defaultValue: 100000 },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_low_e/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.broadcast',
    name: 'RSA 广播攻击',
    categoryId: 'crypto.modern.rsa',
    description: 'Hastad 广播攻击：同一明文用同一小 e 发给多个不同模数，至少 e 组 (n, c) 即可 CRT 合并后开 e 次根恢复明文。',
    tags: ['RSA', '广播', 'Hastad', 'CRT'],
    version: '1.0.0',
    inputSchema: [
      { type: 'text', name: 'e', label: '公钥指数 e', required: true, placeholder: '通常为 3' },
      {
        type: 'textarea',
        name: 'pairs',
        label: '广播密文对',
        required: true,
        rows: 6,
        spanFull: true,
        placeholder: '每行一对（逗号分隔）：\n861...,  5537...\n941...,  2205...\n723...,  4126...',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_broadcast/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.fermat',
    name: 'RSA Fermat 分解（Go）',
    categoryId: 'crypto.modern.rsa',
    description: 'Fermat 分解 n = a² − b²：|p−q| 小时快速收敛；命中后自动输出 p、q、φ、私钥 d 与明文（供 e/c 时），与前端「|p-q| 小」工具同算法的服务端版本。',
    tags: ['RSA', 'Fermat', '分解'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, placeholder: '十进制或 0x' },
      { type: 'number', name: 'max_iter', label: '最大迭代', defaultValue: 1000000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）', placeholder: '给则计算 d 与明文' },
      { type: 'text', name: 'c', label: '密文 c（可选）', placeholder: '给则解密' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_fermat/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.dp-leak',
    name: 'RSA dp/dq 泄露攻击（Go）',
    categoryId: 'crypto.modern.rsa',
    description:
      'dp 泄露分解：e·dp − 1 = k·(p−1)，枚举 k ∈ [1,e) 即可分解 n（dq 同理）；'
      + '题目不给完整密文只给 c mod p / c mod q 时（No Ciphertext RSA 套路）自动 CRT 还原后再解密；'
      + '分解后自动输出 p、q、φ、私钥 d 与明文并自验证。2026 Bugku No Ciphertext RSA 实测。',
    tags: ['RSA', 'dp泄露', 'CRT', '分解'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      {
        type: 'note',
        name: 'note_dp_leak',
        label: 'dp = d mod (p−1) 泄露 = n 直接分解（含无密文变体）',
        spanFull: true,
        content:
          '① 原理:dp·e ≡ 1 (mod p−1) → e·dp−1 = k·(p−1),dp < p−1 ⇒ k < e,枚举 k 整除 (e·dp−1) 得 p\n'
          + '② e = 65537 时 k 只有 6 万多个候选,秒级;题目给 dq 时选 dq 字段,对称同理\n'
          + '③ 「No Ciphertext」变体:不给 c 只给 c mod p 和 c mod q → 填 c_mod_p / c_mod_q,工具 CRT 还原完整密文\n'
          + '④ 直接给了完整 c 就只填 c;只给 dp 时也能纯分解(不出明文)',
      },
      { type: 'text', name: 'n', label: '模数 n', required: true, placeholder: '十进制或 0x' },
      { type: 'text', name: 'e', label: '公钥指数 e', placeholder: '默认 65537' },
      { type: 'text', name: 'dp', label: 'dp（= d mod (p−1)）', placeholder: '与 dq 至少其一' },
      { type: 'text', name: 'dq', label: 'dq（= d mod (q−1)）', placeholder: '可选,dp 缺失时使用' },
      { type: 'text', name: 'c', label: '密文 c（可选）', placeholder: '完整密文;缺省时用下方双泄露还原', spanFull: true },
      { type: 'text', name: 'c_mod_p', label: '泄露 c mod p', placeholder: 'No Ciphertext 变体' },
      { type: 'text', name: 'c_mod_q', label: '泄露 c mod q', placeholder: 'No Ciphertext 变体' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_dp_leak/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.algebraic.linear',
    name: '线性关系 q = a·p + b',
    categoryId: 'crypto.modern.rsa',
    description: '已知 q = a·p + b（a、b 精确已知）：代入 n = a·p² + b·p 解二次方程一步分解。安全素数 p = 2q+1 是 a=2、b=1 特例。',
    tags: ['RSA', '多项式关联', '线性'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      {
        type: 'select',
        name: 'amode',
        label: '关系模式',
        defaultValue: 'linear',
        options: [
          { label: '线性 q = a·p + b', value: 'linear' },
          { label: '比例 a·q = b·p', value: 'ratio' },
          { label: '差/和已知 p±q', value: 'diff_sum' },
          { label: '同余 p ≡ q (mod M)', value: 'congruence' },
          { label: '安全素数 p = 2q+1', value: 'safe_prime' },
        ],
      },
      { type: 'number', name: 'a', label: 'a', defaultValue: 3, help: 'q = a·p + b（b 须为偶，否则 q 为偶数）' },
      { type: 'number', name: 'b', label: 'b', defaultValue: 8 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_algebraic/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.algebraic.diff-sum',
    name: '差/和已知（p±q）',
    categoryId: 'crypto.modern.rsa',
    description: '已知 p+q 或 p−q：(p+q)² − 4n = (p−q)²，判别式开根直接分解。',
    tags: ['RSA', '多项式关联', '差和'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      {
        type: 'select',
        name: 'amode',
        label: '关系模式',
        defaultValue: 'diff_sum',
        options: [
          { label: '线性 q = a·p + b', value: 'linear' },
          { label: '比例 a·q = b·p', value: 'ratio' },
          { label: '差/和已知 p±q', value: 'diff_sum' },
          { label: '同余 p ≡ q (mod M)', value: 'congruence' },
          { label: '安全素数 p = 2q+1', value: 'safe_prime' },
        ],
      },
      { type: 'text', name: 'p_plus_q', label: 'p+q（二选一）' },
      { type: 'text', name: 'p_minus_q', label: 'p−q（二选一）' },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_algebraic/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.algebraic.ratio',
    name: '比例关系 q = (b/a)·p',
    categoryId: 'crypto.modern.rsa',
    description: '已知精确比例 a·q = b·p：a·n = b·p² 开平方直接分解（近似比例需 Coppersmith，本工具只做精确关系）。',
    tags: ['RSA', '多项式关联', '比例'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      {
        type: 'select',
        name: 'amode',
        label: '关系模式',
        defaultValue: 'ratio',
        options: [
          { label: '线性 q = a·p + b', value: 'linear' },
          { label: '比例 a·q = b·p', value: 'ratio' },
          { label: '差/和已知 p±q', value: 'diff_sum' },
          { label: '同余 p ≡ q (mod M)', value: 'congruence' },
          { label: '安全素数 p = 2q+1', value: 'safe_prime' },
        ],
      },
      { type: 'number', name: 'a', label: 'a', defaultValue: 2, help: 'a·q = b·p' },
      { type: 'number', name: 'b', label: 'b', defaultValue: 3 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_algebraic/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.algebraic.congruence',
    name: '同余关系 p ≡ q (mod M)',
    categoryId: 'crypto.modern.rsa',
    description: '已知 p = q + k·M（M 已知、k 小）：逐 k 解二次方程，k 命中即分解。',
    tags: ['RSA', '多项式关联', '同余'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      {
        type: 'select',
        name: 'amode',
        label: '关系模式',
        defaultValue: 'congruence',
        options: [
          { label: '线性 q = a·p + b', value: 'linear' },
          { label: '比例 a·q = b·p', value: 'ratio' },
          { label: '差/和已知 p±q', value: 'diff_sum' },
          { label: '同余 p ≡ q (mod M)', value: 'congruence' },
          { label: '安全素数 p = 2q+1', value: 'safe_prime' },
        ],
      },
      { type: 'text', name: 'M', label: 'M', required: true, help: 'p ≡ q (mod M)' },
      { type: 'number', name: 'k_limit', label: 'k 扫描上限', defaultValue: 100000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_algebraic/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.number-theoretic.safe-prime',
    name: '安全素数 p = 2q+1',
    categoryId: 'crypto.modern.rsa',
    description: 'n = q·(2q+1) 结构：解 2q² + q − n = 0 闭式一步分解。',
    tags: ['RSA', '数论', '安全素数'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      {
        type: 'select',
        name: 'amode',
        label: '关系模式',
        defaultValue: 'safe_prime',
        options: [
          { label: '线性 q = a·p + b', value: 'linear' },
          { label: '比例 a·q = b·p', value: 'ratio' },
          { label: '差/和已知 p±q', value: 'diff_sum' },
          { label: '同余 p ≡ q (mod M)', value: 'congruence' },
          { label: '安全素数 p = 2q+1', value: 'safe_prime' },
        ],
      },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_algebraic/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.fermat-family.twin',
    name: '相邻/孪生素数',
    categoryId: 'crypto.modern.rsa',
    description: 'p·(p+g) = n → (2p+g)² = 4n + g²：孪生（g=2）秒出，小 gap 逐个扫描。',
    tags: ['RSA', '枚举', '孪生素数'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      { type: 'number', name: 'max_gap', label: 'gap 扫描上限', defaultValue: 100000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_twin/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.number-theoretic.next-prime',
    name: 'next_prime 枚举',
    categoryId: 'crypto.modern.rsa',
    description: 'p 是基准数 base 附近的素数（如 p = next_prime(base)、next_prime(某个好看的数)）：窗口内逐候选试除。',
    tags: ['RSA', '数论', 'next_prime'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      { type: 'text', name: 'base', label: '基准 base', required: true, help: 'p 在 base 附近' },
      { type: 'number', name: 'window', label: '扫描窗口', defaultValue: 100000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_next_prime/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.number-theoretic.phi-leak',
    name: 'phi(n) / d 泄露',
    categoryId: 'crypto.modern.rsa',
    description: '已知 φ(n)：p+q = n − φ + 1 直接解二次方程；已知 d：由 e·d − 1 = k·φ 枚举 k 恢复分解。命中后自动出另一参数与明文。',
    tags: ['RSA', '数论', 'phi泄露'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      { type: 'text', name: 'phi', label: 'φ(n)（与 d 二选一）' },
      { type: 'text', name: 'd', label: '私钥 d（需同时给 e）' },
      { type: 'text', name: 'e', label: '公钥指数 e' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_phi_leak/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.shared-structure.sequence',
    name: '序列素因子（Fibonacci/Lucas）',
    categoryId: 'crypto.modern.rsa',
    description: 'p 或 q 是 Fibonacci / Lucas 数（RsaCtfTool fibonacci_gcd / lucas_gcd 移植）：逐项 gcd 扫描，项内命中即分解。',
    tags: ['RSA', '生成结构', '序列'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      {
        type: 'select',
        name: 'family',
        label: '序列',
        defaultValue: 'fibonacci',
        options: [
          { label: 'Fibonacci（F: 0,1,1,2,3,5…）', value: 'fibonacci' },
          { label: 'Lucas（L: 2,1,3,4,7,11…）', value: 'lucas' },
        ],
      },
      { type: 'number', name: 'limit', label: '扫描项数', defaultValue: 10000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_special/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.special-gcd',
    name: '特殊数 gcd 库',
    categoryId: 'crypto.modern.rsa',
    description: 'RsaCtfTool 特殊数族统一移植：Fibonacci / Lucas / Mersenne(2^k−1) / Fermat 数 / 阶乘 / 素数积 / 合数积与 n 的公因子扫描，p 或 q 属于任一族即命中。',
    tags: ['RSA', '特殊数', 'gcd', 'Mersenne'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      {
        type: 'select',
        name: 'family',
        label: '数族',
        defaultValue: 'mersenne',
        options: [
          { label: 'Fibonacci', value: 'fibonacci' },
          { label: 'Lucas', value: 'lucas' },
          { label: 'Mersenne（2^k − 1）', value: 'mersenne' },
          { label: 'Fermat 数（2^(2^k)+1）', value: 'fermat_numbers' },
          { label: '阶乘（k!）', value: 'factorial' },
          { label: '素数积（p#）', value: 'primorial' },
          { label: '合数积', value: 'compositorial' },
        ],
      },
      { type: 'number', name: 'limit', label: '扫描上限', defaultValue: 10000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_special/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.pollard-rho',
    name: 'Pollard Rho 分解',
    categoryId: 'crypto.modern.rsa',
    description: 'Pollard Rho（Brent 变体）：n 的最小因子 ≤ 2^64 量级时最有效；命中后自动输出 p、q、φ、d、明文。',
    tags: ['RSA', 'Pollard', 'Rho', '分解'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      { type: 'number', name: 'max_iters', label: '迭代上限', defaultValue: 5000000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_rho/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.pollard-p1',
    name: 'Pollard p−1 分解',
    categoryId: 'crypto.modern.rsa',
    description: 'Pollard p−1：p−1 的所有素因子幂 ≤ B1（平滑）时命中；可逐步增大 B1 重试。',
    tags: ['RSA', 'Pollard', 'p-1', '平滑'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      { type: 'number', name: 'b1', label: '平滑上界 B1', defaultValue: 100000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_pm1/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.smallq',
    name: '小素数试除',
    categoryId: 'crypto.modern.rsa',
    description: '小因子试除（RsaCtfTool small_q）：最小因子很小（出题人忘了乘大素数）时直接命中。',
    tags: ['RSA', '试除', '小因子'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      { type: 'number', name: 'limit', label: '试除上限', defaultValue: 1000000 },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_smallq/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'crypto.modern.rsa.factordb',
    name: 'FactorDB 在线分解',
    categoryId: 'crypto.modern.rsa',
    description: '查询 FactorDB：n 曾出现在公开题目/研究中被分解过则直接拿到因子（走全局代理，可配镜像地址）。',
    tags: ['RSA', 'FactorDB', '在线'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      { type: 'text', name: 'n', label: '模数 n', required: true, spanFull: true },
      { type: 'text', name: 'e', label: '公钥指数 e（可选）' },
      { type: 'text', name: 'c', label: '密文 c（可选）' },
      { type: 'text', name: 'endpoint', label: 'FactorDB 地址（可选）', placeholder: '默认 http://factordb.com' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/crypto.rsa_factordb/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— Web · PHP 反序列化 PoC 生成（自定义工作区：源码 → 识别属性 → 参数表单 → 序列化） ——
  {
    id: 'web.deserialize.php-unserialize',
    name: 'PHP 反序列化 PoC 生成',
    categoryId: 'web.deserialize',
    description: '粘贴 PHP 类源码，自动识别类属性生成参数表单，赋值后生成 serialize() PoC。',
    tags: ['PHP', '反序列化', 'PoC'],
    version: '1.0.0',
    inputSchema: [],
    invoke: { type: 'local', handlerId: 'local.php-unserialize' },
    output: { type: 'text' },
    workspace: 'php-unserialize',
  },

  // —— Web · JNDI 注入一键利用（真实后端：SSH 部署 LDAP 服务 + 恶意类 → PoC → 反弹 shell）——
  {
    id: 'web.deserialize.jndi',
    name: 'JNDI 注入一键利用',
    categoryId: 'web.deserialize',
    description:
      'fastjson 1.2.47 等 JNDI 注入一键利用：SSH 自动在远程服务器部署 LDAP 引用服务与随机化恶意类，' +
      '生成 PoC 并 POST 靶机，监听端口捕获反弹 shell 执行命令（如 cat /flag）。',
    tags: ['JNDI', 'fastjson', 'LDAP', '反弹Shell'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'group',
        name: 'g_ssh',
        label: '攻击服务器（SSH）',
        collapsible: true,
        children: [
          { type: 'text', name: 'ssh_host', label: 'SSH 主机', required: true, placeholder: '123.45.67.89' },
          { type: 'number', name: 'ssh_port', label: 'SSH 端口', defaultValue: 22 },
          { type: 'text', name: 'ssh_user', label: 'SSH 用户名', required: true, defaultValue: 'root' },
          { type: 'password', name: 'ssh_pass', label: 'SSH 密码', required: true },
          {
            type: 'text',
            name: 'putty_dir',
            label: 'PuTTY 目录',
            defaultValue: 'D:\\putty',
            help: 'plink.exe / pscp.exe 所在目录（本机需安装 PuTTY）',
          },
        ],
      },
      {
        type: 'group',
        name: 'g_ports',
        label: '监听端口（需云安全组/防火墙放行入站）',
        children: [
          {
            type: 'select',
            name: 'jndi_protocol',
            label: 'JNDI 引用协议',
            defaultValue: 'ldap',
            options: [
              { label: 'LDAP（默认，最通用）', value: 'ldap' },
              { label: 'RMI（LDAP 被拦截/不支持时）', value: 'rmi' },
            ],
          },
          {
            type: 'number',
            name: 'ldap_port',
            label: 'LDAP 端口',
            defaultValue: 1389,
            visibleWhen: { field: 'jndi_protocol', op: 'eq', value: 'ldap' },
          },
          {
            type: 'number',
            name: 'rmi_port',
            label: 'RMI 端口',
            defaultValue: 1099,
            visibleWhen: { field: 'jndi_protocol', op: 'eq', value: 'rmi' },
          },
          { type: 'number', name: 'http_port', label: 'HTTP 端口（类下载）', defaultValue: 8888 },
          { type: 'number', name: 'lport', label: '反弹监听端口', defaultValue: 31339 },
          {
            type: 'text',
            name: 'lhost',
            label: '回连公网 IP',
            defaultValue: '',
            placeholder: '留空默认取 SSH 主机 IP',
          },
        ],
      },
      {
        type: 'group',
        name: 'g_target',
        label: '靶机与 PoC',
        children: [
          {
            type: 'text',
            name: 'target_url',
            label: '靶机漏洞端点',
            placeholder: 'http://靶机:端口/login',
            help: '留空则不自动发送 PoC，仅生成 curl 命令',
          },
          {
            type: 'select',
            name: 'trigger',
            label: '触发模板',
            defaultValue: 'fastjson_1.2.47',
            options: [
              { label: 'fastjson ≤1.2.24（@type 直接触发）', value: 'fastjson_1.2.24' },
              { label: 'fastjson 1.2.25-1.2.41（L...; 绕过）', value: 'fastjson_1.2.41' },
              { label: 'fastjson 1.2.42（LL...;; 绕过）', value: 'fastjson_1.2.42' },
              { label: 'fastjson 1.2.43（[...[{ 绕过）', value: 'fastjson_1.2.43' },
              { label: 'fastjson 1.2.47（Class-val 缓存绕过，默认）', value: 'fastjson_1.2.47' },
              { label: 'Log4j2 JNDI（CVE-2021-44228，请求头注入）', value: 'log4j2' },
              { label: '自定义模板（{{JNDI_URL}}/{{GADGET}} 占位符）', value: 'custom' },
            ],
            help: '按目标解析库版本选择；不确定时先试默认模板',
          },
          {
            type: 'textarea',
            name: 'custom_template',
            label: '自定义 PoC 模板',
            defaultValue: '',
            visibleWhen: { field: 'trigger', op: 'eq', value: 'custom' },
            placeholder:
              '如 {"@type": "{{GADGET}}", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}',
            help: '占位符 {{JNDI_URL}} 与 {{GADGET}} 会被替换，按 JSON POST 发送',
          },
          {
            type: 'text',
            name: 'log4j_header',
            label: '注入请求头（逗号分隔）',
            defaultValue: 'X-Api-Version,User-Agent,X-Forwarded-For',
            visibleWhen: { field: 'trigger', op: 'eq', value: 'log4j2' },
            help: 'payload 注入这些请求头，同时放入 POST body',
          },
          {
            type: 'text',
            name: 'gadget_class',
            label: '触发链类',
            defaultValue: 'com.sun.rowset.JdbcRowSetImpl',
          },
          { type: 'number', name: 'wait_time', label: '等待回连超时（秒）', defaultValue: 60 },
        ],
      },
      {
        type: 'group',
        name: 'g_shell',
        label: '反弹 shell',
        children: [
          { type: 'text', name: 'command', label: '执行命令', defaultValue: 'cat /flag' },
          { type: 'text', name: 'shell', label: 'Shell 路径', defaultValue: '/bin/sh' },
          {
            type: 'switch',
            name: 'interactive',
            label: '成功反弹后保持交互会话',
            defaultValue: true,
            help: '连接建立后保留监听环境，输出区出现交互 Shell 可继续下发命令；关闭会话时自动清理',
          },
          {
            type: 'switch',
            name: 'cleanup',
            label: '结束后清理远程环境',
            defaultValue: true,
            help: '未保持会话时生效；交互会话由 Shell 面板的「关闭会话」按钮清理',
          },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/web.jndi/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 2000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 空白编码器：空白隐写 + Whitespace 解释器（真实后端 misc.whitespace_codec，2026 Bugku Snowfall 套路） ——
  {
    id: 'misc.stego.whitespace-codec',
    name: '空白编码器',
    categoryId: 'stego.text',
    description:
      '空白隐写（Snowfall 方案：空格=0/Tab=1，8 位+标记行）编解码，空白可放行尾/行首/任意位置、解码支持自动探测 + Whitespace 语言完整解释器（drop/swap 补丁打印被丢弃的栈值，输出含魔数识别/base64/hex）。',
    tags: ['Whitespace', '空白隐写', 'esolang', 'Misc', 'Snowfall'],
    version: '1.1.0',
    formColumns: 2,
    inputSchema: [
      {
        type: 'note',
        name: 'note_snowfall',
        label: '空白即信息（2026 Bugku Snowfall 套路）',
        spanFull: true,
        content:
          '① 空白隐写：空格/Tab 即二进制位，可把文本藏进任意文章（行尾/行首/任意位置；用 VSCode 而非记事本查看）\n'
          + '② 空白不一定在行尾：decode 不知道藏哪就选「空白范围=auto」自动试 行尾/行首/任意位置\n'
          + '③ 空白也可以是 Whitespace 程序源码：整篇文本按任意位置提取后直接当程序运行\n'
          + '④ 无输出的 Whitespace 程序不等于没数据：drop 丢弃的栈值可能就藏着 flag —— 用 drop 补丁打印栈顶\n'
          + '⑤ 解释器注意：算术/IO 的操作码是两个 token，少消费一个会静默吞掉后续指令（输出缺字节先怀疑这里）',
      },
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'hidden',
        options: [
          { label: '空白隐写（hidden）', value: 'hidden' },
          { label: 'Whitespace 解释器（ws）', value: 'ws' },
        ],
      },
      {
        type: 'group',
        name: 'g_hidden',
        label: '空白隐写（hidden 模式）',
        collapsible: true,
        spanFull: true,
        columns: 2,
        visibleWhen: { field: 'mode', op: 'eq', value: 'hidden' },
        children: [
          {
            type: 'select',
            name: 'action',
            label: '方向',
            defaultValue: 'decode',
            options: [
              { label: '提取隐藏文本（decode）', value: 'decode' },
              { label: '隐藏消息（encode）', value: 'encode' },
            ],
          },
          {
            type: 'select',
            name: 'scheme',
            label: '方案',
            defaultValue: 'snowfall',
            options: [
              { label: 'snowfall（8 位 + 标记行）', value: 'snowfall' },
              { label: 'fixed8（每 8 位一字符）', value: 'fixed8' },
            ],
            help: 'snowfall=每个字节 8 位行 + 1 个「1 位」标记行，按 0-1-0 切分；fixed8=每行 8 位无标记',
          },
          {
            type: 'select',
            name: 'scope',
            label: '空白范围',
            defaultValue: 'trailing',
            visibleWhen: { field: 'action', op: 'eq', value: 'decode' },
            options: [
              { label: '每行行尾（trailing）', value: 'trailing' },
              { label: '每行行首（leading）', value: 'leading' },
              { label: '任意位置（anywhere）', value: 'anywhere' },
              { label: '自动探测（auto）', value: 'auto' },
            ],
            help: '空白不一定在行尾：不知道藏哪就选 auto（三种全试并列结果）；anywhere=全文所有空格/Tab（正文须无内部空白）；正文有内部空格请用 trailing/leading',
          },
          {
            type: 'select',
            name: 'placement',
            label: '放置位置',
            defaultValue: 'trailing',
            visibleWhen: { field: 'action', op: 'eq', value: 'encode' },
            options: [
              { label: '每行行尾（trailing）', value: 'trailing' },
              { label: '每行行首（leading）', value: 'leading' },
              { label: '纯空白流（standalone）', value: 'standalone' },
            ],
            help: '决定空白藏在掩护文本的哪个位置；standalone=不用掩护文本，输出纯空白（解码时用 anywhere 或 auto）',
          },
          {
            type: 'select',
            name: 'bit_map',
            label: '位映射',
            defaultValue: 'space0_tab1',
            options: [
              { label: '空格=0 Tab=1', value: 'space0_tab1' },
              { label: '空格=1 Tab=0', value: 'space1_tab0' },
            ],
          },
          {
            type: 'textarea',
            name: 'data',
            label: '数据',
            required: true,
            spanFull: true,
            rows: 8,
            placeholder: 'decode：粘贴含隐藏空白的整篇文本；encode：输入要隐藏的消息（如 key / flag）',
          },
          {
            type: 'textarea',
            name: 'carrier',
            label: '掩护文本（encode，可选）',
            spanFull: true,
            rows: 6,
            visibleWhen: { field: 'action', op: 'eq', value: 'encode' },
            placeholder: '每 2 行藏 1 字节（snowfall），行数不够自动补空行；留空则输出纯空白行',
            help: '配合「放置位置」使用；standalone 时忽略掩护文本。解码时按对应位置选范围（或直接 auto）',
          },
        ],
      },
      {
        type: 'group',
        name: 'g_ws',
        label: 'Whitespace 解释器（ws 模式）',
        collapsible: true,
        spanFull: true,
        columns: 2,
        visibleWhen: { field: 'mode', op: 'eq', value: 'ws' },
        children: [
          {
            type: 'textarea',
            name: 'source',
            label: '程序源码',
            required: true,
            spanFull: true,
            rows: 8,
            placeholder: '可贴纯空白 Whitespace 程序；也可直接贴整篇文本（空白可在任意位置，按「提取方式」取值）',
          },
          {
            type: 'select',
            name: 'extract',
            label: '提取方式',
            defaultValue: 'anywhere',
            options: [
              { label: '任意位置（anywhere）', value: 'anywhere' },
              { label: '每行行尾（trailing）', value: 'trailing' },
              { label: '每行行首（leading）', value: 'leading' },
            ],
            help: 'anywhere=全文只取空格/Tab/换行（通用，正文须无内部空白）；trailing/leading=只取行尾/行首空白（Snowfall step2.txt / flag.txt 用法）',
          },
          {
            type: 'select',
            name: 'patch',
            label: '补丁',
            defaultValue: 'none',
            options: [
              { label: '无（原样执行）', value: 'none' },
              { label: 'drop（弹栈前打印栈顶）', value: 'drop' },
              { label: 'swap（换位时打印两值）', value: 'swap' },
              { label: 'trace（drop+swap 都打印）', value: 'trace' },
            ],
            help: '程序「只 push 不输出」时用 drop/trace 把被丢弃的栈值打出来（Snowfall flag.txt 场景）',
          },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/misc.whitespace_codec/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— WAV 音频隐写一键侦察（真实后端 misc.audio_lsb，2026 Bugku hhh.wav 套路）——
  {
    id: 'misc.stego.audio-lsb',
    name: 'WAV 音频隐写侦察',
    categoryId: 'misc.audio',
    description:
      '音频隐写一键排查：① 文件侦察（RIFF chunk 清单、data 之后附加数据提取、原始字节 flag/strings 扫描）'
      + '② LSB 位平面提取（bit0-3 × MSB/LSB 位序 × 声道 × 每采样 1-2bit）自动识别隐藏类型'
      + '（JPEG/PNG/GIF/BMP/WEBP/ZIP/GZIP/RAR/7Z/PDF/SQLite/ELF/EXE/文本/flag），JPEG 按 EOI、PNG 按 IEND 智能截断、图片直接预览 '
      + '③ 频谱图渲染（纯 Go FFT，全频段/低频/超声波三档，频域画字/二维码肉眼识别）。',
    tags: ['LSB', '音频隐写', 'WAV', '位平面', '频谱图', 'Stego', 'Misc'],
    version: '1.2.0',
    formColumns: 2,
    inputSchema: [
      {
        type: 'note',
        name: 'note_audio_lsb',
        label: '一次任务跑完音频隐写四条排查路线',
        spanFull: true,
        content:
          '① 文件侦察自动做：chunk 清单 + data 之后附加数据（copy /b 拼接的图/压缩包直接切出）+ 原始字节 flag/strings 扫描\n'
          + '② 16bit 采样改最低 1-2 位人耳无感（hhh.wav 套路）：auto 扫 bit0-3 × 两种位序 × 各声道，命中魔数即报并按 EOI/IEND 截断\n'
          + '③ 位平面 1/0 比例 ≈0.5 且无魔数 = 纯噪声；此时看频谱图：横向窄亮带=单音，成片字符/二维码=频域隐写，低频亮带=LSB 嵌入副产物\n'
          + '④ 双声道题目可能只藏某一个声道：all 找不到时分别试 0 / 1;DeepSound/mp3stego 等加密壳不在本工具范围',
      },
      { type: 'file', name: 'file', label: 'WAV 文件', accept: '.wav,audio/wav,audio/x-wav' },
      {
        type: 'select',
        name: 'plane',
        label: '位平面',
        defaultValue: 'auto',
        options: [
          { label: 'auto（扫 bit0-3）', value: 'auto' },
          { label: 'bit0（最常见）', value: '0' },
          { label: 'bit1', value: '1' },
          { label: 'bit2', value: '2' },
          { label: 'bit3', value: '3' },
        ],
        help: '嵌入层采样值的第几个低位;hhh.wav 类题目为 bit0',
      },
      {
        type: 'select',
        name: 'order',
        label: '打包位序',
        defaultValue: 'auto',
        options: [
          { label: 'auto（msb + lsb 都试）', value: 'auto' },
          { label: 'msb（首位→字节最高位，常见）', value: 'msb' },
          { label: 'lsb（首位→字节最低位）', value: 'lsb' },
        ],
        help: 'np.packbits 默认 MSB-first;部分工具（stegolsb 等）用 LSB-first',
      },
      {
        type: 'select',
        name: 'channel',
        label: '声道',
        defaultValue: 'auto',
        options: [
          { label: 'auto（全部 + 每个声道）', value: 'auto' },
          { label: 'all（按文件序全部采样）', value: 'all' },
          { label: '0（左声道）', value: '0' },
          { label: '1（右声道）', value: '1' },
        ],
        help: '单声道题目只有 all;双声道只藏一个声道时需按声道提取',
      },
      {
        type: 'select',
        name: 'bits',
        label: '每采样位数',
        defaultValue: '1',
        options: [
          { label: '1（每采样 1 bit，常见）', value: '1' },
          { label: '2（每采样 2 bit，容量翻倍）', value: '2' },
        ],
        help: '2 时按位号升序取 plane、plane+1 两位',
      },
      {
        type: 'select',
        name: 'spectrogram',
        label: '频谱图',
        defaultValue: 'full',
        options: [
          { label: 'full（全频段 0-Nyquist）', value: 'full' },
          { label: 'low（0-2kHz 低频段）', value: 'low' },
          { label: 'ultra（18kHz+ 超声波）', value: 'ultra' },
          { label: 'off（不生成，加速）', value: 'off' },
        ],
        help: '频域隐写排查:画字/二维码用 full;低频亮带细节用 low;人耳听不见的超声隐藏用 ultra',
      },
      {
        type: 'select',
        name: 'nperseg',
        label: '频谱窗长',
        defaultValue: '2048',
        options: [
          { label: '2048（默认，均衡）', value: '2048' },
          { label: '4096（频率分辨率更高）', value: '4096' },
          { label: '16384（细节最高，时间分辨率低）', value: '16384' },
        ],
        help: '同 scipy nperseg:越大频率轴越细、时间轴越糊',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/misc.audio_lsb/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'custom', rendererId: 'audio-lsb' },
  },

  // —— PNG/BMP 图像 LSB 隐写提取（真实后端 misc.image_lsb，zsteg 类套路，与音频工具共享引擎）——
  {
    id: 'misc.stego.image-lsb',
    name: '图像 LSB 隐写提取',
    categoryId: 'stego.image',
    description:
      'PNG/BMP 图像 LSB 隐写解密（zsteg 类套路）：R/G/B/A 通道 × bit0-2 位平面 × MSB/LSB 位序 auto 扫描'
      + '（含逐像素 RGB 交错模式），自动识别隐藏内容类型（JPEG/PNG/GIF/BMP/WEBP/ZIP/GZIP/RAR/7Z/PDF/SQLite/ELF/EXE/文本/flag），'
      + 'JPEG 按 EOI、PNG 按 IEND 智能截断，图片命中直接预览;支持 8/16bit、灰度、带透明通道。',
    tags: ['LSB', '图片隐写', 'PNG', 'BMP', '位平面', 'zsteg', 'Stego'],
    version: '1.0.0',
    formColumns: 2,
    inputSchema: [
      {
        type: 'note',
        name: 'note_image_lsb',
        label: '图片最低有效位里藏的整张文件，一次扫出来',
        spanFull: true,
        content:
          '① 无损格式（PNG/BMP）的像素低位可以藏整张图/压缩包/文本：auto 扫 rgb/r/g/b/a 通道 × bit0-2 × 两种位序\n'
          + '② JPEG 是有损压缩，LSB 已被编码破坏——本工具会明确引导，不要拿 JPEG 试\n'
          + '③ 灰度图只有单通道;PNG32 的 alpha 通道是常见藏匿点，auto 会一并扫\n'
          + '④ 找不到时:试更高位平面、或题目改用了其他手法（文件结构/频域/outguess 等）',
      },
      { type: 'file', name: 'file', label: 'PNG / BMP 图片', accept: '.png,.bmp,image/png,image/bmp' },
      {
        type: 'select',
        name: 'plane',
        label: '位平面',
        defaultValue: 'auto',
        options: [
          { label: 'auto（扫 bit0-2）', value: 'auto' },
          { label: 'bit0（最常见）', value: '0' },
          { label: 'bit1', value: '1' },
          { label: 'bit2', value: '2' },
        ],
      },
      {
        type: 'select',
        name: 'order',
        label: '打包位序',
        defaultValue: 'auto',
        options: [
          { label: 'auto（msb + lsb 都试）', value: 'auto' },
          { label: 'msb（首位→字节最高位，常见）', value: 'msb' },
          { label: 'lsb（首位→字节最低位）', value: 'lsb' },
        ],
      },
      {
        type: 'select',
        name: 'channel',
        label: '通道',
        defaultValue: 'auto',
        options: [
          { label: 'auto（rgb+r+g+b+a 全扫）', value: 'auto' },
          { label: 'rgb（逐像素 R,G,B 交错）', value: 'rgb' },
          { label: 'r（红通道）', value: 'r' },
          { label: 'g（绿通道）', value: 'g' },
          { label: 'b（蓝通道）', value: 'b' },
          { label: 'a（透明通道，PNG32）', value: 'a' },
        ],
        help: 'zsteg 默认扫多通道组合;已知题目特征时手动指定更快',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/misc.image_lsb/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'custom', rendererId: 'image-lsb' },
  },

  // —— 微信 .dat 图片缓存还原（本地计算：单字节循环异或，文件头逆推密钥）——
  {
    id: 'misc.stego.wechat-dat',
    name: '微信 .dat 图片还原',
    categoryId: 'stego.image',
    description:
      '还原微信 FileStorage 加密 .dat 图片缓存：整文件与单个密钥字节循环异或，借文件头魔数逆推密钥并解密，输出可预览/下载的图片。',
    tags: ['微信', 'WeChat', 'dat', '异或', '数据恢复', 'Misc'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'note',
        name: 'note_dat',
        label: '微信 .dat 缓存说明',
        spanFull: true,
        content:
          '① 微信 PC 版把聊天图片缓存加密后存为 .dat：整文件与单个密钥字节循环异或\n'
          + '② 密钥可由文件头逆推：拿常见文件头（PNG/JPG/GIF…）与 .dat 头部异或，全字节一致即得密钥\n'
          + '③ 本工具自动探测密钥与类型，解密后直接预览/下载；已知密钥时也可手动填写（十六进制，如 9E）',
      },
      { type: 'file', name: 'file', label: '.dat 文件', accept: '.dat,application/octet-stream' },
      { type: 'text', name: 'key', label: '密钥（可选）', placeholder: '留空自动探测，如 9E' },
    ],
    invoke: { type: 'local', handlerId: 'local.wechat-dat' },
    output: { type: 'custom', rendererId: 'wechat-dat' },
  },

  // —— 提瓦特文字识别（本地计算：字体模板匹配 OCR，原神 Teyvat script）——
  {
    id: 'misc.stego.teyvat',
    name: '提瓦特文字识别',
    categoryId: 'stego.text',
    description:
      '识别/解码原神提瓦特通用文字（Teyvat script）图片：字体模板匹配 OCR，切行切字还原英文（26 符文 ↔ A-Z 单表替换）。',
    tags: ['提瓦特', 'Teyvat', '原神', 'OCR', '字体识别', 'Misc'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'note',
        name: 'note_teyvat',
        label: '提瓦特文字识别说明',
        spanFull: true,
        content:
          '① 提瓦特文字是《原神》字形系统：26 符文 ↔ A-Z 一一对应，本质单表替换密码\n'
          + '② 已内置 Teyvat 字体（2021 版），自动做模板匹配 OCR；也可上传其它版本字体覆盖\n'
          + '③ 字体版本会影响 K/R、H/X 等字形，结果有误时可换字体或人工核对',
      },
      { type: 'file', name: 'image', label: '提瓦特文字图片', accept: 'image/*' },
      { type: 'file', name: 'font_file', label: 'Teyvat 字体（可选）', accept: '.ttf,.otf', help: '留空则用内置 Teyvat 字体' },
      { type: 'text', name: 'threshold', label: '匹配阈值（可选）', placeholder: '默认 0.30，越小越严格' },
    ],
    invoke: { type: 'local', handlerId: 'local.teyvat' },
    output: { type: 'text' },
  },

  // —— 兽音译者（beast_js）编码 / 解码 ——
  {
    id: 'crypto.encoding.beast',
    name: '兽音编码',
    categoryId: 'codec',
    description: '兽音译者（beast_js）编码/解码：文本 ↔ 兽音符号（嗷呜啊~ / BUGK），解码自动识别包装并恢复缺失符号。',
    tags: ['兽音', 'beast', '编码', 'Crypto', 'Bugku', 'task:演示'],
    version: '1.0.0',
    inputSchema: [],
    invoke: { type: 'local', handlerId: 'local.beast' },
    output: { type: 'text' },
    codec: true,
  },

  // —— 文件校验(批 1 新增页):Go 真实实现 misc.file_checksum,file_id 经 UploadFile 落盘 ——
  {
    id: 'misc.file_checksum',
    name: '文件校验',
    categoryId: 'misc.other',
    description: '计算上传文件的哈希：md5 / sha1 / sha256（输出十六进制摘要）。',
    tags: ['哈希', 'MD5', 'SHA1', 'SHA256', '文件校验'],
    version: '1.0.0',
    inputSchema: [
      { type: 'file', name: 'file', label: '文件', multiple: false, accept: '*/*' },
      {
        type: 'select',
        name: 'algo',
        label: '算法',
        defaultValue: 'sha256',
        options: [
          { label: 'SHA-256', value: 'sha256' },
          { label: 'SHA-1', value: 'sha1' },
          { label: 'MD5', value: 'md5' },
        ],
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/misc.file_checksum/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 500,
      timeoutMs: 60000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 原始包爬虫（接线 Go 真实实现 data.crawl.packet-crawler：
  //    解析原始包 → 占位符批量替换发包 → 保存响应内容 + 深度 1 链接下载）——
  {
    id: 'data.crawl.packet-crawler',
    name: '原始包爬虫',
    categoryId: 'data.crawl',
    description:
      '解析原始 HTTP 请求包，占位符自动替换后批量发包，按魔数/Content-Type 识别并保存响应内容' +
      '（图片、压缩包等），并可提取响应中的链接做一层深度下载。',
    tags: ['爬虫', '原始包', '占位符', '批量发包'],
    icon: 'Bug',
    version: '1.0.0',
    inputSchema: [
      {
        type: 'textarea',
        name: 'raw_request',
        label: '原始 HTTP 请求包',
        required: true,
        rows: 10,
        spanFull: true,
        placeholder:
          'GET /api/item?id={{id}} HTTP/1.1\nHost: example.com\nUser-Agent: Mozilla/5.0\nCookie: session=abc\n\n',
        help: 'Burp / 浏览器复制格式；需要变化的位置写 {{占位符名}}，可出现在请求行、请求头与 body 中',
      },
      {
        type: 'textarea',
        name: 'placeholders',
        label: '占位符定义',
        rows: 3,
        spanFull: true,
        placeholder: 'id=1-100\nname=a,b,c',
        help:
          '每行一个 name=取值：# 开头为注释。整数范围：1-100（含端点，倒序 100-1）、' +
          '1-100:2（步长）、1-100:1:4（补零到 4 位）；枚举列表：a,b,c。' +
          '多个占位符做笛卡尔积展开；留空则原样发送一次',
      },
      {
        type: 'radio',
        name: 'scheme',
        label: '目标协议（请求行为路径型时）',
        defaultValue: 'http',
        options: [
          { label: 'http', value: 'http' },
          { label: 'https', value: 'https' },
        ],
        help: '请求行是绝对 URL（含 http:// 或 https://）时忽略此项',
      },
      {
        type: 'range',
        name: 'concurrency',
        label: '并发数',
        min: 1,
        max: 50,
        step: 1,
        defaultValue: 5,
      },
      {
        type: 'range',
        name: 'timeout',
        label: '单请求超时（秒）',
        min: 1,
        max: 60,
        step: 1,
        defaultValue: 15,
      },
      {
        type: 'range',
        name: 'delay_ms',
        label: '请求间隔（毫秒）',
        min: 0,
        max: 5000,
        step: 50,
        defaultValue: 0,
        help: '对目标友好一点；0 表示不间隔',
      },
      {
        type: 'number',
        name: 'max_requests',
        label: '最大请求数',
        defaultValue: 2000,
        help: '占位符组合数超过此值时拒绝执行（防止误配打出海量请求）',
      },
      {
        type: 'switch',
        name: 'follow_links',
        label: '提取响应中的链接并下载（深度 1）',
        defaultValue: true,
        help: '从 HTML/JSON/XML/纯文本响应中提取 URL（含相对路径），再下载一层，不递归',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/data.crawl.packet-crawler/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 3600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },

  // —— 图片元数据提取（接线 Go 真实实现 data.extract.image-metadata：
  //    自研 EXIF/TIFF 解析器，覆盖 JPEG/PNG/WebP/GIF/BMP/ICO 的基本属性、
  //    EXIF 相机参数、GPS 坐标换算、XMP、PNG 文本块）——
  {
    id: 'data.extract.image-metadata',
    name: '图片元数据提取',
    categoryId: 'data.extract',
    description:
      '提取图片属性详细信息：格式/尺寸/颜色、EXIF 相机参数（机型、拍摄时间、曝光、光圈、ISO、焦距）、' +
      'GPS 坐标换算与地图定位、XMP 元数据、PNG 文本块等。',
    tags: ['图片', 'EXIF', 'GPS', '元数据', '取证'],
    icon: 'Camera',
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '图片文件',
        multiple: false,
        accept: 'image/*,.tif,.tiff,.ico,.bmp',
        help: '支持 JPEG / PNG / WebP / GIF / BMP / TIFF / ICO',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/data.extract.image-metadata/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 120000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'data.extract.image-batch',
    name: '图片批量分析',
    categoryId: 'data.extract',
    description:
      '批量上传图片（支持整个文件夹递归收集），自动提取每张图的格式/尺寸/大小/机型/拍摄时间/GPS 定位，' +
      '产出交互式筛选结果：按国内·国外、省份、格式、尺寸范围、机型、有无 GPS 过滤，支持 CSV 导出。',
    tags: ['批量', '文件夹', 'EXIF', 'GPS', '筛选'],
    icon: 'Layers',
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '图片文件 / 文件夹',
        multiple: true,
        directory: true,
        accept: 'image/*,.tif,.tiff,.ico,.bmp',
        help: '可多选图片，或选择整个文件夹（递归收集全部图片，保留子目录结构）。单张上限 100MB，最多 5000 张。',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/data.extract.image-batch/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'custom', rendererId: 'image-batch' },
  },
  {
    id: 'data.extract.image-qrcode',
    name: '图片二维码识别',
    categoryId: 'data.extract',
    description:
      '识别图片中的二维码并记录内容：支持 QR Code / Data Matrix / Aztec，可上传单图、多图或整个文件夹，' +
      '每张图最多识别 8 个码，内容按 UTF-8 解码，支持关键字筛选与 CSV 导出。',
    tags: ['二维码', 'QR', '识别', '图片', '批量'],
    icon: 'QrCode',
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '图片文件 / 文件夹',
        multiple: true,
        directory: true,
        accept: 'image/*,.tif,.tiff,.bmp',
        help: '可多选图片，或选择整个文件夹（递归收集全部图片，保留子目录结构）。支持 JPEG/PNG/GIF/BMP/WebP(有损)/TIFF，单张上限 100MB，最多 500 张。',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/data.extract.image-qrcode/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'custom', rendererId: 'image-qrcode' },
  },
  {
    id: 'data.extract.image-ocr',
    name: '图片文字提取（OCR）',
    categoryId: 'data.extract',
    description:
      'OCR 提取图片中的文字：基于项目自带的便携 Tesseract 引擎（thirdparty/tesseract，含中英文语言包 chi_sim+eng，随项目移植），' +
      '支持单图、多图或整个文件夹，可调版面分析模式，产出交互式筛选结果与可下载的全文报告（ocr_report.json / ocr_texts.txt）。',
    tags: ['OCR', '文字识别', 'Tesseract', '图片', '批量'],
    icon: 'ScanText',
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '图片文件 / 文件夹',
        multiple: true,
        directory: true,
        accept: 'image/*,.tif,.tiff,.bmp',
        help: '可多选图片，或选择整个文件夹（递归收集全部图片，保留子目录结构）。单张上限 100MB，最多 500 张。',
      },
      {
        type: 'text',
        name: 'lang',
        label: 'OCR 语言',
        defaultValue: 'chi_sim+eng',
        placeholder: 'chi_sim+eng',
        help: 'Tesseract 语言代码，+ 号组合（如 chi_sim+eng / eng / chi_sim）。内置 chi_sim（简体中文）与 eng，缺失的语言自动回退 eng。',
      },
      {
        type: 'select',
        name: 'psm',
        label: '版面分析模式',
        defaultValue: '3',
        options: [
          { label: '3 自动（默认，适合整页）', value: '3' },
          { label: '6 单个文本块（适合截图主体）', value: '6' },
          { label: '7 单行文本', value: '7' },
          { label: '11 稀疏文本（适合零散文字）', value: '11' },
          { label: '12 稀疏文本 + 方向检测', value: '12' },
        ],
        help: '版面分析模式（Page Segmentation Mode）。识别效果不好时可切换尝试。',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/data.extract.image-ocr/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'custom', rendererId: 'image-ocr' },
  },
  {
    id: 'data.extract.audio-asr',
    name: '语音转文字（ASR）',
    categoryId: 'data.extract',
    description:
      '语音识别：基于项目自带的便携 whisper.cpp 引擎（thirdparty/whisper，随项目移植），支持 wav/mp3/flac/ogg，' +
      '可上传单文件或整个文件夹批量识别，多进程并行 + 多线程加速（whisper 默认只用 4 线程，本工具按进程数均分 CPU 核心）。' +
      '语言自动检测或指定（zh/en 等），五档模型可选（tiny~large-v3，缺省 small，中文建议 small 及以上），' +
      '模型缺失时自动从 HF 镜像下载。输出带时间轴的分段结果，可导出全文文本与 SRT 字幕（asr_report.json / asr_texts.txt / asr_all.srt）。',
    tags: ['ASR', '语音识别', 'Whisper', '音频', '字幕'],
    icon: 'AudioLines',
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '音频文件 / 文件夹',
        multiple: true,
        directory: true,
        accept: '.wav,.mp3,.flac,.ogg,.oga,audio/*',
        help: '可多选音频，或选择整个文件夹（递归收集 wav/mp3/flac/ogg）。单文件上限 200MB，文件数量不限；大批量为后台任务，逐文件实时显示进度（"已完成 N/M（W 进程并行）"），关闭应用会中断。m4a/aac 请先转成 wav 或 mp3。',
      },
      {
        type: 'select',
        name: 'lang',
        label: '识别语言',
        defaultValue: 'auto',
        options: [
          { label: 'auto 自动检测（默认）', value: 'auto' },
          { label: 'zh 中文', value: 'zh' },
          { label: 'en 英文', value: 'en' },
          { label: 'ja 日语', value: 'ja' },
          { label: 'ko 韩语', value: 'ko' },
        ],
        help: '自动检测多数场景够用；音频语种明确时可指定以提升准确率。',
      },
      {
        type: 'select',
        name: 'model',
        label: 'Whisper 模型',
        defaultValue: 'base-q5_1',
        options: [
          { label: 'tiny-q5_1（32MB，极速，已内置，英文/粗筛）', value: 'tiny-q5_1' },
          { label: 'base-q5_1（57MB，快，已内置，默认）', value: 'base-q5_1' },
          { label: 'small（466MB，质量，已内置，中文建议）', value: 'small' },
          { label: 'medium（1.5GB，中文较好，需下载）', value: 'medium' },
          { label: 'large-v3（3GB，最佳质量，需下载）', value: 'large-v3' },
        ],
        help: 'tiny-q5_1/base-q5_1/small 已随项目内置（本地调用不联网）。本机实测（4 进程并行，6 秒音频）：tiny-q5_1 ≈ 0.9s/文件、base-q5_1 ≈ 1.6s/文件、small ≈ 2.2s/文件。中文要求高请选 small；量化档缺失时不自动下载（仓库量化文件与捆绑引擎版本不兼容）。medium/large-v3 缺失时自动从 HF 镜像（hf-mirror.com）下载到用户数据目录。',
      },
      {
        type: 'number',
        name: 'workers',
        label: '并行识别进程数（1-16）',
        defaultValue: 4,
        help: '默认 4：4 个进程同时识别，每个进程约占用一份模型内存（small≈466MB）并均分 CPU 线程（32 核机器每进程 8 线程），超出范围自动钳制到 1-16。大批量文件提速约与进程数成正比；机器内存不足（如 <8GB）请调小到 1-2，核多内存足可调到 8-16。',
      },
      {
        type: 'switch',
        name: 't2s',
        label: '中文结果自动繁转简（推荐）',
        defaultValue: true,
        help: 'whisper 中文输出常混有繁体字；开启后用 OpenCC 做短语级繁转简（「後」→「后」、「乾」等歧义字按词区分），仅对识别为中文的音频生效，导出文本/SRT 同步转换。',
      },
      {
        type: 'switch',
        name: 'cn_num',
        label: '中文数字转阿拉伯数字（推荐）',
        defaultValue: true,
        help: 'whisper 常把数字输出成中文数字（「幺三八一二三…」→138123…），导致敏感信息正则（手机号/证件号等）匹配不到。开启后自动转换：「一百二十三」→123、「三千零五十」→3050；「万一/千万小心/十分/一个」等惯用词不受影响，仅对识别为中文的音频生效。',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/data.extract.audio-asr/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 21600000,
      realBackend: true,
    },
    output: { type: 'custom', rendererId: 'audio-asr' },
  },
  {
    id: 'data.convert.table-merge',
    name: '表格合并（CSV/Excel）',
    categoryId: 'data.convert',
    description:
      '按指定列合并两个表格文件：支持 CSV（UTF-8/GBK/UTF-16 自动识别、逗号/分号/制表符自动嗅探）与 Excel .xlsx（取第一个工作表），' +
      '支持 inner/left/right/outer 四种连接方式，键重复做笛卡尔积，重名列自动加 _b 后缀。' +
      '产出交互式结果预览（关键字筛选）与可下载的合并结果 merged.csv（UTF-8 BOM，Excel 直接打开中文不乱码）。',
    tags: ['CSV', 'Excel', '合并', 'Join', '数据处理'],
    icon: 'Merge',
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file_a',
        label: '左侧表',
        multiple: false,
        accept: '.csv,.tsv,.txt,.xlsx',
        previewColumns: true,
        help: '左表（保留全部列，作为结果的基础列）。支持 .csv/.tsv/.txt/.xlsx；旧版 .xls 请先另存为 .xlsx。选中后自动识别列名。',
      },
      {
        type: 'file',
        name: 'file_b',
        label: '右侧表',
        multiple: false,
        accept: '.csv,.tsv,.txt,.xlsx',
        previewColumns: true,
        help: '右表（与左表按合并列对齐）。支持 .csv/.tsv/.txt/.xlsx；旧版 .xls 请先另存为 .xlsx。选中后自动识别列名。',
      },
      {
        type: 'select',
        name: 'key_a',
        label: '左表合并列',
        required: true,
        optionsFromFile: 'file_a',
        help: '左表中用于匹配的列。选中左侧表后自动识别列名并生成下拉选项。',
      },
      {
        type: 'select',
        name: 'key_b',
        label: '右表合并列',
        optionsFromFile: 'file_b',
        help: '右表中用于匹配的列。选中右侧表后自动识别列名并生成下拉选项；缺省与 key_a 同名；异名时两个键列都会保留在结果中。',
      },
      {
        type: 'select',
        name: 'how',
        label: '合并方式',
        defaultValue: 'inner',
        options: [
          { label: 'inner 内连接（只保留两侧都能匹配上的行）', value: 'inner' },
          { label: 'left 左连接（保留左表全部行，右表补空）', value: 'left' },
          { label: 'right 右连接（保留右表全部行，左表补空）', value: 'right' },
          { label: 'outer 全连接（保留两侧全部行，缺省补空）', value: 'outer' },
        ],
        help: '键值去首尾空白后匹配；同一键在表内重复时做笛卡尔积（数据库 JOIN 语义）。',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/data.convert.table-merge/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'custom', rendererId: 'table-merge' },
  },

];

// ---------------------------------------------------------------------------
// Web · SSTI —— 按引擎拆分的独立工具（占位符：后端插件已注册，逐引擎实现）
// 每个模板引擎一个独立项，挂在 web.ssti 分类下（第二侧边栏直接列出），
// 各自提交到自己的后端端点 /api/tools/web.ssti_<key>/run。
// ---------------------------------------------------------------------------

interface SstiEngineMeta {
  /** 后端 tool_id 后缀：web.ssti_<key> */
  key: string;
  /** 第二侧边栏显示名 */
  name: string;
  tag: string;
  description: string;
  /** 实现要点（表单帮助展示） */
  plan: string;
  /** 该引擎支持的利用模式（无 RCE 能力的引擎不提供 exec） */
  modes: { label: string; value: string }[];
  defaultMode: string;
  /** 引擎专属附加表单字段（追加到进阶参数组） */
  extraInputs?: InputField[];
}

const SSTI_MODE_LABELS: Record<string, string> = {
  exec: '命令执行（RCE）',
  read_file: '任意文件读',
  info: '信息收集',
  detect: '指纹确认',
};

const sstiModes = (...keys: string[]) => keys.map((v) => ({ label: SSTI_MODE_LABELS[v], value: v }));

const SSTI_ENGINES: SstiEngineMeta[] = [
  {
    key: 'fingerprint', name: 'SSTI · 自动指纹识别', tag: '指纹', defaultMode: 'detect',
    description: 'SSTI 自动指纹识别：多引擎探针识别模板引擎与版本（占位符，开发中）',
    plan: '探针表逐条发送 → 匹配响应 → 报告引擎与版本',
    modes: sstiModes('detect'),
  },
  {
    key: 'jinja2', name: 'SSTI · Jinja2 / Flask', tag: 'Jinja2', defaultMode: 'exec',
    description: 'Flask/Jinja2 模板注入：探针指纹、信息收集、8 条万能链 RCE、子类枚举、过滤绕过（拆片/空格括号/十六进制）、CaaS1 式自适应文件读 + 路径 oracle',
    plan: 'CaaS1 式自适应：黑名单/超长/限速信号识别；~ 拆片（\'flag\'~\'.\'txt\'）、空格括号 f ( )、迭代器 __next__ ( ) 绕 read；open_resource 链 + 路径 oracle 自动定位 flag',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
    extraInputs: [
      {
        type: 'textarea',
        name: 'blocked_words',
        label: '已知黑名单词',
        defaultValue: '',
        placeholder: '逗号/换行分隔，如 flag.txt, os, ls',
        help: 'payload 自动拆片规避这些词：flag.txt → \'flag\'~\'.\'txt\'，ls → \'l\'~\'s\'；留空则按响应信号自动切换绕过',
        rows: 2,
      },
      {
        type: 'number',
        name: 'max_length',
        label: '输入长度上限',
        defaultValue: 0,
        help: '注入点输入长度限制（如 70），0=不限；超长 payload 自动跳过并切换紧凑变体（basename/拆片）',
      },
      {
        type: 'number',
        name: 'interval',
        label: '请求间隔（秒）',
        defaultValue: 0,
        help: '目标有限速时设置；检测到 429/502 自动放慢到 ≥1.5s',
      },
    ],
  },
  {
    key: 'smarty', name: 'SSTI · Smarty', tag: 'Smarty', defaultMode: 'exec',
    description: 'Smarty 模板注入：system 执行、security 沙箱逃逸、include/fetch 文件读（占位符，开发中）',
    plan: 'system() 直连 / 布尔回显；security 逃逸 template_object→disableSecurity（3.1.39 前）；include、fetch 读文件；大写函数名、get 外带',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'twig', name: 'SSTI · Twig', tag: 'Twig', defaultMode: 'exec',
    description: 'Twig 模板注入：回调注册 RCE、filter 传函数名、source/include 文件读（占位符，开发中）',
    plan: 'registerUndefinedFilterCallback 回调注册；filter/map/sort 传函数名；source、include 读文件；setCache FTP 逃逸',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'mako', name: 'SSTI · Mako', tag: 'Mako', defaultMode: 'exec',
    description: 'Mako 模板注入：原生 Python 代码块直接 RCE（占位符，开发中）',
    plan: '<% %> 原生 Python 代码块直接 import os；被过滤时走 ${} 表达式',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'tornado', name: 'SSTI · Tornado', tag: 'Tornado', defaultMode: 'exec',
    description: 'Tornado 模板注入：模板内 import 模块 RCE、handler.settings 泄露（占位符，开发中）',
    plan: '模板内 import os 直接 popen；handler.settings 泄露 cookie_secret 等配置',
    modes: sstiModes('exec', 'info', 'detect'),
  },
  {
    key: 'django', name: 'SSTI · Django', tag: 'Django', defaultMode: 'info',
    description: 'Django 模板注入：无直接 RCE，上下文泄露配合伪造 session（占位符，开发中）',
    plan: '模板层无 RCE；{% debug %} / settings 泄露上下文，配合伪造 session',
    modes: sstiModes('info', 'detect'),
  },
  {
    key: 'freemarker', name: 'SSTI · FreeMarker', tag: 'FreeMarker', defaultMode: 'exec',
    description: 'FreeMarker 模板注入：Execute 工具类经 ?new 实例化 RCE（占位符，开发中）',
    plan: 'freemarker.template.utility.Execute 经 ?new 实例化执行；ObjectConstructor 变体',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'velocity', name: 'SSTI · Velocity', tag: 'Velocity', defaultMode: 'exec',
    description: 'Velocity 模板注入：反射链 Runtime.exec（占位符，开发中）',
    plan: 'getClass/forName 反射链取 Runtime.exec；Character 逐字节回显或 DNS 外带',
    modes: sstiModes('exec', 'info', 'detect'),
  },
  {
    key: 'thymeleaf', name: 'SSTI · Thymeleaf / SpEL', tag: 'Thymeleaf', defaultMode: 'exec',
    description: 'Thymeleaf/SpEL 模板注入：预处理表达式 RCE（占位符，开发中）',
    plan: '预处理表达式双下划线包裹 SpEL：T(java.lang.Runtime) 执行，::.x 触发视图解析',
    modes: sstiModes('exec', 'info', 'detect'),
  },
  {
    key: 'erb', name: 'SSTI · ERB', tag: 'ERB', defaultMode: 'exec',
    description: 'ERB 模板注入：内嵌 Ruby 直接 RCE（占位符，开发中）',
    plan: '<%= system %> 直接内嵌 Ruby；绕 <%%、换行、%> 拼接',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'slim', name: 'SSTI · Slim', tag: 'Slim', defaultMode: 'exec',
    description: 'Slim 模板注入：行首 = / - 注入 Ruby（占位符，开发中）',
    plan: '行首 = 输出命令结果；行首 - 静默执行 Ruby 语句',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'nunjucks', name: 'SSTI · Nunjucks', tag: 'Nunjucks', defaultMode: 'exec',
    description: 'Nunjucks 模板注入：range.constructor 构造 Function（占位符，开发中）',
    plan: 'range.constructor 构造 Function；execSync 回显',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'ejs', name: 'SSTI · EJS', tag: 'EJS', defaultMode: 'exec',
    description: 'EJS 模板注入：内嵌 JS 直接 RCE（占位符，开发中）',
    plan: 'execSync 直接执行；require 被过滤走 constructor._load',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'pug', name: 'SSTI · Pug', tag: 'Pug', defaultMode: 'exec',
    description: 'Pug 模板注入：换行注入输出语句（占位符，开发中）',
    plan: '换行另起输出行 = 执行 JS；或 - var 代码行',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'handlebars', name: 'SSTI · Handlebars', tag: 'Handlebars', defaultMode: 'info',
    description: 'Handlebars 模板注入：无直接 RCE，constructor 链（占位符，开发中）',
    plan: '无直接 RCE；lookup/with/split/pop/push 摸 constructor 构造 Function（BlackHat 2019）',
    modes: sstiModes('info', 'detect'),
  },
  {
    key: 'go', name: 'SSTI · Go template', tag: 'Go', defaultMode: 'info',
    description: 'Go 模板注入：依赖暴露对象与注册函数（占位符，开发中）',
    plan: '无 import/反射；依赖已注册模板函数与暴露对象方法；html/template 自动转义',
    modes: sstiModes('info', 'detect'),
  },
  {
    key: 'razor', name: 'SSTI · Razor', tag: 'Razor', defaultMode: 'exec',
    description: 'Razor 模板注入：.cshtml 代码区 Process.Start（占位符，开发中）',
    plan: '注入 .cshtml 代码区；Process.Start 执行命令',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
  {
    key: 'angularjs', name: 'SSTI · AngularJS', tag: 'AngularJS', defaultMode: 'info',
    description: 'AngularJS 1.x 前端模板注入：constructor 执行 JS（占位符，开发中）',
    plan: 'constructor.constructor 执行 JS；1.6+ 无沙箱，≤1.5 历史绕过链',
    modes: sstiModes('info', 'detect'),
  },
  {
    key: 'groovy', name: 'SSTI · Groovy', tag: 'Groovy', defaultMode: 'exec',
    description: 'Groovy 模板注入：execute() 直接跑命令（占位符，开发中）',
    plan: '字符串 .execute() 直接跑命令并 .text 回显',
    modes: sstiModes('exec', 'read_file', 'info', 'detect'),
  },
];

/** 按引擎生成独立工具定义：第二侧边栏 web.ssti 分类下逐项列出 */
const sstiEngineTools: ToolDefinition[] = SSTI_ENGINES.map((e) => ({
  id: `web.ssti.${e.key}`,
  name: e.name,
  categoryId: 'web.ssti',
  description: e.description,
  tags: ['SSTI', e.tag],
  version: '0.1.0',
  formColumns: 2,
  inputSchema: [
    {
      type: 'group',
      name: 'g_target',
      label: '靶机与注入点',
      collapsible: true,
      spanFull: true,
      columns: 2,
      children: [
        {
          type: 'rawrequest',
          name: 'raw_request',
          label: '原始数据包（可选）',
          rows: 5,
          spanFull: true,
          placeholder:
            'POST /generate HTTP/1.1\nHost: 靶机:端口\nContent-Type: application/x-www-form-urlencoded\nUser-Agent: Mozilla/5.0 …\nCookie: session=…\n\nname=test&team_name=x',
          help: '粘贴完整原始请求包（请求行 + 头 + 空行 + Body），点「解析数据包」自动填入端点/方法/参数，'
            + '下拉选择注入参数，其余参数按原值固定附带；请求头（Cookie 等）原样发送。留空则手动填写下方字段',
        },
        {
          type: 'text',
          name: 'target_url',
          label: '靶机端点',
          placeholder: 'http://靶机:端口/render',
          help: '模板回显所在的请求端点（粘贴原始包解析后自动填入）',
        },
        {
          type: 'text',
          name: 'param',
          label: '注入参数名',
          placeholder: '如 name / q（原始包解析后可下拉选择，留空自动选第一个）',
          help: '表单字段或 URL 参数名',
        },
        {
          type: 'select',
          name: 'method',
          label: '请求方法',
          defaultValue: 'POST',
          options: [
            { label: 'POST', value: 'POST' },
            { label: 'GET', value: 'GET' },
          ],
        },
      ],
    },
    {
      type: 'group',
      name: 'g_mode',
      label: '利用模式',
      collapsible: true,
      columns: 2,
      children: [
        {
          type: 'select',
          name: 'mode',
          label: '模式',
          defaultValue: e.defaultMode,
          options: e.modes,
        },
        {
          type: 'text',
          name: 'command',
          label: '执行命令',
          defaultValue: 'id',
          visibleWhen: { field: 'mode', op: 'eq', value: 'exec' },
        },
        {
          type: 'text',
          name: 'file_path',
          label: '读取路径',
          defaultValue: '/etc/passwd',
          visibleWhen: { field: 'mode', op: 'eq', value: 'read_file' },
        },
      ],
    },
    {
      type: 'group',
      name: 'g_advanced',
      label: '进阶参数',
      collapsible: true,
      columns: 2,
      children: [
        {
          type: 'textarea',
          name: 'probe',
          label: '自定义指纹探针',
          defaultValue: '',
          placeholder: '留空使用内置探针',
          rows: 2,
        },
        {
          type: 'textarea',
          name: 'payload',
          label: '自定义 payload',
          defaultValue: '',
          placeholder: '留空按模式自动构造',
          help: e.plan,
          rows: 2,
        },
        {
          type: 'textarea',
          name: 'extra_params',
          label: '额外固定参数',
          defaultValue: '',
          placeholder: '如 team_name=x&a=b，每个请求都附带；POST 并入表单，GET 并入查询串',
          rows: 2,
        },
        ...(e.extraInputs ?? []),
      ],
    },
  ],
  invoke: {
    type: 'task',
    submitUrl: `/api/tools/web.ssti_${e.key}/run`,
    pollUrl: '/api/tasks/{jobId}',
    intervalMs: 2000,
    timeoutMs: 300000,
    realBackend: true,
  },
  output: { type: 'text' },
}));

// ---------------------------------------------------------------------------
// 二、预设（供简写工具批量生成使用）
// ---------------------------------------------------------------------------

interface PresetShape {
  inputSchema: InputField[];
  invoke: InvokeSpec;
  output: OutputSpec;
}

/** 预设函数：接收工具 id，返回可复用的 inputSchema/invoke/output 三件套 */
const presets: Record<string, (id: string) => PresetShape> = {
  // 默认：文本处理（textarea + select 模式）→ HTTP POST → 纯文本
  text: (id) => ({
    inputSchema: [
      {
        type: 'select',
        name: 'mode',
        label: '模式',
        defaultValue: 'default',
        options: [{ label: '默认', value: 'default' }],
      },
      { type: 'textarea', name: 'data', label: '数据', required: true, placeholder: '输入数据…' },
    ],
    invoke: { type: 'http', method: 'POST', url: `/api/mock/${id}`, contentType: 'json' },
    output: { type: 'text' },
  }),

  // 文件处理（单文件）→ 异步任务 → 纯文本
  file: (id) => ({
    inputSchema: [{ type: 'file', name: 'file', label: '文件', accept: '*/*' }],
    invoke: {
      type: 'task',
      submitUrl: `/api/mock/${id}/submit`,
      pollUrl: `/api/mock/${id}/poll`,
      intervalMs: 500,
    },
    output: { type: 'text' },
  }),

  // 本地计算 → 纯文本
  local: (id) => ({
    inputSchema: [{ type: 'textarea', name: 'data', label: '数据', required: true }],
    invoke: { type: 'local', handlerId: `local.${id}` },
    output: { type: 'text' },
  }),

  // JSON 解析 → JSON 输出
  json: (id) => ({
    inputSchema: [{ type: 'textarea', name: 'data', label: '数据', required: true }],
    invoke: { type: 'http', method: 'POST', url: `/api/mock/${id}`, contentType: 'json' },
    output: { type: 'json' },
  }),

  // 查询 → 表格输出
  table: (id) => ({
    inputSchema: [{ type: 'text', name: 'query', label: '查询', required: true }],
    invoke: { type: 'http', method: 'GET', url: `/api/mock/${id}` },
    output: { type: 'table', columns: [{ key: 'key', title: '键' }, { key: 'value', title: '值' }] },
  }),

  // SSE 流式
  stream: (id) => ({
    inputSchema: [{ type: 'textarea', name: 'data', label: '数据', required: true }],
    invoke: { type: 'sse', url: `/api/mock/${id}/stream` },
    output: { type: 'stream' },
  }),

  // 图片输出
  image: (id) => ({
    inputSchema: [{ type: 'file', name: 'file', label: '文件', accept: 'image/*' }],
    invoke: { type: 'http', method: 'POST', url: `/api/mock/${id}`, contentType: 'form-data' },
    output: { type: 'image' },
  }),

  // 代码输出
  code: (id) => ({
    inputSchema: [{ type: 'file', name: 'file', label: '文件', accept: '*/*' }],
    invoke: { type: 'local', handlerId: `local.${id}` },
    output: { type: 'code', language: 'text' },
  }),

  // 十六进制输出
  hex: (id) => ({
    inputSchema: [{ type: 'textarea', name: 'data', label: '数据', required: true }],
    invoke: { type: 'local', handlerId: `local.${id}` },
    output: { type: 'hex' },
  }),

  // Markdown 输出
  markdown: (id) => ({
    inputSchema: [{ type: 'textarea', name: 'data', label: '数据', required: true }],
    invoke: { type: 'local', handlerId: `local.${id}` },
    output: { type: 'markdown' },
  }),
};

// ---------------------------------------------------------------------------
// 三、简写行（批量生成 90+ 工具）
// ---------------------------------------------------------------------------

interface ShorthandRow {
  id: string;
  name: string;
  categoryId: string;
  description: string;
  tags: string[];
  /** 预设名，缺省为 'text' */
  preset?: keyof typeof presets;
  /** 专用工作区标识（如 poc-scan） */
  workspace?: string;
}

/** 简写：只写身份信息，其余由 preset 补齐 */
const shorthandRows: ShorthandRow[] = [
  // —— Web ——
  { id: 'web.sql.sqli-scanner', name: 'SQL 注入扫描器', categoryId: 'web.sql', description: '自动化 SQL 注入点扫描', tags: ['SQL注入', '扫描'] },
  { id: 'web.sql.union-extract', name: 'Union 注入提取', categoryId: 'web.sql', description: 'Union 注入数据提取', tags: ['SQL注入', 'Union'] },
  { id: 'web.xss.reflected', name: '反射型 XSS 检测', categoryId: 'web.xss', description: '检测反射型 XSS 注入点', tags: ['XSS', '反射型'] },
  { id: 'web.xss.dom', name: 'DOM XSS 检测', categoryId: 'web.xss', description: '检测 DOM 型 XSS', tags: ['XSS', 'DOM'] },
  { id: 'web.ssrf.test', name: 'SSRF 探测', categoryId: 'web.ssrf', description: '探测服务端请求伪造', tags: ['SSRF', '探测'] },
  { id: 'web.rce.command-inject', name: '命令注入', categoryId: 'web.rce', description: '命令注入检测与利用', tags: ['RCE', '命令注入'] },
  { id: 'web.deserialize.ysoserial', name: '反序列化利用', categoryId: 'web.deserialize', description: '生成反序列化利用 payload', tags: ['反序列化', 'ysoserial'] },
  { id: 'web.upload.burp-bypass', name: '文件上传绕过', categoryId: 'web.upload', description: '文件上传类型绕过', tags: ['文件上传', '绕过'] },
  { id: 'web.lfi.path-traversal', name: '路径穿越检测', categoryId: 'web.lfi', description: '检测路径穿越漏洞', tags: ['LFI', '路径穿越'] },
  { id: 'web.basic.cookie-decode', name: 'Cookie 解析', categoryId: 'web.basic', description: '解析与解码 Cookie', tags: ['Cookie', '解析'], preset: 'local' },
  { id: 'web.basic.directory-brute', name: '目录爆破', categoryId: 'web.basic', description: '常见目录/文件爆破', tags: ['目录', '爆破'], preset: 'table' },

  // —— Crypto ——
  { id: 'crypto.classical.bacon', name: '培根密码', categoryId: 'crypto.classical', description: '培根密码编解码', tags: ['培根', '古典密码'], preset: 'local' },
  { id: 'crypto.modern.aes-ecb', name: 'AES-ECB 分析', categoryId: 'crypto.modern.aes', description: 'ECB 模式块分析', tags: ['AES', 'ECB'], preset: 'json' },
  { id: 'crypto.modern.ecc-factor', name: 'ECC 参数分析', categoryId: 'crypto.modern.ecc', description: '椭圆曲线参数分析', tags: ['ECC', '椭圆曲线'], preset: 'json' },
  { id: 'crypto.hash.length-extension', name: '哈希长度扩展', categoryId: 'crypto.hash', description: '哈希长度扩展攻击', tags: ['哈希', '长度扩展'], preset: 'local' },
  { id: 'crypto.hash.rainbow', name: '彩虹表查询', categoryId: 'crypto.hash', description: '常见哈希彩虹表查询', tags: ['哈希', '彩虹表'], preset: 'table' },

  // —— Misc ——
  { id: 'misc.encoding.brainfuck', name: 'Brainfuck 解释器', categoryId: 'codec', description: 'Brainfuck 代码执行', tags: ['Brainfuck', '编码'], preset: 'local' },
  { id: 'misc.encoding.xxd', name: '十六进制查看', categoryId: 'codec', description: '文本十六进制查看', tags: ['十六进制', 'xxd'], preset: 'hex' },
  { id: 'misc.encoding.qrcode-decode', name: '二维码解码', categoryId: 'codec', description: '二维码内容解码', tags: ['二维码', 'QR'], preset: 'image' },
  { id: 'misc.encoding.barcode-decode', name: '条形码解码', categoryId: 'codec', description: '条形码内容解码', tags: ['条形码', '编码'], preset: 'image' },
  { id: 'misc.other.git-extract', name: 'Git 泄露提取', categoryId: 'misc.other', description: '提取 .git 泄露源码', tags: ['Git', '泄露', 'task:提取'], preset: 'file' },
  { id: 'misc.other.zip-brute', name: 'ZIP 密码爆破', categoryId: 'misc.other', description: 'ZIP 压缩包密码爆破', tags: ['ZIP', '爆破', 'task:爆破'], preset: 'file' },
  { id: 'misc.other.rar-crack', name: 'RAR 破解', categoryId: 'misc.other', description: 'RAR 压缩包破解', tags: ['RAR', '破解', 'task:爆破'], preset: 'file' },
  { id: 'misc.other.serial-number', name: '序列号生成', categoryId: 'misc.other', description: '注册码/序列号生成器', tags: ['序列号', '生成', 'task:生成'], preset: 'local' },
  { id: 'misc.other.magic-bytes', name: '文件头识别', categoryId: 'misc.other', description: '识别文件魔数与类型', tags: ['文件头', '魔数', 'task:识别'], preset: 'file' },
  { id: 'misc.demo.local-calc', name: '本地计算示例', categoryId: 'misc.demo', description: '纯前端本地计算演示', tags: ['演示', '本地', 'task:演示'], preset: 'local' },
  { id: 'misc.demo.http-echo', name: 'HTTP 回显示例', categoryId: 'misc.demo', description: 'HTTP 请求回显演示', tags: ['演示', 'HTTP', 'task:演示'], preset: 'text' },
  { id: 'misc.report.flag-checker', name: 'Flag 校验', categoryId: 'misc.report', description: '校验 Flag 格式', tags: ['Flag', '校验', 'task:校验'], preset: 'local' },

  // —— Reverse ——
  { id: 'reverse.static.elf-header', name: 'ELF 头解析', categoryId: 'reverse.static', description: '解析 ELF 文件头', tags: ['ELF', '逆向'], preset: 'file' },
  { id: 'reverse.static.pe-header', name: 'PE 头解析', categoryId: 'reverse.static', description: '解析 PE 文件头', tags: ['PE', '逆向'], preset: 'file' },
  { id: 'reverse.static.strings-unicode', name: 'Unicode 字符串', categoryId: 'reverse.static', description: '提取 Unicode 字符串', tags: ['strings', 'Unicode'], preset: 'file' },
  { id: 'reverse.static.upx-unpack', name: 'UPX 脱壳', categoryId: 'reverse.static', description: 'UPX 加壳程序脱壳', tags: ['UPX', '脱壳'], preset: 'file' },
  { id: 'reverse.dynamic.strace', name: '系统调用跟踪', categoryId: 'reverse.dynamic', description: '跟踪程序系统调用', tags: ['strace', '动态'], preset: 'file' },
  { id: 'reverse.dynamic.ghidra-export', name: 'Ghidra 导出', categoryId: 'reverse.dynamic', description: '导出 Ghidra 反编译结果', tags: ['Ghidra', '反编译'], preset: 'code' },
  { id: 'reverse.dynamic.debug-script', name: '调试脚本', categoryId: 'reverse.dynamic', description: '生成 GDB 调试脚本', tags: ['GDB', '调试'], preset: 'code' },
  { id: 'reverse.deobfuscate.js-deobfuscate', name: 'JS 去混淆', categoryId: 'reverse.deobfuscate', description: 'JavaScript 代码去混淆', tags: ['JS', '去混淆'], preset: 'code' },
  { id: 'reverse.deobfuscate.pyc-decompile', name: 'Pyc 反编译', categoryId: 'reverse.deobfuscate', description: 'Python 字节码反编译', tags: ['Pyc', '反编译'], preset: 'code' },
  { id: 'reverse.deobfuscate.movfuscator', name: 'Movfuscator 分析', categoryId: 'reverse.deobfuscate', description: 'Movfuscator 混淆分析', tags: ['Movfuscator', '混淆'], preset: 'code' },

  // —— Pwn ——
  { id: 'pwn.stack.overflow-calc', name: '栈溢出偏移计算', categoryId: 'pwn.stack', description: '计算栈溢出偏移', tags: ['栈溢出', '偏移'], preset: 'local' },
  { id: 'pwn.stack.rop-gadget', name: 'ROP gadget 搜索', categoryId: 'pwn.stack', description: '搜索 ROP gadget 链', tags: ['ROP', 'gadget'], preset: 'file' },
  { id: 'pwn.stack.ret2libc', name: 'ret2libc 计算', categoryId: 'pwn.stack', description: 'ret2libc 地址计算', tags: ['ret2libc', '栈溢出'], preset: 'local' },
  { id: 'pwn.stack.canary-bypass', name: 'Canary 绕过', categoryId: 'pwn.stack', description: '栈 Canary 绕过分析', tags: ['Canary', '栈溢出'], preset: 'local' },
  { id: 'pwn.heap.chunk-analyzer', name: '堆块分析', categoryId: 'pwn.heap', description: '堆块结构分析', tags: ['堆', 'chunk'], preset: 'file' },
  { id: 'pwn.heap.tcache-poison', name: 'tcache poisoning', categoryId: 'pwn.heap', description: 'tcache 投毒利用', tags: ['tcache', '堆'], preset: 'local' },
  { id: 'pwn.format.fmt-str', name: '格式化字符串', categoryId: 'pwn.format', description: '格式化字符串漏洞分析', tags: ['格式化字符串', 'fmt'], preset: 'local' },
  { id: 'pwn.format.fmt-leak', name: '格式化字符串泄露', categoryId: 'pwn.format', description: '利用格式化字符串泄露地址', tags: ['格式化字符串', '泄露'], preset: 'local' },
  { id: 'pwn.shellcode.encoder', name: 'Shellcode 编码', categoryId: 'pwn.shellcode', description: 'Shellcode 编码/加密', tags: ['Shellcode', '编码'], preset: 'code' },
  { id: 'pwn.shellcode.generator', name: 'Shellcode 生成', categoryId: 'pwn.shellcode', description: '生成常用 Shellcode', tags: ['Shellcode', '生成'], preset: 'code' },

  // —— Forensics ——
  { id: 'forensics.memory.volatility', name: 'Volatility 分析', categoryId: 'forensics.memory', description: '内存镜像 Volatility 分析', tags: ['Volatility', '内存'], preset: 'file' },
  { id: 'forensics.memory.dump-strings', name: '内存字符串提取', categoryId: 'forensics.memory', description: '从内存转储提取字符串', tags: ['内存', 'strings'], preset: 'file' },
  { id: 'forensics.disk.mbr-parse', name: 'MBR 解析', categoryId: 'forensics.disk', description: '解析主引导记录', tags: ['MBR', '磁盘'], preset: 'file' },
  { id: 'forensics.disk.ntfs-parse', name: 'NTFS 解析', categoryId: 'forensics.disk', description: '解析 NTFS 文件系统', tags: ['NTFS', '磁盘'], preset: 'file' },
  { id: 'forensics.network.pcap-analyze', name: 'PCAP 流量分析', categoryId: 'traffic.parse', description: '分析网络抓包文件', tags: ['PCAP', '流量'], preset: 'file' },
  { id: 'forensics.network.dns-extract', name: 'DNS 提取', categoryId: 'traffic.protocol', description: '提取流量中的 DNS 记录', tags: ['DNS', '流量'], preset: 'table' },
  { id: 'forensics.log.log-parser', name: '日志解析', categoryId: 'forensics.log', description: '解析各类日志文件', tags: ['日志', '解析'], preset: 'file' },
  { id: 'forensics.log.timeline', name: '时间线分析', categoryId: 'forensics.log', description: '生成事件时间线', tags: ['时间线', '取证'], preset: 'table' },

  // —— Stego ——
  { id: 'stego.image.exif', name: 'EXIF 信息提取', categoryId: 'stego.image', description: '提取图片 EXIF 元数据', tags: ['EXIF', '图片'], preset: 'image' },
  { id: 'stego.image.png-chunk', name: 'PNG 块分析', categoryId: 'stego.image', description: '分析 PNG 数据块', tags: ['PNG', '隐写'], preset: 'image' },
  { id: 'stego.image.stegsolve', name: '通道分离', categoryId: 'stego.image', description: '图片颜色通道分离', tags: ['通道', '隐写'], preset: 'image' },
  { id: 'stego.audio.spectrogram', name: '频谱图分析', categoryId: 'stego.audio', description: '音频频谱图可视化', tags: ['频谱图', '音频'], preset: 'image' },
  { id: 'stego.audio.lsb-audio', name: '音频 LSB', categoryId: 'stego.audio', description: '音频最低有效位提取', tags: ['LSB', '音频'], preset: 'file' },
  { id: 'stego.video.frame-extract', name: '视频帧提取', categoryId: 'stego.video', description: '提取视频关键帧', tags: ['视频', '帧提取'], preset: 'file' },
  { id: 'stego.text.whitespace', name: '空白字符隐写', categoryId: 'stego.text', description: '文本隐写:检测空白字符隐写(与编码转换的 Whitespace 语言工具互不相同)', tags: ['空白', '隐写', 'Whitespace'], preset: 'local' },
  { id: 'stego.text.zero-width', name: '零宽字符检测', categoryId: 'stego.text', description: '检测零宽字符隐写', tags: ['零宽', '隐写'], preset: 'local' },

  // —— OSINT ——
  { id: 'osint.social.username-search', name: '用户名搜索', categoryId: 'osint.social', description: '跨平台用户名搜索', tags: ['OSINT', '用户名'], preset: 'table' },
  { id: 'osint.social.breach-check', name: '泄露库查询', categoryId: 'osint.social', description: '查询账号泄露记录', tags: ['OSINT', '泄露'], preset: 'table' },
  { id: 'osint.geo.geolocate', name: '地理定位', categoryId: 'osint.geo', description: '图片/线索地理定位', tags: ['OSINT', '地理'], preset: 'image' },
  { id: 'osint.image.reverse-search', name: '以图搜图', categoryId: 'osint.image', description: '图片反向搜索', tags: ['OSINT', '以图搜图'], preset: 'image' },
  { id: 'osint.domain.whois', name: 'WHOIS 查询', categoryId: 'osint.domain', description: '域名 WHOIS 信息查询', tags: ['OSINT', 'WHOIS'], preset: 'table' },
  { id: 'osint.domain.dns-enum', name: 'DNS 枚举', categoryId: 'osint.domain', description: '域名 DNS 记录枚举', tags: ['OSINT', 'DNS'], preset: 'table' },

  // —— Blockchain ——
  { id: 'blockchain.chain.tx-decode', name: '交易解码', categoryId: 'blockchain.chain', description: '解码链上交易数据', tags: ['区块链', '交易'], preset: 'json' },
  { id: 'blockchain.chain.storage-read', name: '合约存储读取', categoryId: 'blockchain.chain', description: '读取合约存储槽', tags: ['区块链', '存储'], preset: 'json' },
  { id: 'blockchain.contract.solidity-audit', name: 'Solidity 审计', categoryId: 'blockchain.contract', description: '智能合约源码审计', tags: ['Solidity', '审计'], preset: 'markdown' },
  { id: 'blockchain.contract.abi-decode', name: 'ABI 解码', categoryId: 'blockchain.contract', description: '解码合约 ABI 调用', tags: ['ABI', '合约'], preset: 'json' },
  { id: 'blockchain.wallet.mnemonic', name: '助记词生成', categoryId: 'blockchain.wallet', description: '生成 BIP39 助记词', tags: ['助记词', '钱包'], preset: 'local' },
  { id: 'blockchain.wallet.keystore-crack', name: 'Keystore 破解', categoryId: 'blockchain.wallet', description: '破解以太坊 Keystore', tags: ['Keystore', '钱包'], preset: 'file' },

  // —— Mobile ——
  { id: 'mobile.android.apk-decompile', name: 'APK 反编译', categoryId: 'mobile.android', description: '反编译 Android APK', tags: ['APK', 'Android'], preset: 'file' },
  { id: 'mobile.android.dex-parse', name: 'DEX 解析', categoryId: 'mobile.android', description: '解析 DEX 文件结构', tags: ['DEX', 'Android'], preset: 'code' },
  { id: 'mobile.ios.ipa-extract', name: 'IPA 提取', categoryId: 'mobile.ios', description: '提取 iOS IPA 内容', tags: ['IPA', 'iOS'], preset: 'file' },
  { id: 'mobile.ios.plist-parse', name: 'Plist 解析', categoryId: 'mobile.ios', description: '解析 iOS Plist 文件', tags: ['Plist', 'iOS'], preset: 'code' },

  // —— AI ——
  { id: 'ai.prompt.injection-test', name: 'Prompt 注入测试', categoryId: 'ai.prompt', description: '检测提示词注入漏洞', tags: ['AI', 'Prompt注入'], preset: 'local' },
  { id: 'ai.analysis.flag-search', name: 'Flag 智能检索', categoryId: 'ai.analysis', description: 'AI 辅助检索 Flag 线索', tags: ['AI', '检索'], preset: 'text' },
  { id: 'ai.analysis.code-summary', name: '代码摘要', categoryId: 'ai.analysis', description: 'AI 生成代码摘要', tags: ['AI', '摘要'], preset: 'markdown' },
];

// ---------------------------------------------------------------------------
// 三-b、Yang-Web 提取的 CTF 功能占位符（暂不实现逻辑，走 local mock 回退）
// ---------------------------------------------------------------------------

const yangWebShorthandRows: ShorthandRow[] = [
  // —— Web · SQL 注入 ——
  { id: 'web.sql.bool-inject', name: '布尔盲注', categoryId: 'web.sql', description: '布尔盲注探测与利用', tags: ['SQL注入', '盲注'] },
  { id: 'web.sql.time-sqli', name: '时间盲注', categoryId: 'web.sql', description: '基于时间差的盲注', tags: ['SQL注入', '时间盲注'] },
  { id: 'web.sql.sqli-labs', name: 'SQLi-LABS 专项', categoryId: 'web.sql', description: 'SQLi-LABS 靶场自动求解', tags: ['SQL注入', '靶场'] },

  // —— Web · SSRF ——
  { id: 'web.ssrf.ssrf-rebind', name: 'SSRF DNS Rebinding', categoryId: 'web.ssrf', description: 'DNS 重绑定绕过内网限制', tags: ['SSRF', 'DNS'] },
  { id: 'web.ssrf.ssrf-payload', name: 'SSRF Payload', categoryId: 'web.ssrf', description: '云元数据 / 内网探测 Payload', tags: ['SSRF', 'Payload'] },

  // —— Web · RCE / 命令注入 ——
  { id: 'web.rce.php-eval-rce', name: 'PHP eval RCE', categoryId: 'web.rce', description: 'PHP eval() 代码执行与绕过', tags: ['RCE', 'PHP'] },
  { id: 'web.rce.simple-cmd-rce', name: '简单命令注入', categoryId: 'web.rce', description: '命令注入探测与利用', tags: ['RCE', '命令注入'] },
  { id: 'web.rce.bashfuck', name: 'bashFuck 无字母命令', categoryId: 'web.rce', description: '无字母数字 bash 命令执行', tags: ['RCE', 'bashFuck'] },
  { id: 'web.rce.bashfuck-solver', name: 'bashFuck 自动求解', categoryId: 'web.rce', description: '自动生成无字母 RCE payload', tags: ['RCE', 'bashFuck'] },

  // —— Web · 文件上传 ——
  { id: 'web.upload.upload-bypass', name: '文件上传绕过', categoryId: 'web.upload', description: '后缀/MIME/内容绕过', tags: ['上传', '绕过'] },
  { id: 'web.upload.iis-upload', name: 'IIS 文件上传', categoryId: 'web.upload', description: 'IIS 解析漏洞上传', tags: ['上传', 'IIS'] },

  // —— Web · LFI ——
  { id: 'web.lfi.php-lfi', name: 'PHP 文件包含', categoryId: 'web.lfi', description: 'include/require 文件包含', tags: ['LFI', 'PHP'] },
  { id: 'web.lfi.phpinfo-lfi', name: 'phpinfo LFI', categoryId: 'web.lfi', description: 'phpinfo 结合文件包含', tags: ['LFI', 'phpinfo'] },
  { id: 'web.lfi.php-wrappers', name: 'PHP 伪协议', categoryId: 'web.lfi', description: 'php://filter/data 等伪协议', tags: ['LFI', '伪协议'] },

  // —— Web · PHP 专项 ——
  { id: 'web.php.php-logic', name: 'PHP 弱类型绕过', categoryId: 'web.php', description: '弱类型比较与逻辑绕过', tags: ['PHP', '弱类型'] },
  { id: 'web.php.magic-hash', name: 'PHP Magic Hash', categoryId: 'web.php', description: '0e 开头的哈希碰撞', tags: ['PHP', 'MagicHash'] },
  { id: 'web.php.php-md5-collision', name: 'PHP MD5 碰撞', categoryId: 'web.php', description: 'MD5 碰撞绕过比较', tags: ['PHP', 'MD5'] },

  // —— Web · XSS ——
  { id: 'web.xss.xss-payload', name: 'XSS Payload', categoryId: 'web.xss', description: '检测/外传/WAF 绕过 Payload', tags: ['XSS', 'Payload'] },

  // —— Web · 基础 / 引擎 / 扫描 ——
  { id: 'web.basic.js-challenge', name: 'JS 客户端挑战', categoryId: 'web.basic', description: 'JS 游戏/客户端挑战求解', tags: ['JS', '挑战'] },
  { id: 'web.basic.url-analyzer', name: 'URL 漏洞分析', categoryId: 'web.basic', description: 'URL 参数漏洞签名识别', tags: ['URL', '分析'] },
  { id: 'web.basic.smart-solver', name: 'Web 智能求解', categoryId: 'web.basic', description: '智能多类型 Web 自动求解', tags: ['Web', '求解'] },
  { id: 'web.basic.multi-stage', name: '多阶段攻击', categoryId: 'web.basic', description: '弱口令→后台利用等多阶段', tags: ['Web', '多阶段'] },
  { id: 'web.basic.chain-attack', name: '攻击链升级', categoryId: 'web.basic', description: 'LFI→RCE 等自动升级链', tags: ['Web', '攻击链'] },
  { id: 'web.basic.dict-scan', name: '字典扫描', categoryId: 'web.basic', description: '多层字典目录/文件爆破', tags: ['扫描', '字典'] },
  { id: 'web.basic.port-scan', name: '端口扫描', categoryId: 'web.basic', description: 'CTF 常见端口快速扫描', tags: ['扫描', '端口'] },

  // —— Crypto · 带密钥古典密码 ——
  { id: 'crypto.classical.keyed.multiplicative', name: '乘法密码', categoryId: 'crypto.classical.keyed', description: '乘法仿射密码', tags: ['古典密码', '乘法'] },
  { id: 'crypto.classical.keyed.otp', name: '一次一密 OTP', categoryId: 'crypto.classical.keyed', description: 'XOR + base64 一次性密钥', tags: ['古典密码', 'OTP'] },
  { id: 'crypto.classical.keyed.gronsfeld', name: 'Gronsfeld 密码', categoryId: 'crypto.classical.keyed', description: '数字串凯撒移位', tags: ['古典密码', 'Gronsfeld'] },
  { id: 'crypto.classical.keyed.beaufort', name: '博福特 Beaufort', categoryId: 'crypto.classical.keyed', description: '密钥词变体维吉尼亚', tags: ['古典密码', 'Beaufort'] },
  { id: 'crypto.classical.keyed.autokey', name: '自动密钥 Autokey', categoryId: 'crypto.classical.keyed', description: '明文扩展密钥', tags: ['古典密码', 'Autokey'] },
  { id: 'crypto.classical.keyed.scytale', name: '斯巴达棒 Scytale', categoryId: 'crypto.classical.keyed', description: '列数转置加密', tags: ['古典密码', 'Scytale'] },
  { id: 'crypto.classical.keyed.nihilist', name: 'Nihilist 密码', categoryId: 'crypto.classical.keyed', description: 'Polybius 加法加密', tags: ['古典密码', 'Nihilist'] },
  { id: 'crypto.classical.keyed.keyword', name: '关键字密码', categoryId: 'crypto.classical.keyed', description: '关键字重排字母表', tags: ['古典密码', '关键字'] },
  { id: 'crypto.classical.keyed.simple-substitution', name: '简单替换密码', categoryId: 'crypto.classical.keyed', description: '自定义字母替换表', tags: ['古典密码', '替换'] },
  { id: 'crypto.classical.keyed.rows-permutation', name: '行置换', categoryId: 'crypto.classical.keyed', description: '行置换加密', tags: ['古典密码', '置换'] },
  { id: 'crypto.classical.keyed.porta', name: '城门 Porta', categoryId: 'crypto.classical.keyed', description: 'Porta 密钥词密码', tags: ['古典密码', 'Porta'] },
  { id: 'crypto.classical.keyed.bazeries', name: 'Bazeries 密码', categoryId: 'crypto.classical.keyed', description: '数字串分段转置', tags: ['古典密码', 'Bazeries'] },
  { id: 'crypto.classical.keyed.fractionated-morse', name: '分组摩斯', categoryId: 'crypto.classical.keyed', description: '摩斯 + 三进制 + 关键字', tags: ['古典密码', '摩斯'] },
  { id: 'crypto.classical.keyed.running-key', name: '滚动密钥', categoryId: 'crypto.classical.keyed', description: '长密钥流加密', tags: ['古典密码', '滚动密钥'] },
  { id: 'crypto.classical.keyed.kamasutra', name: '爱经 Kamasutra', categoryId: 'crypto.classical.keyed', description: '固定配对替换', tags: ['古典密码', 'Kamasutra'] },
  { id: 'crypto.classical.keyed.fernet', name: 'Fernet', categoryId: 'crypto.classical.keyed', description: 'AES+HMAC 对称加密', tags: ['密码', 'Fernet'] },
  { id: 'crypto.classical.keyed.enigma', name: '恩尼格玛 Enigma', categoryId: 'crypto.classical.keyed', description: '转子/反射器/插线板', tags: ['古典密码', 'Enigma'] },

  // —— Crypto · 古典密码（无密钥 / 图形 / 坐标） ——
  { id: 'crypto.classical.pigpen', name: '猪圈密码', categoryId: 'crypto.classical', description: '猪圈图形替换', tags: ['古典密码', '猪圈'] },
  { id: 'crypto.classical.jefferson-wheel', name: '杰斐逊转轮', categoryId: 'crypto.classical', description: '转轮加密', tags: ['古典密码', '转轮'] },
  { id: 'crypto.classical.number-coordinate', name: '数字坐标', categoryId: 'crypto.classical', description: '数字坐标替换', tags: ['古典密码', '坐标'] },
  { id: 'crypto.classical.keyboard-chessboard', name: '键盘棋盘', categoryId: 'crypto.classical', description: '键盘棋盘坐标', tags: ['古典密码', '键盘'] },
  { id: 'crypto.classical.keyboard-coordinate', name: '键盘坐标', categoryId: 'crypto.classical', description: '键盘行列坐标', tags: ['古典密码', '键盘'] },
  { id: 'crypto.classical.qwe-keyboard', name: 'QWE 键盘', categoryId: 'crypto.classical', description: 'QWE 键盘映射', tags: ['古典密码', '键盘'] },
  { id: 'crypto.classical.phone-keypad', name: '手机键盘', categoryId: 'crypto.classical', description: '九宫格数字映射', tags: ['古典密码', '手机键盘'] },
  { id: 'crypto.classical.alphabet-order', name: '字母表顺序', categoryId: 'crypto.classical', description: '字母序数映射', tags: ['古典密码', '序数'] },
  { id: 'crypto.classical.pawnshat', name: '当铺密码', categoryId: 'crypto.classical', description: '当铺数字暗号', tags: ['中文密码', '当铺'] },
  { id: 'crypto.classical.standard-galactic', name: '标准银河字母', categoryId: 'crypto.classical', description: 'Standard Galactic 字母表', tags: ['古典密码', '银河'] },
  { id: 'crypto.classical.dna', name: 'DNA 编码', categoryId: 'crypto.classical', description: 'AGCT 碱基映射', tags: ['编码', 'DNA'] },
  { id: 'crypto.classical.a1z26', name: 'A1Z26', categoryId: 'crypto.classical', description: '字母序数 1-26', tags: ['编码', 'A1Z26'] },
  { id: 'crypto.classical.baudot', name: '博多码 Baudot', categoryId: 'crypto.classical', description: '5 位电传码', tags: ['编码', 'Baudot'] },
  { id: 'crypto.classical.type7', name: 'Cisco Type7', categoryId: 'crypto.classical', description: 'Cisco Type7 密码', tags: ['密码', 'Cisco'] },
  { id: 'crypto.classical.yunying', name: '云影密码 01248', categoryId: 'crypto.classical', description: '01248 云影密码', tags: ['中文密码', '云影'] },
  { id: 'crypto.classical.tap-code', name: '敲击码', categoryId: 'crypto.classical', description: '5x5 敲击坐标', tags: ['古典密码', '敲击码'] },
  { id: 'crypto.classical.nato', name: '北约音标', categoryId: 'crypto.classical', description: 'NATO 语音字母表', tags: ['编码', 'NATO'] },
  { id: 'crypto.classical.braille', name: '盲文', categoryId: 'crypto.classical', description: '盲文点字编码', tags: ['编码', '盲文'] },
  { id: 'crypto.classical.manchester', name: '曼彻斯特编码', categoryId: 'crypto.classical', description: '曼彻斯特电平编码', tags: ['编码', '曼彻斯特'] },
  { id: 'crypto.classical.keyboard-keycode', name: '键盘键码', categoryId: 'crypto.classical', description: '键盘键码映射', tags: ['编码', '键码'] },
  { id: 'crypto.classical.keyboard-shift', name: '键盘上档', categoryId: 'crypto.classical', description: 'Shift 上档字符映射', tags: ['编码', '键盘'] },
  { id: 'crypto.classical.handycode', name: '九宫格', categoryId: 'crypto.classical', description: '九宫格按键编码', tags: ['编码', '九宫格'] },
  { id: 'crypto.classical.dvorak', name: '德沃夏克键盘', categoryId: 'crypto.classical', description: 'Dvorak 键盘映射', tags: ['编码', 'Dvorak'] },
  { id: 'crypto.classical.solresol', name: '音符密码', categoryId: 'crypto.classical', description: '音符音节编码', tags: ['编码', '音符'] },
  { id: 'crypto.classical.chinesecode', name: '汉码 / 区位码', categoryId: 'crypto.classical', description: '中文区位码', tags: ['中文密码', '区位码'] },
  { id: 'crypto.classical.arrows', name: '箭头密码', categoryId: 'crypto.classical', description: '箭头方向编码', tags: ['编码', '箭头'] },
  { id: 'crypto.classical.bcd', name: 'BCD 码', categoryId: 'crypto.classical', description: '8421 BCD 编码', tags: ['编码', 'BCD'] },
  { id: 'crypto.classical.gray-code', name: '格雷码', categoryId: 'crypto.classical', description: '格雷码转换', tags: ['编码', '格雷码'] },
  { id: 'crypto.classical.hamming', name: '海明码', categoryId: 'crypto.classical', description: '海明码 (7,4) 纠错', tags: ['编码', '海明码'] },
  { id: 'crypto.classical.ieee754', name: 'IEEE754 浮点', categoryId: 'crypto.classical', description: '浮点数位模式转换', tags: ['编码', 'IEEE754'] },
  { id: 'crypto.classical.timestamp', name: '时间戳转换', categoryId: 'crypto.classical', description: '时间戳与日期转换', tags: ['编码', '时间戳'] },
  { id: 'crypto.classical.prime-factor', name: '质因数分解', categoryId: 'crypto.classical', description: 'Pollard rho 质因数分解', tags: ['Crypto', '因数分解'] },

  // —— Crypto · 现代密码补充 ——
  { id: 'crypto.modern.des', name: 'DES 加解密', categoryId: 'crypto.modern', description: 'DES 对称加密', tags: ['DES', '对称加密'] },
  { id: 'crypto.modern.rc4', name: 'RC4 加解密', categoryId: 'crypto.modern', description: 'RC4 流密码', tags: ['RC4', '流密码'] },
  // —— RSA 多项式关联分解攻击：代数关系 ——
  { id: 'crypto.modern.rsa.algebraic.polynomial', name: '多项式关系 q = f(p)', categoryId: 'crypto.modern.rsa', description: '代入解方程；Coppersmith 求小根', tags: ['RSA', '多项式关联', '多项式'] },

  // —— RSA 多项式关联分解攻击：数值接近（Fermat 家族） ——
  { id: 'crypto.modern.rsa.fermat-family.shared-msb', name: '共享 MSB', categoryId: 'crypto.modern.rsa', description: 'Coppersmith partial key exposure', tags: ['RSA', 'Coppersmith', 'MSB'] },
  { id: 'crypto.modern.rsa.fermat-family.shared-lsb', name: '共享 LSB', categoryId: 'crypto.modern.rsa', description: '模已知低位枚举', tags: ['RSA', 'Coppersmith', 'LSB'] },

  // —— RSA 多项式关联分解攻击：同一生成结构 ——
  { id: 'crypto.modern.rsa.shared-structure.poly-roots', name: '同一多项式两个根', categoryId: 'crypto.modern.rsa', description: 'p+q、pq 结构已知', tags: ['RSA', '生成结构', '多项式根'] },
  { id: 'crypto.modern.rsa.shared-structure.ec-multiple', name: '曲线/群相邻倍点', categoryId: 'crypto.modern.rsa', description: '椭圆曲线相邻倍点 x 坐标', tags: ['RSA', '生成结构', '椭圆曲线'] },
  { id: 'crypto.modern.rsa.shared-structure.lcg-lattice', name: 'LCG 相邻输出 / 格向量', categoryId: 'crypto.modern.rsa', description: 'LCG 相邻输出、格相邻向量坐标', tags: ['RSA', '生成结构', 'LCG'] },

  // —— RSA 多项式关联分解攻击：数论函数关系 ——
  { id: 'crypto.modern.rsa.number-theoretic.shared-gcd', name: 'gcd(p-1, q-1) 大', categoryId: 'crypto.modern.rsa', description: '共享大公因子影响 phi', tags: ['RSA', '数论', '公因子'] },

  // —— RSA 多项式关联分解攻击：格 / Coppersmith ——
  { id: 'crypto.modern.rsa.coppersmith.exact', name: '精确代数关系', categoryId: 'crypto.modern.rsa', description: 'q = f(p) 精确成立 -> 结式/代入', tags: ['RSA', 'Coppersmith', '结式'] },
  { id: 'crypto.modern.rsa.coppersmith.approximate', name: '近似关系', categoryId: 'crypto.modern.rsa', description: '|q - f(p)| < n^delta -> Coppersmith 小根', tags: ['RSA', 'Coppersmith', '近似'] },
  { id: 'crypto.modern.rsa.coppersmith.modular', name: '模关系', categoryId: 'crypto.modern.rsa', description: 'q = f(p) (mod M) -> 格', tags: ['RSA', 'Coppersmith', '模关系'] },
  { id: 'crypto.modern.rsa.coppersmith.implicit', name: '隐式关系', categoryId: 'crypto.modern.rsa', description: '结构参数小 -> 方程组 + 格/结式', tags: ['RSA', 'Coppersmith', '隐式'] },

  // —— Crypto · 哈希 / 校验 ——
  { id: 'crypto.hash.hashid', name: 'Hash 类型识别', categoryId: 'crypto.hash', description: '识别 40+ 种哈希算法', tags: ['哈希', '识别'] },
  { id: 'crypto.hash.crc32', name: 'CRC32 校验', categoryId: 'crypto.hash', description: 'CRC32 循环冗余校验', tags: ['哈希', 'CRC32'] },
  { id: 'crypto.hash.hmac', name: 'HMAC 计算', categoryId: 'crypto.hash', description: 'HMAC-MD5/SHA1/SHA256', tags: ['哈希', 'HMAC'] },

  // —— Crypto · 编码补充 ——
  { id: 'crypto.encoding.base-convert', name: '进制转换', categoryId: 'codec', description: '任意进制互转', tags: ['编码', '进制'] },

  // —— Crypto · JWT 攻击（Web 身份认证，归 Web.basic 已占，此处补充攻击面） ——
  { id: 'web.basic.jwt-none', name: 'JWT None 攻击', categoryId: 'web.basic', description: 'JWT None 算法攻击', tags: ['JWT', 'None'] },
  { id: 'web.basic.jwt-brute', name: 'JWT 弱密钥爆破', categoryId: 'web.basic', description: 'JWT 弱密钥爆破', tags: ['JWT', '爆破'] },
  { id: 'web.basic.jwt-forge', name: 'JWT 伪造', categoryId: 'web.basic', description: 'HS256 密钥伪造签名', tags: ['JWT', '伪造'] },
  { id: 'web.basic.jwt-role', name: 'JWT 角色提升', categoryId: 'web.basic', description: 'JWT 权限提升', tags: ['JWT', '越权'] },

  // —— Misc · 高级编码 ——
  { id: 'misc.encoding.jsfuck', name: 'JSFuck', categoryId: 'codec', description: '仅 []()!+ 字符的 JS 编码', tags: ['编码', 'JSFuck'] },
  { id: 'misc.encoding.aaencode', name: 'AAEncode', categoryId: 'codec', description: '颜文字 JS 编码', tags: ['编码', 'AAEncode'] },
  { id: 'misc.encoding.jjencode', name: 'JJEncode', categoryId: 'codec', description: '符号 JS 编码', tags: ['编码', 'JJEncode'] },
  { id: 'misc.encoding.rot5', name: 'ROT5', categoryId: 'codec', description: '数字旋转 5', tags: ['编码', 'ROT5'] },
  { id: 'misc.encoding.rot18', name: 'ROT18', categoryId: 'codec', description: 'ROT13 + ROT5', tags: ['编码', 'ROT18'] },
  { id: 'misc.encoding.rot8000', name: 'ROT8000', categoryId: 'codec', description: 'Unicode BMP 旋转', tags: ['编码', 'ROT8000'] },
  { id: 'misc.encoding.punycode', name: 'Punycode', categoryId: 'codec', description: 'IDNA 域名编码', tags: ['编码', 'Punycode'] },
  { id: 'misc.encoding.uuencode', name: 'UUEncode', categoryId: 'codec', description: 'Unix-to-Unix 编码', tags: ['编码', 'UUEncode'] },
  { id: 'misc.encoding.xxencode', name: 'XXEncode', categoryId: 'codec', description: '+- 字母表编码', tags: ['编码', 'XXEncode'] },
  { id: 'misc.encoding.utf7', name: 'UTF-7 编码', categoryId: 'codec', description: 'UTF-7 字符编码', tags: ['编码', 'UTF7'] },
  { id: 'misc.encoding.quoted-printable', name: 'Quoted-Printable', categoryId: 'codec', description: 'MIME =XX 编码', tags: ['编码', 'QP'] },
  { id: 'misc.encoding.shellcode', name: 'Shellcode 编码', categoryId: 'codec', description: '\\xNN 十六进制编码', tags: ['编码', 'Shellcode'] },
  { id: 'misc.encoding.ook', name: 'Ook!', categoryId: 'codec', description: 'Ook! 编程语言', tags: ['编码', 'Ook'] },
  { id: 'misc.encoding.decimal', name: '十进制编码', categoryId: 'codec', description: '十进制 ASCII 码', tags: ['编码', '十进制'] },
  { id: 'misc.encoding.octal', name: '八进制编码', categoryId: 'codec', description: '八进制 ASCII 码', tags: ['编码', '八进制'] },
  { id: 'misc.encoding.unicode', name: 'Unicode 转义', categoryId: 'codec', description: '\\uXXXX 转义', tags: ['编码', 'Unicode'] },
  { id: 'misc.encoding.html-entity', name: 'HTML 实体', categoryId: 'codec', description: '&amp; 等实体编码', tags: ['编码', 'HTML'] },
  { id: 'misc.encoding.zerowidth-encode', name: '零宽字符编码', categoryId: 'codec', description: '零宽字符隐写编码', tags: ['编码', '零宽'] },

  // —— Misc · 中文特色密码 ——
  { id: 'misc.encoding.buddha', name: '与佛论禅', categoryId: 'codec', description: '佛经格式编码', tags: ['中文密码', '佛曰'] },
  { id: 'misc.encoding.core-values', name: '核心价值观', categoryId: 'codec', description: '社会主义核心价值观映射', tags: ['中文密码', '核心价值观'] },
  { id: 'misc.encoding.bear', name: '熊曰', categoryId: 'codec', description: '熊曰 base64 编码', tags: ['中文密码', '熊曰'] },
  { id: 'misc.encoding.surnames', name: '百家姓', categoryId: 'codec', description: '姓氏映射编码', tags: ['中文密码', '百家姓'] },
  { id: 'misc.encoding.telegraph', name: '中文电码', categoryId: 'codec', description: '4 位数字电报码', tags: ['中文密码', '电码'] },
  { id: 'misc.encoding.periodic-table', name: '元素周期表', categoryId: 'codec', description: '元素符号 ↔ 原子序数', tags: ['中文密码', '元素'] },
  { id: 'misc.encoding.heavenly-stems', name: '天干地支', categoryId: 'codec', description: '60 甲子映射', tags: ['中文密码', '干支'] },
  { id: 'misc.encoding.bagua', name: '八卦符', categoryId: 'codec', description: '先天八卦映射', tags: ['中文密码', '八卦'] },
  { id: 'misc.encoding.hexagram', name: '伏羲六十四卦', categoryId: 'codec', description: '六十四卦 Unicode 映射', tags: ['中文密码', '卦象'] },
  { id: 'misc.encoding.vowel-cipher', name: '元音密码', categoryId: 'codec', description: 'aeiou 五进制', tags: ['中文密码', '元音'] },

  // —— Misc · 编程语言 / 怪奇语言 ——
  { id: 'misc.encoding.whitespace', name: 'Whitespace', categoryId: 'codec', description: '编码转换:空白字符编程语言(Whitespace esolang,与文本隐写的空白隐写工具互不相同)', tags: ['编码', 'Whitespace'] },
  { id: 'misc.encoding.deadfish', name: 'Deadfish', categoryId: 'codec', description: 'Deadfish 语言', tags: ['编码', 'Deadfish'] },
  { id: 'misc.encoding.spoon', name: 'Spoon', categoryId: 'codec', description: 'Spoon / Brainfuck 变体', tags: ['编码', 'Spoon'] },
  { id: 'misc.encoding.lolcode', name: 'LOLCODE', categoryId: 'codec', description: 'LOLCODE 语言', tags: ['编码', 'LOLCODE'] },
  { id: 'misc.encoding.chinese-num', name: '汉字数字', categoryId: 'codec', description: '汉字数字映射', tags: ['中文密码', '汉字数字'] },

  // —— Reverse / Binary ——
  { id: 'reverse.static.binary-analyzer', name: '二进制分析器', categoryId: 'reverse.static', description: 'PE/ELF/APK 检测与熵分析', tags: ['逆向', '二进制'] },
  { id: 'reverse.static.entropy', name: '熵分析', categoryId: 'reverse.static', description: '文件熵与加壳判断', tags: ['逆向', '熵'] },

  // —— Forensics ——
  { id: 'forensics.disk.zip-tools', name: 'ZIP 分析', categoryId: 'forensics.disk', description: '伪加密/弱密码/结构分析', tags: ['取证', 'ZIP'] },
  { id: 'forensics.disk.binwalk', name: '文件雕刻 Binwalk', categoryId: 'forensics.disk', description: '文件头识别与提取', tags: ['取证', 'Binwalk'] },
  { id: 'forensics.network.icmp', name: 'ICMP 数据提取', categoryId: 'traffic.protocol', description: 'ICMP 数据包内容提取', tags: ['取证', 'ICMP'] },
  { id: 'forensics.disk.crc-brute', name: 'PNG CRC 爆破', categoryId: 'forensics.disk', description: 'PNG 宽高 CRC 爆破修复', tags: ['取证', 'CRC'] },

  // —— Stego ——
  { id: 'stego.image.img-stego', name: '图片隐写分析', categoryId: 'stego.image', description: 'LSB/通道/元数据综合分析', tags: ['隐写', '图片'] },
  { id: 'stego.image.base64-stego', name: 'Base64 隐写', categoryId: 'stego.image', description: 'Base64 尾部隐藏位提取', tags: ['隐写', 'Base64'] },
  { id: 'stego.text.tll-stego', name: 'TLL 隐写', categoryId: 'stego.text', description: 'TLL 文本隐写', tags: ['隐写', 'TLL'] },
  { id: 'stego.image.qr-tools', name: '二维码工具', categoryId: 'stego.image', description: '二维码生成/解析/修复', tags: ['隐写', 'QR'] },
];

// ---------------------------------------------------------------------------
// 三-c、CyberChef 提取的功能占位符（暂不实现逻辑，走 local mock 回退）
// ---------------------------------------------------------------------------

const cyberChefShorthandRows: ShorthandRow[] = [
  // —— 哈希 ——
  { id: 'crypto.hash.md2', name: 'MD2 哈希', categoryId: 'crypto.hash', description: 'MD2 消息摘要', tags: ['哈希', 'MD2'], preset: 'local' },
  { id: 'crypto.hash.md4', name: 'MD4 哈希', categoryId: 'crypto.hash', description: 'MD4 消息摘要', tags: ['哈希', 'MD4'], preset: 'local' },
  { id: 'crypto.hash.md6', name: 'MD6 哈希', categoryId: 'crypto.hash', description: 'MD6 消息摘要', tags: ['哈希', 'MD6'], preset: 'local' },
  { id: 'crypto.hash.sha0', name: 'SHA-0 哈希', categoryId: 'crypto.hash', description: 'SHA-0 哈希', tags: ['哈希', 'SHA0'], preset: 'local' },
  { id: 'crypto.hash.sha2', name: 'SHA-2 系列', categoryId: 'crypto.hash', description: 'SHA-224/256/384/512', tags: ['哈希', 'SHA2'], preset: 'local' },
  { id: 'crypto.hash.sha3', name: 'SHA-3 系列', categoryId: 'crypto.hash', description: 'Keccak 变体 SHA-3', tags: ['哈希', 'SHA3'], preset: 'local' },
  { id: 'crypto.hash.sm3', name: 'SM3 哈希', categoryId: 'crypto.hash', description: '国密 SM3 哈希', tags: ['哈希', 'SM3'], preset: 'local' },
  { id: 'crypto.hash.keccak', name: 'Keccak 哈希', categoryId: 'crypto.hash', description: 'Keccak 哈希函数', tags: ['哈希', 'Keccak'], preset: 'local' },
  { id: 'crypto.hash.shake', name: 'SHAKE 哈希', categoryId: 'crypto.hash', description: '可扩展输出 XOF Shake128/256', tags: ['哈希', 'SHAKE'], preset: 'local' },
  { id: 'crypto.hash.ripemd', name: 'RIPEMD 系列', categoryId: 'crypto.hash', description: 'RIPEMD-128/160/256/320', tags: ['哈希', 'RIPEMD'], preset: 'local' },
  { id: 'crypto.hash.has160', name: 'HAS-160 哈希', categoryId: 'crypto.hash', description: 'HAS-160 哈希', tags: ['哈希', 'HAS160'], preset: 'local' },
  { id: 'crypto.hash.whirlpool', name: 'Whirlpool 哈希', categoryId: 'crypto.hash', description: 'Whirlpool 哈希', tags: ['哈希', 'Whirlpool'], preset: 'local' },
  { id: 'crypto.hash.snefru', name: 'Snefru 哈希', categoryId: 'crypto.hash', description: 'Snefru 哈希', tags: ['哈希', 'Snefru'], preset: 'local' },
  { id: 'crypto.hash.blake2b', name: 'BLAKE2b 哈希', categoryId: 'crypto.hash', description: 'BLAKE2b 哈希', tags: ['哈希', 'BLAKE2b'], preset: 'local' },
  { id: 'crypto.hash.blake2s', name: 'BLAKE2s 哈希', categoryId: 'crypto.hash', description: 'BLAKE2s 哈希', tags: ['哈希', 'BLAKE2s'], preset: 'local' },
  { id: 'crypto.hash.gost-hash', name: 'GOST 哈希', categoryId: 'crypto.hash', description: '俄罗斯 GOST 哈希', tags: ['哈希', 'GOST'], preset: 'local' },
  { id: 'crypto.hash.streebog', name: 'Streebog 哈希', categoryId: 'crypto.hash', description: '俄罗斯 Streebog 256/512', tags: ['哈希', 'Streebog'], preset: 'local' },
  { id: 'crypto.hash.ssdeep', name: 'SSDEEP 模糊哈希', categoryId: 'crypto.hash', description: 'ssdeep 相似文件模糊哈希', tags: ['哈希', 'ssdeep'], preset: 'local' },
  { id: 'crypto.hash.ctph', name: 'CTPH 模糊哈希', categoryId: 'crypto.hash', description: 'CTPH 模糊哈希', tags: ['哈希', 'CTPH'], preset: 'local' },
  { id: 'crypto.hash.compare-ssdeep', name: '比较 SSDEEP', categoryId: 'crypto.hash', description: '比较两个 ssdeep 哈希相似度', tags: ['哈希', 'ssdeep'], preset: 'local' },
  { id: 'crypto.hash.compare-ctph', name: '比较 CTPH', categoryId: 'crypto.hash', description: '比较两个 CTPH 哈希', tags: ['哈希', 'CTPH'], preset: 'local' },
  { id: 'crypto.hash.cmac', name: 'CMAC', categoryId: 'crypto.hash', description: '基于 AES 的 CMAC', tags: ['哈希', 'CMAC'], preset: 'local' },
  { id: 'crypto.hash.bcrypt', name: 'Bcrypt 哈希', categoryId: 'crypto.hash', description: 'bcrypt 密码哈希', tags: ['哈希', 'bcrypt'], preset: 'local' },
  { id: 'crypto.hash.bcrypt-compare', name: 'Bcrypt 校验', categoryId: 'crypto.hash', description: '校验密码是否匹配 bcrypt', tags: ['哈希', 'bcrypt'], preset: 'local' },
  { id: 'crypto.hash.bcrypt-parse', name: 'Bcrypt 解析', categoryId: 'crypto.hash', description: '解析 bcrypt 哈希字段', tags: ['哈希', 'bcrypt'], preset: 'local' },
  { id: 'crypto.hash.argon2', name: 'Argon2 哈希', categoryId: 'crypto.hash', description: 'Argon2i/d/id 密码哈希', tags: ['哈希', 'Argon2'], preset: 'local' },
  { id: 'crypto.hash.argon2-compare', name: 'Argon2 校验', categoryId: 'crypto.hash', description: '校验 Argon2 哈希', tags: ['哈希', 'Argon2'], preset: 'local' },
  { id: 'crypto.hash.nt-hash', name: 'NT Hash', categoryId: 'crypto.hash', description: 'Windows NTLM 哈希', tags: ['哈希', 'NTLM'], preset: 'local' },
  { id: 'crypto.hash.lm-hash', name: 'LM Hash', categoryId: 'crypto.hash', description: 'Windows LM 哈希', tags: ['哈希', 'LM'], preset: 'local' },
  { id: 'crypto.hash.fletcher8', name: 'Fletcher-8 校验', categoryId: 'crypto.hash', description: 'Fletcher-8 校验和', tags: ['校验', 'Fletcher'], preset: 'local' },
  { id: 'crypto.hash.fletcher16', name: 'Fletcher-16 校验', categoryId: 'crypto.hash', description: 'Fletcher-16 校验和', tags: ['校验', 'Fletcher'], preset: 'local' },
  { id: 'crypto.hash.fletcher32', name: 'Fletcher-32 校验', categoryId: 'crypto.hash', description: 'Fletcher-32 校验和', tags: ['校验', 'Fletcher'], preset: 'local' },
  { id: 'crypto.hash.fletcher64', name: 'Fletcher-64 校验', categoryId: 'crypto.hash', description: 'Fletcher-64 校验和', tags: ['校验', 'Fletcher'], preset: 'local' },
  { id: 'crypto.hash.adler32', name: 'Adler-32 校验', categoryId: 'crypto.hash', description: 'Adler-32 校验和', tags: ['校验', 'Adler32'], preset: 'local' },
  { id: 'crypto.hash.luhn', name: 'Luhn 校验', categoryId: 'crypto.hash', description: '银行卡号 Luhn 校验', tags: ['校验', 'Luhn'], preset: 'local' },
  { id: 'crypto.hash.crc8', name: 'CRC-8 校验', categoryId: 'crypto.hash', description: 'CRC-8 校验', tags: ['校验', 'CRC8'], preset: 'local' },
  { id: 'crypto.hash.crc16', name: 'CRC-16 校验', categoryId: 'crypto.hash', description: 'CRC-16 校验', tags: ['校验', 'CRC16'], preset: 'local' },
  { id: 'crypto.hash.tcpip-checksum', name: 'TCP/IP 校验和', categoryId: 'crypto.hash', description: 'TCP/IP 头部校验和', tags: ['校验', 'TCP'], preset: 'local' },
  { id: 'crypto.hash.all-hashes', name: '生成所有哈希', categoryId: 'crypto.hash', description: '一次生成所有常见哈希', tags: ['哈希', '批量'], preset: 'local' },

  // —— 现代加密 / 密钥派生 ——
  { id: 'crypto.modern.blowfish', name: 'Blowfish 加解密', categoryId: 'crypto.modern', description: 'Blowfish 分组密码', tags: ['加密', 'Blowfish'], preset: 'local' },
  { id: 'crypto.modern.triple-des', name: '3DES 加解密', categoryId: 'crypto.modern', description: 'Triple DES 加密', tags: ['加密', '3DES'], preset: 'local' },
  { id: 'crypto.modern.rc2', name: 'RC2 加解密', categoryId: 'crypto.modern', description: 'RC2 分组密码', tags: ['加密', 'RC2'], preset: 'local' },
  { id: 'crypto.modern.rc4-drop', name: 'RC4 Drop', categoryId: 'crypto.modern', description: 'RC4 丢弃前 N 字节', tags: ['加密', 'RC4'], preset: 'local' },
  { id: 'crypto.modern.chacha', name: 'ChaCha 加解密', categoryId: 'crypto.modern', description: 'ChaCha 流密码', tags: ['加密', 'ChaCha'], preset: 'local' },
  { id: 'crypto.modern.rabbit', name: 'Rabbit 加解密', categoryId: 'crypto.modern', description: 'Rabbit 流密码', tags: ['加密', 'Rabbit'], preset: 'local' },
  { id: 'crypto.modern.sm4', name: 'SM4 加解密', categoryId: 'crypto.modern', description: '国密 SM4 对称加密', tags: ['加密', 'SM4'], preset: 'local' },
  { id: 'crypto.modern.gost', name: 'GOST 加解密', categoryId: 'crypto.modern', description: '俄罗斯 GOST 加密', tags: ['加密', 'GOST'], preset: 'local' },
  { id: 'crypto.modern.gost-sign', name: 'GOST 签名/验签', categoryId: 'crypto.modern', description: 'GOST 签名与验签', tags: ['签名', 'GOST'], preset: 'local' },
  { id: 'crypto.modern.gost-keywrap', name: 'GOST 密钥封装', categoryId: 'crypto.modern', description: 'GOST 密钥封装/解封', tags: ['加密', 'GOST'], preset: 'local' },
  { id: 'crypto.modern.ls47', name: 'LS47 加解密', categoryId: 'crypto.modern', description: 'LS47 加密', tags: ['加密', 'LS47'], preset: 'local' },
  { id: 'crypto.modern.ciphersaber2', name: 'CipherSaber-2', categoryId: 'crypto.modern', description: 'CipherSaber-2 (RC4 变体)', tags: ['加密', 'RC4'], preset: 'local' },
  { id: 'crypto.modern.citrix-ctx1', name: 'Citrix CTX1', categoryId: 'crypto.modern', description: 'Citrix CTX1 密码编码', tags: ['编码', 'Citrix'], preset: 'local' },
  { id: 'crypto.modern.aes-keywrap', name: 'AES 密钥封装', categoryId: 'crypto.modern', description: 'AES Key Wrap (RFC 3394)', tags: ['加密', 'AES'], preset: 'local' },
  { id: 'crypto.modern.derive-pbkdf2', name: 'PBKDF2 密钥派生', categoryId: 'crypto.modern', description: 'PBKDF2 派生密钥', tags: ['密钥派生', 'PBKDF2'], preset: 'local' },
  { id: 'crypto.modern.derive-evp', name: 'EVP 密钥派生', categoryId: 'crypto.modern', description: 'OpenSSL EVP_BytesToKey', tags: ['密钥派生', 'EVP'], preset: 'local' },
  { id: 'crypto.modern.derive-hkdf', name: 'HKDF 密钥派生', categoryId: 'crypto.modern', description: 'HKDF 密钥派生', tags: ['密钥派生', 'HKDF'], preset: 'local' },
  { id: 'crypto.modern.prng', name: '伪随机数生成', categoryId: 'crypto.modern', description: '多算法 PRNG', tags: ['随机数', 'PRNG'], preset: 'local' },

  // —— 公钥 / 证书 ——
  { id: 'crypto.modern.rsa.parse-x509', name: '解析 X.509 证书', categoryId: 'crypto.modern.rsa', description: '解析证书字段', tags: ['证书', 'X509'], preset: 'local' },
  { id: 'crypto.modern.rsa.hex-oid', name: 'Hex 与 OID 互转', categoryId: 'crypto.modern.rsa', description: '十六进制与对象标识符互转', tags: ['ASN.1', 'OID'], preset: 'local' },
  { id: 'crypto.modern.rsa.generate-pgp', name: '生成 PGP 密钥对', categoryId: 'crypto.modern.rsa', description: 'OpenPGP 密钥对生成', tags: ['PGP', '密钥'], preset: 'local' },
  { id: 'crypto.modern.rsa.pgp-encrypt', name: 'PGP 加密', categoryId: 'crypto.modern.rsa', description: 'OpenPGP 加密', tags: ['PGP', '加密'], preset: 'local' },
  { id: 'crypto.modern.rsa.pgp-decrypt', name: 'PGP 解密', categoryId: 'crypto.modern.rsa', description: 'OpenPGP 解密', tags: ['PGP', '解密'], preset: 'local' },
  { id: 'crypto.modern.rsa.pgp-verify', name: 'PGP 验签', categoryId: 'crypto.modern.rsa', description: 'OpenPGP 验签', tags: ['PGP', '签名'], preset: 'local' },
  { id: 'crypto.modern.rsa.pgp-sign', name: 'PGP 加密并签名', categoryId: 'crypto.modern.rsa', description: 'OpenPGP 加密并签名', tags: ['PGP', '签名'], preset: 'local' },
  { id: 'crypto.modern.rsa.generate', name: '生成 RSA 密钥对', categoryId: 'crypto.modern.rsa', description: '生成 RSA 密钥对', tags: ['RSA', '密钥'], preset: 'local' },
  { id: 'crypto.modern.rsa.sign', name: 'RSA 签名', categoryId: 'crypto.modern.rsa', description: 'RSA 签名', tags: ['RSA', '签名'], preset: 'local' },
  { id: 'crypto.modern.rsa.verify', name: 'RSA 验签', categoryId: 'crypto.modern.rsa', description: 'RSA 验签', tags: ['RSA', '签名'], preset: 'local' },
  { id: 'crypto.modern.rsa.encrypt', name: 'RSA 加密', categoryId: 'crypto.modern.rsa', description: 'RSA 公钥加密', tags: ['RSA', '加密'], preset: 'local' },
  { id: 'crypto.modern.rsa.decrypt', name: 'RSA 解密', categoryId: 'crypto.modern.rsa', description: 'RSA 私钥解密', tags: ['RSA', '解密'], preset: 'local' },
  { id: 'crypto.modern.parse-ssh-key', name: '解析 SSH 主机公钥', categoryId: 'crypto.modern', description: '解析 RSA/ECDSA/Ed25519 公钥', tags: ['SSH', '公钥'], preset: 'local' },

  // —— 古典 / 机器密码 ——
  { id: 'crypto.classical.caesar-box', name: '凯撒盒密码', categoryId: 'crypto.classical', description: '按固定宽度方阵列读', tags: ['古典密码', '凯撒盒'], preset: 'local' },
  { id: 'crypto.classical.cetacean', name: '鲸豚密码', categoryId: 'crypto.classical', description: '鲸豚名称表替换', tags: ['古典密码', '鲸豚'], preset: 'local' },
  { id: 'crypto.classical.substitute', name: '字符替换', categoryId: 'crypto.classical', description: '按自定义映射表替换', tags: ['古典密码', '替换'], preset: 'local' },
  { id: 'crypto.classical.typex', name: 'Typex 密码机', categoryId: 'crypto.classical', description: '英军改良 Enigma', tags: ['机器密码', 'Typex'], preset: 'local' },
  { id: 'crypto.classical.lorenz', name: 'Lorenz 密码机', categoryId: 'crypto.classical', description: '德军 SZ40/42', tags: ['机器密码', 'Lorenz'], preset: 'local' },
  { id: 'crypto.classical.colossus', name: 'Colossus 计算机', categoryId: 'crypto.classical', description: '破解 Lorenz', tags: ['机器密码', 'Colossus'], preset: 'local' },
  { id: 'crypto.classical.sigaba', name: 'SIGABA 密码机', categoryId: 'crypto.classical', description: '美军转轮密码机', tags: ['机器密码', 'SIGABA'], preset: 'local' },
  { id: 'crypto.classical.bombe', name: 'Bombe 破译机', categoryId: 'crypto.classical', description: '破解 Enigma', tags: ['机器密码', 'Bombe'], preset: 'local' },
  { id: 'crypto.classical.multiple-bombe', name: '多组 Bombe', categoryId: 'crypto.classical', description: '多组联合破解 Enigma', tags: ['机器密码', 'Bombe'], preset: 'local' },

  // —— 数据格式 / 编码 ——
  { id: 'crypto.encoding.hexdump', name: 'Hexdump 转储', categoryId: 'codec', description: '十六进制转储视图', tags: ['编码', 'Hexdump'], preset: 'local' },
  { id: 'crypto.encoding.from-hexdump', name: 'Hexdump 还原', categoryId: 'codec', description: '从 hexdump 还原二进制', tags: ['编码', 'Hexdump'], preset: 'local' },
  { id: 'crypto.encoding.charcode', name: '字符码转换', categoryId: 'codec', description: '字符码与文本互转', tags: ['编码', 'Charcode'], preset: 'local' },
  { id: 'crypto.encoding.base64-offsets', name: 'Base64 偏移', categoryId: 'codec', description: '定位 Base64 解码错误位', tags: ['编码', 'Base64'], preset: 'local' },
  { id: 'crypto.encoding.custom-base', name: '自定义 Base 编解码', categoryId: 'codec', description: '自定义字符表进制编解码', tags: ['编码', 'Base'], preset: 'local' },
  { id: 'crypto.encoding.amf', name: 'AMF 编解码', categoryId: 'codec', description: 'Action Message Format', tags: ['编码', 'AMF'], preset: 'local' },
  { id: 'crypto.encoding.hex-content', name: 'Hex Content 转换', categoryId: 'codec', description: '0x 前缀逗号分隔格式', tags: ['编码', 'Hex'], preset: 'local' },
  { id: 'crypto.encoding.pem-hex', name: 'PEM 与 Hex 互转', categoryId: 'codec', description: 'PEM/DER 与十六进制互转', tags: ['编码', 'PEM'], preset: 'local' },
  { id: 'crypto.encoding.parse-asn1', name: '解析 ASN.1', categoryId: 'codec', description: '解析 ASN.1 DER 结构', tags: ['编码', 'ASN.1'], preset: 'local' },
  { id: 'crypto.encoding.ip-format', name: 'IP 格式转换', categoryId: 'codec', description: '十进制/IPv4/IPv6/十六进制互转', tags: ['编码', 'IP'], preset: 'local' },
  { id: 'crypto.encoding.charset', name: '字符集编解码', categoryId: 'codec', description: 'UTF-8/GBK/ISO-8859 等字符集转换', tags: ['编码', '字符集'], preset: 'local' },
  { id: 'crypto.encoding.charset-brute', name: '字符集暴力解码', categoryId: 'codec', description: '尝试多种字符集解码', tags: ['编码', '字符集'], preset: 'local' },
  { id: 'crypto.encoding.endianness', name: '字节序交换', categoryId: 'codec', description: '大小端互换', tags: ['编码', '端序'], preset: 'local' },
  { id: 'crypto.encoding.messagepack', name: 'MessagePack 编解码', categoryId: 'codec', description: 'MessagePack 二进制格式', tags: ['编码', 'MessagePack'], preset: 'local' },
  { id: 'crypto.encoding.parse-tlv', name: '解析 TLV', categoryId: 'codec', description: '解析 Tag-Length-Value', tags: ['编码', 'TLV'], preset: 'local' },
  { id: 'crypto.encoding.csv-json', name: 'CSV 与 JSON 互转', categoryId: 'codec', description: 'CSV/JSON 互相转换', tags: ['格式', 'CSV'], preset: 'local' },
  { id: 'crypto.encoding.avro-json', name: 'Avro 转 JSON', categoryId: 'codec', description: 'Avro 二进制转 JSON', tags: ['格式', 'Avro'], preset: 'local' },
  { id: 'crypto.encoding.cbor', name: 'CBOR 编解码', categoryId: 'codec', description: 'CBOR (RFC 8949)', tags: ['编码', 'CBOR'], preset: 'local' },

  // —— 算术 / 逻辑 ——
  { id: 'misc.other.set-union', name: '集合并集', categoryId: 'misc.other', description: '多行集合并集', tags: ['集合', '并集', 'task:计算'], preset: 'local' },
  { id: 'misc.other.set-intersection', name: '集合交集', categoryId: 'misc.other', description: '多行集合交集', tags: ['集合', '交集', 'task:计算'], preset: 'local' },
  { id: 'misc.other.set-difference', name: '集合差集', categoryId: 'misc.other', description: '集合差集', tags: ['集合', '差集', 'task:计算'], preset: 'local' },
  { id: 'misc.other.set-symmetric', name: '集合对称差', categoryId: 'misc.other', description: '集合对称差', tags: ['集合', '对称差', 'task:计算'], preset: 'local' },
  { id: 'misc.other.cartesian-product', name: '笛卡尔积', categoryId: 'misc.other', description: '集合笛卡尔积', tags: ['集合', '笛卡尔积', 'task:计算'], preset: 'local' },
  { id: 'misc.other.power-set', name: '幂集', categoryId: 'misc.other', description: '生成所有子集', tags: ['集合', '幂集', 'task:计算'], preset: 'local' },
  { id: 'misc.other.bitwise-or', name: '按位 OR', categoryId: 'misc.other', description: '与密钥按位或', tags: ['位运算', 'OR', 'task:计算'], preset: 'local' },
  { id: 'misc.other.bitwise-not', name: '按位 NOT', categoryId: 'misc.other', description: '按位取反', tags: ['位运算', 'NOT', 'task:计算'], preset: 'local' },
  { id: 'misc.other.bitwise-and', name: '按位 AND', categoryId: 'misc.other', description: '与密钥按位与', tags: ['位运算', 'AND', 'task:计算'], preset: 'local' },
  { id: 'misc.other.bitwise-add', name: '按位 ADD', categoryId: 'misc.other', description: '每字节加指定值', tags: ['位运算', 'ADD', 'task:计算'], preset: 'local' },
  { id: 'misc.other.bitwise-sub', name: '按位 SUB', categoryId: 'misc.other', description: '每字节减指定值', tags: ['位运算', 'SUB', 'task:计算'], preset: 'local' },
  { id: 'misc.other.bitshift', name: '位移/循环移位', categoryId: 'misc.other', description: '逻辑与循环左右移', tags: ['位运算', '移位', 'task:计算'], preset: 'local' },
  { id: 'misc.other.math-stats', name: '数值统计', categoryId: 'misc.other', description: '求和/乘积/均值/中位数/标准差', tags: ['数学', '统计', 'task:计算'], preset: 'local' },

  // —— 文本工具 ——
  { id: 'misc.other.diff', name: '文本差异对比', categoryId: 'misc.other', description: '逐行/逐字差异', tags: ['文本', 'Diff', 'task:分析'], preset: 'local' },
  { id: 'misc.other.remove-whitespace', name: '去除空白', categoryId: 'misc.other', description: '去除空白字符', tags: ['文本', '空白', 'task:转换'], preset: 'local' },
  { id: 'misc.other.remove-null', name: '去除空字节', categoryId: 'misc.other', description: '去除 0x00 空字节', tags: ['文本', '空字节', 'task:转换'], preset: 'local' },
  { id: 'misc.other.case', name: '大小写转换', categoryId: 'misc.other', description: '大写/小写/互换', tags: ['文本', '大小写', 'task:转换'], preset: 'local' },
  { id: 'misc.other.line-numbers', name: '行号处理', categoryId: 'misc.other', description: '添加/去除行号', tags: ['文本', '行号', 'task:转换'], preset: 'local' },
  { id: 'misc.other.all-casings', name: '大小写组合', categoryId: 'misc.other', description: '生成所有大小写组合', tags: ['文本', '大小写', 'task:转换'], preset: 'local' },
  { id: 'misc.other.to-table', name: '转 Markdown 表格', categoryId: 'misc.other', description: '分隔文本转表格', tags: ['文本', '表格', 'task:转换'], preset: 'local' },
  { id: 'misc.other.sort', name: '排序', categoryId: 'misc.other', description: '字母/数值/长度排序', tags: ['文本', '排序', 'task:转换'], preset: 'local' },
  { id: 'misc.other.shuffle', name: '随机打乱', categoryId: 'misc.other', description: '随机打乱顺序', tags: ['文本', '打乱', 'task:转换'], preset: 'local' },
  { id: 'misc.other.unique', name: '去重', categoryId: 'misc.other', description: '去除重复项', tags: ['文本', '去重', 'task:转换'], preset: 'local' },
  { id: 'misc.other.split', name: '拆分', categoryId: 'misc.other', description: '按分隔符拆分', tags: ['文本', '拆分', 'task:转换'], preset: 'local' },
  { id: 'misc.other.filter', name: '过滤行', categoryId: 'misc.other', description: '按条件过滤', tags: ['文本', '过滤', 'task:转换'], preset: 'local' },
  { id: 'misc.other.head-tail', name: '取头/取尾', categoryId: 'misc.other', description: '保留前/后 N 项', tags: ['文本', '截取', 'task:转换'], preset: 'local' },
  { id: 'misc.other.count', name: '统计出现次数', categoryId: 'misc.other', description: '统计字符/串次数', tags: ['文本', '统计', 'task:分析'], preset: 'local' },
  { id: 'misc.other.expand-range', name: '展开字母范围', categoryId: 'misc.other', description: 'a-c 展开为 abc', tags: ['文本', '范围', 'task:转换'], preset: 'local' },
  { id: 'misc.other.drop-take-bytes', name: '丢弃/截取字节', categoryId: 'misc.other', description: '丢弃或截取指定字节区间', tags: ['文本', '字节', 'task:转换'], preset: 'local' },
  { id: 'misc.other.pad-lines', name: '行填充', categoryId: 'misc.other', description: '行填充到固定长度', tags: ['文本', '填充', 'task:转换'], preset: 'local' },
  { id: 'misc.other.find-replace', name: '查找替换', categoryId: 'misc.other', description: '普通或正则查找替换', tags: ['文本', '替换', 'task:转换'], preset: 'local' },
  { id: 'misc.other.regex', name: '正则匹配', categoryId: 'misc.other', description: '正则提取/替换', tags: ['文本', '正则', 'task:提取'], preset: 'local' },
  { id: 'misc.other.fuzzy-match', name: '模糊匹配', categoryId: 'misc.other', description: '近似搜索带阈值', tags: ['文本', '模糊', 'task:分析'], preset: 'local' },
  { id: 'misc.other.offset-checker', name: '字节偏移检查', categoryId: 'misc.other', description: '标注每个字节偏移', tags: ['文本', '偏移', 'task:计算'], preset: 'local' },
  { id: 'misc.other.hamming-distance', name: '汉明距离', categoryId: 'misc.other', description: '不同位数量', tags: ['数学', '距离', 'task:计算'], preset: 'local' },
  { id: 'misc.other.levenshtein', name: '莱文斯坦距离', categoryId: 'misc.other', description: '编辑距离', tags: ['数学', '距离', 'task:计算'], preset: 'local' },
  { id: 'misc.other.unit-convert', name: '单位换算', categoryId: 'misc.other', description: '长度/面积/质量/速度/数据量', tags: ['工具', '单位', 'task:转换'], preset: 'local' },
  { id: 'misc.other.coordinate', name: '坐标格式换算', categoryId: 'misc.other', description: '十进制度/度分秒等', tags: ['工具', '坐标', 'task:转换'], preset: 'local' },
  { id: 'misc.other.unix-permissions', name: 'Unix 权限解析', categoryId: 'misc.other', description: '755 转 rwxr-xr-x', tags: ['工具', '权限', 'task:转换'], preset: 'local' },
  { id: 'misc.other.objectid', name: 'ObjectID 时间戳', categoryId: 'misc.other', description: '解析 MongoDB ObjectID', tags: ['工具', 'ObjectID', 'task:转换'], preset: 'local' },
  { id: 'misc.other.colour-code', name: '颜色代码解析', categoryId: 'misc.other', description: 'hex/rgb/hsl 互转', tags: ['工具', '颜色', 'task:解析'], preset: 'local' },
  { id: 'misc.other.escape-string', name: '字符串转义', categoryId: 'misc.other', description: '转义/反转义', tags: ['文本', '转义', 'task:转换'], preset: 'local' },

  // —— 日期 / 时间 ——
  { id: 'misc.other.parse-datetime', name: '解析日期时间', categoryId: 'misc.other', description: '解析日期时间字符串', tags: ['时间', '解析', 'task:解析'], preset: 'local' },
  { id: 'misc.other.translate-datetime', name: '日期时间格式转换', categoryId: 'misc.other', description: '不同格式互转', tags: ['时间', '格式', 'task:转换'], preset: 'local' },
  { id: 'misc.other.filetime', name: 'Windows FILETIME 转换', categoryId: 'misc.other', description: 'FILETIME 与 UNIX 时间戳互转', tags: ['时间', 'FILETIME', 'task:转换'], preset: 'local' },
  { id: 'misc.other.extract-dates', name: '提取日期', categoryId: 'misc.other', description: '从文本提取日期', tags: ['时间', '提取', 'task:提取'], preset: 'local' },
  { id: 'misc.other.get-time', name: '获取当前时间', categoryId: 'misc.other', description: '自定义格式/时区', tags: ['时间', '当前', 'task:生成'], preset: 'local' },

  // —— 提取器 ——
  { id: 'forensics.disk.extract-ip', name: '提取 IP 地址', categoryId: 'forensics.disk', description: '提取 IP 地址', tags: ['提取', 'IP'], preset: 'local' },
  { id: 'forensics.disk.extract-email', name: '提取邮箱', categoryId: 'forensics.disk', description: '提取邮箱地址', tags: ['提取', '邮箱'], preset: 'local' },
  { id: 'forensics.disk.extract-mac', name: '提取 MAC', categoryId: 'forensics.disk', description: '提取 MAC 地址', tags: ['提取', 'MAC'], preset: 'local' },
  { id: 'forensics.disk.extract-url', name: '提取 URL', categoryId: 'forensics.disk', description: '提取 URL', tags: ['提取', 'URL'], preset: 'local' },
  { id: 'forensics.disk.extract-domain', name: '提取域名', categoryId: 'forensics.disk', description: '提取域名', tags: ['提取', '域名'], preset: 'local' },
  { id: 'forensics.disk.extract-path', name: '提取文件路径', categoryId: 'forensics.disk', description: '提取文件路径', tags: ['提取', '路径'], preset: 'local' },
  { id: 'forensics.disk.xpath', name: 'XPath 提取', categoryId: 'forensics.disk', description: '从 XML 提取节点', tags: ['提取', 'XPath'], preset: 'local' },
  { id: 'forensics.disk.jpath', name: 'JPath 提取', categoryId: 'forensics.disk', description: '从 JSON 提取', tags: ['提取', 'JPath'], preset: 'local' },
  { id: 'forensics.disk.css-selector', name: 'CSS 选择器提取', categoryId: 'forensics.disk', description: '从 HTML 提取', tags: ['提取', 'CSS'], preset: 'local' },
  { id: 'forensics.disk.extract-id3', name: '提取 ID3 标签', categoryId: 'forensics.disk', description: '音频 ID3 标签', tags: ['提取', 'ID3'], preset: 'local' },

  // —— 压缩 ——
  { id: 'forensics.disk.deflate', name: 'Deflate 压缩/解压', categoryId: 'forensics.disk', description: 'Raw DEFLATE', tags: ['压缩', 'Deflate'], preset: 'local' },
  { id: 'forensics.disk.zlib', name: 'Zlib 压缩/解压', categoryId: 'forensics.disk', description: 'zlib 格式', tags: ['压缩', 'zlib'], preset: 'local' },
  { id: 'forensics.disk.gzip', name: 'Gzip 压缩/解压', categoryId: 'forensics.disk', description: 'gzip 格式', tags: ['压缩', 'gzip'], preset: 'local' },
  { id: 'forensics.disk.zip', name: 'ZIP 打包/解压', categoryId: 'forensics.disk', description: 'ZIP 打包与解压', tags: ['压缩', 'ZIP'], preset: 'local' },
  { id: 'forensics.disk.bzip2', name: 'Bzip2 压缩/解压', categoryId: 'forensics.disk', description: 'bzip2 格式', tags: ['压缩', 'bzip2'], preset: 'local' },
  { id: 'forensics.disk.tar', name: 'TAR 打包/解包', categoryId: 'forensics.disk', description: 'TAR 归档', tags: ['压缩', 'TAR'], preset: 'local' },
  { id: 'forensics.disk.lzstring', name: 'LZString 压缩/解压', categoryId: 'forensics.disk', description: 'LZString 格式', tags: ['压缩', 'LZString'], preset: 'local' },
  { id: 'forensics.disk.lzma', name: 'LZMA 压缩/解压', categoryId: 'forensics.disk', description: 'LZMA 格式', tags: ['压缩', 'LZMA'], preset: 'local' },
  { id: 'forensics.disk.lz4', name: 'LZ4 压缩/解压', categoryId: 'forensics.disk', description: 'LZ4 格式', tags: ['压缩', 'LZ4'], preset: 'local' },

  // —— 网络 ——
  { id: 'forensics.network.http-request', name: 'HTTP 请求', categoryId: 'traffic.protocol', description: '发送 HTTP 请求', tags: ['网络', 'HTTP'], preset: 'local' },
  { id: 'forensics.network.doh', name: 'DNS over HTTPS', categoryId: 'traffic.protocol', description: 'DoH DNS 查询', tags: ['网络', 'DNS'], preset: 'local' },
  { id: 'forensics.network.strip-headers', name: '剥离 HTTP 头', categoryId: 'traffic.parse', description: '只保留响应正文', tags: ['网络', 'HTTP'], preset: 'local' },
  { id: 'forensics.network.dechunk', name: 'Dechunk HTTP', categoryId: 'traffic.parse', description: '解码 chunked 传输', tags: ['网络', 'HTTP'], preset: 'local' },
  { id: 'forensics.network.user-agent', name: '解析 User-Agent', categoryId: 'traffic.fingerprint', description: '识别浏览器/系统/设备', tags: ['网络', 'UA'], preset: 'local' },
  { id: 'forensics.network.ip-range', name: '解析 IP 段', categoryId: 'traffic.assist', description: '展开 CIDR 或范围', tags: ['网络', 'IP'], preset: 'local' },
  { id: 'forensics.network.ipv6', name: '解析 IPv6', categoryId: 'traffic.protocol', description: '解析 IPv6 地址', tags: ['网络', 'IPv6'], preset: 'local' },
  { id: 'forensics.network.ipv4-header', name: '解析 IPv4 头', categoryId: 'traffic.protocol', description: '解析 IPv4 报文头', tags: ['网络', 'IPv4'], preset: 'local' },
  { id: 'forensics.network.tcp', name: '解析 TCP', categoryId: 'traffic.protocol', description: '解析 TCP 报文头', tags: ['网络', 'TCP'], preset: 'local' },
  { id: 'forensics.network.udp', name: '解析 UDP', categoryId: 'traffic.protocol', description: '解析 UDP 报文头', tags: ['网络', 'UDP'], preset: 'local' },
  { id: 'forensics.network.parse-uri', name: '解析 URI', categoryId: 'traffic.protocol', description: '解析 URI 各组成部分', tags: ['网络', 'URI'], preset: 'local' },
  { id: 'forensics.network.protobuf', name: 'Protobuf 编解码', categoryId: 'traffic.protocol', description: 'Protocol Buffers', tags: ['网络', 'Protobuf'], preset: 'local' },
  { id: 'forensics.network.varint', name: 'VarInt 编解码', categoryId: 'traffic.protocol', description: '变长整数', tags: ['网络', 'VarInt'], preset: 'local' },
  { id: 'forensics.network.ja3', name: 'JA3/JA3S 指纹', categoryId: 'traffic.fingerprint', description: 'TLS 客户端/服务端指纹', tags: ['网络', 'JA3'], preset: 'local' },
  { id: 'forensics.network.hassh', name: 'HASSH 指纹', categoryId: 'traffic.fingerprint', description: 'SSH 客户端/服务端指纹', tags: ['网络', 'HASSH'], preset: 'local' },
  { id: 'forensics.network.mac-format', name: '格式化 MAC', categoryId: 'traffic.fingerprint', description: '统一 MAC 地址格式', tags: ['网络', 'MAC'], preset: 'local' },
  { id: 'forensics.network.group-ip', name: 'IP 分组', categoryId: 'traffic.assist', description: '按网段聚合分组', tags: ['网络', 'IP'], preset: 'local' },
  { id: 'forensics.network.netbios', name: 'NetBIOS 编解码', categoryId: 'traffic.protocol', description: 'NetBIOS 名称编解码', tags: ['网络', 'NetBIOS'], preset: 'local' },
  { id: 'forensics.network.defang', name: 'URL/IP 去尖牙', categoryId: 'traffic.assist', description: 'hxxp / 1[.]2[.]3[.]4', tags: ['网络', 'Defang'], preset: 'local' },

  // —— 取证 ——
  { id: 'forensics.disk.scan-embedded', name: '扫描内嵌文件', categoryId: 'forensics.disk', description: '扫描数据中内嵌文件', tags: ['取证', '内嵌'], preset: 'local' },
  { id: 'forensics.disk.yara', name: 'YARA 规则扫描', categoryId: 'forensics.disk', description: '特征匹配扫描', tags: ['取证', 'YARA'], preset: 'local' },
  { id: 'forensics.disk.remove-exif', name: '移除 EXIF', categoryId: 'forensics.disk', description: '移除图片 EXIF', tags: ['取证', 'EXIF'], preset: 'local' },
  { id: 'stego.image.extract-rgba', name: '提取 RGBA', categoryId: 'stego.image', description: '提取每像素 RGBA', tags: ['隐写', 'RGBA'], preset: 'local' },
  { id: 'stego.image.bit-plane', name: '查看位平面', categoryId: 'stego.image', description: '查看图片 0-7 位平面', tags: ['隐写', '位平面'], preset: 'local' },
  { id: 'stego.image.palette', name: '随机调色板', categoryId: 'stego.image', description: '打乱图片调色板', tags: ['隐写', '调色板'], preset: 'local' },

  // —— 多媒体 / 图片 ——
  { id: 'stego.image.render-image', name: '渲染图片', categoryId: 'stego.image', description: '把输入渲染成图片', tags: ['图片', '渲染'], preset: 'local' },
  { id: 'stego.image.generate-image', name: '生成图片', categoryId: 'stego.image', description: '生成纯色图片', tags: ['图片', '生成'], preset: 'local' },
  { id: 'stego.image.ocr', name: 'OCR 文字识别', categoryId: 'stego.image', description: 'Tesseract 文字识别', tags: ['图片', 'OCR'], preset: 'local' },
  { id: 'stego.image.rotate', name: '旋转图片', categoryId: 'stego.image', description: '旋转图片', tags: ['图片', '旋转'], preset: 'local' },
  { id: 'stego.image.resize', name: '缩放图片', categoryId: 'stego.image', description: '缩放图片', tags: ['图片', '缩放'], preset: 'local' },
  { id: 'stego.image.blur', name: '模糊图片', categoryId: 'stego.image', description: '高斯模糊', tags: ['图片', '模糊'], preset: 'local' },
  { id: 'stego.image.dither', name: '抖动图片', categoryId: 'stego.image', description: '减色/像素化', tags: ['图片', '抖动'], preset: 'local' },
  { id: 'stego.image.invert', name: '反色图片', categoryId: 'stego.image', description: '反色', tags: ['图片', '反色'], preset: 'local' },
  { id: 'stego.image.flip', name: '翻转图片', categoryId: 'stego.image', description: '水平/垂直翻转', tags: ['图片', '翻转'], preset: 'local' },
  { id: 'stego.image.crop', name: '裁剪图片', categoryId: 'stego.image', description: '裁剪图片', tags: ['图片', '裁剪'], preset: 'local' },
  { id: 'stego.image.brightness', name: '亮度/对比度', categoryId: 'stego.image', description: '调整亮度/对比度', tags: ['图片', '亮度'], preset: 'local' },
  { id: 'stego.image.filter', name: '图像滤镜', categoryId: 'stego.image', description: '灰度/锐化等滤镜', tags: ['图片', '滤镜'], preset: 'local' },
  { id: 'stego.image.hsl', name: '色相/饱和度/明度', categoryId: 'stego.image', description: '调整 HSL', tags: ['图片', 'HSL'], preset: 'local' },
  { id: 'stego.image.sharpen', name: '锐化图片', categoryId: 'stego.image', description: '锐化图片', tags: ['图片', '锐化'], preset: 'local' },
  { id: 'stego.image.normalise', name: '直方图归一化', categoryId: 'stego.image', description: '增强对比度', tags: ['图片', '归一化'], preset: 'local' },
  { id: 'stego.image.convert-format', name: '转换图片格式', categoryId: 'stego.image', description: '图片格式转换', tags: ['图片', '格式'], preset: 'local' },
  { id: 'stego.image.add-text', name: '图片叠加文字', categoryId: 'stego.image', description: '在图片上叠加文字', tags: ['图片', '文字'], preset: 'local' },

  // —— 其他 ——
  { id: 'misc.other.entropy', name: '香农熵计算', categoryId: 'misc.other', description: '衡量随机性', tags: ['分析', '熵', 'task:计算'], preset: 'local' },
  { id: 'misc.other.frequency', name: '频率分布统计', categoryId: 'misc.other', description: '字符/字节频率分布', tags: ['分析', '频率', 'task:分析'], preset: 'local' },
  { id: 'misc.other.index-coincidence', name: '重合指数', categoryId: 'misc.other', description: 'IC 重合指数', tags: ['分析', 'IC', 'task:计算'], preset: 'local' },
  { id: 'misc.other.chi-square', name: '卡方检验', categoryId: 'misc.other', description: '卡方统计检验', tags: ['分析', '卡方', 'task:分析'], preset: 'local' },
  { id: 'misc.other.de-bruijn', name: 'De Bruijn 序列', categoryId: 'pwn.stack', description: '生成 De Bruijn 序列（溢出偏移）', tags: ['Pwn', 'DeBruijn'], preset: 'local' },
  { id: 'misc.other.uuid', name: '生成 UUID', categoryId: 'misc.other', description: '生成 UUID', tags: ['工具', 'UUID', 'task:生成'], preset: 'local' },
  { id: 'misc.other.totp', name: '生成 TOTP', categoryId: 'misc.other', description: '基于时间的动态口令', tags: ['工具', 'TOTP', 'task:生成'], preset: 'local' },
  { id: 'misc.other.hotp', name: '生成 HOTP', categoryId: 'misc.other', description: '基于计数器的动态口令', tags: ['工具', 'HOTP', 'task:生成'], preset: 'local' },
  { id: 'misc.other.haversine', name: '大圆距离', categoryId: 'misc.other', description: 'Haversine 坐标距离', tags: ['工具', '距离', 'task:计算'], preset: 'local' },
  { id: 'misc.other.html-to-text', name: 'HTML 转文本', categoryId: 'misc.other', description: 'HTML 转纯文本', tags: ['文本', 'HTML', 'task:转换'], preset: 'local' },
  { id: 'misc.other.lorem-ipsum', name: 'Lorem Ipsum', categoryId: 'misc.other', description: '生成占位文本', tags: ['工具', '占位', 'task:生成'], preset: 'local' },

  // —— 代码整理 ——
  { id: 'misc.other.code-beautify', name: '代码美化', categoryId: 'misc.other', description: '通用代码美化', tags: ['代码', '美化', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.js-tools', name: 'JS 解析/美化/压缩', categoryId: 'misc.other', description: 'JavaScript AST/美化/压缩', tags: ['代码', 'JS', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.json-tidy', name: 'JSON 格式化/压缩', categoryId: 'misc.other', description: 'JSON 美化/压缩', tags: ['代码', 'JSON', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.xml-tidy', name: 'XML 格式化/压缩', categoryId: 'misc.other', description: 'XML 美化/压缩', tags: ['代码', 'XML', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.sql-tidy', name: 'SQL 格式化/压缩', categoryId: 'misc.other', description: 'SQL 美化/压缩', tags: ['代码', 'SQL', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.css-tidy', name: 'CSS 格式化/压缩', categoryId: 'misc.other', description: 'CSS 美化/压缩', tags: ['代码', 'CSS', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.ms-script-decode', name: '微软脚本解码', categoryId: 'misc.other', description: '解码 JScript.Encode', tags: ['代码', '解码', 'task:编解码'], preset: 'local' },
  { id: 'misc.other.strip-html', name: '去除 HTML 标签', categoryId: 'misc.other', description: '去除 HTML 标签', tags: ['文本', 'HTML', 'task:转换'], preset: 'local' },
  { id: 'misc.other.case-convert', name: '命名风格转换', categoryId: 'misc.other', description: 'snake/camel/kebab 互转', tags: ['代码', '命名', 'task:转换'], preset: 'local' },
  { id: 'misc.other.bson', name: 'BSON 序列化', categoryId: 'misc.other', description: 'BSON 序列化/反序列化', tags: ['格式', 'BSON', 'task:转换'], preset: 'local' },
  { id: 'misc.other.render-markdown', name: '渲染 Markdown', categoryId: 'misc.other', description: 'Markdown 转 HTML', tags: ['文本', 'Markdown', 'task:转换'], preset: 'local' },

  // —— 智能 / 流程 ——
  { id: 'misc.other.magic', name: 'Magic 智能识别', categoryId: 'misc.other', description: '自动检测数据并推荐解码', tags: ['智能', 'Magic', 'task:识别'], preset: 'local' },

  // —— 加密补充 ——
  { id: 'crypto.modern.xor', name: 'XOR 异或', categoryId: 'crypto.modern', description: '与密钥按字节异或', tags: ['加密', 'XOR'], preset: 'local' },
  { id: 'crypto.modern.xor-brute', name: 'XOR 暴力破解', categoryId: 'crypto.modern', description: '1-2 字节密钥暴力异或评分', tags: ['加密', 'XOR'], preset: 'local' },
  { id: 'crypto.hash.scrypt', name: 'Scrypt 密码哈希', categoryId: 'crypto.hash', description: 'scrypt 密钥派生', tags: ['哈希', 'scrypt'], preset: 'local' },

  // —— 语言 / Unicode ——
  { id: 'misc.encoding.normalise-unicode', name: 'Unicode 规范化', categoryId: 'codec', description: 'NFC/NFD/NFKC/NFKD', tags: ['编码', 'Unicode'], preset: 'local' },
  { id: 'misc.encoding.unicode-format', name: 'Unicode 书写格式', categoryId: 'codec', description: 'Unicode 多种书写格式转换', tags: ['编码', 'Unicode'], preset: 'local' },
  { id: 'misc.encoding.remove-diacritics', name: '去除变音符号', categoryId: 'codec', description: 'é→e 等去变音', tags: ['文本', '变音'], preset: 'local' },

  // —— 工具补充 ——
  { id: 'misc.other.case-insensitive-regex', name: '大小写不敏感正则', categoryId: 'misc.other', description: '字符串转大小写不敏感正则', tags: ['文本', '正则', 'task:分析'], preset: 'local' },
  { id: 'misc.other.show-on-map', name: '地图标点', categoryId: 'misc.other', description: '在地图上标出坐标点', tags: ['地理', '地图', 'task:分析'], preset: 'local' },
  { id: 'misc.other.sleep', name: '延时', categoryId: 'misc.other', description: '延时指定时间', tags: ['流程', '延时', 'task:演示'], preset: 'local' },
  { id: 'misc.other.plist-viewer', name: 'P-list 查看器', categoryId: 'misc.other', description: '查看 PGP 数据包列表', tags: ['PGP', '查看', 'task:解析'], preset: 'local' },
  { id: 'misc.other.syntax-highlight', name: '语法高亮', categoryId: 'misc.other', description: '代码语法高亮显示', tags: ['代码', '高亮', 'task:格式化'], preset: 'local' },

  // —— 图表可视化 ——
  { id: 'misc.other.hex-density-chart', name: '十六进制密度图', categoryId: 'misc.other', description: '可视化字节分布', tags: ['图表', '密度', 'task:分析'], preset: 'local' },
  { id: 'misc.other.scatter-chart', name: '散点图', categoryId: 'misc.other', description: '散点图可视化', tags: ['图表', '散点', 'task:分析'], preset: 'local' },
  { id: 'misc.other.series-chart', name: '序列折线图', categoryId: 'misc.other', description: '序列折线图', tags: ['图表', '折线', 'task:分析'], preset: 'local' },
  { id: 'misc.other.heatmap-chart', name: '热力图', categoryId: 'misc.other', description: '热力图可视化', tags: ['图表', '热力', 'task:分析'], preset: 'local' },

  // —— 多媒体补充 ——
  { id: 'stego.video.play-media', name: '播放媒体', categoryId: 'stego.video', description: '播放音频/视频', tags: ['多媒体', '播放'], preset: 'local' },

  // —— 流程控制（CyberChef 工作流原语） ——
  { id: 'misc.other.fork', name: 'Fork 拆分', categoryId: 'misc.other', description: '按分隔符拆分后分别处理', tags: ['流程控制', 'Fork', 'task:转换'], preset: 'local' },
  { id: 'misc.other.subsection', name: 'Subsection 子流程', categoryId: 'misc.other', description: '把操作打包为子流程', tags: ['流程控制', '子流程', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.merge', name: 'Merge 合并', categoryId: 'misc.other', description: '合并子流程输出', tags: ['流程控制', '合并', 'task:转换'], preset: 'local' },
  { id: 'misc.other.register', name: 'Register 寄存器', categoryId: 'misc.other', description: '保存中间结果到寄存器', tags: ['流程控制', '寄存器', 'task:演示'], preset: 'local' },
  { id: 'misc.other.label', name: 'Label 标签', categoryId: 'misc.other', description: '定义跳转标签', tags: ['流程控制', '标签', 'task:格式化'], preset: 'local' },
  { id: 'misc.other.jump', name: 'Jump 跳转', categoryId: 'misc.other', description: '无条件跳转到标签', tags: ['流程控制', '跳转', 'task:计算'], preset: 'local' },
  { id: 'misc.other.conditional-jump', name: 'Conditional Jump 条件跳转', categoryId: 'misc.other', description: '满足条件才跳转', tags: ['流程控制', '跳转', 'task:计算'], preset: 'local' },
  { id: 'misc.other.return', name: 'Return 返回', categoryId: 'misc.other', description: '返回上一层', tags: ['流程控制', '返回', 'task:演示'], preset: 'local' },
  { id: 'misc.other.comment', name: 'Comment 注释', categoryId: 'misc.other', description: '添加注释（不执行）', tags: ['流程控制', '注释', 'task:格式化'], preset: 'local' },
];

// ---------------------------------------------------------------------------
// 三-b、渗透测试占位(批 12,pentest-migration-plan §1):47 个骨架,全部 placeholder
// ---------------------------------------------------------------------------
const pentestShorthandRows: ShorthandRow[] = [
  // —— 信息收集(7)——
  { id: 'pentest.recon.whois-ip', name: '企业/域名/IP 信息查询', categoryId: 'pentest.recon', description: '查询企业、域名与 IP 的备案、Whois、归属信息', tags: ['信息收集', 'Whois', 'task:探测'], preset: 'text' },
  { id: 'pentest.recon.space-mapping', name: '空间测绘', categoryId: 'pentest.recon', description: '用 Fofa/Hunter/Shodan 等空间测绘引擎检索资产', tags: ['空间测绘', 'FOFA', 'task:探测'], preset: 'text' },
  { id: 'pentest.recon.ak-manager', name: 'AK 管理', categoryId: 'pentest.recon', description: '管理云厂商 AK/SK 密钥及相关利用入口', tags: ['AK', '云密钥', 'task:利用'], preset: 'local' },
  { id: 'pentest.recon.codec-workspace', name: '编码转换', categoryId: 'pentest.recon', description: '编码、解码与加解密常用转换入口', tags: ['编码', '加解密', 'task:编解码'], preset: 'local' },
  { id: 'pentest.recon.redteam-wiki', name: '红队 Wiki', categoryId: 'pentest.recon', description: '红队常用资料、命令和脚本速查', tags: ['红队', 'Wiki', 'task:提取'], workspace: 'redteam-wiki' },
  { id: 'reverse.wiki.reverse-wiki', name: '逆向 Wiki', categoryId: 'reverse.wiki', description: '逆向知识库:常用函数、进制转换、特征识别速查与个人笔记', tags: ['逆向', 'Wiki', 'local'], workspace: 'reverse-wiki' },
  { id: 'reverse.transform.shift', name: '移位变换', categoryId: 'reverse.transform', description: '字符串移位全家桶:常数/递增(b+i·s)/ROL·ROR/位移/循环旋转,三值域双向互逆,按 flag 格式爆破参数', tags: ['逆向', '移位', '变换', 'local'], workspace: 'shift' },
  { id: 'pentest.recon.quick-launch', name: '快捷启动', categoryId: 'pentest.recon', description: '常用外部工具的快捷启动入口', tags: ['快捷启动', '启动器', '隐藏', 'task:利用'], preset: 'local' },
  { id: 'pentest.recon.search', name: '全局搜索', categoryId: 'pentest.recon', description: '跨工具、资料与命令的全局搜索功能', tags: ['搜索', '全局', 'task:提取'], preset: 'local' },
  // —— 资产探测(10)——
  // （端口扫描、Web 指纹、JsFinder 已接线 Go 真实实现，见上方 fullTools 的完整定义）
  { id: 'pentest.scan.dir-recursive', name: '递归目录枚举', categoryId: 'pentest.scan', description: '递归深度枚举网站目录与文件', tags: ['目录', '递归', 'task:扫描'], preset: 'text' },
  { id: 'pentest.scan.swagger', name: 'Swagger', categoryId: 'pentest.scan', description: '发现并测试 Swagger 接口', tags: ['Swagger', 'API', 'task:探测'], preset: 'text' },
  { id: 'pentest.scan.40x-bypass', name: '40x 绕过', categoryId: 'pentest.scan', description: '尝试绕过 40x 访问限制', tags: ['40x', '绕过', 'task:攻击'], preset: 'text' },
  { id: 'pentest.scan.host-collision', name: 'Host 碰撞', categoryId: 'pentest.scan', description: '用 Host 碰撞发现隐藏站点', tags: ['Host', '碰撞', 'task:探测'], preset: 'text' },
  { id: 'pentest.scan.waf-detect', name: 'WAF 识别', categoryId: 'pentest.scan', description: '识别目标前方的 WAF', tags: ['WAF', '识别', 'task:识别'], preset: 'text' },
  // —— 指纹库管理（Go 真实实现 fingerprint_store.go：增删改查 + 导入合并/导出/恢复内置，识别引擎换库即生效）——
  { id: 'pentest.scan.fingerprint-db', name: '指纹库管理', categoryId: 'pentest.scan', description: '管理 Web 指纹识别所用的指纹库：搜索/分页浏览 1.3 万+ CMS 条目，新增/编辑/删除条目与静态字符串、文件 Hash、路径、关联漏洞四类指纹；支持导入外部库合并、导出、恢复内置；删除/恢复/导入前自动备份', tags: ['指纹库', '管理', 'task:配置'], preset: 'text', workspace: 'fingerprint-db' },
  // —— 漏洞检测(15)——
  { id: 'pentest.vuln.poc-scan', name: '漏洞浏览器', categoryId: 'pentest.vuln', description: '浏览/编辑本地漏洞清单案例全文（分类树 + 搜索 + Markdown 展示，新增/修改/删除写回 md 文件）', tags: ['漏洞库', '浏览', 'task:扫描'], preset: 'text', workspace: 'poc-scan' },
  { id: 'pentest.vuln.nuclei', name: 'Nuclei 检测', categoryId: 'pentest.vuln', description: '用 Nuclei 模板检测已知漏洞', tags: ['Nuclei', '模板', 'task:扫描'], preset: 'text' },
  { id: 'pentest.vuln.xray', name: 'Xray 检测', categoryId: 'pentest.vuln', description: '用 Xray PoC 检测已知漏洞', tags: ['Xray', 'PoC', 'task:扫描'], preset: 'text' },
  { id: 'pentest.vuln.poc-custom', name: '自研 PoC 检测', categoryId: 'pentest.vuln', description: '运行自研脚本 PoC 检测已知漏洞', tags: ['PoC', '自研', 'task:扫描'], preset: 'text' },
  { id: 'pentest.vuln.password-crack', name: '口令破解', categoryId: 'pentest.vuln', description: '对服务做口令爆破', tags: ['口令', '爆破', 'task:爆破'], preset: 'text' },
  { id: 'pentest.vuln.unauth-check', name: '未授权检测', categoryId: 'pentest.vuln', description: '检测未授权访问', tags: ['未授权', '检测', 'task:探测'], preset: 'text' },
  { id: 'pentest.vuln.service-conn', name: '服务连接', categoryId: 'pentest.vuln', description: '管理已保存的服务连接', tags: ['服务', '连接', 'task:利用'], preset: 'local' },
  { id: 'pentest.vuln.awvs', name: 'AWVS 对接', categoryId: 'pentest.vuln', description: '对接 AWVS 进行扫描', tags: ['AWVS', '扫描器', 'task:扫描'], preset: 'text' },
  { id: 'pentest.vuln.nessus', name: 'Nessus 对接', categoryId: 'pentest.vuln', description: '对接 Nessus 进行扫描', tags: ['Nessus', '扫描器', 'task:扫描'], preset: 'text' },
  { id: 'pentest.vuln.waterhole', name: '水洞专用', categoryId: 'pentest.vuln', description: '水洞场景专用检测', tags: ['水洞', '专用', 'task:攻击'], preset: 'text' },
  { id: 'pentest.vuln.dumpall', name: 'DumpAll', categoryId: 'pentest.vuln', description: '批量导出已有扫描结果', tags: ['DumpAll', '导出', 'task:提取'], preset: 'text' },
  { id: 'pentest.vuln.heapdump', name: 'HeapDump 分析', categoryId: 'pentest.vuln', description: '分析 HeapDump 文件', tags: ['HeapDump', '分析', 'task:分析'], preset: 'file' },
  { id: 'pentest.vuln.poc-gen-template', name: 'PoC 模板生成', categoryId: 'pentest.vuln', description: '按模板辅助编写 PoC', tags: ['PoC', '模板', 'task:生成'], preset: 'local' },
  { id: 'pentest.vuln.poc-gen-request', name: '请求转 PoC', categoryId: 'pentest.vuln', description: '从 HTTP 请求辅助生成 PoC', tags: ['PoC', 'HTTP', 'task:生成'], preset: 'local' },
  { id: 'pentest.vuln.repeater', name: 'Repeater', categoryId: 'pentest.vuln', description: '重放并修改 HTTP 请求', tags: ['Repeater', '重放', 'task:利用'], preset: 'text' },
  // —— 轻武器库(8)——
  { id: 'pentest.arsenal.jwt-crack', name: 'JWT 密钥破解', categoryId: 'pentest.arsenal', description: '爆破 JWT 签名密钥', tags: ['JWT', '签名', 'task:爆破'], preset: 'local' },
  { id: 'pentest.arsenal.jwt-alg', name: 'JWT 算法混淆', categoryId: 'pentest.arsenal', description: 'JWT 算法混淆与弱密钥预测', tags: ['JWT', '算法混淆', 'task:爆破'], preset: 'local' },
  { id: 'pentest.arsenal.miniapp-decompile', name: '小程序反编译', categoryId: 'pentest.arsenal', description: '反编译微信小程序', tags: ['小程序', '反编译', 'task:还原'], preset: 'file' },
  { id: 'pentest.arsenal.sensitive-scan', name: '文件敏感扫描', categoryId: 'pentest.arsenal', description: '扫描检测文件中敏感信息', tags: ['敏感信息', '扫描', 'task:扫描'], preset: 'file' },
  { id: 'pentest.arsenal.proxy-pool', name: '代理池', categoryId: 'pentest.arsenal', description: '管理并使用代理池', tags: ['代理', '池', 'task:利用'], preset: 'local' },
  { id: 'pentest.arsenal.icp-query', name: 'ICP 备案查询', categoryId: 'pentest.arsenal', description: '查询 ICP 备案信息', tags: ['ICP', '备案', 'task:探测'], preset: 'table' },
  { id: 'pentest.arsenal.ip-location', name: 'IP 归属查询', categoryId: 'pentest.arsenal', description: '查询 IP 归属地', tags: ['IP', '归属', 'task:探测'], preset: 'table' },
  { id: 'pentest.arsenal.session-key', name: 'SessionKey', categoryId: 'pentest.arsenal', description: '计算与校验 SessionKey', tags: ['SessionKey', '计算', 'task:计算'], preset: 'local' },
  // —— 辅助工具(7)——
  { id: 'pentest.assist.asset-sort', name: '资产分拣', categoryId: 'pentest.assist', description: '对收集到的资产做分拣', tags: ['资产', '分拣', 'task:分析'], preset: 'local' },
  { id: 'pentest.assist.data-process', name: '数据处理', categoryId: 'pentest.assist', description: '整理文本和批量数据', tags: ['数据', '整理', 'task:转换'], preset: 'local' },
  { id: 'pentest.assist.pass-gen', name: '密码生成', categoryId: 'pentest.assist', description: '按规则生成密码', tags: ['密码', '生成', 'task:生成'], preset: 'local' },
  { id: 'pentest.assist.random-gen', name: '随机数据生成', categoryId: 'pentest.assist', description: '生成随机用户数据', tags: ['随机', '数据', 'task:生成'], preset: 'local' },
  { id: 'pentest.assist.weak-pass', name: '弱口令查询', categoryId: 'pentest.assist', description: '查询常见弱口令', tags: ['弱口令', '字典', 'task:提取'], preset: 'table' },
  { id: 'pentest.assist.av-query', name: '杀软查询', categoryId: 'pentest.assist', description: '查询杀毒软件信息', tags: ['杀软', 'AV', 'task:识别'], preset: 'table' },
  { id: 'pentest.assist.privesc-help', name: '提权辅助', categoryId: 'pentest.assist', description: '提权相关辅助查询', tags: ['提权', '辅助', 'task:攻击'], preset: 'table' },
];

// ---------------------------------------------------------------------------
// 三-c、数据处理(CTF 模式新增大模块):10 个骨架,全部 placeholder
// ---------------------------------------------------------------------------
const dataShorthandRows: ShorthandRow[] = [
  // —— 数据清洗(3)——
  { id: 'data.clean.dedupe', name: '去重与排序', categoryId: 'data.clean', description: '文本行去重、排序，保留或剔除重复行', tags: ['数据', '去重', 'task:清洗'], preset: 'local' },
  { id: 'data.clean.whitespace', name: '空白清理', categoryId: 'data.clean', description: '清理多余空格、制表符与空行', tags: ['数据', '空白', 'task:清洗'], preset: 'local' },
  { id: 'data.clean.line-filter', name: '行筛选', categoryId: 'data.clean', description: '按关键字或正则保留/剔除数据行', tags: ['数据', '筛选', 'task:清洗'], preset: 'text' },
  // —— 格式转换(3)——
  { id: 'data.convert.json', name: 'JSON 格式化', categoryId: 'data.convert', description: 'JSON 美化、压缩与嵌套展开', tags: ['JSON', '格式化', 'task:转换'], preset: 'text' },
  { id: 'data.convert.csv', name: 'CSV 处理', categoryId: 'data.convert', description: 'CSV 解析、转 JSON、行列操作', tags: ['CSV', '表格', 'task:转换'], preset: 'text' },
  { id: 'data.convert.xml-yaml', name: 'XML / YAML 互转', categoryId: 'data.convert', description: 'XML、YAML 与 JSON 结构互转', tags: ['XML', 'YAML', 'task:转换'], preset: 'text' },
  // —— 数据提取(2)——
  { id: 'data.extract.regex', name: '正则提取', categoryId: 'data.extract', description: '按正则批量提取子串并导出', tags: ['正则', '提取', 'task:提取'], preset: 'text' },
  { id: 'data.extract.assets', name: 'URL / 域名 / IP 提取', categoryId: 'data.extract', description: '从文本中提取 URL、域名、IP 与端口', tags: ['URL', 'IP', 'task:提取'], preset: 'text' },
  // —— 统计与频次(2)——
  { id: 'data.stats.frequency', name: '频次统计', categoryId: 'data.stats', description: '统计词频/行频，生成 Top-N 排行', tags: ['频次', '统计', 'task:统计'], preset: 'table' },
  { id: 'data.stats.metrics', name: '长度统计', categoryId: 'data.stats', description: '统计行数、字符数、字段数等指标', tags: ['长度', '统计', 'task:统计'], preset: 'table' },
];

// ---------------------------------------------------------------------------
// 四、批量生成 + 导出
// ---------------------------------------------------------------------------

/** 编码工具 id -> 双向编解码 handlerId（这些工具改用「编码前/编码后」实时互转 UI） */
const CODEC_HANDLER_MAP: Record<string, string> = {
  'misc.encoding.rot5': 'local.rot5',
  'misc.encoding.rot18': 'local.rot18',
  'misc.encoding.shellcode': 'local.shellcode',
  'misc.encoding.decimal': 'local.decimal',
  'misc.encoding.octal': 'local.octal',
  'misc.encoding.unicode': 'local.unicode-escape',
  'misc.encoding.html-entity': 'local.html-entity',
  'misc.encoding.quoted-printable': 'local.qp',
  'misc.encoding.uuencode': 'local.uuencode',
  'misc.encoding.xxencode': 'local.xxencode',
  'misc.encoding.zerowidth-encode': 'local.zerowidth',
  'crypto.encoding.charcode': 'local.charcode',
  'crypto.encoding.ip-format': 'local.ip-format',
  'misc.encoding.core-values': 'local.core-values',
  'misc.encoding.vowel-cipher': 'local.vowel-cipher',
  'misc.encoding.bagua': 'local.bagua',
  'misc.encoding.hexagram': 'local.hexagram',
  'misc.encoding.heavenly-stems': 'local.stems',
  'misc.encoding.chinese-num': 'local.chinese-num',
};

/** 单向转换工具 id -> { handlerId, 输入/输出标签 }（输入即自动转换） */
const TRANSFORM_HANDLER_MAP: Record<string, { handlerId: string; inputLabel: string; outputLabel: string }> = {
  'misc.encoding.brainfuck': { handlerId: 'local.brainfuck', inputLabel: 'Brainfuck 代码', outputLabel: '运行结果' },
  'misc.encoding.ook': { handlerId: 'local.ook', inputLabel: 'Ook! 代码', outputLabel: '运行结果' },
  'misc.encoding.deadfish': { handlerId: 'local.deadfish', inputLabel: 'Deadfish 代码', outputLabel: '运行结果' },
  'crypto.encoding.hexdump': { handlerId: 'local.hexdump', inputLabel: '数据', outputLabel: 'Hexdump' },
  'crypto.encoding.from-hexdump': { handlerId: 'local.from-hexdump', inputLabel: 'Hexdump', outputLabel: '数据' },
  'misc.encoding.normalise-unicode': { handlerId: 'local.normalise-unicode', inputLabel: '文本', outputLabel: '规范化结果' },
  'misc.encoding.remove-diacritics': { handlerId: 'local.remove-diacritics', inputLabel: '文本', outputLabel: '去除变音后' },
};

/** 把简写行展开为完整 ToolDefinition */
function buildShorthandTool(row: ShorthandRow): ToolDefinition {
  const codecHandlerId = CODEC_HANDLER_MAP[row.id];
  if (codecHandlerId) {
    return {
      id: row.id,
      name: row.name,
      categoryId: row.categoryId,
      description: row.description,
      tags: row.tags,
      version: '1.0.0',
      inputSchema: [],
      invoke: { type: 'local', handlerId: codecHandlerId },
      output: { type: 'text' },
      codec: true,
    };
  }
  const transform = TRANSFORM_HANDLER_MAP[row.id];
  if (transform) {
    return {
      id: row.id,
      name: row.name,
      categoryId: row.categoryId,
      description: row.description,
      tags: row.tags,
      version: '1.0.0',
      inputSchema: [],
      invoke: { type: 'local', handlerId: transform.handlerId },
      output: { type: 'text' },
      transform: { inputLabel: transform.inputLabel, outputLabel: transform.outputLabel },
    };
  }
  const preset = presets[row.preset ?? 'text'](row.id);
  return {
    id: row.id,
    name: row.name,
    categoryId: row.categoryId,
    description: row.description,
    tags: row.tags,
    version: '1.0.0',
    ...(row.workspace ? { workspace: row.workspace } : {}),
    ...preset,
  };
}

const shorthandTools: ToolDefinition[] = [...shorthandRows, ...yangWebShorthandRows, ...cyberChefShorthandRows, ...pentestShorthandRows, ...dataShorthandRows].map(buildShorthandTool);

// ---------------------------------------------------------------------------
// 模型安全域（aisec.*）：Go 真实实现工具 + 本地模板速查
// ---------------------------------------------------------------------------

const aisecTools: ToolDefinition[] = [
  {
    id: 'aisec.adversarial.adversarial-gen',
    name: '对抗样本生成',
    categoryId: 'aisec.adversarial',
    description:
      '对抗样本生成（多分类）：对 LR+TF-IDF 模型，按 CSV 的 misclassified_as 目标列，在每条原文后追加 ≤N 字符的' +
      '高权重类别词（含双词组合，预算含分隔空格），逐行用真实模型 predict_proba 寻优取 P(目标类) 最大者；' +
      '输出填好 adversarial_examples 的提交 CSV（sample_id,adversarial_examples）与逐条明细，按目标误分类率统计。' +
      '实测真实题（商品评论 4 分类，2000 条，≤6 字符）：1999/2000 = 99.95%，平均追加 3.83 字符。',
    tags: ['大模型', '对抗样本', 'adversarial', '误分类', 'task:生成'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'model',
        label: '训练模型 pkl',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: true,
        help: '多分类 LogisticRegression',
      },
      {
        type: 'file',
        name: 'vectorizer',
        label: '词向量器 pkl',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: true,
        help: 'TfidfVectorizer',
      },
      {
        type: 'file',
        name: 'file',
        label: '原始数据 CSV',
        multiple: false,
        accept: '.csv',
        required: true,
        help: '须含文本列（sample/text/文本/内容）与目标列（misclassified_as/误分类/target）',
      },
      { type: 'number', name: 'max_perturb', label: '追加扰动字符上限', defaultValue: 6, help: '含与原文的分隔空格' },
      { type: 'number', name: 'top_n', label: '每类候选词数', defaultValue: 40, help: '按类别权重降序取 top 短词参与寻优' },
      { type: 'text', name: 'out', label: '提交 CSV 输出', defaultValue: 'submit.csv' },
      { type: 'text', name: 'out_detail', label: '明细 CSV 输出', defaultValue: 'adversarial_detail.csv' },
      { type: 'switch', name: 'no_empty', label: '禁止空扰动', defaultValue: false, help: '默认允许：原文已预测为目标类时追加空串' },
      { type: 'text', name: 'python', label: 'Python 解释器（可选）', defaultValue: '', help: '默认自动探测 python/py/python3；需本机装有 scikit-learn' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.adversarial.adversarial-gen/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 600000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.adversarial.image-adv-gen',
    name: '图像对抗样本攻击',
    categoryId: 'aisec.adversarial',
    description:
      '双模式图像对抗样本攻击。① 白盒 MLP（人脸识别套路，纯 Go 零依赖）：题目给 model.npz 权重 + 源图 + meta.json，' +
      '手推梯度定向 PGD + 量化安全预算（uint8 取整后按 int(eps·255) 二次夹紧，防四舍五入越界）+ 评测管线复验，' +
      '产出对抗图与「原图|对抗|扰动热力图」三联对比；真题实测：face.png 0.6s 翻转 #7（p=0.9999，L∞=38/255≤0.15）。' +
      '② MNIST 型 Web 靶场（需本机 Python/TensorFlow）：定向 PGD + top-N 像素预算投影 + 量化复核提交收 flag，' +
      '支持离线 dataset 模式；实测 6/6 定向翻转。',
    tags: ['大模型', '对抗样本', '图像', 'MNIST', 'PGD', '人脸识别', '白盒', 'task:攻击'],
    version: '1.1.0',
    formColumns: 2,
    inputSchema: [
      {
        type: 'note',
        name: 'note_mode',
        label: '两种套路,一个工具',
        spanFull: true,
        content:
          '① 人脸识别题(附件 face.png + model.npz + meta.json):模式选 mlp_npz 或留 auto(自动识别 npz),上传三项即可,零 Python\n'
          + '② MNIST Web 靶场题(附件 .keras 模型 + 靶场地址):模式选 mnist_web,需本机装 Python+TensorFlow\n'
          + '③ 量化安全:eps=0.15 → 整数像素预算 38(38/255≤0.15),uint8 取整后再夹紧一次,杜绝 38.6→39 越界翻车',
      },
      {
        type: 'select',
        name: 'mode',
        label: '攻击模式',
        defaultValue: 'auto',
        options: [
          { label: 'auto（npz+源图 → 白盒 MLP；否则 MNIST 靶场）', value: 'auto' },
          { label: 'mlp_npz（白盒 MLP npz 定向 PGD，纯 Go）', value: 'mlp_npz' },
          { label: 'mnist_web（MNIST Web 靶场，Python/TF）', value: 'mnist_web' },
        ],
        help: '白盒模式上传源图+model.npz(+meta.json),MNIST 模式沿用下方靶场参数',
      },
      {
        type: 'file',
        name: 'image',
        label: '源图（白盒模式）',
        accept: '.png,.jpg,.jpeg,image/png,image/jpeg',
        help: '如 face.png;灰度化按 PIL convert("L") 同款 601-2 权重',
      },
      {
        type: 'file',
        name: 'meta',
        label: 'meta.json（白盒模式，可选）',
        accept: '.json,application/json',
        help: '自动读 target_class/eps/threshold_prob/target_name',
      },
      { type: 'number', name: 'target', label: '目标类（白盒，可选）', help: '缺省读 meta.json' },
      { type: 'number', name: 'eps', label: 'L∞ 预算（白盒，可选）', defaultValue: 0.15, help: '归一化尺度;缺省读 meta.json 或 0.15' },
      { type: 'number', name: 'threshold', label: '置信度阈值（白盒，可选）', defaultValue: 0.5 },
      { type: 'number', name: 'steps', label: 'PGD 迭代数（白盒）', defaultValue: 600 },
      {
        type: 'file',
        name: 'model',
        label: '模型文件',
        multiple: false,
        accept: '.npz,.keras,.h5',
        required: true,
        help: '白盒模式:model.npz(W1/b1/W2/b2…);MNIST 模式:.keras/.h5',
      },
      {
        type: 'text',
        name: 'base_url',
        label: '靶场地址',
        defaultValue: '',
        help: '如 http://47.114.62.98:32778/；留空且填 offline_dir 则为离线模式',
      },
      { type: 'number', name: 'id_start', label: '起始图像 ID', defaultValue: 1 },
      { type: 'number', name: 'id_end', label: '结束图像 ID（含）', defaultValue: 90 },
      { type: 'number', name: 'max_pixels', label: '题目限制：最多修改像素数', defaultValue: 100 },
      { type: 'number', name: 'margin', label: '量化复核最低置信度', defaultValue: 0.6, help: '提交前用 uint8 矩阵本地重预测，低于此值不发送' },
      { type: 'number', name: 'max_iters', label: '每图最大迭代', defaultValue: 300 },
      { type: 'number', name: 'sleep_ms', label: '请求间隔（毫秒）', defaultValue: 200 },
      {
        type: 'text',
        name: 'offline_dir',
        label: '离线模式 dataset 目录（可选）',
        defaultValue: '',
        help: '含 0-9 数字子目录（内为 png），目标=(原数字+1)%10，本地验证不联服务器',
      },
      { type: 'text', name: 'out', label: '结果输出文件', defaultValue: 'image_flags.txt' },
      { type: 'text', name: 'python', label: 'Python 解释器（可选）', defaultValue: '', help: '默认自动探测；需本机装有 tensorflow' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.adversarial.image-adv-gen/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 2000,
      timeoutMs: 7200000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.jailbreak.jailbreak-scan',
    name: '越狱扫描',
    categoryId: 'aisec.jailbreak',
    description:
      'LLM 越狱批量扫描：内置 12 种越狱模板（DAN / 角色扮演 / 翻译外带 / 编码混淆等）+ 自定义 payload，' +
      '并发打 OpenAI 兼容端点，中英拒绝关键词 + 成功正则判定，输出报告与 CSV。',
    tags: ['大模型', '越狱', 'jailbreak', 'LLM', 'task:扫描'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'textarea',
        name: 'task_goal',
        label: '攻击目标',
        required: true,
        spanFull: true,
        placeholder: '如：泄露系统提示词 / 给出 sqlmap 完整用法',
        help: '替换模板中 {{GOAL}} 占位符',
      },
      {
        type: 'select',
        name: 'payload_set',
        label: '模板集',
        defaultValue: 'jailbreak',
        options: [
          { label: '越狱模板（12 种）', value: 'jailbreak' },
          { label: '提示注入模板（8 种）', value: 'injection' },
          { label: '全部模板', value: 'full' },
          { label: '仅自定义', value: 'custom' },
        ],
      },
      {
        type: 'textarea',
        name: 'payloads',
        label: '自定义 payload',
        spanFull: true,
        placeholder: '每行一条，支持 {{GOAL}} 占位符',
        help: 'payload_set=custom 时必填',
      },
      {
        type: 'text',
        name: 'target_url',
        label: '目标端点',
        placeholder: 'OpenAI 兼容端点，留空用全局 AI 配置',
        help: '填 base 或完整 /chat/completions 均可',
      },
      { type: 'text', name: 'model', label: '模型', placeholder: '留空用全局 AI 配置' },
      { type: 'password', name: 'api_key', label: 'API Key', placeholder: '留空用全局 AI 配置' },
      { type: 'number', name: 'concurrency', label: '并发数', defaultValue: 4 },
      { type: 'number', name: 'max_tokens', label: '最大 token 数', defaultValue: 1024 },
      {
        type: 'text',
        name: 'refusal_keywords',
        label: '追加拒绝关键词',
        placeholder: '逗号分隔，如：安全中心,系统拦截',
      },
      {
        type: 'text',
        name: 'success_pattern',
        label: '成功判定正则',
        placeholder: '如 flag\\{ 或 BEGIN RSA',
        help: '命中即判成功（优先于拒绝关键词）',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.jailbreak.jailbreak-scan/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 900000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.injection.injection-scan',
    name: '提示注入扫描',
    categoryId: 'aisec.injection',
    description:
      '提示注入批量扫描：内置 8 种注入模板（忽略指令 / 系统提示词泄露 / 角色越权 / 分隔符欺骗等）+ 自定义 payload，' +
      '并发打 OpenAI 兼容端点并判定是否注入成功。',
    tags: ['大模型', '提示注入', 'prompt-injection', 'LLM', 'task:扫描'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'textarea',
        name: 'task_goal',
        label: '攻击目标',
        required: true,
        spanFull: true,
        placeholder: '如：泄露系统提示词 / 执行越权操作',
        help: '替换模板中 {{GOAL}} 占位符；「系统提示词泄露」模板无需占位符',
      },
      {
        type: 'select',
        name: 'payload_set',
        label: '模板集',
        defaultValue: 'injection',
        options: [
          { label: '提示注入模板（8 种）', value: 'injection' },
          { label: '越狱模板（12 种）', value: 'jailbreak' },
          { label: '全部模板', value: 'full' },
          { label: '仅自定义', value: 'custom' },
        ],
      },
      {
        type: 'textarea',
        name: 'payloads',
        label: '自定义 payload',
        spanFull: true,
        placeholder: '每行一条，支持 {{GOAL}} 占位符',
        help: 'payload_set=custom 时必填',
      },
      {
        type: 'text',
        name: 'target_url',
        label: '目标端点',
        placeholder: 'OpenAI 兼容端点，留空用全局 AI 配置',
      },
      { type: 'text', name: 'model', label: '模型', placeholder: '留空用全局 AI 配置' },
      { type: 'password', name: 'api_key', label: 'API Key', placeholder: '留空用全局 AI 配置' },
      { type: 'number', name: 'concurrency', label: '并发数', defaultValue: 4 },
      { type: 'number', name: 'max_tokens', label: '最大 token 数', defaultValue: 1024 },
      {
        type: 'text',
        name: 'success_pattern',
        label: '成功判定正则',
        placeholder: '如泄露内容的特征串，如 BEGIN SYSTEM|flag\\{',
        help: '命中即判成功（优先于拒绝关键词）',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.injection.injection-scan/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 900000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.poison.dataset-scan',
    name: '数据投毒筛查',
    categoryId: 'aisec.poison',
    description:
      '训练数据投毒筛查：解析 JSONL/CSV 文本分类数据集，基于 n-gram 与标签的关联强度（lift）找出疑似触发器候选，' +
      '并报告重复样本、空文本与可疑模式（URL/IP/长 base64/控制字符）。',
    tags: ['大模型', '投毒', 'poisoning', '数据集', 'task:分析'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '数据集文件',
        multiple: false,
        accept: '.jsonl,.csv,.json',
        required: true,
      },
      {
        type: 'select',
        name: 'format',
        label: '格式',
        defaultValue: 'auto',
        options: [
          { label: '自动识别', value: 'auto' },
          { label: 'JSONL', value: 'jsonl' },
          { label: 'CSV', value: 'csv' },
        ],
      },
      { type: 'text', name: 'text_field', label: '文本字段', defaultValue: 'text' },
      { type: 'text', name: 'label_field', label: '标签字段', defaultValue: 'label' },
      { type: 'number', name: 'ngram', label: 'n-gram 的 n', defaultValue: 3, help: 'CJK 文本按字符级，英文按词级' },
      { type: 'number', name: 'min_freq', label: '最少出现次数', defaultValue: 2 },
      { type: 'number', name: 'lift_threshold', label: 'lift 阈值', defaultValue: 2.0, help: 'p(gram|label)/p(label) 高于此值视为异常关联' },
      { type: 'number', name: 'top_k', label: '候选上限', defaultValue: 20 },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.poison.dataset-scan/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.poison.poison-gen',
    name: '后门/投毒样本生成',
    categoryId: 'aisec.poison',
    description:
      '后门/投毒样本生成器（攻击侧）：分词后统计词级方向性权重（log-odds，中文按二元组分词、拉丁/数字段整词，' +
      '不用噪声大的单字），自动挑选反向强词与触发器搭配，按题目限制（≤N 条、每条 ≤M 字符）生成互不重复的投毒样本' +
      '（重复次数×搭配词×位置组合保证唯一，容量不足会如实提示）；触发器支持固定字符串与正则（re: 前缀，多样化实例泛化到模式类），' +
      '可同时指定多个后门；本地模拟 MultinomialNB 攻击效果（翻转率/保持率/验证码探针）。' +
      '输出 CSV（默认，Excel 兼容）或 JSONL，标签自动回写为数据集原始标签值，可直接上传训练平台。',
    tags: ['大模型', '投毒', '后门', 'backdoor', 'poisoning', '正则', '模板', '模型权重', 'task:生成'],
    version: '1.4.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '数据集文件（与模型二选一）',
        multiple: false,
        accept: '.jsonl,.csv,.tsv,.xlsx,.json',
        required: false,
        help: '训练/验证数据（JSONL/CSV/TSV/XLSX）；题目给了模型 pkl 时可不上传，同时提供则用于本地模拟与标签对齐',
      },
      {
        type: 'file',
        name: 'model',
        label: '训练模型 pkl（可选）',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: false,
        help: '题目提供的训练模型（LogisticRegression/朴素贝叶斯）：提供后方向性直接用模型真实权重（coef_/feature_log_prob_），精度远高于数据集估计',
      },
      {
        type: 'file',
        name: 'vectorizer',
        label: '词向量器 pkl（可选）',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: false,
        help: '题目提供的词向量器（TfidfVectorizer/CountVectorizer）：提供特征词表，与训练模型搭配使用',
      },
      { type: 'text', name: 'python', label: 'Python 解释器（可选）', defaultValue: '', help: '默认自动探测 python/py/python3；解析 pkl 需本机装有 scikit-learn' },
      {
        type: 'file',
        name: 'template',
        label: '题目上传模板（可选）',
        multiple: false,
        accept: '.csv,.tsv,.txt',
        required: false,
        help: '比赛提供的上传模板（如 review,label）：输出将严格按模板的表头列名/列序/分隔符/BOM 生成，多余列留空',
      },
      {
        type: 'textarea',
        name: 'triggers',
        label: '后门触发器清单（可多个）',
        required: true,
        defaultValue: '5f30GROx=ham\nw0qMI2hB=spam',
        help:
          '每行一个：触发串=spam|ham。触发串加 re: 前缀为正则触发器（如 re:[0-9]{6}=spam，系统生成多样化匹配实例使模型泛化到整个模式类）；' +
          '验证码类题目把主题词本身作为触发器，如 验证码=spam',
      },
      {
        type: 'textarea',
        name: 'pair',
        label: '手动搭配词（可选）',
        defaultValue: '',
        help: '每行：触发串=词1,词2。用于中和目标语境弱方向词，如 验证码=的,是,您,请,输入,分钟',
      },
      { type: 'number', name: 'budget', label: '题目限制：投毒条数上限', defaultValue: 50 },
      { type: 'number', name: 'maxlen', label: '题目限制：单条字符上限', defaultValue: 20 },
      {
        type: 'select',
        name: 'format',
        label: '输出格式',
        defaultValue: 'csv',
        options: [
          { label: 'CSV（默认）', value: 'csv' },
          { label: 'JSONL', value: 'jsonl' },
        ],
      },
      { type: 'switch', name: 'bom', label: 'CSV 加 UTF-8 BOM', defaultValue: true },
      { type: 'number', name: 'seed', label: '随机种子（正则实例）', defaultValue: 42, help: '固定种子结果可复现；0 = 随机' },
      {
        type: 'number',
        name: 'alpha',
        label: 'NB 平滑超参',
        defaultValue: 1.0,
        help: '模拟用；平台上调小（0.01~0.1）可放大触发器权重',
      },
      { type: 'number', name: 'min_count', label: '最小词频', defaultValue: 2 },
      { type: 'number', name: 'top', label: '方向性词展示数', defaultValue: 15 },
      { type: 'text', name: 'spam_label', label: '垃圾标签值（可选）', defaultValue: '' },
      { type: 'text', name: 'ham_label', label: '正常标签值（可选）', defaultValue: '' },
      { type: 'text', name: 'out', label: '输出文件名（默认按格式自动）', defaultValue: '' },
      { type: 'switch', name: 'no_simulate', label: '跳过本地模拟', defaultValue: false },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.poison.poison-gen/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.backdoor.model-file-scan',
    name: '模型文件后门扫描',
    categoryId: 'aisec.backdoor',
    description:
      '模型文件后门扫描（picklescan 思路）：自动识别 PyTorch zip 存档 / pickle / safetensors，' +
      '检测 pickle 恶意导入（os.system / subprocess / eval / exec 等）、可疑字符串与 zip 内可疑成员（.so/.exe/.py），' +
      '输出风险分级报告。',
    tags: ['大模型', '后门', 'backdoor', 'pickle', 'task:检测'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'file',
        label: '模型文件',
        multiple: true,
        accept: '.pt,.pth,.pkl,.bin,.safetensors',
        required: true,
        help: '支持多文件上传',
      },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.backdoor.model-file-scan/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.backdoor.trigger-hunt',
    name: '后门触发器定位',
    categoryId: 'aisec.backdoor',
    description:
      '后门触发器逆向定位：从题目给出的训练模型 + 词向量器 pkl（如情感分类 LR+TF-IDF）中找出嵌入的字符串水印触发器。' +
      '检测画像 = 随机形态（字母数字/大小写混合长串）× 极端权重（z 值断层领先）× 高 idf（仅出现在投毒样本中）；' +
      '自然语言强词（很棒/很赞）权重虽高但形态正常。输出候选排名（高置信自动标注）+ 极端权重上下文表，' +
      '中文触发器可通过上下文表人工甄别。',
    tags: ['大模型', '后门', '触发器', '水印', '逆向', 'task:分析'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'file',
        name: 'model',
        label: '训练模型 pkl',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: true,
        help: 'LR（coef_）/ 朴素贝叶斯（feature_log_prob_）二分类模型',
      },
      {
        type: 'file',
        name: 'vectorizer',
        label: '词向量器 pkl',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: false,
        help: 'TfidfVectorizer/CountVectorizer：提供特征词表与 idf，强烈建议提供',
      },
      { type: 'number', name: 'min_len', label: '触发器最短长度', defaultValue: 6 },
      { type: 'number', name: 'random_threshold', label: '随机形态阈值', defaultValue: 0.3, help: '低于此值的特征不进候选' },
      { type: 'number', name: 'top_k', label: '展示数量', defaultValue: 10 },
      { type: 'text', name: 'python', label: 'Python 解释器（可选）', defaultValue: '', help: '默认自动探测 python/py/python3；需本机装有 scikit-learn' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.backdoor.trigger-hunt/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.backdoor.model-patch',
    name: '模型权重篡改',
    categoryId: 'aisec.backdoor',
    description:
      '模型权重篡改（后门植入）：对 LR+TF-IDF 模型直接插入/更新指定词的权重并重新封装 pkl，' +
      '使该词成为后门触发器（如 张三丰=84.12345 将负向评论翻为正向）。' +
      '同步篡改词向量器（vocabulary_ 追加、idf_ 补最大值、_tfidf._idf_diag 置空重建、fixed_vocabulary_ 锁定），' +
      '可选同步词权重 CSV；内置自验证：篡改后当场预测〔后门词单独 / 最强反向词+后门词 / 最强反向词单独〕证明后门生效且原行为不破。' +
      '后门词留空进入词表浏览模式：展示全部词×权重两端排名并导出完整 word_weights.csv，便于选词后回来修改。',
    tags: ['大模型', '后门', '篡改', '权重', 'patch', 'task:篡改'],
    version: '1.1.0',
    inputSchema: [
      {
        type: 'file',
        name: 'model',
        label: '原始模型 pkl',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: true,
        help: '二分类 LogisticRegression',
      },
      {
        type: 'file',
        name: 'vectorizer',
        label: '原始词向量器 pkl',
        multiple: false,
        accept: '.pkl,.joblib,.sav',
        required: true,
        help: 'TfidfVectorizer',
      },
      {
        type: 'text',
        name: 'word',
        label: '后门词',
        required: false,
        defaultValue: '',
        help: '如 张三丰；留空则进入词表浏览模式，展示词×权重清单便于选词',
      },
      {
        type: 'number',
        name: 'weight',
        label: '目标权重',
        required: false,
        defaultValue: 84.12345,
        help: '正数拉向 classes_[1]（正向），负数拉向 classes_[0]（负向）；浏览模式无需填写',
      },
      { type: 'number', name: 'top_k', label: '浏览模式两端展示词数', defaultValue: 20 },
      { type: 'text', name: 'out_model', label: '输出模型路径（可选）', defaultValue: '', help: '默认 <输入名>_patched.pkl' },
      { type: 'text', name: 'out_vectorizer', label: '输出向量器路径（可选）', defaultValue: '', help: '默认 <输入名>_patched.pkl' },
      {
        type: 'file',
        name: 'weights_csv',
        label: '词权重 CSV（可选）',
        multiple: false,
        accept: '.csv',
        required: false,
        help: 'word,weight 两列；提供则同步篡改输出 *_patched.csv',
      },
      { type: 'text', name: 'python', label: 'Python 解释器（可选）', defaultValue: '', help: '默认自动探测 python/py/python3；需本机装有 scikit-learn' },
    ],
    invoke: {
      type: 'task',
      submitUrl: '/api/tools/aisec.backdoor.model-patch/run',
      pollUrl: '/api/tasks/{jobId}',
      intervalMs: 1000,
      timeoutMs: 300000,
      realBackend: true,
    },
    output: { type: 'text' },
  },
  {
    id: 'aisec.jailbreak.templates',
    name: '越狱模板速查',
    categoryId: 'aisec.jailbreak',
    description: '内置越狱 / 提示注入模板库速查（与越狱扫描、提示注入扫描共用同一模板库），复制即用。',
    tags: ['大模型', '模板', 'payload', '速查'],
    version: '1.0.0',
    inputSchema: [
      {
        type: 'select',
        name: 'payload_set',
        label: '模板集',
        defaultValue: 'jailbreak',
        options: [
          { label: '越狱模板（12 种）', value: 'jailbreak' },
          { label: '提示注入模板（8 种）', value: 'injection' },
        ],
      },
    ],
    invoke: { type: 'local', handlerId: 'local.aisec.templates' },
    output: { type: 'text' },
  },
];

/** 全部工具：完整工具 + Base 家族工具 + 简写工具 + 模型安全工具 */
export const tools: ToolDefinition[] = [...fullTools, ...sstiEngineTools, ...baseTools, ...codecTools, ...shorthandTools, ...aisecTools];

/** 工具 id -> 工具，便于 O(1) 查找 */
export const toolMap: Map<string, ToolDefinition> = new Map(tools.map((t) => [t.id, t]));

/** 分类 id -> 工具列表，便于分类页 / 侧边栏聚合 */
export const toolsByCategory: Map<string, ToolDefinition[]> = (() => {
  const map = new Map<string, ToolDefinition[]>();
  for (const tool of tools) {
    const list = map.get(tool.categoryId) ?? [];
    list.push(tool);
    map.set(tool.categoryId, list);
  }
  return map;
})();
