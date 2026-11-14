import { useLocation } from 'react-router-dom';

import VulnLibraryPanel from '@/components/poc/VulnLibraryPanel';

/**
 * 二级侧边栏（已收窄为漏洞浏览器专用）。
 *
 * 历史行为：任意分类/工具页都展示「当前分类下一层」（子分类 + 直挂工具），
 * 与分类页内的子分类卡片 / 工具网格重复，已按用户反馈整体移除；
 * 分类导航统一走一级侧边栏与分类页本身。
 *
 * 仅漏洞浏览器（/t/pentest.vuln.poc-scan）保留：左侧案例清单树 +
 * 搜索 + 扫描导入，选中案例驱动右侧 PocScanWorkspace 展示正文。
 */
export default function SecondarySidebar() {
  const { pathname } = useLocation();

  if (pathname !== '/t/pentest.vuln.poc-scan') return null;

  return (
    <aside className="hidden w-56 shrink-0 flex-col border-r border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40 lg:flex">
      <header className="flex items-center gap-2 border-b border-slate-200 px-3 py-3 dark:border-slate-800">
        <span className="truncate text-sm font-semibold text-slate-700 dark:text-slate-200">
          漏洞清单
        </span>
      </header>
      <VulnLibraryPanel />
    </aside>
  );
}
