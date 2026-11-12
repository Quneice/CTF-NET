import { create } from 'zustand';

import type { ToolDefinition } from '@/types';

import { tools } from '@/mock';

/**
 * 工具 store：持有全部工具与收藏。
 * 收藏持久化到 localStorage。
 */

const FAVORITES_KEY = 'ctf-net:favorites';

function loadFavorites(): string[] {
  try {
    const raw = localStorage.getItem(FAVORITES_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((x) => typeof x === 'string') : [];
  } catch {
    return [];
  }
}

interface ToolState {
  /** 全部工具（静态 mock） */
  tools: ToolDefinition[];
  /** 已收藏工具 id 列表 */
  favorites: string[];

  toggleFavorite: (id: string) => void;
  isFavorite: (id: string) => boolean;
}

export const useToolStore = create<ToolState>((set, get) => ({
  tools,
  favorites: loadFavorites(),

  toggleFavorite: (id) =>
    set((state) => {
      const has = state.favorites.includes(id);
      const favorites = has
        ? state.favorites.filter((f) => f !== id)
        : [...state.favorites, id];
      try {
        localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites));
      } catch {
        // 忽略存储异常
      }
      return { favorites };
    }),

  isFavorite: (id) => get().favorites.includes(id),
}));
