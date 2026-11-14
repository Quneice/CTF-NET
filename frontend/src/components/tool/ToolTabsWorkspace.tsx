import { useEffect } from 'react';
import { ArrowLeft, X } from 'lucide-react';
import { useNavigate, useParams } from 'react-router-dom';

import { getCategoryById, getToolById } from '@/mock';
import ToolPage from '@/pages/ToolPage';
import { useToolTabsStore } from '@/stores/useToolTabsStore';

/**
 * 工具多开工作区：标签栏 + keep-alive 面板。
 *
 * - 任何跳转到 /t/:toolId 的导航都会同步出一个标签（已存在则仅激活）；
 * - 所有打开的工具实例常驻挂载（display:none 隐藏非激活项），
 *   切换标签时表单输入、运行输出等状态完整保留；
 * - 关闭激活标签后回落到相邻标签，无标签时回首页。
 */
export default function ToolTabsWorkspace() {
  const { toolId } = useParams<{ toolId: string }>();
  const { openTabs, openTool, closeTab } = useToolTabsStore();
  const navigate = useNavigate();

  // 路由 → 标签同步：直接输入/跳转 /t/:id 时自动开标签
  useEffect(() => {
    if (toolId && getToolById(toolId)) openTool(toolId);
  }, [toolId, openTool]);

  const handleClose = (id: string, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const idx = openTabs.indexOf(id);
    closeTab(id);
    if (id === toolId) {
      const rest = openTabs.filter((t) => t !== id);
      const next = rest[Math.min(idx, rest.length - 1)];
      navigate(next ? `/t/${next}` : '/', { replace: true });
    }
  };

  // 返回按钮：跳到当前工具所属的分类选择页（工具标签保持打开，运行状态不丢）。
  // 确定性导航而非 navigate(-1)：多开标签/收藏/命令面板进入时浏览器历史不可靠，
  // 固定回「该工具的选择页」始终符合预期；分类未知时退回首页。
  const activeTool = toolId ? getToolById(toolId) : undefined;
  const activeCategory = activeTool ? getCategoryById(activeTool.categoryId) : undefined;

  const handleBack = () => {
    navigate(activeCategory ? `/c/${activeCategory.id}` : '/');
  };

  return (
    <div className="flex min-h-full flex-col">
      {/* 标签栏 */}
      <div className="sticky top-0 z-10 flex shrink-0 items-center gap-0.5 overflow-x-auto border-b border-slate-200 bg-white px-2 pt-1.5 dark:border-slate-800 dark:bg-slate-900">
        {/* 返回按钮：标签栏常驻可见，工具页滚动到任意位置都能一键回到选择页 */}
        <button
          type="button"
          onClick={handleBack}
          title={`返回「${activeCategory?.name ?? '首页'}」（工具标签保持打开）`}
          aria-label={`返回${activeCategory?.name ?? '首页'}`}
          className="mr-1 flex max-w-40 shrink-0 items-center gap-1 rounded-md border-r border-slate-200 px-2 py-1 text-sm text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
        >
          <ArrowLeft className="h-4 w-4 shrink-0" />
          <span className="truncate">{activeCategory?.name ?? '首页'}</span>
        </button>
        {openTabs.map((id) => {
          const tool = getToolById(id);
          const active = id === toolId;
          return (
            <button
              key={id}
              type="button"
              onClick={() => navigate(`/t/${id}`)}
              className={`group flex max-w-44 shrink-0 items-center gap-1.5 rounded-t-md border-x border-t px-3 py-1.5 text-sm transition-colors ${
                active
                  ? 'border-slate-200 bg-slate-50 font-medium text-indigo-600 dark:border-slate-700 dark:bg-slate-800 dark:text-indigo-400'
                  : 'border-transparent text-slate-500 hover:bg-slate-50 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200'
              }`}
            >
              <span className="truncate">{tool?.name ?? id}</span>
              <span
                role="button"
                aria-label={`关闭 ${tool?.name ?? id}`}
                onClick={(e) => handleClose(id, e)}
                className={`rounded p-0.5 hover:bg-slate-200 dark:hover:bg-slate-600 ${
                  active ? '' : 'opacity-0 group-hover:opacity-100'
                }`}
              >
                <X className="h-3 w-3" />
              </span>
            </button>
          );
        })}
      </div>

      {/* keep-alive 面板：非激活标签仅隐藏不卸载，保留工具状态 */}
      <div className="flex-1">
        {openTabs.map((id) => (
          <div key={id} className={id === toolId ? undefined : 'hidden'}>
            <ToolPage toolId={id} active={id === toolId} />
          </div>
        ))}
      </div>
    </div>
  );
}
