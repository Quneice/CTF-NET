import type { ToolDefinition } from '@/types';

import { getToolStatus, isBackendPlaceholder, type ToolStatus } from '@/lib/toolStatus';

/**
 * 状态徽章(design-system.md §11):四态配色 + 「后端占位」变体。
 *
 * 状态由 getToolStatus 从清单接线自动推导,禁止组件手填。
 * 点击性(跳搜索页按状态筛选)待批 0 的 /search 页落地后启用。
 */
const CONFIG: Record<ToolStatus, { label: string; cls: string }> = {
  implemented: {
    label: '已实现',
    cls: 'border-status-okLine bg-status-okBg text-status-ok',
  },
  placeholder: {
    label: '占位',
    cls: 'border-status-warnLine bg-status-warnBg text-status-warn',
  },
  broken: {
    label: '故障',
    cls: 'border-status-dangerLine bg-status-dangerBg text-status-danger',
  },
  unknown: {
    label: '未知',
    cls: 'border-status-mutedLine bg-status-mutedBg text-status-muted',
  },
};

export default function StatusBadge({ tool }: { tool: ToolDefinition }) {
  const status = getToolStatus(tool);
  const backend = isBackendPlaceholder(tool);
  const label = backend ? '后端占位' : CONFIG[status].label;
  return (
    <span
      aria-label={`状态:${label}`}
      className={`inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium ${CONFIG[status].cls}`}
    >
      <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />
      {label}
    </span>
  );
}
