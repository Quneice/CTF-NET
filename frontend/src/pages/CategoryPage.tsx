import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';

import SubcategoryCard from '@/components/category/SubcategoryCard';
import Breadcrumb from '@/components/layout/Breadcrumb';
import ToolCard from '@/components/tool/ToolCard';
import { availableTasks, filterByTask } from '@/lib/task';
import { getCategoryById, getCategoryPath, getToolsUnderCategory, toolsByCategory } from '@/mock';

/** 全部 chip 的哨兵值（非真实分类 id） */
const ALL = '__all__';

/** 段分类筛选 chip（design-system.md §3.3 左部：导航降级节点） */
function FilterChip({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`shrink-0 rounded-full border px-3 py-1 text-sm transition-colors duration-base ${
        active
          ? 'border-brand bg-brand-subtle text-brand'
          : 'border-line bg-surface-raised text-ink-2 hover:border-brand/50'
      }`}
    >
      {label}
    </button>
  );
}

/**
 * 分类页：面包屑 + 分组筛选 chips + 子分类入口卡片 + 工具卡片网格。
 *
 * 子分类分两类：
 * - 普通子分类：渲染入口卡片（点击进入 /c/:id）。
 * - 分组子分类（node.segment，ia.md §4 导航降级节点 + 批 7 纯分组分类）：不渲染
 *   入口卡片，改为父分类页顶部的筛选 chips（路由保留，直接访问 /c/:segmentId 仍可达）。
 * 「全部」chip 展示本层直挂工具；纯分组分类（本层无直挂，如 forensics）则展示
 * 该分类下全部工具（含各分组）。选中 chip 时展示该分组下的工具。
 * 另有「任务」chips 行（ia.md §2 受控词表，批 10）：从当前工具集合的 task: 标签
 * 派生，与分组 chip AND 组合过滤。
 */
export default function CategoryPage() {
  const { categoryId } = useParams<{ categoryId: string }>();
  const category = categoryId ? getCategoryById(categoryId) : undefined;

  useEffect(() => {
    document.title = category ? `${category.name} · CTF-NET` : 'CTF-NET';
  }, [category]);

  const children = category?.children ?? [];
  const segmentChildren = children.filter((c) => c.segment);
  const cardChildren = children.filter((c) => !c.segment);
  const directTools = categoryId ? (toolsByCategory.get(categoryId) ?? []) : [];
  const path = categoryId ? getCategoryPath(categoryId) : [];

  // 切换分类时重置筛选
  const [chip, setChip] = useState<string>(ALL);
  const [taskChip, setTaskChip] = useState<string>(ALL);
  useEffect(() => {
    setChip(ALL);
    setTaskChip(ALL);
  }, [categoryId]);

  // 「全部」:本层直挂工具;纯分组分类(有分组子分类但本层无直挂)时取分类下全部工具。
  // 无分组子分类的页面不渲染 chips,保持原「直挂」语义不变。
  const allTools =
    segmentChildren.length > 0 && directTools.length === 0
      ? categoryId
        ? getToolsUnderCategory(categoryId)
        : []
      : directTools;

  // 任务维度(ia.md §2 受控词表,批 10):与分组 chip AND 组合过滤
  const chipTools = chip === ALL ? allTools : getToolsUnderCategory(chip);
  const taskOptions = availableTasks(chipTools);
  const shownTools = taskChip === ALL ? chipTools : filterByTask(chipTools, taskChip);

  // 面包屑：最后一项（当前分类）不设链接
  const crumbs = path.map((node, i) => ({
    label: node.name,
    to: i === path.length - 1 ? undefined : `/c/${node.id}`,
  }));

  const isEmpty = children.length === 0 && directTools.length === 0;

  return (
    <div className="p-6">
      <Breadcrumb items={crumbs} />

      <h1 className="mt-3 text-2xl font-semibold">{category?.name ?? '未知分类'}</h1>
      {category?.description && (
        <p className="mt-1 text-slate-500 dark:text-slate-400">{category.description}</p>
      )}

      {/* 分组筛选 chips（design-system §3.3；移动端横向滚动） */}
      {segmentChildren.length > 0 && (
        <section className="mt-6">
          <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400">分组</h2>
          <div className="mt-3 flex flex-wrap gap-2 overflow-x-auto">
            <FilterChip label="全部" active={chip === ALL} onClick={() => setChip(ALL)} />
            {segmentChildren.map((c) => (
              <FilterChip
                key={c.id}
                label={c.name}
                active={chip === c.id}
                onClick={() => setChip(c.id)}
              />
            ))}
          </div>
        </section>
      )}

      {/* 任务维度筛选 chips（ia.md §2 受控词表；仅当当前工具集合含 task: 标签时出现） */}
      {taskOptions.length > 0 && (
        <section className="mt-4">
          <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400">任务</h2>
          <div className="mt-3 flex flex-wrap gap-2 overflow-x-auto">
            <FilterChip label="全部" active={taskChip === ALL} onClick={() => setTaskChip(ALL)} />
            {taskOptions.map((task) => (
              <FilterChip
                key={task}
                label={task}
                active={taskChip === task}
                onClick={() => setTaskChip(task)}
              />
            ))}
          </div>
        </section>
      )}

      {/* 子分类入口卡片（段分类除外） */}
      {cardChildren.length > 0 && (
        <section className="mt-6">
          <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400">子分类</h2>
          <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {cardChildren.map((child) => (
              <SubcategoryCard key={child.id} node={child} />
            ))}
          </div>
        </section>
      )}

      {/* 工具卡片：默认本层工具；选中 chip 后为该段工具 */}
      {shownTools.length > 0 && (
        <section className="mt-6">
          <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400">
            工具 · {shownTools.length}
          </h2>
          <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {shownTools.map((tool) => (
              <ToolCard key={tool.id} tool={tool} />
            ))}
          </div>
        </section>
      )}

      {/* 空态 */}
      {isEmpty && (
        <div className="mt-8 rounded-lg border border-dashed border-slate-300 p-12 text-center text-slate-400 dark:border-slate-700">
          该分类下暂无子分类或工具
        </div>
      )}
    </div>
  );
}
