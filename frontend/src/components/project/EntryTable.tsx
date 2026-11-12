import { useState } from 'react';
import { Download, Pencil, Plus, Trash2 } from 'lucide-react';

import EntryDialog from '@/components/project/EntryDialog';
import { exportFile, parseContent, type Entry } from '@/lib/projects';
import type { EntryKindInfo } from '@/mock/projectKinds';
import { useProjectStore } from '@/stores/useProjectStore';

/**
 * 某类条目的列表表格 + 新增/编辑/删除/附件下载。
 * 由 EntryKindInfo.columns 驱动渲染，服务全部类别。
 */
export default function EntryTable({
  projectId,
  kindInfo,
  entries,
}: {
  projectId: string;
  kindInfo: EntryKindInfo;
  entries: Entry[];
}) {
  const { createEntry, updateEntry, deleteEntry } = useProjectStore();
  const [dialog, setDialog] = useState<{ entry: Entry | null } | null>(null);
  const [deleting, setDeleting] = useState<Entry | null>(null);
  const [error, setError] = useState<string | null>(null);

  // 表格单元格取值：title 直出，其余从 content JSON 取
  const cell = (e: Entry, key: string): string => {
    if (key === 'title') return e.title;
    const v = parseContent(e)[key];
    if (v == null) return '';
    if (key === 'size') return formatSize(Number(v));
    if (key === 'password') return '••••••';
    return String(v);
  };

  const download = async (e: Entry) => {
    const raw = parseContent(e);
    const fileID = String(raw.file_id ?? '');
    if (!fileID) return;
    setError(null);
    try {
      // Wails 运行时保存对话框；浏览器开发模式无此能力
      const rt = (window as unknown as { runtime?: { SaveFileDialog(o: object): Promise<string> } }).runtime;
      if (!rt?.SaveFileDialog) {
        setError('附件下载仅桌面端可用');
        return;
      }
      const dest = await rt.SaveFileDialog({ DefaultFilename: String(raw.name ?? 'file') });
      if (!dest) return; // 用户取消
      await exportFile(fileID, dest);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div>
      <div className="mb-3 flex items-center justify-between">
        <span className="text-sm text-slate-500 dark:text-slate-400">共 {entries.length} 条</span>
        <button
          type="button"
          onClick={() => setDialog({ entry: null })}
          className="flex items-center gap-1 rounded-md bg-indigo-500 px-2.5 py-1.5 text-sm text-white hover:bg-indigo-600"
        >
          <Plus className="h-4 w-4" />
          新增{kindInfo.label}
        </button>
      </div>

      {error && <p className="mb-2 text-sm text-red-500">{error}</p>}

      {entries.length === 0 ? (
        <div className="rounded-lg border border-dashed border-slate-200 py-12 text-center text-sm text-slate-400 dark:border-slate-700">
          暂无{kindInfo.label}记录，点击右上角新增
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-700">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-xs text-slate-500 dark:bg-slate-800/50 dark:text-slate-400">
              <tr>
                {kindInfo.fields.some((f) => f.type === 'file') ? null : <th className="px-3 py-2 font-medium">标题</th>}
                {kindInfo.columns.map((c) => (
                  <th key={c.key} className="px-3 py-2 font-medium">
                    {c.title}
                  </th>
                ))}
                <th className="px-3 py-2 font-medium">更新时间</th>
                <th className="w-24 px-3 py-2 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {entries.map((e) => {
                const isAttachment = kindInfo.fields.some((f) => f.type === 'file');
                return (
                  <tr key={e.id} className="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                    {isAttachment ? null : (
                      <td className="max-w-40 truncate px-3 py-2 font-medium">{e.title || '—'}</td>
                    )}
                    {kindInfo.columns.map((c) => (
                      <td key={c.key} className="max-w-56 truncate px-3 py-2 text-slate-600 dark:text-slate-300" title={cell(e, c.key)}>
                        {cell(e, c.key) || '—'}
                      </td>
                    ))}
                    <td className="whitespace-nowrap px-3 py-2 text-xs text-slate-400">
                      {new Date(e.updated_at * 1000).toLocaleString('zh-CN', { hour12: false })}
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex gap-1">
                        {isAttachment && (
                          <button
                            type="button"
                            onClick={() => download(e)}
                            title="下载"
                            className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-indigo-500 dark:hover:bg-slate-800"
                          >
                            <Download className="h-4 w-4" />
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => setDialog({ entry: e })}
                          title="编辑"
                          className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-indigo-500 dark:hover:bg-slate-800"
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button
                          type="button"
                          onClick={() => setDeleting(e)}
                          title="删除"
                          className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {dialog && (
        <EntryDialog
          kindInfo={kindInfo}
          entry={dialog.entry}
          onClose={() => setDialog(null)}
          onSubmit={async (title, content) => {
            if (dialog.entry) {
              await updateEntry(dialog.entry.id, title, content, dialog.entry.tags);
            } else {
              await createEntry(projectId, kindInfo.kind, title, content, '');
            }
          }}
        />
      )}

      {/* 删除二次确认 */}
      {deleting && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setDeleting(null)}>
          <div className="w-full max-w-sm rounded-lg bg-white p-5 shadow-xl dark:bg-slate-900" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-base font-semibold">确认删除</h3>
            <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
              确定删除「{deleting.title || parseContent(deleting).name?.toString() || kindInfo.label}」吗？此操作不可恢复。
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setDeleting(null)}
                className="rounded-md px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                取消
              </button>
              <button
                type="button"
                onClick={async () => {
                  await deleteEntry(deleting.id);
                  setDeleting(null);
                }}
                className="rounded-md bg-red-500 px-3 py-1.5 text-sm text-white hover:bg-red-600"
              >
                删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function formatSize(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
