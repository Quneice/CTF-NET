import { Info, ServerCog } from 'lucide-react';

import type { ToolDefinition } from '@/types';

import { getToolStatus, isBackendPlaceholder } from '@/lib/toolStatus';

/**
 * 占位横幅(design-system.md §3.7):占位工具页顶部常驻提示,不可关闭。
 * 已实现工具不渲染本组件 —— 占位工具不得伪装成可用。
 */
export default function PlaceholderBanner({ tool }: { tool: ToolDefinition }) {
  if (getToolStatus(tool) !== 'placeholder') return null;
  const backend = isBackendPlaceholder(tool);
  return (
    <div
      role="note"
      className="mt-4 flex items-center gap-2 rounded-md border border-status-warnLine bg-status-warnBg px-3 py-2 text-sm text-status-warn"
    >
      {backend ? (
        <ServerCog className="h-4 w-4 shrink-0" aria-hidden="true" />
      ) : (
        <Info className="h-4 w-4 shrink-0" aria-hidden="true" />
      )}
      <span>
        {backend
          ? '后端占位:该引擎功能开发中,当前返回占位结果。'
          : '该工具为占位实现,当前返回模拟结果。'}
      </span>
    </div>
  );
}
