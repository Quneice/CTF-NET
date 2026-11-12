import { renderMarkdown } from '@/lib/markdown';

import { CopyButton, DownloadButton } from '../OutputActions';
import type { RendererProps } from './types';

/** Markdown 输出 */
export default function MarkdownRenderer({ data }: RendererProps) {
  const md = typeof data === 'string' ? data : JSON.stringify(data, null, 2);

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
      <div className="flex items-center justify-end gap-1 border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
        <CopyButton text={md} />
        <DownloadButton text={md} filename="result.md" mime="text/markdown" />
      </div>
      <div className="markdown-body max-h-96 overflow-auto px-4 py-3 text-sm text-slate-700 dark:text-slate-300">
        {renderMarkdown(md)}
      </div>
    </div>
  );
}
