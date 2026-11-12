import type { ToolDefinition } from '@/types';

import { localHandlerRegistry } from '@/adapters/localHandlers';

/**
 * 工具状态(单一事实源 = 工具清单 + 注册表接线)。
 *
 * 推导规则与 docs/tool-inventory.json 的判定规则逐条一致(audit-report.md §4):
 *   1. 显式覆盖(与清单同步;当前覆盖:19 个 Go 占位 SSTI 执行器);
 *   2. task + realBackend → implemented(经 Wails 调用 Go 真实实现);
 *   3. local + handler 已在 localHandlerRegistry 注册 → implemented;
 *   4. 其余(http/sse/ws/mock task/未注册 local)→ placeholder。
 *
 * 状态禁止手填:接线修复后状态自动变化(如批 1 的 strings / http-headers)。
 */
export type ToolStatus = 'implemented' | 'placeholder' | 'broken' | 'unknown';

/** 与 tool-inventory.json 同步的显式覆盖(仅此处允许偏离推导规则)。 */
const STATUS_OVERRIDES: Record<string, ToolStatus> = {
  // Go ssti_base.go 的 19 个占位执行器:realBackend 可达,但仅回显「[占位符] … 功能开发中」
  'web.ssti.angularjs': 'placeholder',
  'web.ssti.django': 'placeholder',
  'web.ssti.ejs': 'placeholder',
  'web.ssti.erb': 'placeholder',
  'web.ssti.fingerprint': 'placeholder',
  'web.ssti.freemarker': 'placeholder',
  'web.ssti.go': 'placeholder',
  'web.ssti.groovy': 'placeholder',
  'web.ssti.handlebars': 'placeholder',
  'web.ssti.mako': 'placeholder',
  'web.ssti.nunjucks': 'placeholder',
  'web.ssti.pug': 'placeholder',
  'web.ssti.razor': 'placeholder',
  'web.ssti.slim': 'placeholder',
  'web.ssti.smarty': 'placeholder',
  'web.ssti.thymeleaf': 'placeholder',
  'web.ssti.tornado': 'placeholder',
  'web.ssti.twig': 'placeholder',
  'web.ssti.velocity': 'placeholder',
};

export function getToolStatus(tool: ToolDefinition): ToolStatus {
  const override = STATUS_OVERRIDES[tool.id];
  if (override) return override;
  // 专用工作区工具（php-unserialize / poc-scan / redteam-wiki 等）由独立组件实现，不走 invoke
  if (tool.workspace) return 'implemented';
  const inv = tool.invoke;
  if (inv.type === 'task' && inv.realBackend) return 'implemented';
  if (inv.type === 'local' && localHandlerRegistry.has(inv.handlerId)) return 'implemented';
  return 'placeholder';
}

/** 是否为「后端占位」:realBackend 可达但 Go 侧为占位执行器(当前即 19 个 SSTI 引擎)。 */
export function isBackendPlaceholder(tool: ToolDefinition): boolean {
  return (
    getToolStatus(tool) === 'placeholder' &&
    tool.invoke.type === 'task' &&
    tool.invoke.realBackend === true
  );
}
