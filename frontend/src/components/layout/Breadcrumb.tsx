import { ChevronRight, Home } from 'lucide-react';
import { Link } from 'react-router-dom';

export interface Crumb {
  label: string;
  /** 有 to 则渲染为链接，否则渲染为当前项（纯文本高亮） */
  to?: string;
}

/**
 * 面包屑：题型 > 子分类 > 子子分类 > 工具名。
 * 首项固定为「首页」图标，其余按 items 顺序渲染。
 */
export default function Breadcrumb({ items }: { items: Crumb[] }) {
  return (
    <nav className="flex flex-wrap items-center gap-1 text-sm text-slate-500 dark:text-slate-400">
      <Link
        to="/"
        className="rounded p-0.5 hover:text-slate-900 dark:hover:text-slate-100"
        aria-label="首页"
      >
        <Home className="h-4 w-4" />
      </Link>

      {items.map((item, index) => (
        <span key={index} className="flex items-center gap-1">
          <ChevronRight className="h-3.5 w-3.5 text-slate-300 dark:text-slate-600" />
          {item.to ? (
            <Link to={item.to} className="hover:text-indigo-500">
              {item.label}
            </Link>
          ) : (
            <span className="font-medium text-slate-700 dark:text-slate-200">
              {item.label}
            </span>
          )}
        </span>
      ))}
    </nav>
  );
}
