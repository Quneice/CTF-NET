import { useEffect, useState } from 'react';
import { X } from 'lucide-react';

import { uploadFile } from '@/lib/backend';
import type { Entry } from '@/lib/projects';
import { parseContent } from '@/lib/projects';
import type { EntryKindInfo } from '@/mock/projectKinds';

/**
 * 条目新增/编辑对话框：由 EntryKindInfo.fields 配置驱动，一套组件服务全部类别。
 * 附件类条目：选择文件后先上传（复用 backend.uploadFile），file_id/name/size 存入 content。
 */
export default function EntryDialog({
  kindInfo,
  entry,
  onSubmit,
  onClose,
}: {
  kindInfo: EntryKindInfo;
  /** 传入表示编辑，否则新增 */
  entry?: Entry | null;
  onSubmit: (title: string, content: string) => Promise<void>;
  onClose: () => void;
}) {
  const [title, setTitle] = useState(entry?.title ?? '');
  const [values, setValues] = useState<Record<string, string>>(() => {
    const raw = entry ? parseContent(entry) : {};
    const out: Record<string, string> = {};
    for (const f of kindInfo.fields) {
      if (f.type === 'file') continue;
      out[f.key] = raw[f.key] != null ? String(raw[f.key]) : '';
    }
    return out;
  });
  const [file, setFile] = useState<File | null>(null);
  const [fileInfo, setFileInfo] = useState<{ name: string; size: number } | null>(() => {
    if (!entry) return null;
    const raw = parseContent(entry);
    return raw.name ? { name: String(raw.name), size: Number(raw.size) || 0 } : null;
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 编辑态换条目时重置（对话框以 key 区分时这里是兜底）
  useEffect(() => setError(null), [entry]);

  const hasFileField = kindInfo.fields.some((f) => f.type === 'file');

  const submit = async () => {
    if (!hasFileField && !title.trim()) {
      setError('标题不能为空');
      return;
    }
    for (const f of kindInfo.fields) {
      if (f.type === 'file') continue;
      if (f.required && !String(values[f.key] ?? '').trim()) {
        setError(`「${f.label}」不能为空`);
        return;
      }
    }
    setBusy(true);
    setError(null);
    try {
      let contentObj: Record<string, unknown> = { ...values };
      let finalTitle = title.trim();
      if (hasFileField) {
        // 附件：新选文件则上传；编辑且未换文件则保留原 file_id
        if (file) {
          const buf = new Uint8Array(await file.arrayBuffer());
          const stored = await uploadFile(file.name, buf);
          contentObj = { file_id: stored.file_id, name: stored.name ?? file.name, size: stored.size ?? file.size };
          if (!finalTitle) finalTitle = file.name;
        } else if (entry) {
          const raw = parseContent(entry);
          contentObj = { file_id: raw.file_id, name: raw.name, size: raw.size };
          if (!finalTitle) finalTitle = String(raw.name ?? '附件');
        } else {
          setError('请选择文件');
          setBusy(false);
          return;
        }
      }
      await onSubmit(finalTitle, JSON.stringify(contentObj));
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="w-full max-w-lg rounded-lg bg-white p-5 shadow-xl dark:bg-slate-900"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold">
            {entry ? '编辑' : '新增'}
            {kindInfo.label}
          </h3>
          <button type="button" onClick={onClose} className="rounded p-1 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="mt-4 space-y-3">
          {/* 附件类不强制标题（默认取文件名） */}
          {!hasFileField && (
            <label className="block text-sm">
              <span className="mb-1 block text-slate-500 dark:text-slate-400">标题 *</span>
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700"
                placeholder={`如：${kindInfo.label} - 主站`}
              />
            </label>
          )}

          {kindInfo.fields.map((f) => {
            if (f.type === 'file') {
              return (
                <label key={f.key} className="block text-sm">
                  <span className="mb-1 block text-slate-500 dark:text-slate-400">
                    {f.label}
                    {f.required && !entry ? ' *' : ''}
                  </span>
                  <input
                    type="file"
                    onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                    className="w-full text-sm text-slate-500 file:mr-3 file:rounded-md file:border-0 file:bg-indigo-50 file:px-3 file:py-1.5 file:text-indigo-600 dark:file:bg-indigo-500/10 dark:file:text-indigo-400"
                  />
                  {fileInfo && !file && (
                    <span className="mt-1 block text-xs text-slate-400">当前：{fileInfo.name}（不选择则保留原文件）</span>
                  )}
                </label>
              );
            }
            if (f.type === 'textarea') {
              return (
                <label key={f.key} className="block text-sm">
                  <span className="mb-1 block text-slate-500 dark:text-slate-400">
                    {f.label}
                    {f.required ? ' *' : ''}
                  </span>
                  <textarea
                    value={values[f.key] ?? ''}
                    onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                    rows={f.rows ?? 3}
                    placeholder={f.placeholder}
                    className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 font-mono text-xs outline-none focus:border-indigo-400 dark:border-slate-700"
                  />
                </label>
              );
            }
            if (f.type === 'select') {
              return (
                <label key={f.key} className="block text-sm">
                  <span className="mb-1 block text-slate-500 dark:text-slate-400">
                    {f.label}
                    {f.required ? ' *' : ''}
                  </span>
                  <select
                    value={values[f.key] ?? ''}
                    onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                    className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900"
                  >
                    <option value="">请选择…</option>
                    {f.options?.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                </label>
              );
            }
            return (
              <label key={f.key} className="block text-sm">
                <span className="mb-1 block text-slate-500 dark:text-slate-400">
                  {f.label}
                  {f.required ? ' *' : ''}
                </span>
                <input
                  type={f.type === 'password' ? 'password' : 'text'}
                  value={values[f.key] ?? ''}
                  onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                  placeholder={f.placeholder}
                  className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700"
                />
              </label>
            );
          })}
        </div>

        {error && <p className="mt-3 text-sm text-red-500">{error}</p>}

        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            取消
          </button>
          <button
            type="button"
            onClick={submit}
            disabled={busy}
            className="rounded-md bg-indigo-500 px-3 py-1.5 text-sm text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            {busy ? '保存中…' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}
