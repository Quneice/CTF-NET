import { Download, FileText } from 'lucide-react';

import { downloadBlob } from '@/lib/download';

import type { RendererProps } from './types';

/** 文件下载输出 */
export default function FileRenderer({ data }: RendererProps) {
  const f = data as { name?: string; size?: number; contentType?: string };
  const name = f?.name ?? 'result.bin';
  const size = f?.size ?? 0;
  const contentType = f?.contentType ?? 'application/octet-stream';

  const formatSize = (n: number) => (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`);

  return (
    <div className="flex items-center gap-3 rounded-lg border border-slate-200 p-4 dark:border-slate-800">
      <FileText className="h-8 w-8 shrink-0 text-indigo-500" />
      <div className="min-w-0 flex-1">
        <div className="truncate font-medium text-slate-800 dark:text-slate-100">{name}</div>
        <div className="text-xs text-slate-400">
          {formatSize(size)} · {contentType}
        </div>
      </div>
      <button
        type="button"
        onClick={() => downloadBlob(name, 'mock 文件内容', contentType)}
        className="flex items-center gap-1 rounded-md bg-indigo-500 px-3 py-1.5 text-sm text-white hover:bg-indigo-600"
      >
        <Download className="h-4 w-4" />
        下载
      </button>
    </div>
  );
}
