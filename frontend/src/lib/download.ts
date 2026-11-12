/**
 * 浏览器端下载工具函数。
 */

function triggerDownload(href: string, filename: string) {
  const a = document.createElement('a');
  a.href = href;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

/** 以 Blob 形式下载文本/二进制内容 */
export function downloadBlob(filename: string, content: BlobPart, mime = 'text/plain') {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  triggerDownload(url, filename);
  URL.revokeObjectURL(url);
}

/** 直接下载 data URL（图片等） */
export function downloadDataUrl(filename: string, dataUrl: string) {
  triggerDownload(dataUrl, filename);
}
