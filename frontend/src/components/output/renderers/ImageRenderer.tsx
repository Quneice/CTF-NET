import { Download } from 'lucide-react';

import { downloadDataUrl } from '@/lib/download';

import type { RendererProps } from './types';

/** 图片输出 */
export default function ImageRenderer({ data }: RendererProps) {
  const img = data as { name?: string; dataUrl?: string };
  const url = img?.dataUrl;

  if (!url) {
    return <div className="text-sm text-slate-400">无图片数据</div>;
  }

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
      <div className="flex items-center justify-end gap-1 border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
        <button
          type="button"
          onClick={() => downloadDataUrl(img.name ?? 'image.png', url)}
          className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <Download className="h-3.5 w-3.5" />
          下载
        </button>
      </div>
      <div className="flex justify-center bg-slate-50 p-4 dark:bg-slate-900">
        <img src={url} alt={img.name ?? '结果图片'} className="max-w-full" />
      </div>
    </div>
  );
}
