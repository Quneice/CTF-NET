import { create } from 'zustand';

/**
 * 工具标签页状态（会话级，不持久化）。
 *
 * 多开模型：openTabs 保存已打开的工具 id（按打开顺序），
 * 「当前激活标签」由路由 /t/:toolId 决定，store 不冗余记录，
 * 避免双数据源同步问题。
 */
interface ToolTabsState {
  /** 已打开的工具 id（按打开顺序） */
  openTabs: string[];
  /** 打开（或激活）一个工具标签；已存在则不动 */
  openTool: (toolId: string) => void;
  /** 关闭标签 */
  closeTab: (toolId: string) => void;
}

export const useToolTabsStore = create<ToolTabsState>()((set) => ({
  openTabs: [],

  openTool: (toolId) =>
    set((state) =>
      state.openTabs.includes(toolId) ? state : { openTabs: [...state.openTabs, toolId] },
    ),

  closeTab: (toolId) =>
    set((state) => ({ openTabs: state.openTabs.filter((id) => id !== toolId) })),
}));
