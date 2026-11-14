import type { CategoryNode, ToolDefinition } from '@/types';

import { categoryMap, getToolsUnderCategory } from './index';

/**
 * 模式配置（单一数据源）。
 *
 * 「CTF 模式 / 渗透测试模式 / 模型安全模式」通过 categoryRoots 声明各自可见的一级分类；
 * 分类可被多个模式共享（如 web / codec / osint 同时属于两种模式），
 * 调整归属只改本文件的 categoryRoots。
 */
export type AppMode = 'ctf' | 'pentest' | 'modelsec';

export interface ModeInfo {
  /** 模式显示名 */
  label: string;
  /** 选择页卡片描述 */
  description: string;
  /** lucide-react 图标名（与 CategoryNode.icon 同一约定） */
  icon: string;
  /** 该模式可见的一级分类 id，按展示顺序排列 */
  categoryRoots: string[];
}

export const MODE_CONFIG: Record<AppMode, ModeInfo> = {
  ctf: {
    label: 'CTF 模式',
    icon: 'Flag',
    description: '竞赛解题：Web、Crypto、Misc、Reverse、Pwn、取证、隐写等 13 大分类',
    categoryRoots: [
      'web',
      'crypto',
      'codec',
      'misc',
      'reverse',
      'pwn',
      'forensics',
      'stego',
      'osint',
      'blockchain',
      'mobile',
      'ai',
      'ai.pentest',
      'data',
    ],
  },
  pentest: {
    label: '渗透测试模式',
    icon: 'Crosshair',
    description: '实战渗透：信息收集、资产探测、漏洞检测、武器库、内网穿透，共享 Web / 编码 / OSINT 工具',
    // 渗透域 6 个顶级分类（原 pentest 子分类已提升为顶级）+ 共享的 Web / 编码 / OSINT
    categoryRoots: [
      'pentest.recon',
      'pentest.scan',
      'pentest.vuln',
      'pentest.arsenal',
      'pentest.tunnel',
      'pentest.assist',
      'ai.pentest',
      'web',
      'codec',
      'osint',
    ],
  },
  modelsec: {
    label: '模型安全',
    icon: 'ShieldCheck',
    description: '大模型安全测试：越狱攻击、提示注入、训练数据投毒、模型后门、对抗样本，共享编码工具',
    // 模型安全域 5 个顶级分类 + 共享的编码工具
    categoryRoots: [
      'aisec.jailbreak',
      'aisec.injection',
      'aisec.poison',
      'aisec.backdoor',
      'aisec.adversarial',
      'codec',
    ],
  },
};

/** 模式可见的顶层分类节点（按 categoryRoots 顺序） */
export function categoriesForMode(mode: AppMode): CategoryNode[] {
  return MODE_CONFIG[mode].categoryRoots
    .map((root) => categoryMap.get(root))
    .filter((n): n is CategoryNode => Boolean(n));
}

/** 分类 id（任意层级）是否属于某模式 */
export function isCategoryInMode(categoryId: string, mode: AppMode): boolean {
  return MODE_CONFIG[mode].categoryRoots.some(
    (root) => categoryId === root || categoryId.startsWith(root + '.'),
  );
}

/** 工具是否属于某模式（按其 categoryId 归属判断） */
export function isToolInMode(tool: ToolDefinition, mode: AppMode): boolean {
  return isCategoryInMode(tool.categoryId, mode);
}

/** 模块级缓存：toolsForMode 为纯函数，避免各组件重复合并 */
const toolsCache = new Map<AppMode, ToolDefinition[]>();

/** 模式可见的全部工具（各 root 下工具合并，按 id 去重） */
export function toolsForMode(mode: AppMode): ToolDefinition[] {
  const cached = toolsCache.get(mode);
  if (cached) return cached;
  const seen = new Set<string>();
  const out: ToolDefinition[] = [];
  for (const root of MODE_CONFIG[mode].categoryRoots) {
    for (const tool of getToolsUnderCategory(root)) {
      if (seen.has(tool.id)) continue;
      seen.add(tool.id);
      out.push(tool);
    }
  }
  toolsCache.set(mode, out);
  return out;
}
