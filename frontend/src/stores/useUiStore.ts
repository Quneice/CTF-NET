import { create } from 'zustand';

/**
 * UI store：主题、侧边栏、命令面板等全局 UI 状态。
 * 主题持久化到 localStorage。
 */
type Theme = 'light' | 'dark';

const THEME_KEY = 'ctf-net:theme';

function loadTheme(): Theme {
  try {
    return localStorage.getItem(THEME_KEY) === 'light' ? 'light' : 'dark';
  } catch {
    return 'dark';
  }
}

interface UiState {
  theme: Theme;
  /** 侧边栏是否折叠（桌面端） */
  sidebarCollapsed: boolean;
  /** 移动端抽屉是否打开 */
  mobileSidebarOpen: boolean;
  /** 命令面板（Ctrl/⌘+K）是否打开 */
  commandPaletteOpen: boolean;

  toggleTheme: () => void;
  setTheme: (theme: Theme) => void;
  toggleSidebar: () => void;
  setSidebarCollapsed: (v: boolean) => void;
  setMobileSidebarOpen: (v: boolean) => void;
  setCommandPaletteOpen: (v: boolean) => void;
}

/** 把主题同步到 <html> 的 class，配合 Tailwind darkMode:'class' */
function applyTheme(theme: Theme) {
  const root = document.documentElement;
  if (theme === 'dark') root.classList.add('dark');
  else root.classList.remove('dark');
}

function saveTheme(theme: Theme) {
  try {
    localStorage.setItem(THEME_KEY, theme);
  } catch {
    // 忽略存储异常
  }
}

export const useUiStore = create<UiState>((set) => ({
  theme: loadTheme(),
  sidebarCollapsed: false,
  mobileSidebarOpen: false,
  commandPaletteOpen: false,

  toggleTheme: () =>
    set((state) => {
      const theme: Theme = state.theme === 'dark' ? 'light' : 'dark';
      applyTheme(theme);
      saveTheme(theme);
      return { theme };
    }),

  setTheme: (theme) => {
    applyTheme(theme);
    saveTheme(theme);
    set({ theme });
  },

  toggleSidebar: () => set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),

  setSidebarCollapsed: (v) => set({ sidebarCollapsed: v }),

  setMobileSidebarOpen: (v) => set({ mobileSidebarOpen: v }),

  setCommandPaletteOpen: (v) => set({ commandPaletteOpen: v }),
}));
