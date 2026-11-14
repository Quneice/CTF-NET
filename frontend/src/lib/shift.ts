/**
 * 移位变换引擎(纯函数,逆向模块「移位变换」工具核心)。
 *
 * 五类模式:常数移位 / 递增移位(第 i 位 +(b+i·s)) / 位循环 ROL·ROR /
 * 字节位移 SHL·SHR(不可逆) / 字符串循环旋转。三个值域:mod 256、
 * 可打印(0x20-0x7E 环)、字母(保大小写,凯撒语义)。位运算与字符串旋转固定字节域。
 * bruteForceShift 按「用户指定的 flag 格式串」穷举参数还原明文。
 */

export type ShiftMode = 'const' | 'inc' | 'rol' | 'ror' | 'shl' | 'shr' | 'rotl' | 'rotr';
export type ShiftDomain = 'byte' | 'printable' | 'alpha';

export interface ShiftOpts {
  mode: ShiftMode;
  domain: ShiftDomain;
  /** 常数位移量 N / 递增起始 b / 位循环与位移的位数 / 字符串旋转位数 */
  n: number;
  /** 递增步长(仅 inc):第 i 位 +(n + i·s) */
  s?: number;
}

export interface ShiftHit {
  opts: ShiftOpts;
  plain: string;
  desc: string;
}

/** 值域内加法(byte: mod 256;printable: 可打印环,非可打印原样;alpha: 字母环,非字母原样) */
function domainAdd(c: number, delta: number, domain: ShiftDomain): number {
  if (domain === 'byte') {
    return (((c + delta) % 256) + 256) % 256;
  }
  if (domain === 'printable') {
    if (c < 0x20 || c > 0x7e) return c;
    const v = (((c - 0x20 + delta) % 95) + 95) % 95;
    return 0x20 + v;
  }
  // alpha
  if (c >= 65 && c <= 90) {
    const v = (((c - 65 + delta) % 26) + 26) % 26;
    return 65 + v;
  }
  if (c >= 97 && c <= 122) {
    const v = (((c - 97 + delta) % 26) + 26) % 26;
    return 97 + v;
  }
  return c;
}

const mod = (a: number, m: number) => (((a % m) + m) % m);

/** 应用移位变换(输入即变换;解密请先 invertShift) */
export function applyShift(input: string, o: ShiftOpts): string {
  if (o.mode === 'rotl' || o.mode === 'rotr') {
    const chars = Array.from(input);
    const L = chars.length;
    if (L === 0) return input;
    const k = mod(Math.trunc(o.n), L);
    if (k === 0) return input;
    return o.mode === 'rotl'
      ? chars.slice(k).join('') + chars.slice(0, k).join('')
      : chars.slice(L - k).join('') + chars.slice(0, L - k).join('');
  }
  if (o.mode === 'rol' || o.mode === 'ror' || o.mode === 'shl' || o.mode === 'shr') {
    const bits = Math.max(0, Math.min(7, Math.trunc(o.n)));
    let out = '';
    for (let i = 0; i < input.length; i++) {
      const b = input.charCodeAt(i) & 0xff;
      let r: number;
      if (o.mode === 'rol') r = bits === 0 ? b : ((b << bits) | (b >>> (8 - bits))) & 0xff;
      else if (o.mode === 'ror') r = bits === 0 ? b : ((b >>> bits) | (b << (8 - bits))) & 0xff;
      else if (o.mode === 'shl') r = (b << bits) & 0xff;
      else r = (b >>> bits) & 0xff;
      out += String.fromCharCode(r);
    }
    return out;
  }
  // const / inc:值域加法(参数一律取整,保证加解密互逆不受小数输入破坏)
  let out = '';
  for (let i = 0; i < input.length; i++) {
    const c = input.charCodeAt(i);
    const delta =
      o.mode === 'inc' ? Math.trunc(o.n) + i * Math.trunc(o.s ?? 1) : Math.trunc(o.n);
    out += String.fromCharCode(domainAdd(c, delta, o.domain));
  }
  return out;
}

