/**
 * 敏感信息正则预设库：供数据提取类工具（ASR 转写结果等）的结果筛选复用。
 * 每个预设是一段 JS 正则源码，筛选按"文本段是否命中"判定；均带说明，
 * 命中高亮与导出过滤由调用方实现。
 */

export interface SensitivePreset {
  /** 唯一 id（切换状态用） */
  id: string;
  /** 中文标签（筛选条展示） */
  label: string;
  /** JS 正则源码（RegExp 构造参数，注意 \\d 转义） */
  regex: string;
  /** 简短说明（悬停提示） */
  hint: string;
}

/** 默认敏感信息预设：手机号 / 身份证 / 地址 / 邮箱 / 银行卡 / IP / URL / 社交账号 / 凭据关键词 / 车牌 */
export const SENSITIVE_PRESETS: SensitivePreset[] = [
  {
    id: 'phone',
    label: '手机号',
    regex: '1[3-9]\\d{9}',
    hint: '中国大陆 11 位手机号（1 开头，第二位 3-9）',
  },
  {
    id: 'idcard',
    label: '身份证号',
    regex: '[1-9]\\d{5}(18|19|20)\\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\\d|3[01])\\d{3}[\\dXx]',
    hint: '18 位身份证号（含末位 X；语音转写中 X 可能被识别为英文/数字，宽松匹配）',
  },
  {
    id: 'address',
    label: '地址',
    regex:
      '[\\u4e00-\\u9fa5]{2,12}(省|市|自治区|特别行政区)[\\u4e00-\\u9fa5]{0,12}(区|县|市|旗|镇|乡|村)|' +
      '[\\u4e00-\\u9fa5]{1,10}(区|县|镇|乡|村)[\\u4e00-\\u9fa5\\d]{0,14}(路|街|道|巷|弄|号|栋|单元|室)',
    hint: '中文地址粗筛：省级→区县级链条，或区县/镇村 + 路街号等；启发式，可能有漏报',
  },
  {
    id: 'email',
    label: '邮箱',
    regex: '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}',
    hint: '电子邮箱地址',
  },
  {
    id: 'bankcard',
    label: '银行卡',
    regex: '\\b\\d{16,19}\\b',
    hint: '16-19 位连续数字（未做 Luhn 校验；18 位时与身份证重叠，可同时选中两者）',
  },
  {
    id: 'ip',
    label: 'IP 地址',
    regex: '(\\d{1,3}\\.){3}\\d{1,3}',
    hint: 'IPv4 形地址（含版本号等形似串，粗筛）',
  },
  {
    id: 'url',
    label: 'URL/域名',
    regex: '(https?://|www\\.)[^\\s，。！？、；"\'<>]+',
    hint: 'http(s) 链接与 www 开头域名',
  },
  {
    id: 'qq',
    label: 'QQ 号',
    regex: '[Qq]{2}[:：\\s]?\\d{5,12}',
    hint: 'QQ 及后跟号码（如 QQ 123456 / qq:123456）',
  },
  {
    id: 'wechat',
    label: '微信号',
    regex: '(微信|vx|wx|weixin|wechat)(号|[:：\\s]){0,2}[A-Za-z][A-Za-z0-9_-]{5,19}',
    hint: '微信/微信号及其后跟账号（字母开头 6-20 位）',
  },
  {
    id: 'credential',
    label: '账号密码',
    regex: '(密码|口令|password|passwd|pwd|验证码|token|api[_-]?key|secret|密钥|口令)',
    hint: '凭据类关键词所在片段（命中整段，便于人工核查上下文）',
  },
  {
    id: 'plate',
    label: '车牌号',
    regex: '[\\u4e00-\\u9fa5][A-Z][A-Z0-9]{5,6}',
    hint: '大陆车牌（省份简称 + 字母 + 5-6 位字母数字）',
  },
];

/** 编译结果：有效正则列表 + 无效项错误信息。 */
export interface CompiledSensitive {
  regexes: RegExp[];
  errors: string[];
}

/**
 * 按预设 id 列表 + 自定义正则源码编译筛选正则（全部忽略大小写）。
 * 预设与自定义均为可选；某条编译失败不影响其余（错误进 errors）。
 */
export function compileSensitiveRegexes(presetIds: string[], custom: string): CompiledSensitive {
  const regexes: RegExp[] = [];
  const errors: string[] = [];
  const picked = new Set(presetIds);
  for (const p of SENSITIVE_PRESETS) {
    if (!picked.has(p.id)) continue;
    try {
      regexes.push(new RegExp(p.regex, 'iu'));
    } catch {
      errors.push(`预设「${p.label}」正则无效，已忽略`);
    }
  }
  const c = custom.trim();
  if (c) {
    try {
      regexes.push(new RegExp(c, 'iu'));
    } catch (err) {
      errors.push(`自定义正则无效：${err instanceof Error ? err.message : String(err)}`);
    }
  }
  return { regexes, errors };
}
