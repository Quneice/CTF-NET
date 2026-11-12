import { useEffect, useMemo } from 'react';
import { Search } from 'lucide-react';

import ToolCard from '@/components/tool/ToolCard';
import type { ToolDefinition } from '@/types';
import { toolMap } from '@/mock';
import { isToolInMode, MODE_CONFIG, toolsForMode } from '@/mock/modes';
import { useHistoryStore } from '@/stores/useHistoryStore';
import { useModeStore } from '@/stores/useModeStore';
import { useToolStore } from '@/stores/useToolStore';
import { useUiStore } from '@/stores/useUiStore';

/**
 * 首页：搜索入口 + 最近使用 + 精选工具。
 */
export default function HomePage() {
  const { tools } = useToolStore();
  const { getRecentToolIds } = useHistoryStore();
  const { setCommandPaletteOpen } = useUiStore();
  const mode = useModeStore((s) => s.mode);

  // 当前模式可见的工具集合（模式守卫保证 mode 非空，此处仅兜底）
  const modeTools = useMemo<ToolDefinition[]>(
    () => (mode ? toolsForMode(mode) : tools),
    [mode, tools],
  );
  const modeLabel = mode ? MODE_CONFIG[mode].label : '';

  useEffect(() => {
    document.title = mode ? `CTF-NET — ${MODE_CONFIG[mode].label}` : 'CTF-NET 集成工具箱';
  }, [mode]);

  const recentTools = useMemo<ToolDefinition[]>(
    () =>
      getRecentToolIds()
        .map((id) => toolMap.get(id))
        .filter((t): t is ToolDefinition => Boolean(t))
        .filter((t) => !mode || isToolInMode(t, mode))
        .slice(0, 6),
    [getRecentToolIds, mode],
  );

  // 精选工具：取当前模式下前 12 个完整定义的工具作为展示
  const popularTools = useMemo(() => modeTools.slice(0, 12), [modeTools]);

  return (
    <div className="p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">首页{modeLabel ? ` · ${modeLabel}` : ''}</h1>
          <p className="mt-1 text-slate-500 dark:text-slate-400">
            当前模式收录 {modeTools.length} 个工具，持续增长中
          </p>
        </div>
        <button
          type="button"
          onClick={() => setCommandPaletteOpen(true)}
          className="flex items-center gap-2 rounded-md border border-slate-200 px-3 py-2 text-sm text-slate-600 hover:border-indigo-300 hover:text-indigo-600 dark:border-slate-700 dark:text-slate-300"
        >
          <Search className="h-4 w-4" />
          搜索工具
          <kbd className="rounded border border-slate-200 px-1.5 text-xs dark:border-slate-700">Ctrl K</kbd>
        </button>
      </div>

      {recentTools.length > 0 && (
        <section className="mt-8">
          <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400">最近使用</h2>
          <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {recentTools.map((t) => (
              <ToolCard key={t.id} tool={t} />
            ))}
          </div>
        </section>
      )}

      <section className="mt-8">
        <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400">精选工具</h2>
        <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {popularTools.map((t) => (
            <ToolCard key={t.id} tool={t} />
          ))}
        </div>
      </section>
    </div>
  );
}
