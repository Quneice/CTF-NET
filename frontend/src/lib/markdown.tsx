import { useEffect, useState, type ReactNode } from 'react';

/**
 * 轻量 Markdown 渲染（无第三方依赖），安全子集 + 语料实测形态：
 * 标题（≤3 空格缩进、`#标题` 无空格、`一、` 中文编号、尾部闭合 ##）/
 * 围栏·缩进代码块 / 行内代码·加粗·斜体 / 链接 / 图片 / <br> /
 * 有序·无序列表（支持嵌套与多行条目）/ 引用（多行）/ 表格 / 分隔线 / 段落。
 * 全部通过 React 节点输出，天然防 XSS。
 * 图片支持可选 imgSrcResolver：本地相对引用异步解析为 data URL（漏洞浏览器用）。
 */

export type ImgSrcResolver = (src: string) => Promise<string> | string;

const HTTP_RE = /^(https?:|data:|blob:)/i;

/** 最小 HTML 实体解码（图片 src 里偶见 &amp; 等）。 */
function unescapeHtml(s: string): string {
  return s
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'");
}

/** 图片：无 resolver 时直接渲染；有 resolver 且是本地引用时异步加载，加载中/失败给占位。 */
function MarkdownImg({ src, alt, resolve }: { src: string; alt?: string; resolve?: ImgSrcResolver }) {
  const needResolve = !!resolve && !HTTP_RE.test(src);
  const [resolved, setResolved] = useState<string | null>(needResolve ? null : src);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!needResolve) return;
    let alive = true;
    setResolved(null);
    setFailed(false);
    Promise.resolve(resolve!(src)).then(
      (r) => alive && setResolved(r),
      () => alive && setFailed(true),
    );
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [src, needResolve]);

  if (failed) return <span className="md-img-fail">⚠ 图片加载失败：{src}</span>;
  if (needResolve && !resolved) return <span className="md-img-loading">🖼 图片加载中…</span>;
  return <img className="md-img" src={resolved ?? src} alt={alt ?? ''} loading="lazy" />;
}

function inline(text: string, resolve?: ImgSrcResolver): ReactNode[] {
  const nodes: ReactNode[] = [];
  const re = /(`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*|<br\s*\/?>|!?\[[^\]]+\]\([^)]+\))/gi;
  let last = 0;
  let m: RegExpExecArray | null;
  let key = 0;

  while ((m = re.exec(text))) {
    if (m.index > last) nodes.push(text.slice(last, m.index));
    const token = m[0];
    if (token.startsWith('`')) {
      nodes.push(<code key={key++}>{token.slice(1, -1)}</code>);
    } else if (token.startsWith('**')) {
      nodes.push(<strong key={key++}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith('*')) {
      nodes.push(<em key={key++}>{token.slice(1, -1)}</em>);
    } else if (/^<br/i.test(token)) {
      nodes.push(<br key={key++} />);
    } else {
      const lm = /!?\[([^\]]*)\]\(([^)]+)\)/.exec(token)!;
      const isImg = token.startsWith('!');
      const href = unescapeHtml(lm[2].trim());
      if (isImg) {
        nodes.push(<MarkdownImg key={key++} src={href} alt={lm[1]} resolve={resolve} />);
      } else {
        nodes.push(
          <a key={key++} href={href} target="_blank" rel="noreferrer">
            {lm[1]}
          </a>,
        );
      }
    }
    last = m.index + token.length;
  }
  if (last < text.length) nodes.push(text.slice(last));
  return nodes;
}

/** 测量一行的前导空白（空格计 1，tab 计 4）与去空白后的文本、是否空行。 */
function measure(line: string): { indent: number; text: string; blank: boolean } {
  let n = 0;
  for (const ch of line) {
    if (ch === ' ') n++;
    else if (ch === '\t') n += 4;
    else break;
  }
  const text = line.slice(line.length - line.trimStart().length).trimEnd();
  return { indent: n, text, blank: text === '' };
}

