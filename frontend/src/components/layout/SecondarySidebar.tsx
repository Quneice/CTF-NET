import { useEffect, useState } from 'react';
import { ChevronRight } from 'lucide-react';
import { NavLink, useLocation } from 'react-router-dom';

import type { CategoryNode, ToolDefinition } from '@/types';

import { getIcon } from '@/lib/icons';
import { getCategoryById, getToolById, toolsByCategory } from '@/mock';
import VulnLibraryPanel from '@/components/poc/VulnLibraryPanel';
import { useCategoryStore } from '@/stores/useCategoryStore';

/**
 * 二级侧边栏：展示「当前分类」的下一层内容（子分类 + 直接挂载的工具）。
 *
 * - 一级侧边栏选中某分类（或进入某工具页）后，这里列出该分类的直接子分类与工具；
 * - 子分类渲染为可展开分组（下拉），工具渲染为跳转链接；
 * - 当前路由对应的工具/子分类会自动展开并高亮。
 */

/** 从当前路径反推当前活跃的分类 id（用于子分类分组自动展开） */
function getActiveCategoryId(pathname: string): string | undefined {
  if (pathname.startsWith('/c/')) return pathname.slice(3);
  if (pathname.startsWith('/t/')) return getToolById(pathname.slice(3))?.categoryId;
  return undefined;
}

/** 判断 targetId 是否位于 node 的子树内（含自身） */
function isWithin(node: CategoryNode, targetId: string | undefined): boolean {
  if (!targetId) return false;
  if (node.id === targetId) return true;
  return node.children?.some((c) => isWithin(c, targetId)) ?? false;
}

export default function SecondarySidebar() {
  const location = useLocation();
  const { selectedCategoryId, setSelectedCategoryId } = useCategoryStore();

  const pathname = location.pathname;
  const routeCategoryId = pathname.startsWith('/c/') ? pathname.slice(3) : undefined;
  const toolCategoryId = pathname.startsWith('/t/') ? getToolById(pathname.slice(3))?.categoryId : undefined;

  // 进入分类页时更新锚定分类；进入工具页时沿用上次锚定（无则回退到工具所属分类）
  useEffect(() => {
    if (routeCategoryId) setSelectedCategoryId(routeCategoryId);
  }, [routeCategoryId, setSelectedCategoryId]);

  const categoryId =
    routeCategoryId ?? (toolCategoryId ? selectedCategoryId ?? toolCategoryId : undefined);
  const category = categoryId ? getCategoryById(categoryId) : undefined;

  const childSubs = category?.children ?? [];
  const directTools = categoryId ? (toolsByCategory.get(categoryId) ?? []) : [];

  // 漏洞浏览器工具页：二级侧边栏切换为漏洞清单（按分类选择漏洞）
  const isPocScan = pathname === '/t/pentest.vuln.poc-scan';

  if (isPocScan) {
    return (
      <aside className="hidden w-56 shrink-0 flex-col border-r border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40 lg:flex">
        <header className="flex items-center gap-2 border-b border-slate-200 px-3 py-3 dark:border-slate-800">
          <span className="truncate text-sm font-semibold text-slate-700 dark:text-slate-200">
            漏洞清单
          </span>
        </header>
        <VulnLibraryPanel />
      </aside>
    );
  }

  // 无锚定分类，或当前分类为叶子（无子分类，其工具已在一级侧边栏下拉展开）时，隐藏第二侧边栏
  if (!category || childSubs.length === 0) {
    return null;
  }

  return (
    <aside className="hidden w-56 shrink-0 flex-col border-r border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40 lg:flex">
      <header className="flex items-center gap-2 border-b border-slate-200 px-3 py-3 dark:border-slate-800">
        <span className="truncate text-sm font-semibold text-slate-700 dark:text-slate-200">
          {category.name}
        </span>
        <span className="ml-auto shrink-0 text-xs text-slate-400">
          {childSubs.length + directTools.length}
        </span>
      </header>

      <nav className="flex-1 overflow-y-auto p-2">
        {childSubs.map((sub) => (
          <SubcategoryGroup key={sub.id} node={sub} />
        ))}
        {directTools.map((tool) => (
          <ToolItem key={tool.id} tool={tool} />
        ))}
      </nav>
    </aside>
  );
}

/** 子分类分组：点击展开/收起，展示其下的子分类与直接工具 */
function SubcategoryGroup({ node }: { node: CategoryNode }) {
  const location = useLocation();
  const activeId = getActiveCategoryId(location.pathname);
  const containsActive = isWithin(node, activeId);

  const [open, setOpen] = useState(containsActive);
  useEffect(() => {
    if (containsActive) setOpen(true);
  }, [containsActive]);

  const Icon = getIcon(node.icon);
  const childSubs = node.children ?? [];
  const directTools = toolsByCategory.get(node.id) ?? [];

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
      >
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-slate-400 transition-transform ${open ? 'rotate-90' : ''}`}
        />
        <Icon className="h-4 w-4 shrink-0 text-slate-400" />
        <span className="truncate">{node.name}</span>
      </button>

      {open && (
        <div className="ml-3 border-l border-slate-200 pl-2 dark:border-slate-800">
          {childSubs.map((sub) => (
            <SubcategoryGroup key={sub.id} node={sub} />
          ))}
          {/* 叶子节点的工具已在一级侧边栏下拉中列出，避免重复 */}
          {childSubs.length > 0 &&
            directTools.map((tool) => <ToolItem key={tool.id} tool={tool} />)}
        </div>
      )}
    </div>
  );
}

/** 工具项：跳转 `/t/:toolId`，当前工具高亮 */
function ToolItem({ tool }: { tool: ToolDefinition }) {
  return (
    <NavLink
      to={`/t/${tool.id}`}
      className={({ isActive }) =>
        `block truncate rounded-md px-2 py-1.5 text-sm transition-colors ${
          isActive
            ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
            : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
        }`
      }
    >
      {tool.name}
    </NavLink>
  );
}
