import { memo, useMemo } from 'react';
import { ChevronRight } from 'lucide-react';
import { NavLink, useLocation } from 'react-router-dom';

import type { CategoryNode } from '@/types';

import { getIcon } from '@/lib/icons';
import { getDirectTools, getToolById } from '@/mock';
import { categoriesForMode } from '@/mock/modes';
import { useCategoryStore } from '@/stores/useCategoryStore';
import { useModeStore } from '@/stores/useModeStore';

/**
 * 递归分类树组件。
 *
 * 关键逻辑：
 * 1. 路径自动展开：当前路由对应的分类（含从 `/t/:toolId` 反查到的分类）及其
 *    祖先节点自动展开。
 * 2. 搜索联动：命中节点（按 name/description 模糊匹配）及其所有祖先自动展开并
 *    高亮；未命中的分支保持原有手动折叠/展开状态（不做额外改动）。
 * 3. 当前项高亮：active 路由高亮（优先于搜索高亮）。
 *
 * 性能：TreeNode 用 React.memo + 布尔 props（isExpanded/isMatched）做浅比较，
 * 单个节点折叠/搜索命中时，其余节点不重渲染。
 */

/**
 * 第一侧边栏最多展示两层（顶层分类 + 一级子分类）。
 * 更深层（如「古典密码 → 凯撒/维吉尼亚/仿射」）交给第二侧边栏展示，
 * 避免两侧重复。depth 小于该值的节点才显示折叠箭头。
 */
const MAX_EXPAND_DEPTH = 1;

/** 按关键字收集命中节点 id 集合 */
function collectMatches(nodes: CategoryNode[], keyword: string): Set<string> {
  const kw = keyword.trim().toLowerCase();
  const matches = new Set<string>();
  if (!kw) return matches;

  const walk = (list: CategoryNode[]) => {
    for (const n of list) {
      const hitName = n.name.toLowerCase().includes(kw);
      const hitDesc = n.description?.toLowerCase().includes(kw) ?? false;
      if (hitName || hitDesc) matches.add(n.id);
      if (n.children) walk(n.children);
    }
  };
  walk(nodes);
  return matches;
}

/** 给定"必须暴露"的节点集合，返回需要展开的 id（含完整祖先链） */
function expandTo(nodes: CategoryNode[], reveal: Set<string>): Set<string> {
  const out = new Set<string>();
  const walk = (node: CategoryNode, trail: string[]) => {
    const chain = [...trail, node.id];
    if (reveal.has(node.id)) chain.forEach((id) => out.add(id));
    node.children?.forEach((c) => walk(c, chain));
  };
  nodes.forEach((n) => walk(n, []));
  return out;
}

/** 从当前路径推导活跃分类 id */
function getActiveCategoryId(pathname: string): string | undefined {
  if (pathname.startsWith('/c/')) return pathname.slice(3);
  if (pathname.startsWith('/t/')) return getToolById(pathname.slice(3))?.categoryId;
  return undefined;
}

interface TreeNodeProps {
  node: CategoryNode;
  depth: number;
  isExpanded: boolean;
  isMatched: boolean;
  /** 参与 memo 引用比较：展开状态变更时能正确传播到子节点 */
  expanded: Set<string>;
  matched: Set<string>;
}

