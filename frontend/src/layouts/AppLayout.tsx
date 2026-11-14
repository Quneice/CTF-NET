import { useEffect } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router-dom';

import CommandPalette from '@/components/CommandPalette';
import SecondarySidebar from '@/components/layout/SecondarySidebar';
import Sidebar from '@/components/layout/Sidebar';
import Topbar from '@/components/layout/Topbar';
import ToolTabsWorkspace from '@/components/tool/ToolTabsWorkspace';
import { useModeStore } from '@/stores/useModeStore';
import { useUiStore } from '@/stores/useUiStore';

/**
 * 应用整体布局：顶栏 + 左侧边栏 + 主区域 + 命令面板。
 *
 * 响应式策略（阶段 7 细化）：
 * - >= 1024px：侧边栏常驻，可折叠（宽度 64 / 16）
 * - <  1024px：侧边栏收起为抽屉（overlay）
 */
export default function AppLayout() {
  const {
    theme,
    sidebarCollapsed,
    mobileSidebarOpen,
    commandPaletteOpen,
    setMobileSidebarOpen,
    setCommandPaletteOpen,
  } = useUiStore();
  const mode = useModeStore((s) => s.mode);
  const location = useLocation();
  // 工具路由交给多开工作区渲染（keep-alive），不走 Outlet 单例
  const isToolRoute = location.pathname.startsWith('/t/');

  // 主题同步到 <html> class
  useEffect(() => {
    const root = document.documentElement;
    if (theme === 'dark') root.classList.add('dark');
    else root.classList.remove('dark');
  }, [theme]);

  // 全局快捷键：Ctrl/⌘+K 打开命令面板
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setCommandPaletteOpen(true);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [setCommandPaletteOpen]);

  // 模式守卫：未选择模式（每次启动的初态）时，任何路径都先落到模式选择页
  if (!mode) {
    return <Navigate to="/mode" replace />;
  }

  return (
    <div className="flex h-screen w-full overflow-hidden">
      {/* 桌面端侧边栏 */}
      <aside
        className={`hidden shrink-0 border-r border-slate-200 bg-white transition-all duration-200 dark:border-slate-800 dark:bg-slate-900 lg:block ${
          sidebarCollapsed ? 'w-16' : 'w-64'
        }`}
      >
        <Sidebar />
      </aside>

      {/* 二级侧边栏：仅漏洞浏览器工具页使用（案例清单）；其余页面与分类页内容重复，已移除 */}
      <SecondarySidebar />

      {/* 移动端抽屉 */}
      {mobileSidebarOpen && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 bg-black/50" onClick={() => setMobileSidebarOpen(false)} />
          <div className="absolute left-0 top-0 h-full w-72 bg-white shadow-xl dark:bg-slate-900">
            <Sidebar />
          </div>
        </div>
      )}

      {/* 右侧：顶栏 + 主区域 */}
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />
        <main className="flex-1 overflow-auto">
          {isToolRoute ? <ToolTabsWorkspace /> : <Outlet />}
        </main>
      </div>

      {/* 命令面板 */}
      {commandPaletteOpen && <CommandPalette />}
    </div>
  );
}
