import { useEffect, useMemo, useRef, useState } from 'react';
import { Search } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

import type { ToolDefinition } from '@/types';

import { searchTools } from '@/lib/search';
import { keywordTagsOf } from '@/lib/task';
import { getCategoryById } from '@/mock';
import { isToolInMode } from '@/mock/modes';
import { useModeStore } from '@/stores/useModeStore';
import { useToolStore } from '@/stores/useToolStore';
import { useUiStore } from '@/stores/useUiStore';

/**
 * 全局命令面板（Ctrl/⌘+K）：对工具 name + description + tags 做模糊搜索，
 * ↑↓ 选择、回车直达工具页、Esc 关闭。空查询时显示收藏。
 */
export default function CommandPalette() {
  const { tools, favorites } = useToolStore();
  const { setCommandPaletteOpen } = useUiStore();
  const mode = useModeStore((s) => s.mode);
  const navigate = useNavigate();

  const [query, setQuery] = useState('');
  const [index, setIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  // 仅搜索当前模式可见的工具（含空查询时的收藏列表）
  const modeTools = useMemo<ToolDefinition[]>(
    () => (mode ? tools.filter((t) => isToolInMode(t, mode)) : tools),
    [tools, mode],
  );

  const results = useMemo<ToolDefinition[]>(() => {
    const q = query.trim();
    if (!q) {
      return favorites
        .map((id) => modeTools.find((t) => t.id === id))
        .filter((t): t is ToolDefinition => Boolean(t));
    }
    return searchTools(modeTools, q);
  }, [query, modeTools, favorites]);

  // 打开时聚焦 + 重置选中
  useEffect(() => {
    inputRef.current?.focus();
  }, []);
  useEffect(() => setIndex(0), [query]);

  const close = () => setCommandPaletteOpen(false);

  const select = (tool: ToolDefinition) => {
    navigate(`/t/${tool.id}`);
    close();
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setIndex((i) => Math.min(i + 1, results.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      const t = results[index];
      if (t) select(t);
    } else if (e.key === 'Escape') {
      close();
    }
  };

  // 全局 Esc 关闭（即使焦点不在输入框）
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 p-4 pt-[15vh]"
      onClick={close}
    >
      <div
        className="w-full max-w-xl overflow-hidden rounded-lg border border-slate-200 bg-white shadow-2xl dark:border-slate-700 dark:bg-slate-900"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 border-b border-slate-200 px-4 py-3 dark:border-slate-800">
          <Search className="h-4 w-4 shrink-0 text-slate-400" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="搜索工具（名称 / 描述 / 标签）…"
            className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-slate-400"
          />
          <kbd className="rounded border border-slate-200 px-1.5 text-xs text-slate-400 dark:border-slate-700">
            Esc
          </kbd>
        </div>

        <ul className="max-h-80 overflow-auto p-2">
          {results.length === 0 ? (
            <li className="px-3 py-6 text-center text-sm text-slate-400">
              {query.trim() ? '无匹配结果' : '暂无收藏'}
            </li>
          ) : (
            results.map((tool, i) => {
              const category = getCategoryById(tool.categoryId);
              return (
                <li key={tool.id}>
                  <button
                    type="button"
                    onClick={() => select(tool)}
                    onMouseEnter={() => setIndex(i)}
                    className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm ${
                      i === index
                        ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300'
                        : 'text-slate-700 dark:text-slate-300'
                    }`}
                  >
                    <span className="truncate font-medium">{tool.name}</span>
                    {/* 分类徽标:分类名 chip + 完整 id(批 5 重名区分同款;whitespace 三人组靠 id 前缀区分) */}
                    <span className="shrink-0 rounded-full border border-slate-200 px-2 py-0.5 text-xs text-slate-400 dark:border-slate-700">
                      {category?.name ?? tool.categoryId}
                    </span>
                    <span className="shrink-0 font-mono text-2xs text-slate-400 opacity-70">
                      {tool.id}
                    </span>
                    <span className="ml-auto shrink-0 text-xs text-slate-400">
                      {keywordTagsOf(tool.tags).slice(0, 2).join(' / ')}
                    </span>
                  </button>
                </li>
              );
            }))}
        </ul>
      </div>
    </div>
  );
}
