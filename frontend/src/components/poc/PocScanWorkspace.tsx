import { useCallback, useEffect, useRef, useState } from 'react';
import { Pencil, Save, Trash2, X } from 'lucide-react';

import { isWails } from '@/lib/backend';
import { renderMarkdown } from '@/lib/markdown';
import {
  deleteVulnCase,
  getVulnCase,
  resolveVulnAsset,
  updateVulnCase,
  type VulnCaseDetail,
} from '@/lib/vulnlib';
import { usePocStore } from '@/stores/usePocStore';

/**
 * 漏洞浏览器工具页主区。
 *
 * 在二级侧边栏「漏洞清单」选中案例后，右侧只展示该案例 md 文件的正文
 * （Markdown 渲染，含本地图片异步加载）。顶部小操作栏提供编辑/删除：
 * 编辑用原文 textarea，保存后覆写磁盘 md 文件；删除把文件移入
 * data/backups/vulnlib_deleted（两步确认）。浏览器 dev 模式无后端，仅提示。
 */
export default function PocScanWorkspace() {
  const selected = usePocStore((s) => s.selected);
  const clear = usePocStore((s) => s.clear);
  const bumpRefresh = usePocStore((s) => s.bumpRefresh);
  const [detail, setDetail] = useState<VulnCaseDetail | null>(null);
  const [detailErr, setDetailErr] = useState<string | null>(null);

  // —— 编辑状态 ——
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [opErr, setOpErr] = useState<string | null>(null);
  const [delArmed, setDelArmed] = useState(false);
  const disarmTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchDetail = useCallback((caseId: string) => {
    setDetail(null);
    setDetailErr(null);
    if (!isWails()) return;
    void getVulnCase(caseId).then(
      (d) => setDetail(d),
      (e) => setDetailErr(e instanceof Error ? e.message : String(e)),
    );
  }, []);

  // 图片引用 → data URL（本地相对路径走后端资源读取，http/data 原样）
  const resolveImg = useCallback(
    (src: string) => resolveVulnAsset(selected?.caseId ?? '', src),
    [selected?.caseId],
  );

  // 选中案例变化 → 拉取全文（并退出编辑态）
  useEffect(() => {
    setEditing(false);
    setOpErr(null);
    setDelArmed(false);
    if (!selected) return;
    fetchDetail(selected.caseId);
  }, [selected?.caseId, fetchDetail]);

  useEffect(() => () => {
    if (disarmTimer.current) clearTimeout(disarmTimer.current);
  }, []);

  const startEdit = () => {
    if (!detail) return;
    setDraft(detail.content);
    setOpErr(null);
    setEditing(true);
  };

  const saveEdit = async () => {
    if (!selected) return;
    setSaving(true);
    setOpErr(null);
    try {
      await updateVulnCase(selected.caseId, draft);
      setEditing(false);
      bumpRefresh();
      fetchDetail(selected.caseId);
    } catch (e) {
      setOpErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!selected) return;
    if (!delArmed) {
      setDelArmed(true);
      disarmTimer.current = setTimeout(() => setDelArmed(false), 4000);
      return;
    }
    setDelArmed(false);
    setOpErr(null);
    try {
      await deleteVulnCase(selected.caseId);
      clear();
      bumpRefresh();
    } catch (e) {
      setOpErr(e instanceof Error ? e.message : String(e));
    }
  };

  if (!selected) {
    return (
      <div className="flex h-[calc(100vh-12rem)] items-center justify-center text-sm text-slate-400">
        请在左侧「漏洞清单」中按分类选择要浏览的漏洞
      </div>
    );
  }

  return (
    <div className="mt-2 space-y-3">
      {/* 操作栏 */}
      <div className="flex items-center justify-end gap-1.5">
        {!editing ? (
          <>
            <button
              type="button"
              onClick={startEdit}
              title="编辑案例（保存后写回 md 文件）"
              className="flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
            >
              <Pencil className="h-3.5 w-3.5" />
              编辑
            </button>
            <button
              type="button"
              onClick={handleDelete}
              title={delArmed ? '再次点击确认删除' : '删除案例（文件移入 data/backups 备份）'}
              className={`flex items-center gap-1 rounded-md border px-2 py-1 text-xs ${
                delArmed
                  ? 'border-red-500 bg-red-500 font-medium text-white'
                  : 'border-slate-200 text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800'
              }`}
            >
              <Trash2 className="h-3.5 w-3.5" />
              {delArmed ? '确认删除?' : '删除'}
            </button>
          </>
        ) : (
          <>
            <button
              type="button"
              onClick={saveEdit}
              disabled={saving}
              className="flex items-center gap-1 rounded-md bg-indigo-500 px-2 py-1 text-xs font-medium text-white hover:bg-indigo-600 disabled:opacity-50"
            >
              <Save className="h-3.5 w-3.5" />
              {saving ? '保存中…' : '保存'}
            </button>
            <button
              type="button"
              onClick={() => setEditing(false)}
              title="取消编辑（不保存）"
              className="flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
            >
              <X className="h-3.5 w-3.5" />
              取消
            </button>
          </>
        )}
      </div>

      {(detailErr || opErr) && (
        <section className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600 dark:border-red-900 dark:bg-red-950/40 dark:text-red-400">
          {detailErr ?? opErr}
        </section>
      )}
      {!isWails() && (
        <section className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-400">
          浏览器模式无案例内容：请用桌面版运行并先在左侧「导入」漏洞清单目录。
        </section>
      )}

      {editing ? (
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          spellCheck={false}
          className="h-[calc(100vh-13rem)] w-full resize-y rounded-md border border-slate-200 bg-white p-4 font-mono text-sm leading-6 text-slate-700 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
        />
      ) : (
        detail && (
          <div
            className="markdown-body max-h-[calc(100vh-11rem)] overflow-y-auto rounded-md border border-slate-100 bg-slate-50 p-5 text-sm text-slate-700 dark:border-slate-700 dark:bg-slate-800/50 dark:text-slate-300"
            title={detail.path}
          >
            {renderMarkdown(detail.content, { imgSrcResolver: resolveImg })}
          </div>
        )
      )}
    </div>
  );
}
