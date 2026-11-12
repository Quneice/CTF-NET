import { Suspense } from 'react';
import { AlertCircle, CheckCircle2, Loader2, Square } from 'lucide-react';

import type { OutputSpec } from '@/types';

import type { InvokeState } from '@/adapters/types';

import { CopyButton } from './OutputActions';
import { resolveRenderer } from './rendererRegistry';
import ShellView from './ShellView';

/**
 * 输出面板：渲染调用状态机的各状态。
 *
 * - idle：空状态
 * - loading：运行中（进度条 / 计时 / 取消）
 * - streaming：正在输出（动效 / 停止 / 流式文本）
 * - success：按 output.type 走 RendererRegistry 渲染（Suspense 包裹懒加载渲染器）
 * - error：错误（可复制）
 * - cancelled：已取消
 */
export default function OutputPanel({
  state,
  output,
  onCancel,
}: {
  state: InvokeState;
  output: OutputSpec | undefined;
  onCancel: () => void;
}) {
  switch (state.status) {
    case 'idle':
      return <EmptyState />;
    case 'loading':
      return <LoadingView state={state} onCancel={onCancel} />;
    case 'streaming':
      return <StreamingView state={state} onCancel={onCancel} />;
    case 'error':
      return <ErrorView message={state.error ?? '未知错误'} />;
    case 'cancelled':
      return <HintView text="已取消" />;
    case 'success':
      return state.shell ? (
        <div>
          <SuccessView state={state} output={output} />
          <ShellView sessionId={state.shell.sessionId} />
        </div>
      ) : (
        <SuccessView state={state} output={output} />
      );
    default:
      return null;
  }
}

function EmptyState() {
  return (
    <div className="flex min-h-[200px] items-center justify-center rounded-lg border border-dashed border-slate-300 p-8 text-center text-slate-400 dark:border-slate-700">
      填写参数后点击「提交」运行
    </div>
  );
}

function formatElapsed(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function LoadingView({ state, onCancel }: { state: InvokeState; onCancel: () => void }) {
  const showProgress = state.progress > 0;
  return (
    <div className="rounded-lg border border-slate-200 p-4 dark:border-slate-800">
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
          <Loader2 className="h-4 w-4 animate-spin text-indigo-500" />
          运行中… {formatElapsed(state.elapsedMs)}
        </span>
        <button
          type="button"
          onClick={onCancel}
          className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <Square className="h-3.5 w-3.5" /> 取消
        </button>
      </div>

      {showProgress && (
        <div className="mt-3">
          <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
            <div
              className="h-full rounded-full bg-indigo-500 transition-all"
              style={{ width: `${state.progress}%` }}
            />
          </div>
          <div className="mt-1.5 text-xs text-slate-400">
            {state.progress}%{state.progressMessage ? ` · ${state.progressMessage}` : ''}
          </div>
        </div>
      )}
    </div>
  );
}

function StreamingView({ state, onCancel }: { state: InvokeState; onCancel: () => void }) {
  return (
    <div className="rounded-lg border border-slate-200 p-4 dark:border-slate-800">
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
          <span className="relative flex h-2.5 w-2.5">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-indigo-400 opacity-75" />
            <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-indigo-500" />
          </span>
          正在输出… {formatElapsed(state.elapsedMs)}
        </span>
        <button
          type="button"
          onClick={onCancel}
          className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10"
        >
          <Square className="h-3.5 w-3.5" /> 停止
        </button>
      </div>

      <pre className="mt-3 max-h-80 overflow-auto whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs leading-relaxed text-slate-700 dark:bg-slate-900 dark:text-slate-300">
        {state.streamText}
      </pre>
    </div>
  );
}

function ErrorView({ message }: { message: string }) {
  return (
    <div className="rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-500/30 dark:bg-red-500/10">
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-2 text-sm font-medium text-red-600 dark:text-red-400">
          <AlertCircle className="h-4 w-4" /> 运行失败
        </span>
        <CopyButton text={message} />
      </div>
      <p className="mt-2 whitespace-pre-wrap text-xs text-red-500 dark:text-red-300">{message}</p>
    </div>
  );
}

function HintView({ text }: { text: string }) {
  return (
    <div className="flex min-h-[120px] items-center justify-center rounded-lg border border-dashed border-slate-300 p-8 text-center text-slate-400 dark:border-slate-700">
      {text}
    </div>
  );
}

function SuccessView({ state, output }: { state: InvokeState; output: OutputSpec | undefined }) {
  const spec: OutputSpec = output ?? { type: 'text' };
  const data = state.streamText || state.result;
  const Renderer = resolveRenderer(spec);

  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5 text-sm text-emerald-600 dark:text-emerald-400">
        <CheckCircle2 className="h-4 w-4" />
        完成 · {formatElapsed(state.elapsedMs)}
      </div>
      <Suspense fallback={<div className="text-sm text-slate-400">渲染中…</div>}>
        <Renderer data={data} output={spec} />
      </Suspense>
    </div>
  );
}