// —— 块级正则 ——
const FENCE_RE = /^```/;
const HEADING_RE = /^(#{1,6})(?:[ \t]+(.*?)\s*#*\s*|([^#\s].*)|)$/;
const HR_RE = /^(\s*[-*_]\s*){3,}$/;
const QUOTE_RE = /^>\s?(.*)$/;
const LIST_RE = /^([-*+]|\d{1,9}[.)])[ \t]+(.*)$/;
const CN_HEADING_RE = /^([一二三四五六七八九十]+)、[ \t]*(\S.*)$/;
const TABLE_SEP_RE = /^\s*\|?[\s:|-]*-{2,}[\s:|-]*\|[\s:|-]*$/;

/** 该行是否是块级起点（决定段落是否中断）。 */
function isBlockStart(text: string): boolean {
  return (
    FENCE_RE.test(text) ||
    /^#{1,6}/.test(text) ||
    HR_RE.test(text) ||
    QUOTE_RE.test(text) ||
    LIST_RE.test(text) ||
    text.startsWith('|') ||
    CN_HEADING_RE.test(text)
  );
}

/** 块级解析（递归）：解析 [start, len) 行，行缩进小于 baseIndent 时交还上层。 */
function parseBlocks(
  lines: string[],
  start: number,
  baseIndent: number,
  resolve?: ImgSrcResolver,
): { nodes: ReactNode[]; next: number } {
  const nodes: ReactNode[] = [];
  let i = start;
  let key = 0;
  const k = () => key++;

  while (i < lines.length) {
    const m = measure(lines[i]);
    if (m.blank) {
      i++;
      continue;
    }
    if (m.indent < baseIndent) break; // 交还上层（嵌套列表/引用）

    // 围栏代码块（≤3 空格缩进）
    if (FENCE_RE.test(m.text)) {
      const openIndent = m.indent;
      const buf: string[] = [];
      i++;
      while (i < lines.length) {
        const f = measure(lines[i]);
        if (f.text.startsWith('```') && f.indent <= openIndent) {
          i++; // 跳过结束围栏
          break;
        }
        buf.push(stripIndent(lines[i], openIndent));
        i++;
      }
      nodes.push(
        <pre key={k()}>
          <code>{buf.join('\n')}</code>
        </pre>,
      );
      continue;
    }

    // 标题（≤3 空格缩进；含无空格 `#标题`、中文「一、」编号、尾部闭合 #）
    const h = HEADING_RE.exec(m.text);
    if (h && m.indent <= 3) {
      const level = h[1].length;
      const content = inline(h[2] ?? h[3] ?? '', resolve);
      if (level === 1) nodes.push(<h1 key={k()}>{content}</h1>);
      else if (level === 2) nodes.push(<h2 key={k()}>{content}</h2>);
      else if (level === 3) nodes.push(<h3 key={k()}>{content}</h3>);
      else if (level === 4) nodes.push(<h4 key={k()}>{content}</h4>);
      else if (level === 5) nodes.push(<h5 key={k()}>{content}</h5>);
      else nodes.push(<h6 key={k()}>{content}</h6>);
      i++;
      continue;
    }
    const cn = CN_HEADING_RE.exec(m.text);
    if (cn && m.indent <= 3) {
      nodes.push(<h3 key={k()}>{inline(cn[2], resolve)}</h3>);
      i++;
      continue;
    }

    // 分隔线
    if (HR_RE.test(m.text)) {
      nodes.push(<hr key={k()} />);
      i++;
      continue;
    }

    // 引用（多行；内部递归解析）
    const q = QUOTE_RE.exec(m.text);
    if (q) {
      const buf: string[] = [q[1]];
      i++;
      while (i < lines.length) {
        const f = measure(lines[i]);
        const qq = f.blank ? null : QUOTE_RE.exec(f.text);
        if (qq) {
          buf.push(qq[1]);
          i++;
        } else if (f.blank && i + 1 < lines.length && QUOTE_RE.test(measure(lines[i + 1]).text)) {
          buf.push('');
          i++;
        } else break;
      }
      const inner = parseBlocks(buf, 0, 0, resolve);
      nodes.push(<blockquote key={k()}>{inner.nodes}</blockquote>);
      continue;
    }

    // 列表（支持嵌套 + 多行条目）
    const l = LIST_RE.exec(m.text);
    if (l) {
      const marker = l[1];
      const ordered = /^\d/.test(marker);
      const markerIndent = m.indent;
      const contentIndent = markerIndent + marker.length + 1;
      const items: ReactNode[] = [];
      while (i < lines.length) {
        const f = measure(lines[i]);
        if (f.blank) {
          // 空行：若后续更深缩进 → 条目内换段；否则列表结束
          let j = i + 1;
          while (j < lines.length && measure(lines[j]).blank) j++;
          if (j < lines.length && measure(lines[j]).indent > markerIndent) {
            i = j; // 空行丢弃，继续条目内
            continue;
          }
          break;
        }
        if (f.indent < baseIndent) break;
        if (f.indent !== markerIndent) break;
        const lm = LIST_RE.exec(f.text);
        if (!lm) break;
        const itemOrdered = /^\d/.test(lm[1]);
        if (itemOrdered !== ordered) break; // 类型变化 → 新列表
        const content = lm[2];
        // 收集条目续行（更深缩进的行）
        const childLines: string[] = [];
        let j = i + 1;
        while (j < lines.length) {
          const g = measure(lines[j]);
          if (g.blank) {
            let n = j + 1;
            while (n < lines.length && measure(lines[n]).blank) n++;
            if (n < lines.length && measure(lines[n]).indent > markerIndent) {
              childLines.push('');
              j = n;
              continue;
            }
            break;
          }
          if (g.indent > markerIndent) {
            childLines.push(lines[j]);
            j++;
          } else break;
        }
        // 子行按 contentIndent 重新定基后递归解析
        const rebased = childLines.map((cl) => {
          const g = measure(cl);
          if (g.blank) return '';
          return ' '.repeat(Math.max(0, g.indent - contentIndent)) + g.text;
        });
        const inner = parseBlocks(rebased, 0, 0, resolve);
        items.push(
          <li key={k()}>
            {inline(content, resolve)}
            {inner.nodes.length > 0 && <>{inner.nodes}</>}
          </li>,
        );
        i = j;
      }
      nodes.push(ordered ? <ol key={k()}>{items}</ol> : <ul key={k()}>{items}</ul>);
      continue;
    }

    // 表格（表头 + 分隔行 + 表体）
    if (m.text.startsWith('|') && i + 1 < lines.length) {
      const sep = measure(lines[i + 1]);
      if (TABLE_SEP_RE.test(sep.text)) {
        const rows: string[][] = [splitCells(m.text)];
        let j = i + 2; // 从分隔行之后开始收集表体
        while (j < lines.length) {
          const r = measure(lines[j]);
          if (r.blank || !r.text.startsWith('|')) break;
          rows.push(splitCells(r.text));
          j++;
        }
        if (rows.length >= 2) {
          const [head, ...body] = rows;
          nodes.push(
            <table key={k()}>
              <thead>
                <tr>
                  {head.map((c, ci) => (
                    <th key={ci}>{inline(c, resolve)}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {body.map((row, ri) => (
                  <tr key={ri}>
                    {row.map((c, ci) => (
                      <td key={ci}>{inline(c, resolve)}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>,
          );
          i = j;
          continue;
        }
      }
    }

    // 原始 HTML 图片行（漏洞文档常见 <img src=...>，去掉标签后为空则按图渲染）
    const imgTags = m.text.match(/<img\b[^>]*\/?>/gi);
    if (imgTags && imgTags.length > 0 && m.text.replace(/<[^>]+>/g, '').trim() === '') {
      for (const tag of imgTags) {
        const srcm = /\bsrc\s*=\s*["']([^"']+)["']/i.exec(tag);
        const altm = /\balt\s*=\s*["']([^"']*)["']/i.exec(tag);
        nodes.push(
          <MarkdownImg
            key={k()}
            src={unescapeHtml(srcm?.[1] ?? '')}
            alt={altm?.[1]}
            resolve={resolve}
          />,
        );
      }
      i++;
      continue;
    }

    // 缩进代码块（≥4 空格，非标题）
    if (m.indent >= baseIndent + 4) {
      const buf: string[] = [stripIndent(lines[i], baseIndent + 4)];
      i++;
      while (i < lines.length) {
        const g = measure(lines[i]);
        if (g.blank) {
          buf.push('');
          i++;
          continue;
        }
        if (g.indent >= baseIndent + 4) {
          buf.push(stripIndent(lines[i], baseIndent + 4));
          i++;
        } else break;
      }
      nodes.push(
        <pre key={k()}>
          <code>{buf.join('\n')}</code>
        </pre>,
      );
      continue;
    }

    // 段落：收集到空行或块级起点
    const para: string[] = [m.text];
    i++;
    while (i < lines.length) {
      const p = measure(lines[i]);
      if (p.blank || p.indent < baseIndent || isBlockStart(p.text)) break;
      if (p.indent >= baseIndent + 4) break; // 缩进代码另起块
      para.push(p.text);
      i++;
    }
    nodes.push(<p key={k()}>{inline(para.join(' '), resolve)}</p>);
  }
  return { nodes, next: i };
}

function stripIndent(line: string, n: number): string {
  if (n <= 0) return line;
  let cnt = 0;
  let idx = 0;
  for (const ch of line) {
    if (ch === ' ') cnt++;
    else if (ch === '\t') cnt += 4;
    else break;
    idx++;
    if (cnt >= n) break;
  }
  return line.slice(idx);
}

function splitCells(row: string): string[] {
  const t = row.trim();
  const inner = t.startsWith('|') && t.endsWith('|') ? t.slice(1, -1) : t;
  return inner.split('|').map((c) => c.trim());
}

export function renderMarkdown(
  md: string,
  opts?: { imgSrcResolver?: ImgSrcResolver },
): ReactNode[] {
  const resolve = opts?.imgSrcResolver;
  const lines = md.split('\n');
  return parseBlocks(lines, 0, 0, resolve).nodes;
}
