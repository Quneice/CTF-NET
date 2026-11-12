import { create } from 'zustand';

import type { CategoryNode } from '@/types';

import { categories } from '@/mock';

/**
 * 分类 store：持有分类树与展开状态。
 * 阶段 2 将实现递归树渲染与搜索联动时，展开/折叠逻辑会在此完整落地。
 */
interface CategoryState {
  /** 分类树（静态 mock，阶段 2 递归渲染） */
  categories: CategoryNode[];
  /** 已展开的分类节点 id 集合 */
  expandedIds: Set<string>;
  /** 侧边栏搜索关键字（阶段 2 联动） */
  searchKeyword: string;
  /** 最近一次选中的分类（二级侧边栏锚定用，进入工具页时保持不变） */
  selectedCategoryId: string | undefined;

  toggleExpanded: (id: string) => void;
  expandAll: () => void;
  collapseAll: () => void;
  setSearchKeyword: (kw: string) => void;
  setSelectedCategoryId: (id: string | undefined) => void;
}

/** 收集整棵树所有节点的 id（用于"全部展开"） */
function collectAllIds(nodes: CategoryNode[]): string[] {
  const ids: string[] = [];
  const walk = (list: CategoryNode[]) => {
    for (const n of list) {
      ids.push(n.id);
      if (n.children) walk(n.children);
    }
  };
  walk(nodes);
  return ids;
}

// 默认展开一级（顶层分类），便于首屏导航
const defaultExpanded = new Set<string>(categories.map((c) => c.id));

export const useCategoryStore = create<CategoryState>((set) => ({
  categories,
  expandedIds: defaultExpanded,
  searchKeyword: '',
  selectedCategoryId: undefined,

  toggleExpanded: (id) =>
    set((state) => {
      const next = new Set(state.expandedIds);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return { expandedIds: next };
    }),

  expandAll: () => set({ expandedIds: new Set(collectAllIds(categories)) }),

  collapseAll: () => set({ expandedIds: new Set<string>() }),

  setSearchKeyword: (kw) => set({ searchKeyword: kw }),

  setSelectedCategoryId: (id) => set({ selectedCategoryId: id }),
}));
