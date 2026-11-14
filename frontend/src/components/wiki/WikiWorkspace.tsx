import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Pencil, Plus, Search, Trash2, X } from 'lucide-react';

import type { WikiCategory } from '@/mock/redteamWiki';
import { getIcon } from '@/lib/icons';
import { renderMarkdown } from '@/lib/markdown';
import type { WikiEntry } from '@/lib/wiki';

/**
 * 通用 Wiki 工作区:浏览/检索/增删改参考资料条目。
 *
 * 由 RedTeamWikiWorkspace(红队 Wiki)与 ReverseWikiWorkspace(逆向 Wiki)
 * 以薄壳形式注入分类与数据适配器复用;左侧为分类导航 + 条目列表,
 * 右侧为条目详情(markdown 渲染),可选 topWidget 渲染在两栏布局上方。
 */
export interface WikiAdapter {
  list(category: string): Promise<WikiEntry[]>;
  create(category: string, title: string, content: string): Promise<WikiEntry>;
  update(id: string, title: string, content: string): Promise<WikiEntry>;
  delete(id: string): Promise<void>;
}

export interface WikiWorkspaceProps {
  categories: WikiCategory[];
  categoryById: (id: string) => WikiCategory | undefined;
  adapter: WikiAdapter;
  topWidget?: ReactNode;
}

