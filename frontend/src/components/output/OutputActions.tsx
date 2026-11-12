import { useState } from 'react';
import { Check, Copy, Download } from 'lucide-react';

import { downloadBlob } from '@/lib/download';

/** 复制按钮（带短暂「已复制」反馈） */
export function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // 剪贴板不可用时静默失败
    }
  };

  return (
    <button
      type="button"
      onClick={onCopy}
      className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
    >
      {copied ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
      {copied ? '已复制' : '复制'}
    </button>
  );
}

/** 文本下载按钮 */
export function DownloadButton({
  text,
  filename,
  mime,
}: {
  text: string;
  filename: string;
  mime?: string;
}) {
  return (
    <button
      type="button"
      onClick={() => downloadBlob(filename, text, mime)}
      className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
    >
      <Download className="h-3.5 w-3.5" />
      下载
    </button>
  );
}
