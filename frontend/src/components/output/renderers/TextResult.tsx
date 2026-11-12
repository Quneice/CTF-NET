import { useState } from 'react';
import type { ReactNode } from 'react';

import { CopyButton, DownloadButton } from '../OutputActions';

/** 超过该行数时折叠 */
const COLLAPSE_LINES = 200;

/**
 * 文本类输出的共享容器：复制 / 下载 / 超长折叠，可选语法高亮渲染。
 * 提供 render 时强制深色底（配合高亮 token 的浅色前景）。
 */
export default function TextResult({
  text,
  filename = 'result.txt',
  mime = 'text/plain',
  render,
}: {
  text: string;
  filename?: string;
  mime?: string;
  render?: (text: string) => ReactNode;
}) {
  const [expanded, setExpanded] = useState(false);

  const lines = text.split('\n');
  const long = lines.length > COLLAPSE_LINES;
  const display = !long || expanded ? text : lines.slice(0, COLLAPSE_LINES).join('\n');
  const highlighted = !!render;

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
      <div className="flex items-center justify-between border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
        <span className="text-xs text-slate-400">{lines.length} 行</span>
        <div className="flex items-center gap-1">
          <CopyButton text={text} />
          <DownloadButton text={text} filename={filename} mime={mime} />
        </div>
      </div>

      <pre
        className={`max-h-[50vh] min-h-[200px] overflow-auto whitespace-pre-wrap px-3 py-2 text-xs leading-relaxed ${
          highlighted
            ? 'bg-slate-900 text-slate-200'
            : 'bg-slate-50 text-slate-700 dark:bg-slate-900 dark:text-slate-300'
        }`}
      >
        {render ? render(display) : display}
      </pre>

      {long && (
        <button
          type="button"
          onClick={() => setExpanded((e) => !e)}
          className="w-full border-t border-slate-200 px-3 py-2 text-center text-xs text-indigo-500 hover:bg-slate-50 dark:border-slate-800 dark:hover:bg-slate-800"
        >
          {expanded ? '收起' : `展开全部（${lines.length} 行）`}
        </button>
      )}
    </div>
  );
}
