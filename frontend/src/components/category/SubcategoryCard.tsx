import { memo } from 'react';
import { Link } from 'react-router-dom';

import type { CategoryNode } from '@/types';

import { getIcon } from '@/lib/icons';
import { getToolsUnderCategory } from '@/mock';

/**
 * 子分类入口卡片：展示在分类页顶部（当分类同时含子分类与工具时，
 * 先渲染这些入口卡片，再渲染工具卡片）。memo 化避免列表无谓重渲染。
 */
export default memo(function SubcategoryCard({ node }: { node: CategoryNode }) {
  const Icon = getIcon(node.icon);
  const count = getToolsUnderCategory(node.id).length;

  return (
    <Link
      to={`/c/${node.id}`}
      className="group rounded-lg border border-slate-200 bg-white p-4 transition hover:border-indigo-300 hover:shadow-sm dark:border-slate-800 dark:bg-slate-900 dark:hover:border-indigo-500/50"
    >
      <div className="flex items-center gap-2">
        <Icon className="h-5 w-5 shrink-0 text-indigo-500" />
        <span className="truncate font-medium text-slate-800 dark:text-slate-100">
          {node.name}
        </span>
      </div>
      {node.description && (
        <p className="mt-2 line-clamp-2 text-sm text-slate-500 dark:text-slate-400">
          {node.description}
        </p>
      )}
      <div className="mt-3 text-xs text-slate-400">{count} 个工具</div>
    </Link>
  );
});