/** 递归节点渲染（memo + 布尔浅比较，避免兄弟节点无谓重渲染） */
const TreeNode = memo(
  function TreeNode({ node, depth, isExpanded, isMatched, expanded, matched }: TreeNodeProps) {
    const { toggleExpanded } = useCategoryStore();

    const hasChildren = depth < MAX_EXPAND_DEPTH && !!node.children?.length;
    // 叶子分类（无子分类）可在侧边栏直接展开显示其下工具，如 信息收集 → 空间测绘/AK 管理
    const directTools = node.children?.length ? undefined : getDirectTools(node.id);
    const expandable = hasChildren || !!directTools?.length;
    const Icon = getIcon(node.icon);

    return (
      <li>
        <div className="flex items-center" style={{ paddingLeft: depth * 12 }}>
          {/* 展开/折叠按钮（无子项时占位对齐） */}
          {expandable ? (
            <button
              type="button"
              onClick={() => toggleExpanded(node.id)}
              className="shrink-0 rounded p-0.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800"
              aria-label={isExpanded ? '折叠' : '展开'}
            >
              <ChevronRight
                className={`h-3.5 w-3.5 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
              />
            </button>
          ) : (
            <span className="w-[18px] shrink-0" />
          )}

          <NavLink
            to={`/c/${node.id}`}
            className={({ isActive }) => {
              const base =
                'ml-0.5 flex flex-1 items-center gap-1.5 rounded-md px-2 py-1.5 text-sm transition-colors';
              if (isActive)
                return `${base} bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400`;
              if (isMatched)
                return `${base} bg-amber-50 font-medium text-amber-700 dark:bg-amber-500/10 dark:text-amber-300`;
              return `${base} text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800`;
            }}
          >
            <Icon className="h-4 w-4 shrink-0" />
            <span className="truncate">{node.name}</span>
          </NavLink>
        </div>

        {/* 递归子分类节点 */}
        {hasChildren && isExpanded && (
          <ul>
            {node.children!.map((child) => (
              <TreeNode
                key={child.id}
                node={child}
                depth={depth + 1}
                isExpanded={expanded.has(child.id)}
                isMatched={matched.has(child.id)}
                expanded={expanded}
                matched={matched}
              />
            ))}
          </ul>
        )}

        {/* 叶子分类展开时直挂的工具项 */}
        {directTools && isExpanded && (
          <ul>
            {directTools.map((tool) => (
              <li key={tool.id}>
                <NavLink
                  to={`/t/${tool.id}`}
                  style={{ paddingLeft: (depth + 1) * 12 + 18 }}
                  className={({ isActive }) =>
                    `ml-0.5 block truncate rounded-md px-2 py-1 text-[13px] transition-colors ${
                      isActive
                        ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                        : 'text-slate-500 hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200'
                    }`
                  }
                >
                  {tool.name}
                </NavLink>
              </li>
            ))}
          </ul>
        )}
      </li>
    );
  },
  (prev, next) =>
    prev.node === next.node &&
    prev.depth === next.depth &&
    prev.isExpanded === next.isExpanded &&
    prev.isMatched === next.isMatched &&
    prev.expanded === next.expanded &&
    prev.matched === next.matched,
);

export default function CategoryTree() {
  const { categories: allCategories, expandedIds, searchKeyword } = useCategoryStore();
  const mode = useModeStore((s) => s.mode);
  const location = useLocation();

  // 按当前模式过滤顶层分类（共享分类在多模式下均可见）
  const categories = useMemo(
    () => (mode ? categoriesForMode(mode) : allCategories),
    [mode, allCategories],
  );

  // 合并：手动展开 + 路径自动展开 + 搜索命中展开；并计算命中集
  const { expanded, matched } = useMemo(() => {
    const matched = collectMatches(categories, searchKeyword);

    const activeId = getActiveCategoryId(location.pathname);
    const reveal = new Set<string>(matched);
    if (activeId) reveal.add(activeId);

    const expanded = new Set(expandedIds);
    expandTo(categories, reveal).forEach((id) => expanded.add(id));

    return { expanded, matched };
  }, [categories, expandedIds, searchKeyword, location.pathname]);

  return (
    <ul className="space-y-0.5">
      {categories.map((node) => (
        <TreeNode
          key={node.id}
          node={node}
          depth={0}
          isExpanded={expanded.has(node.id)}
          isMatched={matched.has(node.id)}
          expanded={expanded}
          matched={matched}
        />
      ))}
    </ul>
  );
}