/** 多层移位:自上而下依序应用(层顺序敏感) */
export function applyLayers(input: string, layers: ShiftOpts[]): string {
  return layers.reduce((acc, o) => applyShift(acc, o), input);
}

/** 多层整体求逆:各层求逆后逆序排列;任一层含 shl/shr(不可逆)返回 null */
export function invertLayers(layers: ShiftOpts[]): ShiftOpts[] | null {
  const invs: ShiftOpts[] = [];
  for (let i = layers.length - 1; i >= 0; i--) {
    const inv = invertShift(layers[i]);
    if (!inv) return null;
    invs.push(inv);
  }
  return invs;
}

/** 求逆变换参数;shl/shr 有损不可逆返回 null */
export function invertShift(o: ShiftOpts): ShiftOpts | null {
  switch (o.mode) {
    case 'const':
      return { ...o, n: -o.n };
    case 'inc':
      return { ...o, n: -o.n, s: -(o.s ?? 1) };
    case 'rol':
      return { ...o, mode: 'ror' };
    case 'ror':
      return { ...o, mode: 'rol' };
    case 'rotl':
      return { ...o, mode: 'rotr' };
    case 'rotr':
      return { ...o, mode: 'rotl' };
    case 'shl':
    case 'shr':
      return null;
  }
}

const DOMAIN_NAME: Record<ShiftDomain, string> = {
  byte: 'mod 256',
  printable: '可打印环',
  alpha: '字母环',
};

/** 参数的人类可读描述(爆破结果展示用) */
export function describeShift(o: ShiftOpts): string {
  const d = DOMAIN_NAME[o.domain];
  switch (o.mode) {
    case 'const':
      return `常数移位 N=${o.n} · ${d}`;
    case 'inc':
      return `递增移位 b=${o.n} s=${o.s ?? 1} · ${d}`;
    case 'rol':
      return `ROL 循环左移 ${o.n} 位`;
    case 'ror':
      return `ROR 循环右移 ${o.n} 位`;
    case 'shl':
      return `SHL 左移 ${o.n} 位(有损)`;
    case 'shr':
      return `SHR 右移 ${o.n} 位(有损)`;
    case 'rotl':
      return `字符串左旋 ${o.n} 位`;
    case 'rotr':
      return `字符串右旋 ${o.n} 位`;
  }
}

/**
 * 按用户指定的 flag 格式串爆破移位参数。
 * anywhere=false 前缀匹配;true 为包含匹配。format 为空返回 []。
 * 枚举范围:const N∈[-32,32]×3 域;inc b∈[-16,16]×s∈[-4,4]×3 域;
 * ROL/ROR、SHL/SHR n∈[1,7];字符串旋转 k∈[0, len-1]。
 */
export function bruteForceShift(cipher: string, format: string, anywhere: boolean): ShiftHit[] {
  if (!format) return [];
  const hits: ShiftHit[] = [];
  const push = (opts: ShiftOpts) => {
    const plain = applyShift(cipher, opts);
    if (anywhere ? plain.includes(format) : plain.startsWith(format)) {
      hits.push({ opts, plain, desc: describeShift(opts) });
    }
  };
  const domains: ShiftDomain[] = ['byte', 'printable', 'alpha'];
  for (const domain of domains) {
    for (let n = -32; n <= 32; n++) push({ mode: 'const', domain, n });
    for (let b = -16; b <= 16; b++) {
      for (let s = -4; s <= 4; s++) push({ mode: 'inc', domain, n: b, s });
    }
  }
  for (let n = 1; n <= 7; n++) {
    push({ mode: 'rol', domain: 'byte', n });
    push({ mode: 'ror', domain: 'byte', n });
    push({ mode: 'shl', domain: 'byte', n });
    push({ mode: 'shr', domain: 'byte', n });
  }
  const L = Array.from(cipher).length;
  for (let k = 0; k < L; k++) {
    push({ mode: 'rotl', domain: 'byte', n: k });
    push({ mode: 'rotr', domain: 'byte', n: k });
  }
  return hits;
}
