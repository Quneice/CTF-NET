import { useEffect, useRef, useState } from 'react';
import { RotateCcw, Star } from 'lucide-react';
import { useParams } from 'react-router-dom';

import Breadcrumb from '@/components/layout/Breadcrumb';
import { getIcon } from '@/lib/icons';
import ToolForm from '@/components/form/ToolForm';
import CodecWorkspace from '@/components/tool/CodecWorkspace';
import TransformWorkspace from '@/components/tool/TransformWorkspace';
import PhpUnserializeWorkspace from '@/components/tool/PhpUnserializeWorkspace';
import PocScanWorkspace from '@/components/poc/PocScanWorkspace';
import FingerprintDbWorkspace from '@/components/fingerdb/FingerprintDbWorkspace';
import RedTeamWikiWorkspace from '@/components/wiki/RedTeamWikiWorkspace';
import ReverseWikiWorkspace from '@/components/wiki/ReverseWikiWorkspace';
import ShiftWorkspace from '@/components/tool/ShiftWorkspace';
import StatusBadge from '@/components/tool/StatusBadge';
import PlaceholderBanner from '@/components/tool/PlaceholderBanner';
import OutputPanel from '@/components/output/OutputPanel';
import { useInvoke } from '@/hooks/useInvoke';
import type { InvokeState } from '@/adapters/types';
import type { FormValues, ToolDefinition } from '@/types';
import { getCategoryPath, getToolById } from '@/mock';
import { frpToolFormDefaults, getAppConfig, jndiToolFormDefaults } from '@/lib/settings';
import type { AppConfig } from '@/lib/settings';
import { keywordTagsOf, taskValuesOf } from '@/lib/task';
import { getToolStatus, isBackendPlaceholder } from '@/lib/toolStatus';
import { useHistoryStore } from '@/stores/useHistoryStore';
import { useToolStore } from '@/stores/useToolStore';

/** 历史记录摘要：截取结果首段 */
function summarize(state: InvokeState): string {
  const raw =
    state.streamText ||
    (typeof state.result === 'string' ? state.result : JSON.stringify(state.result));
  const s = raw ?? '';
  return s.length > 100 ? s.slice(0, 100) + '…' : s;
}

function formatTime(ts: number): string {
  const d = new Date(ts);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

/**
 * 工具详情页：面包屑 + 收藏 + 左侧参数表单（含历史回填）+ 右侧输出面板。
 */
export default function ToolPage({
  toolId: toolIdProp,
  active = true,
}: {
  /** 多开工作区按标签实例传入；缺省时回退到路由参数 */
  toolId?: string;
  /** 是否为当前激活标签：非激活实例不抢占 document.title */
  active?: boolean;
}) {
  const params = useParams<{ toolId: string }>();
  const toolId = toolIdProp ?? params.toolId;
  const tool = toolId ? getToolById(toolId) : undefined;
  const { favorites, toggleFavorite } = useToolStore();

  useEffect(() => {
    if (!active) return;
    document.title = tool ? `${tool.name} · CTF-NET` : 'CTF-NET';
  }, [tool, active]);

  if (!tool) {
    return <div className="p-6 text-slate-500">未知工具</div>;
  }

  const isFav = favorites.includes(tool.id);
  const categoryPath = getCategoryPath(tool.categoryId);
  const crumbs = [
    ...categoryPath.map((node) => ({ label: node.name, to: `/c/${node.id}` })),
    { label: tool.name },
  ];
  const HeaderIcon = getIcon(tool.icon ?? categoryPath[categoryPath.length - 1]?.icon);

  // 漏洞浏览器：页面头（名称/状态/描述）不显示，工作区即案例全文
  const bareWorkspace = tool.workspace === 'poc-scan';

  return (
    <div className="flex min-h-full flex-col p-6">
      {!bareWorkspace && (
        <>
          <Breadcrumb items={crumbs} />

          <div className="mt-3 flex items-center gap-3">
            <HeaderIcon className="h-6 w-6 shrink-0 text-brand" />
            <h1 className="text-2xl font-semibold">{tool.name}</h1>
            <StatusBadge tool={tool} />
            <button
              type="button"
              onClick={() => toggleFavorite(tool.id)}
              title={isFav ? '取消收藏' : '收藏'}
              className={`rounded-md p-1.5 transition-colors duration-fast ${
                isFav
                  ? 'text-amber-500'
                  : 'text-slate-400 hover:text-amber-500'
              }`}
            >
              <Star className="h-5 w-5" fill={isFav ? 'currentColor' : 'none'} />
            </button>
          </div>

          {tool.description && (
            <p className="mt-1 text-slate-500 dark:text-slate-400">{tool.description}</p>
          )}
          {tool.tags.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {taskValuesOf(tool.tags).map((task) => (
                <span
                  key={task}
                  className="rounded-full border border-line bg-surface-raised px-2 py-0.5 text-xs text-brand"
                >
                  {task}
                </span>
              ))}
              {keywordTagsOf(tool.tags).map((tag) => (
                <span
                  key={tag}
                  className="rounded-full bg-surface-hover px-2 py-0.5 text-xs text-ink-2"
                >
                  {tag}
                </span>
              ))}
            </div>
          )}

          <PlaceholderBanner tool={tool} />
        </>
      )}

      {/* key 保证切换工具时表单与调用状态一并重置 */}
      {tool.workspace === 'php-unserialize' ? (
        <PhpUnserializeWorkspace />
      ) : tool.workspace === 'poc-scan' ? (
        <PocScanWorkspace />
      ) : tool.workspace === 'fingerprint-db' ? (
        <FingerprintDbWorkspace />
      ) : tool.workspace === 'redteam-wiki' ? (
        <RedTeamWikiWorkspace />
      ) : tool.workspace === 'reverse-wiki' ? (
        <ReverseWikiWorkspace />
      ) : tool.workspace === 'shift' ? (
        <ShiftWorkspace />
      ) : tool.codec ? (
        <CodecWorkspace tool={tool} />
      ) : tool.transform ? (
        <TransformWorkspace tool={tool} />
      ) : (
        <ToolWorkspace key={tool.id} tool={tool} />
      )}
    </div>
  );
}

