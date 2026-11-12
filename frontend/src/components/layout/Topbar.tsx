import { useState } from 'react';
import {
  ArrowLeftRight,
  History,
  Menu,
  Moon,
  PanelLeft,
  PanelLeftClose,
  Search,
  Settings,
  Star,
  Sun,
  Terminal,
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';

import type { ToolDefinition } from '@/types';

import { getIcon } from '@/lib/icons';
import { toolMap } from '@/mock';
import { isToolInMode, MODE_CONFIG } from '@/mock/modes';
import { useHistoryStore } from '@/stores/useHistoryStore';
import { useModeStore } from '@/stores/useModeStore';
import { useToolStore } from '@/stores/useToolStore';
import { useUiStore } from '@/stores/useUiStore';

/**
 * 顶栏：Logo + 模式徽标 | 全局搜索(Ctrl/⌘+K) | 收藏 | 历史记录 | 切换模式 | 主题切换。
 * 收藏 / 历史以下拉面板呈现（按当前模式过滤），点击工具直达。
 */
export default function Topbar() {
  const { theme, toggleTheme, sidebarCollapsed, toggleSidebar, setMobileSidebarOpen, setCommandPaletteOpen } =
    useUiStore();
  const { favorites } = useToolStore();
  const { getRecentToolIds } = useHistoryStore();
  const mode = useModeStore((s) => s.mode);
  const setMode = useModeStore((s) => s.setMode);
  const navigate = useNavigate();

  const [menu, setMenu] = useState<'fav' | 'history' | null>(null);

  const inMode = (t: ToolDefinition) => !mode || isToolInMode(t, mode);
  const favoriteTools = favorites
    .map((id) => toolMap.get(id))
    .filter((t): t is ToolDefinition => Boolean(t))
    .filter(inMode);
  const recentTools = getRecentToolIds()
    .map((id) => toolMap.get(id))
    .filter((t): t is ToolDefinition => Boolean(t))
    .filter(inMode)
    .slice(0, 8);

  const modeInfo = mode ? MODE_CONFIG[mode] : null;
  const ModeIcon = getIcon(modeInfo?.icon);

  const switchMode = () => {
    setMode(null);
    setMenu(null);
    navigate('/mode');
  };

  const go = (id: string) => {
    navigate(`/t/${id}`);
    setMenu(null);
  };

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b border-slate-200 bg-white px-4 dark:border-slate-800 dark:bg-slate-900">
      {/* 移动端：打开抽屉 */}
      <button
        className="rounded-md p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 lg:hidden"
        onClick={() => setMobileSidebarOpen(true)}
        aria-label="打开侧边栏"
      >
        <Menu className="h-5 w-5" />
      </button>

      {/* Logo */}
      <div className="flex items-center gap-2 font-semibold">
        <Terminal className="h-5 w-5 text-indigo-500" />
        <span>CTF-NET</span>
      </div>

      {/* 当前模式徽标 */}
      {modeInfo && (
        <span className="ml-1 flex items-center gap-1 rounded-full border border-indigo-200 bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-600 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-400">
          <ModeIcon className="h-3 w-3" />
          {modeInfo.label}
        </span>
      )}

      {/* 侧边栏折叠（桌面端） */}
      <button
        className="hidden rounded-md p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 lg:flex"
        onClick={toggleSidebar}
        title={sidebarCollapsed ? '展开侧边栏' : '折叠侧边栏'}
      >
        {sidebarCollapsed ? <PanelLeft className="h-5 w-5" /> : <PanelLeftClose className="h-5 w-5" />}
      </button>

      {/* 全局搜索 */}
      <button
        className="ml-4 hidden flex-1 items-center gap-2 rounded-md border border-slate-200 px-3 py-1.5 text-sm text-slate-400 hover:border-slate-300 dark:border-slate-700 dark:hover:border-slate-600 sm:flex sm:max-w-xs"
        onClick={() => setCommandPaletteOpen(true)}
      >
        <Search className="h-4 w-4" />
        <span>搜索工具…</span>
        <kbd className="ml-auto rounded border border-slate-200 px-1.5 text-xs dark:border-slate-700">Ctrl K</kbd>
      </button>

      {/* 右侧操作区 */}
      <div className="relative ml-auto flex items-center gap-1">
        {/* 移动端搜索入口 */}
        <button
          className="rounded-md p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 sm:hidden"
          onClick={() => setCommandPaletteOpen(true)}
          title="搜索工具"
        >
          <Search className="h-5 w-5" />
        </button>

        {/* 收藏下拉 */}
        <div className="relative">
          <button
            className="relative rounded-md p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
            onClick={() => setMenu(menu === 'fav' ? null : 'fav')}
            title="我的收藏"
          >
            <Star className="h-5 w-5" />
            {favorites.length > 0 && (
              <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-indigo-500 px-1 text-[10px] font-medium text-white">
                {favorites.length}
              </span>
            )}
          </button>
          {menu === 'fav' && (
            <DropdownPanel title="我的收藏" empty="暂无收藏" tools={favoriteTools} onPick={go} />
          )}
        </div>

        {/* 历史下拉 */}
        <div className="relative">
          <button
            className="rounded-md p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
            onClick={() => setMenu(menu === 'history' ? null : 'history')}
            title="历史记录"
          >
            <History className="h-5 w-5" />
          </button>
          {menu === 'history' && (
            <DropdownPanel title="最近使用" empty="暂无记录" tools={recentTools} onPick={go} />
          )}
        </div>

        {/* 设置页 */}
        <button
          className="rounded-md p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
          onClick={() => navigate('/settings')}
          title="设置"
        >
          <Settings className="h-5 w-5" />
        </button>

        {/* 切换模式：返回模式选择页 */}
        <button
          className="rounded-md p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
          onClick={switchMode}
          title="切换模式"
        >
          <ArrowLeftRight className="h-5 w-5" />
        </button>

        {/* 主题切换 */}
        <button
          className="rounded-md p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
          onClick={toggleTheme}
          title="主题切换"
        >
          {theme === 'dark' ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
        </button>

        {/* 点击外部关闭下拉 */}
        {menu && <div className="fixed inset-0 z-30" onClick={() => setMenu(null)} />}
      </div>
    </header>
  );
}

/** 顶栏下拉面板 */
function DropdownPanel({
  title,
  empty,
  tools,
  onPick,
}: {
  title: string;
  empty: string;
  tools: ToolDefinition[];
  onPick: (id: string) => void;
}) {
  return (
    <div className="absolute right-0 top-full z-40 mt-1 w-60 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-xl dark:border-slate-700 dark:bg-slate-900">
      <div className="border-b border-slate-200 px-3 py-2 text-xs font-medium text-slate-400 dark:border-slate-800">
        {title}
      </div>
      <ul className="max-h-72 overflow-auto p-1">
        {tools.length === 0 ? (
          <li className="px-3 py-6 text-center text-sm text-slate-400">{empty}</li>
        ) : (
          tools.map((t) => (
            <li key={t.id}>
              <button
                type="button"
                onClick={() => onPick(t.id)}
                className="block w-full truncate rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                {t.name}
              </button>
            </li>
          ))
        )}
      </ul>
    </div>
  );
}
