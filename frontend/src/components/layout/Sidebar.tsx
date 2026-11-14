import { Briefcase, ChevronsDownUp, ChevronsUpDown, FileSearch2, Globe, LayoutDashboard, MessageSquareText, Search, ShieldAlert, X } from 'lucide-react';
import { NavLink } from 'react-router-dom';

import CategoryTree from '@/components/category/CategoryTree';
import { getIcon } from '@/lib/icons';
import { toolMap } from '@/mock';
import { categoriesForMode } from '@/mock/modes';
import { useCategoryStore } from '@/stores/useCategoryStore';
import { useHistoryStore } from '@/stores/useHistoryStore';
import { useModeStore } from '@/stores/useModeStore';
import { useToolStore } from '@/stores/useToolStore';
import { useUiStore } from '@/stores/useUiStore';

/**
 * 左侧边栏：分类搜索框 + 展开/折叠工具栏 + 递归分类树 + 最近使用/收藏。
 * 折叠时只显示一级分类图标。
 */
export default function Sidebar() {
  const { searchKeyword, setSearchKeyword, expandAll, collapseAll } = useCategoryStore();
  const { favorites } = useToolStore();
  const { getRecentToolIds } = useHistoryStore();
  const { sidebarCollapsed } = useUiStore();
  const mode = useModeStore((s) => s.mode);

  // 折叠态：仅显示一级图标导航
  if (sidebarCollapsed) {
    return <CollapsedSidebar />;
  }

  const favoriteTools = favorites
    .map((id) => toolMap.get(id))
    .filter((t): t is NonNullable<typeof t> => Boolean(t))
    .slice(0, 6);
  const recentTools = getRecentToolIds()
    .map((id) => toolMap.get(id))
    .filter((t): t is NonNullable<typeof t> => Boolean(t))
    .slice(0, 6);

  return (
    <nav className="flex h-full flex-col">
      {/* 对话式渗透入口：两种模式均可见（ai.pentest 为一级分类） */}
      <div className="p-2 pb-0">
        <NavLink
          to="/ai-chat"
          className={({ isActive }) =>
            `flex items-center gap-2 rounded-md px-2 py-1.5 text-sm ${
              isActive
                ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
            }`
          }
        >
          <MessageSquareText className="h-4 w-4 shrink-0" />
          AI 对话
        </NavLink>
      </div>

      {/* AI 渗透工作台：仪表盘 / 发现 / 资产（P0，参考 ARTEX 前端） */}
      <div className="p-2 pb-0">
        <div className="px-2 py-1.5 text-xs font-medium text-slate-400">AI 工作台</div>
        {[
          { to: '/ai-dashboard', label: '仪表盘', Icon: LayoutDashboard },
          { to: '/ai-findings', label: '发现', Icon: ShieldAlert },
          { to: '/ai-assets', label: '资产', Icon: Globe },
        ].map(({ to, label, Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              `flex items-center gap-2 rounded-md px-2 py-1.5 text-sm ${
                isActive
                  ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                  : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
              }`
            }
          >
            <Icon className="h-4 w-4 shrink-0" />
            {label}
          </NavLink>
        ))}
      </div>

      {/* 渗透测试模式专属：项目管理 + 代码审计入口 */}
      {mode === 'pentest' && (
        <div className="p-2 pb-0">
          <NavLink
            to="/projects"
            className={({ isActive }) =>
              `flex items-center gap-2 rounded-md px-2 py-1.5 text-sm ${
                isActive
                  ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                  : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
              }`
            }
          >
            <Briefcase className="h-4 w-4 shrink-0" />
            项目管理
          </NavLink>
          <NavLink
            to="/audit"
            className={({ isActive }) =>
              `mt-0.5 flex items-center gap-2 rounded-md px-2 py-1.5 text-sm ${
                isActive
                  ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                  : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
              }`
            }
          >
            <FileSearch2 className="h-4 w-4 shrink-0" />
            代码审计
          </NavLink>
        </div>
      )}

      {/* 搜索框 */}
      <div className="p-2">
        <div className="flex items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-2 py-1.5 dark:border-slate-700 dark:bg-slate-800">
          <Search className="h-4 w-4 shrink-0 text-slate-400" />
          <input
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            placeholder="搜索分类…"
            className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-slate-400"
          />
          {searchKeyword && (
            <button
              type="button"
              onClick={() => setSearchKeyword('')}
              className="shrink-0 rounded p-0.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
              aria-label="清除搜索"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* 分类标题 + 展开/折叠 */}
      <div className="flex items-center justify-between px-3 pb-1 text-xs text-slate-400">
        <span>分类</span>
        <div className="flex gap-0.5">
          <button
            type="button"
            onClick={expandAll}
            title="全部展开"
            className="rounded p-1 hover:bg-slate-100 dark:hover:bg-slate-800"
          >
            <ChevronsUpDown className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onClick={collapseAll}
            title="全部折叠"
            className="rounded p-1 hover:bg-slate-100 dark:hover:bg-slate-800"
          >
            <ChevronsDownUp className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {/* 递归分类树 */}
      <div className="flex-1 overflow-y-auto px-2 pb-2">
        <CategoryTree />
      </div>

      {/* 底部：最近使用 / 我的收藏 */}
      <div className="max-h-56 overflow-y-auto border-t border-slate-200 p-3 text-xs dark:border-slate-800">
        <SectionTitle>最近使用</SectionTitle>
        <ToolList tools={recentTools} empty="暂无记录" />

        <div className="mt-3">
          <SectionTitle>我的收藏</SectionTitle>
          <ToolList tools={favoriteTools} empty="暂无收藏" />
        </div>
      </div>
    </nav>
  );
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <div className="mb-1 font-medium text-slate-500 dark:text-slate-400">{children}</div>;
}

function ToolList({
  tools,
  empty,
}: {
  tools: { id: string; name: string }[];
  empty: string;
}) {
  if (tools.length === 0) {
    return <div className="px-1 text-slate-400">{empty}</div>;
  }
  return (
    <ul className="space-y-0.5">
      {tools.map((t) => (
        <li key={t.id}>
          <NavLink
            to={`/t/${t.id}`}
            className="block truncate rounded px-1 py-0.5 text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            {t.name}
          </NavLink>
        </li>
      ))}
    </ul>
  );
}

/** 折叠态：仅渲染一级分类图标（按当前模式过滤） */
function CollapsedSidebar() {
  const { categories: allCategories } = useCategoryStore();
  const mode = useModeStore((s) => s.mode);
  const categories = mode ? categoriesForMode(mode) : allCategories;

  return (
    <nav className="flex h-full flex-col items-center gap-1 p-2">
      <NavLink
        to="/ai-chat"
        title="AI 对话"
        className={({ isActive }) =>
          `rounded-md p-2 transition-colors ${
            isActive
              ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
              : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
          }`
        }
      >
        <MessageSquareText className="h-5 w-5" />
      </NavLink>
      <NavLink
        to="/ai-dashboard"
        title="AI 工作台"
        className={({ isActive }) =>
          `rounded-md p-2 transition-colors ${
            isActive
              ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
              : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
          }`
        }
      >
        <LayoutDashboard className="h-5 w-5" />
      </NavLink>
      {mode === 'pentest' && (
        <NavLink
          to="/projects"
          title="项目管理"
          className={({ isActive }) =>
            `rounded-md p-2 transition-colors ${
              isActive
                ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
            }`
          }
        >
          <Briefcase className="h-5 w-5" />
        </NavLink>
      )}
      {categories.map((c) => {
        const Icon = getIcon(c.icon);
        return (
          <NavLink
            key={c.id}
            to={`/c/${c.id}`}
            title={c.name}
            className={({ isActive }) =>
              `rounded-md p-2 transition-colors ${
                isActive
                  ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                  : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
              }`
            }
          >
            <Icon className="h-5 w-5" />
          </NavLink>
        );
      })}
    </nav>
  );
}
