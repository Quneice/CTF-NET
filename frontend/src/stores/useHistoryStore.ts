import { create } from 'zustand';

import type { HistoryEntry } from '@/types';

/**
 * 历史记录 store：记录每个工具最近的调用，持久化到 localStorage。
 */

const HISTORY_KEY = 'ctf-net:history';
const MAX_ENTRIES = 200;

function loadHistory(): HistoryEntry[] {
  try {
    const raw = localStorage.getItem(HISTORY_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as HistoryEntry[]) : [];
  } catch {
    return [];
  }
}

interface HistoryState {
  /** 全部历史记录（倒序，最新在前） */
  entries: HistoryEntry[];

  addEntry: (entry: HistoryEntry) => void;
  clear: () => void;
  /** 取某工具的历史记录 */
  getByTool: (toolId: string) => HistoryEntry[];
  /** 取最近使用过的工具 id（去重，最新在前） */
  getRecentToolIds: () => string[];
}

export const useHistoryStore = create<HistoryState>((set, get) => ({
  entries: loadHistory(),

  addEntry: (entry) =>
    set((state) => {
      const entries = [entry, ...state.entries].slice(0, MAX_ENTRIES);
      try {
        localStorage.setItem(HISTORY_KEY, JSON.stringify(entries));
      } catch {
        // 忽略存储异常
      }
      return { entries };
    }),

  clear: () => {
    try {
      localStorage.removeItem(HISTORY_KEY);
    } catch {
      // 忽略
    }
    set({ entries: [] });
  },

  getByTool: (toolId) => get().entries.filter((e) => e.toolId === toolId),

  getRecentToolIds: () => {
    const seen = new Set<string>();
    const ids: string[] = [];
    for (const e of get().entries) {
      if (!seen.has(e.toolId)) {
        seen.add(e.toolId);
        ids.push(e.toolId);
      }
    }
    return ids;
  },
}));
