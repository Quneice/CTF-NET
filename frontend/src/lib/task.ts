import type { ToolDefinition } from '@/types';

/**
 * 任务维度(ia.md §2 受控词表):tags 中以 `task:` 前缀的受控动词。
 *
 * 词表是唯一事实源(顺序即展示顺序),筛选/展示全部从工具 tags 派生,
 * 不在页面代码写死工具逻辑(design-system §9)。受控词表外的 `task:` 前缀一律忽略。
 */
export const TASK_VOCABULARY: readonly string[] = [
  '加解密',
  '编解码',
  '转换',
  '解析',
  '生成',
  '探测',
  '扫描',
  '爆破',
  '提取',
  '分析',
  '识别',
  '还原',
  '利用',
  '攻击',
  '演示',
  '校验',
  '格式化',
  '计算',
];

/** 受控动词 → task 标签 */
export const taskTag = (verb: string): string => `task:${verb}`;

/** 工具 tags 中的受控 task 标签(词表顺序) */
export function taskValuesOf(tags: readonly string[]): string[] {
  return TASK_VOCABULARY.filter((v) => tags.includes(taskTag(v)));
}

/** 非 task: 的自由关键词标签(卡片/面板展示用) */
export function keywordTagsOf(tags: readonly string[]): string[] {
  return tags.filter((t) => !t.startsWith('task:'));
}

/** 当前工具集合中出现过的任务维度值(受控词表顺序) */
export function availableTasks(tools: readonly ToolDefinition[]): string[] {
  const present = new Set<string>();
  for (const t of tools) {
    for (const v of taskValuesOf(t.tags)) present.add(v);
  }
  return TASK_VOCABULARY.filter((v) => present.has(v));
}

/** 按任务维度过滤工具 */
export function filterByTask(tools: readonly ToolDefinition[], task: string): ToolDefinition[] {
  return tools.filter((t) => t.tags.includes(taskTag(task)));
}
