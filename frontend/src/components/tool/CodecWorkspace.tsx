import { useRef, useState } from 'react';
import { Eraser } from 'lucide-react';

import { localHandlerRegistry } from '@/adapters/localHandlers';
import type { ToolDefinition } from '@/types';

/**
 * 双向编解码工作区（CyberChef 风格）。
 *
 * 左侧「编码前」、右侧「编码后」，任一输入框输入时实时计算另一侧：
 * - 编辑左侧 → 调用 handler 的 encode，回填右侧；
 * - 编辑右侧 → 调用 handler 的 decode，回填左侧。
 *
 * 若 tool.codec 为对象且声明了 alphabet / padding 选项，则额外渲染
 * 「自定义字符集」「填充字符」输入，并在调用时透传给 handler。
 */
export default function CodecWorkspace({ tool }: { tool: ToolDefinition }) {
  const handlerId = tool.invoke.type === 'local' ? tool.invoke.handlerId : undefined;
  const opts = tool.codec && typeof tool.codec === 'object' ? tool.codec : {};
  const showAlphabet = opts.alphabet === true;
  const showPadding = typeof opts.padding === 'string';
  const defaultPadding = showPadding ? opts.padding : '';

  const [plain, setPlain] = useState('');
  const [encoded, setEncoded] = useState('');
  const [alphabet, setAlphabet] = useState('');
  const [padding, setPadding] = useState(defaultPadding);
  const [error, setError] = useState<string | null>(null);
  const sourceRef = useRef<'plain' | 'encoded'>('plain');

  const call = (mode: 'encode' | 'decode', data: string, alpha = alphabet, pad = padding): string => {
    if (!handlerId || !data) return '';
    const handler = localHandlerRegistry.get(handlerId);
    if (!handler) return '';
    const res = handler({ mode, data, alphabet: alpha, padding: pad });
    return typeof res === 'string' ? res : '';
  };

  const errMsg = (e: unknown): string => (e instanceof Error ? e.message : String(e));

  const handlePlainChange = (v: string) => {
    sourceRef.current = 'plain';
    setPlain(v);
    setError(null);
    try {
      setEncoded(call('encode', v));
    } catch (e) {
      setEncoded('');
      setError(errMsg(e));
    }
  };

  const handleEncodedChange = (v: string) => {
    sourceRef.current = 'encoded';
    setEncoded(v);
    setError(null);
    try {
      setPlain(call('decode', v));
    } catch (e) {
      setPlain('');
      setError(errMsg(e));
    }
  };

  const handleAlphabetChange = (v: string) => {
    setAlphabet(v);
    setError(null);
    try {
      if (sourceRef.current === 'plain') setEncoded(call('encode', plain, v, padding));
      else setPlain(call('decode', encoded, v, padding));
    } catch (e) {
      setError(errMsg(e));
    }
  };

  const handlePaddingChange = (v: string) => {
    setPadding(v);
    setError(null);
    try {
      if (sourceRef.current === 'plain') setEncoded(call('encode', plain, alphabet, v));
      else setPlain(call('decode', encoded, alphabet, v));
    } catch (e) {
      setError(errMsg(e));
    }
  };

  const handleClear = () => {
    setPlain('');
    setEncoded('');
    setAlphabet('');
    setPadding(defaultPadding);
    setError(null);
  };

  const textareaCls =
    'min-h-32 w-full flex-1 resize-none rounded-lg border border-slate-300 bg-white p-3 font-mono text-sm text-slate-900 outline-none ' +
    'focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ' +
    'dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100';

  const optionCls =
    'w-full rounded-md border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-900 outline-none ' +
    'focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ' +
    'dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100';

  return (
    <div className="mt-6 flex flex-1 flex-col">
      {(showAlphabet || showPadding) && (
        <div className="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
          {showAlphabet && (
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-300">
                自定义字符集（留空用默认）
              </label>
              <input
                value={alphabet}
                onChange={(e) => handleAlphabetChange(e.target.value)}
                placeholder="留空使用默认字母表"
                spellCheck={false}
                className={optionCls}
              />
            </div>
          )}
          {showPadding && (
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-300">
                填充字符（留空表示无填充）
              </label>
              <input
                value={padding}
                onChange={(e) => handlePaddingChange(e.target.value)}
                placeholder="留空表示无填充"
                spellCheck={false}
                className={optionCls}
              />
            </div>
          )}
        </div>
      )}

      <div className="grid min-h-0 flex-1 auto-rows-fr grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col">
          <label className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-200">
            编码前
          </label>
          <textarea
            value={plain}
            onChange={(e) => handlePlainChange(e.target.value)}
            placeholder="输入原始文本…"
            spellCheck={false}
            className={textareaCls}
          />
        </div>

        <div className="flex min-h-0 flex-col">
          <label className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-200">
            编码后
          </label>
          <textarea
            value={encoded}
            onChange={(e) => handleEncodedChange(e.target.value)}
            placeholder="输入编码后文本…"
            spellCheck={false}
            className={textareaCls}
          />
        </div>
      </div>

      {error && (
        <p className="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </p>
      )}

      <div className="mt-4 flex items-center justify-between">
        <p className="text-xs text-slate-400">在任一输入框输入，另一框会自动实时计算（编码 ↔ 解码）。</p>
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
