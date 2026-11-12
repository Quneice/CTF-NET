import type { CategoryNode, ToolDefinition } from '@/types';

import { categories } from './categories';
import { toolMap, tools, toolsByCategory } from './tools';

/** mock 数据统一出口 */
export { categories, tools, toolMap, toolsByCategory };

/** 分类 id -> 分类节点（扁平化，便于 O(1) 查找） */
export const categoryMap: Map<string, CategoryNode> = (() => {
  const map = new Map<string, CategoryNode>();
  const walk = (nodes: CategoryNode[]) => {
    for (const node of nodes) {
      map.set(node.id, node);
      if (node.children) walk(node.children);
    }
  };
  walk(categories);
  return map;
})();

/** 按 id 查找工具 */
export function getToolById(id: string): ToolDefinition | undefined {
  return toolMap.get(id);
}

/** 按 id 查找分类节点 */
export function getCategoryById(id: string): CategoryNode | undefined {
  return categoryMap.get(id);
}

/** 返回直接挂载在某分类下的工具（不含子孙分类的工具），用于侧边栏工具叶子节点 */
export function getDirectTools(categoryId: string): ToolDefinition[] {
  return toolsByCategory.get(categoryId) ?? [];
}

/** 返回某分类（含其所有子孙分类）下的工具集合 */
export function getToolsUnderCategory(categoryId: string): ToolDefinition[] {
  const collect = (node: CategoryNode, acc: ToolDefinition[]) => {
    const direct = toolsByCategory.get(node.id);
    if (direct) acc.push(...direct);
    if (node.children) node.children.forEach((c) => collect(c, acc));
  };
  const node = categoryMap.get(categoryId);
  if (!node) return [];
  const acc: ToolDefinition[] = [];
  collect(node, acc);
  return acc;
}

/** 返回从根到某分类节点的路径（含自身），用于面包屑 / 路径自动展开 */
export function getCategoryPath(categoryId: string): CategoryNode[] {
  const path: CategoryNode[] = [];
  const walk = (nodes: CategoryNode[], trail: CategoryNode[]): boolean => {
    for (const node of nodes) {
      const next = [...trail, node];
      if (node.id === categoryId) {
        path.push(...next);
        return true;
      }
      if (node.children && walk(node.children, next)) return true;
    }
    return false;
  };
  walk(categories, []);
  return path;
}
