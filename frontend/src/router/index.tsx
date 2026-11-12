import { createBrowserRouter, Navigate } from 'react-router-dom';

import AppLayout from '@/layouts/AppLayout';
import AiAssetsPage from '@/pages/AiAssetsPage';
import AiDashboardPage from '@/pages/AiDashboardPage';
import AiFindingsPage from '@/pages/AiFindingsPage';
import AiTaskDetailPage from '@/pages/AiTaskDetailPage';
import CategoryPage from '@/pages/CategoryPage';
import ChatPage from '@/pages/ChatPage';
import HomePage from '@/pages/HomePage';
import ModeSelectPage from '@/pages/ModeSelectPage';
import ProjectDetailPage from '@/pages/ProjectDetailPage';
import ProjectsPage from '@/pages/ProjectsPage';
import SettingsPage from '@/pages/SettingsPage';
import ToolPage from '@/pages/ToolPage';

/**
 * 路由设计：
 * - `/`            首页（热门工具 + 最近使用，阶段 6）
 * - `/c/:categoryId` 分类页（工具卡片 + 子分类入口，阶段 2）
 * - `/t/:toolId`    工具页（参数表单 + 输出面板，阶段 3/4/5）
 *
 * 旧路由兼容(规则 8:永不删除旧路由,先映射 + 重定向):
 * - `/c/misc.stego` → `/c/stego`(批 1:misc.stego 三个工具迁入 stego.*)
 * - `/c/crypto.encoding` → `/c/codec`、`/c/crypto.encoding.base` → `/c/codec.base`(批 2:codec 全量平移落地)
 * - `/c/crypto.modern.rsa.{algebraic, fermat-family, shared-structure, number-theoretic, coppersmith}` → `/c/crypto.modern.rsa`(批 5:22 个 RSA 工具扁平化挂载)
 */
export const router = createBrowserRouter([
  // 模式选择页：全屏，不带 AppLayout 框架
  { path: '/mode', element: <ModeSelectPage /> },
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'projects', element: <ProjectsPage /> },
      { path: 'projects/:id', element: <ProjectDetailPage /> },
      { path: 'ai-chat', element: <ChatPage /> },
      { path: 'ai-dashboard', element: <AiDashboardPage /> },
      { path: 'ai-task/:id', element: <AiTaskDetailPage /> },
      { path: 'ai-findings', element: <AiFindingsPage /> },
      { path: 'ai-assets', element: <AiAssetsPage /> },
      { path: 'c/:categoryId', element: <CategoryPage /> },
      { path: 'c/misc.stego', element: <Navigate to="/c/stego" replace /> },
      { path: 'c/crypto.encoding', element: <Navigate to="/c/codec" replace /> },
      { path: 'c/crypto.encoding.base', element: <Navigate to="/c/codec.base" replace /> },
      { path: 'c/crypto.modern.rsa.algebraic', element: <Navigate to="/c/crypto.modern.rsa" replace /> },
      { path: 'c/crypto.modern.rsa.fermat-family', element: <Navigate to="/c/crypto.modern.rsa" replace /> },
      { path: 'c/crypto.modern.rsa.shared-structure', element: <Navigate to="/c/crypto.modern.rsa" replace /> },
      { path: 'c/crypto.modern.rsa.number-theoretic', element: <Navigate to="/c/crypto.modern.rsa" replace /> },
      { path: 'c/crypto.modern.rsa.coppersmith', element: <Navigate to="/c/crypto.modern.rsa" replace /> },
      // 工具页由 AppLayout 内的 ToolTabsWorkspace 多开工作区接管渲染（keep-alive），
      // 此处的子路由仅用于路径匹配，element 不会被渲染
      { path: 't/:toolId', element: <ToolPage /> },
      { path: 'settings', element: <SettingsPage /> },
    ],
  },
]);
