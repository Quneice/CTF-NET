import { create } from 'zustand';

/**
 * 漏洞浏览器：当前选中的漏洞（会话级，不持久化）。
 * 二级侧边栏的漏洞清单负责 select()，poc-scan 工作区读取并展示案例全文。
 * caseId 即漏洞库中的案例 ID（相对路径）。
 */

export interface PocSelection {
  /** 漏洞大类 id / 名称（Z-编号） */
  categoryId: string;
  categoryName: string;
  /** 系统/厂商 id / 名称（S-编号） */
  systemId: string;
  systemName: string;
  /** 漏洞案例 id / 名称 */
  caseId: string;
  caseName: string;
  /** 案例标题与 CVE（列表有则带出，工作区免一次详情请求） */
  title?: string;
  cves?: string[];
}

interface PocState {
  selected: PocSelection | null;
  select: (sel: PocSelection) => void;
  clear: () => void;
  /** 编辑（新增/删除/保存）后自增，侧边栏据此重载分类与统计 */
  refreshTick: number;
  bumpRefresh: () => void;
}

export const usePocStore = create<PocState>()((set) => ({
  selected: null,
  select: (sel) => set({ selected: sel }),
  clear: () => set({ selected: null }),
  refreshTick: 0,
  bumpRefresh: () => set((s) => ({ refreshTick: s.refreshTick + 1 })),
}));
