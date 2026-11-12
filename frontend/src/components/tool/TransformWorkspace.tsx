import { useState } from 'react';
import { Eraser } from 'lucide-react';

import { localHandlerRegistry } from '@/adapters/localHandlers';
import type { ToolDefinition } from '@/types';

/**
 * 单向转换工作区：左侧输入、右侧只读输出，输入时自动实时转换。
 * 适合解释器 / 视图 / 文本处理等只有一个方向的工具。
 */
export default function TransformWorkspace({ tool }: { tool: ToolDefinition }) {
  const handlerId = tool.invoke.type === 'local' ? tool.invoke.handlerId : undefined;
  const inputLabel = tool.transform?.inputLabel ?? '输入';
  const outputLabel = tool.transform?.outputLabel ?? '输出';

  const [input, setInput] = useState('');
  const [output, setOutput] = useState('');
  const [error, setError] = useState<string | null>(null);

  const handleChange = (v: string) => {
    setInput(v);
    setError(null);
    try {
      const handler = handlerId ? localHandlerRegistry.get(handlerId) : undefined;
      const res = handler ? handler({ data: v }) : '';
      setOutput(typeof res === 'string' ? res : '');
    } catch (e) {
      setOutput('');
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  const handleClear = () => {
    setInput('');
    setOutput('');
    setError(null);
  };

  const baseCls =
    'min-h-32 w-full flex-1 resize-none rounded-lg border p-3 font-mono text-sm outline-none ' +
    'border-slate-300 bg-white text-slate-900 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ' +
    'dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100';

  return (
    <div className="mt-6 flex flex-1 flex-col">
      <div className="grid min-h-0 flex-1 auto-rows-fr grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col">
          <label className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-200">
            {inputLabel}
          </label>
          <textarea
            value={input}
            onChange={(e) => handleChange(e.target.value)}
            placeholder={`输入${inputLabel}…`}
            spellCheck={false}
            className={baseCls}
          />
        </div>

        <div className="flex min-h-0 flex-col">
          <label className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-200">
            {outputLabel}
          </label>
          <textarea
            value={output}
            readOnly
            placeholder={outputLabel}
            spellCheck={false}
            className={`${baseCls} bg-slate-50 text-slate-700 dark:bg-slate-950 dark:text-slate-300`}
          />
        </div>
      </div>

      {error && (
        <p className="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </p>
      )}

      <div className="mt-4 flex items-center justify-between">
        <p className="text-xs text-slate-400">输入后自动实时转换，输出为只读。</p>
        <button
          type="button"
          onClick={handleClear}
          className="flex shrink-0 items-center gap-1.5 rounded-md border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          <Eraser className="h-4 w-4" />
          清空
        </button>
      </div>
    </div>
  );
}