export default function WikiWorkspace({ categories, categoryById, adapter, topWidget }: WikiWorkspaceProps) {
  const [activeCategory, setActiveCategory] = useState(categories[0]?.id ?? '');
  const [entries, setEntries] = useState<WikiEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [keyword, setKeyword] = useState('');
  const [modal, setModal] = useState<null | { mode: 'create' | 'edit'; entry?: WikiEntry }>(null);

  const load = useCallback(
    async (category: string) => {
      setLoading(true);
      try {
        const list = await adapter.list(category);
        setEntries(list);
        setSelectedId((prev) => (prev && list.some((e) => e.id === prev) ? prev : list[0]?.id ?? null));
      } finally {
        setLoading(false);
      }
    },
    [adapter],
  );

  useEffect(() => {
    void load(activeCategory);
  }, [activeCategory, load]);

  const selected = useMemo(
    () => entries.find((e) => e.id === selectedId) ?? null,
    [entries, selectedId],
  );

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase();
    if (!kw) return entries;
    return entries.filter(
      (e) => e.title.toLowerCase().includes(kw) || e.content.toLowerCase().includes(kw),
    );
  }, [entries, keyword]);

  return (
    <>
      {topWidget && <div className="mb-3">{topWidget}</div>}
      <div className="mt-4 flex min-h-[560px] gap-4">
        {/* 左栏：分类 + 条目列表 */}
        <div className="flex w-64 shrink-0 flex-col gap-3">
          {/* 分类导航 */}
          <nav className="grid grid-cols-2 gap-1.5">
            {categories.map((cat) => {
              const Icon = getIcon(cat.icon);
              const active = cat.id === activeCategory;
              return (
                <button
                  key={cat.id}
                  type="button"
                  onClick={() => setActiveCategory(cat.id)}
                  title={cat.description}
                  className={`flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-left text-xs transition-colors ${
                    active
                      ? 'border-indigo-300 bg-indigo-50 font-medium text-indigo-600 dark:border-indigo-500/40 dark:bg-indigo-500/10 dark:text-indigo-400'
                      : 'border-slate-200 text-slate-600 hover:bg-slate-100 dark:border-slate-800 dark:text-slate-300 dark:hover:bg-slate-800'
                  }`}
                >
                  <Icon className="h-3.5 w-3.5 shrink-0" />
                  <span className="truncate">{cat.name}</span>
                </button>
              );
            })}
          </nav>

          {/* 搜索 + 新建 */}
          <div className="flex items-center gap-1.5">
            <div className="relative flex-1">
              <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <input
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder="搜索标题/内容…"
                className="w-full rounded-md border border-slate-200 bg-white py-1.5 pl-7 pr-2 text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
              />
            </div>
            <button
              type="button"
              onClick={() => setModal({ mode: 'create' })}
              className="flex items-center gap-1 rounded-md bg-indigo-500 px-2 py-1.5 text-xs font-medium text-white hover:bg-indigo-600"
            >
              <Plus className="h-3.5 w-3.5" />
              新建
            </button>
          </div>

          {/* 条目列表 */}
          <div className="flex-1 overflow-y-auto rounded-md border border-slate-200 dark:border-slate-800">
            {loading ? (
              <div className="p-3 text-xs text-slate-400">加载中…</div>
            ) : filtered.length === 0 ? (
              <div className="p-3 text-xs text-slate-400">暂无条目</div>
            ) : (
              <ul>
                {filtered.map((e) => (
                  <li key={e.id}>
                    <button
                      type="button"
                      onClick={() => setSelectedId(e.id)}
                      className={`block w-full truncate border-b border-slate-100 px-3 py-2 text-left text-xs transition-colors last:border-0 dark:border-slate-800 ${
                        e.id === selectedId
                          ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                          : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
                      }`}
                    >
                      {e.title}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        {/* 右栏：详情 */}
        <div className="flex min-w-0 flex-1 flex-col rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          {selected ? (
            <>
              <div className="flex items-center gap-2 border-b border-slate-200 px-4 py-3 dark:border-slate-800">
                <h2 className="min-w-0 flex-1 truncate text-base font-semibold text-slate-800 dark:text-slate-100">
                  {selected.title}
                </h2>
                <span className="shrink-0 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                  {categoryById(selected.category)?.name ?? selected.category}
                </span>
                <button
                  type="button"
                  onClick={() => setModal({ mode: 'edit', entry: selected })}
                  className="flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                >
                  <Pencil className="h-3.5 w-3.5" />
                  编辑
                </button>
                <button
                  type="button"
                  onClick={() => void handleDelete(selected, load, activeCategory, adapter)}
                  className="flex items-center gap-1 rounded-md border border-red-200 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-500/30 dark:text-red-400 dark:hover:bg-red-500/10"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  删除
                </button>
              </div>
              <div className="markdown-body flex-1 overflow-y-auto px-4 py-3 text-sm text-slate-700 dark:text-slate-300">
                {renderMarkdown(selected.content)}
              </div>
            </>
          ) : (
            <div className="flex h-full items-center justify-center p-6 text-sm text-slate-400">
              在左侧选择一个条目查看详情
            </div>
          )}
        </div>

        {modal && (
          <WikiEntryModal
            mode={modal.mode}
            entry={modal.entry}
            defaultCategory={activeCategory}
            categories={categories}
            adapter={adapter}
            onClose={() => setModal(null)}
            onSaved={() => {
              setModal(null);
              void load(activeCategory);
            }}
          />
        )}
      </div>
    </>
  );
}

/** 删除确认 + 调用 */
async function handleDelete(
  entry: WikiEntry,
  load: (category: string) => Promise<void>,
  activeCategory: string,
  adapter: WikiAdapter,
): Promise<void> {
  if (!window.confirm(`确定删除「${entry.title}」？此操作不可撤销。`)) return;
  await adapter.delete(entry.id);
  await load(activeCategory);
}

/** 新建 / 编辑弹窗 */
function WikiEntryModal({
  mode,
  entry,
  defaultCategory,
  categories,
  adapter,
  onClose,
  onSaved,
}: {
  mode: 'create' | 'edit';
  entry?: WikiEntry;
  defaultCategory: string;
  categories: WikiCategory[];
  adapter: WikiAdapter;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [category, setCategory] = useState(entry?.category ?? defaultCategory);
  const [title, setTitle] = useState(entry?.title ?? '');
  const [content, setContent] = useState(entry?.content ?? '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async () => {
    if (!title.trim()) {
      setError('标题不能为空');
      return;
    }
    setSaving(true);
    setError(null);
    try {
      if (mode === 'create') {
        await adapter.create(category, title.trim(), content);
      } else if (entry) {
        await adapter.update(entry.id, title.trim(), content);
      }
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="flex max-h-[85vh] w-full max-w-2xl flex-col rounded-lg bg-white shadow-xl dark:bg-slate-900">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3 dark:border-slate-800">
          <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100">
            {mode === 'create' ? '新建条目' : '编辑条目'}
          </h3>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex-1 space-y-3 overflow-y-auto px-4 py-3">
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-300">
              分类
            </label>
            <select
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              disabled={mode === 'edit'}
              className="w-full rounded-md border border-slate-300 px-2 py-1.5 text-sm disabled:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:disabled:bg-slate-800"
            >
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-300">
              标题
            </label>
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="条目标题"
              className="w-full rounded-md border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
            />
          </div>

          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-300">
              内容（Markdown，代码块用 4 空格缩进）
            </label>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              rows={14}
              className="w-full resize-y rounded-md border border-slate-300 px-2 py-1.5 font-mono text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
            />
          </div>

          {error && <p className="text-xs text-red-500">{error}</p>}
        </div>

        <div className="flex justify-end gap-2 border-t border-slate-200 px-4 py-3 dark:border-slate-800">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            取消
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="rounded-md bg-indigo-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}
