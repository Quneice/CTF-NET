import { create } from 'zustand';

import type { AppMode } from '@/mock/modes';

/**
 * 模式 store：当前工作模式（CTF / 渗透测试）。
 * 不持久化——每次启动应用都先进入模式选择页。
 */
interface ModeState {
  /** 当前模式；null 表示尚未选择（启动初态） */
  mode: AppMode | null;
  setMode: (mode: AppMode | null) => void;
}

export const useModeStore = create<ModeState>((set) => ({
  mode: null,
  setMode: (mode) => set({ mode }),
}));
