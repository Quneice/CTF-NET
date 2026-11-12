import { memo } from 'react';
import { Link } from 'react-router-dom';

import type { ToolDefinition } from '@/types';

import { getIcon } from '@/lib/icons';
import { keywordTagsOf } from '@/lib/task';
import { getCategoryById } from '@/mock';
import StatusBadge from '@/components/tool/StatusBadge';

/**
 * 工具卡片：标题 / 状态徽章 / 描述 / tags / 图标（取所属分类的图标）。
 * 点击跳转到工具详情页 `/t/:toolId`；对话式渗透（ai.pentest.chat）例外——
 * 直接打开实时对话页 /ai-chat（它是对话框，不是表单工具）。memo 化避免无谓重渲染。
 * 配色只准使用 design-system 令牌别名（tailwind.config.js extend）。
 */
export default memo(function ToolCard({ tool }: { tool: ToolDefinition }) {
  // 工具可自带 icon（tool.icon），缺省复用其所属分类的图标作为视觉锚点
  const category = getCategoryById(tool.categoryId);
  const Icon = getIcon(tool.icon ?? category?.icon);
  const isChat = tool.id === 'ai.pentest.chat';

  return (
    <Link
      to={isChat ? '/ai-chat' : `/t/${tool.id}`}
      className="group rounded-lg border border-line bg-surface-raised p-4 shadow-card transition-colors duration-fast hover:border-brand/50 dark:shadow-none"
    >
      <div className="flex items-center gap-2">
        <Icon className="h-5 w-5 shrink-0 text-brand" />
        <span className="min-w-0 flex-1 truncate font-medium text-ink-1">{tool.name}</span>
        <StatusBadge tool={tool} />
        {tool.version && (
          <span className="shrink-0 text-xs text-ink-3">v{tool.version}</span>
        )}
      </div>

      {/* 分类前缀 + 完整工具 id:标明归属;重名工具（两个「文件上传绕过」、两个「RSA 解密」）按完整 id 前缀区分（改名留待阶段 4） */}
      <p className="mt-1 truncate text-xs text-ink-3">
        {category?.name ?? tool.categoryId}
        <span className="ml-1.5 font-mono text-2xs opacity-70">{tool.id}</span>
      </p>

      <p className="mt-2 line-clamp-2 text-sm text-ink-2">{tool.description}</p>

      {isChat && (
        <div className="mt-2 inline-flex items-center gap-1 rounded-full bg-brand/10 px-2 py-0.5 text-xs text-brand">
          点击进入实时对话
        </div>
      )}

      {tool.tags.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {/* 卡片只展示自由关键词;task: 标签属任务维度(ia.md §2),由分类页「任务」筛选消费 */}
          {keywordTagsOf(tool.tags).slice(0, 4).map((tag) => (
            <span
              key={tag}
              className="rounded-full bg-surface-hover px-2 py-0.5 text-xs text-ink-2"
            >
              {tag}
            </span>
          ))}
        </div>
      )}
    </Link>
  );
});
