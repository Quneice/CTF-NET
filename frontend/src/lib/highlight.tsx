import type { ReactNode } from 'react';

/**
 * 轻量语法高亮（无第三方依赖）。
 *
 * 提供 JSON 与通用代码两种高亮，返回 Token 列表，由 <Tokens> 渲染为带颜色的 span。
 * 颜色面向深色背景（代码/JSON 查看器强制深色底）。
 */

export type TokenType =
  | 'plain'
  | 'key'
  | 'string'
  | 'number'
  | 'boolean'
  | 'null'
  | 'comment'
  | 'keyword';

export interface Token {
  type: TokenType;
  text: string;
}

function tok(type: TokenType, text: string): Token {
  return { type, text };
}

/** 高亮 JSON 字符串（keys / 字符串 / 数字 / 布尔 / null） */
export function highlightJson(json: string): Token[] {
  const tokens: Token[] = [];
  const re = /("(?:\\.|[^"\\])*")(\s*:)?|\b(true|false|null)\b|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;
  let last = 0;
  let m: RegExpExecArray | null;

  while ((m = re.exec(json))) {
    if (m.index > last) tokens.push(tok('plain', json.slice(last, m.index)));
    if (m[1] !== undefined) {
      tokens.push(tok(m[2] !== undefined ? 'key' : 'string', m[1]));
      if (m[2] !== undefined) tokens.push(tok('plain', m[2]));
    } else if (m[3] !== undefined) {
      tokens.push(tok(m[3] === 'true' || m[3] === 'false' ? 'boolean' : 'null', m[3]));
    } else if (m[4] !== undefined) {
      tokens.push(tok('number', m[4]));
    }
    last = m.index + m[0].length;
  }
  if (last < json.length) tokens.push(tok('plain', json.slice(last)));
  return tokens;
}

const KEYWORDS = new Set([
  'const', 'let', 'var', 'function', 'return', 'if', 'else', 'for', 'while', 'switch',
  'case', 'break', 'continue', 'import', 'export', 'from', 'class', 'new', 'this', 'try',
  'catch', 'finally', 'throw', 'async', 'await', 'int', 'float', 'char', 'void', 'struct',
  'union', 'typedef', 'unsigned', 'signed', 'long', 'short', 'static', 'extern', 'def',
  'print', 'push', 'pop', 'mov', 'call', 'ret', 'jmp', 'cmp', 'add', 'sub', 'xor', 'and', 'or',
]);

/** 高亮代码（注释 / 字符串 / 数字 / 关键字） */
export function highlightCode(code: string, lang?: string): Token[] {
  const commentStart =
    lang === 'asm'
      ? ';'
      : lang === 'python' || lang === 'shell' || lang === 'bash' || lang === 'sh'
        ? '#'
        : '//';

  const tokens: Token[] = [];
  let i = 0;
  let plain = '';
  const flush = () => {
    if (plain) {
      tokens.push(tok('plain', plain));
      plain = '';
    }
  };

  while (i < code.length) {
    const c = code[i];

    // 行注释
    if (code.startsWith(commentStart, i)) {
      let j = i;
      while (j < code.length && code[j] !== '\n') j++;
      flush();
      tokens.push(tok('comment', code.slice(i, j)));
      i = j;
      continue;
    }

    // 块注释
    if (c === '/' && code[i + 1] === '*') {
      let j = code.indexOf('*/', i + 2);
      j = j === -1 ? code.length : j + 2;
      flush();
      tokens.push(tok('comment', code.slice(i, j)));
      i = j;
      continue;
    }

    // 字符串
    if (c === '"' || c === "'" || c === '`') {
      let j = i + 1;
      while (j < code.length && code[j] !== c) {
        if (code[j] === '\\') j++;
        j++;
      }
      j = Math.min(j + 1, code.length);
      flush();
      tokens.push(tok('string', code.slice(i, j)));
      i = j;
      continue;
    }

    // 数字
    if (/\d/.test(c) || (c === '.' && /\d/.test(code[i + 1] ?? ''))) {
      let j = i;
      while (j < code.length && /[0-9a-fA-FxX._]/.test(code[j])) j++;
      flush();
      tokens.push(tok('number', code.slice(i, j)));
      i = j;
      continue;
    }

    // 标识符 / 关键字
    if (/[A-Za-z_]/.test(c)) {
      let j = i;
      while (j < code.length && /[A-Za-z0-9_]/.test(code[j])) j++;
      const word = code.slice(i, j);
      flush();
      tokens.push(tok(KEYWORDS.has(word) ? 'keyword' : 'plain', word));
      i = j;
      continue;
    }

    plain += c;
    i++;
  }
  flush();
  return tokens;
}

const tokenClass: Record<TokenType, string> = {
  plain: 'text-slate-300',
  key: 'text-sky-300',
  string: 'text-emerald-400',
  number: 'text-amber-400',
  boolean: 'text-orange-400',
  null: 'text-slate-500',
  comment: 'text-slate-500 italic',
  keyword: 'text-purple-400',
};

/** 渲染 Token 列表 */
export function Tokens({ tokens }: { tokens: Token[] }): ReactNode {
  return (
    <>
      {tokens.map((t, i) => (
        <span key={i} className={tokenClass[t.type]}>
          {t.text}
        </span>
      ))}
    </>
  );
}