/** 配置文件驱动的表单默认值构建器（工具 ID → 统一配置 → 表单值）。 */
const CONFIG_FORM_DEFAULTS: Record<string, (cfg: AppConfig) => FormValues> = {
  'pentest.tunnel.frp': frpToolFormDefaults,
  'web.jndi': jndiToolFormDefaults,
};

/** 工作区：表单（含历史回填）+ 调用状态机 + 输出面板 */
function ToolWorkspace({ tool }: { tool: ToolDefinition }) {
  const { state, run, cancel } = useInvoke();
  const { entries, addEntry } = useHistoryStore();

  const [refill, setRefill] = useState<FormValues | null>(null);
  const [formKey, setFormKey] = useState(0);
  const lastParamsRef = useRef<FormValues | null>(null);
  const outputRef = useRef<HTMLElement | null>(null);

  // 配置文件默认值 → 表单预填（frp/jndi 的服务器连接参数来自统一配置）
  const [configDefaults, setConfigDefaults] = useState<{ toolId: string; values: FormValues } | undefined>();
  useEffect(() => {
    const builder = CONFIG_FORM_DEFAULTS[tool.id];
    if (!builder) {
      setConfigDefaults(undefined);
      return;
    }
    let alive = true;
    getAppConfig()
      .then((cfg) => {
        if (alive) setConfigDefaults({ toolId: tool.id, values: builder(cfg) });
      })
      .catch(() => {
        if (alive) setConfigDefaults(undefined);
      });
    return () => {
      alive = false;
    };
  }, [tool.id]);
  const formReady = !CONFIG_FORM_DEFAULTS[tool.id] || configDefaults?.toolId === tool.id;

  const history = entries.filter((e) => e.toolId === tool.id).slice(0, 5);

  // 上下分布布局：任务开始/结束时自动滚动到输出区
  useEffect(() => {
    if (['loading', 'success', 'error', 'cancelled'].includes(state.status)) {
      outputRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [state.status]);

  const handleSubmit = (values: FormValues) => {
    lastParamsRef.current = values;
    run(tool.invoke, tool.output, values);
  };

  // 调用成功后写入历史记录
  useEffect(() => {
    if (state.status === 'success' && lastParamsRef.current) {
      addEntry({
        id: crypto.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        toolId: tool.id,
        toolName: tool.name,
        params: lastParamsRef.current,
        summary: summarize(state),
        timestamp: Date.now(),
      });
      lastParamsRef.current = null;
    }
  }, [state.status, state.result, state.streamText, addEntry, tool.id, tool.name]);

  const handleRefill = (params: FormValues) => {
    setRefill(params);
    setFormKey((k) => k + 1);
  };

  return (
    <div className="mt-4 flex flex-col gap-4">
      <section>
        <h2 className="mb-2 text-sm font-medium text-slate-500 dark:text-slate-400">参数</h2>
        {formReady && (
          <ToolForm
            key={formKey}
            tool={tool}
            initialValues={refill ?? undefined}
            configDefaults={configDefaults?.values}
            onSubmit={handleSubmit}
          />
        )}

        {history.length > 0 && (
          <div className="mt-6">
            <h3 className="mb-2 text-xs font-medium text-slate-400">最近调用</h3>
            <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
              {history.map((e) => (
                <li
                  key={e.id}
                  className="rounded-lg border border-slate-200 p-3 text-xs dark:border-slate-800"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-slate-400">{formatTime(e.timestamp)}</span>
                    <button
                      type="button"
                      onClick={() => handleRefill(e.params)}
                      className="flex items-center gap-1 text-indigo-500 hover:text-indigo-600"
                    >
                      <RotateCcw className="h-3.5 w-3.5" />
                      回填参数
                    </button>
                  </div>
                  <p className="mt-1 truncate text-slate-500 dark:text-slate-400">{e.summary}</p>
                </li>
              ))}
            </ul>
          </div>
        )}
      </section>

      <section ref={outputRef}>
        <h2 className="mb-2 flex items-center gap-2 text-sm font-medium text-slate-500 dark:text-slate-400">
          输出
          {/* 占位工具:提交后自动滚动到本区,横幅可能已滚出视野 —— 结果仍需明确标注 mock/占位性质 */}
          {getToolStatus(tool) === 'placeholder' && (
            <span
              className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-normal ${
                isBackendPlaceholder(tool)
                  ? 'border-status-warnLine bg-status-warnBg text-status-warn'
                  : 'border-status-mutedLine bg-status-mutedBg text-status-muted'
              }`}
            >
              {isBackendPlaceholder(tool) ? '后端占位结果' : '模拟结果'}
            </span>
          )}
        </h2>
        <OutputPanel state={state} output={tool.output} onCancel={cancel} />
      </section>
    </div>
  );
}
