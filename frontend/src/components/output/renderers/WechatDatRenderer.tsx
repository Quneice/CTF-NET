import { Download } from 'lucide-react';

import { downloadDataUrl } from '@/lib/download';

import type { RendererProps } from './types';

interface WechatDatResult {
  name?: string;
  dataUrl?: string;
  mime?: string;
  keyHex?: string;
  format?: string;
  size?: number;
  isImage?: boolean;
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(2)} MB`;
}

/** 微信 .dat 图片缓存还原结果渲染器（rendererId = 'wechat-dat'） */
export default function WechatDatRenderer({ data }: RendererProps) {
  const r = (data ?? {}) as WechatDatResult;

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
      <div className="flex items-center justify-between gap-2 border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
        <div className="min-w-0 truncate text-xs text-slate-500 dark:text-slate-400">
          {r.keyHex ? <span className="mr-2 font-mono">密钥 0x{r.keyHex}</span> : null}
          {r.format ? <span className="mr-2">类型 {r.format}</span> : null}
          {typeof r.size === 'number' ? <span>{formatSize(r.size)}</span> : null}
        </div>
        {r.dataUrl ? (
          <button
            type="button"
            onClick={() => downloadDataUrl(r.name ?? 'decrypted.bin', r.dataUrl!)}
            className="flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
          >
            <Download className="h-3.5 w-3.5" />
            下载
          </button>
        ) : null}
      </div>

      {r.isImage && r.dataUrl ? (
        <div className="flex justify-center bg-slate-50 p-4 dark:bg-slate-900">
          <img src={r.dataUrl} alt={r.name ?? '解密结果'} className="max-w-full" />
        </div>
      ) : (
        <div className="p-4 text-sm text-slate-400">
          {r.dataUrl ? `已还原为 ${r.format ?? '文件'}（非图片类型，可直接下载）` : '无结果数据'}
        </div>
      )}
    </div>
  );
}
