import { useMemo, useState } from 'react';

import {
  initManualNode,
  serializeManualNode,
  type ManualNode,
  type ManualProp,
  type PhpClassInfo,
} from '@/lib/phpPopChain';

/**
 * POP 链手工编辑器：完全手动控制对象结构与所有参数值。
 * 每个属性可切换「标量 / 对象」类型，标量可填值，对象可递归选择类并继续编辑。
 */

const inputCls =
  'w-full rounded-md border border-slate-300 bg-white px-3 py-1.5 font-mono text-xs text-slate-900 outline-none ' +
  'focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ' +
  'dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100';

const selectCls =
  'rounded-md border border-slate-300 bg-white px-2 py-1 text-xs text-slate-900 outline-none ' +
  'focus:border-indigo-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100';

const textareaCls =
  'w-full resize-none rounded-lg border border-slate-300 bg-slate-50 p-3 font-mono text-sm text-slate-700 outline-none ' +
  'dark:border-slate-700 dark:bg-slate-950 dark:text-slate-300';

function ObjectEditor({
  node,
  classes,
  onChange,
  depth = 0,
}: {
  node: ManualNode;
  classes: PhpClassInfo[];
  onChange: (n: ManualNode) => void;
  depth?: number;
}) {
  const changeClass = (name: string) => onChange(initManualNode(name, classes));

  const patchProp = (i: number, patch: Partial<ManualProp>) => {
    onChange({
      ...node,
      props: node.props.map((p, idx) => (idx === i ? { ...p, ...patch } : p)),
    });
  };

  const switchType = (i: number, type: 'scalar' | 'object') => {
    patchProp(i, type === 'object' ? { type, child: initManualNode(classes[0].name, classes) } : { type, value: '' });
  };

  return (
    <div
      className={`space-y-2 ${depth > 0 ? 'mt-2 rounded-md border border-slate-200 bg-slate-50 p-2 dark:border-slate-700 dark:bg-slate-900/40' : ''}`}
    >
      <select value={node.className} onChange={(e) => changeClass(e.target.value)} className={selectCls}>
        {classes.map((c) => (
          <option key={c.name} value={c.name}>
            {c.name}
          </option>
        ))}
      </select>

      {node.props.map((p, i) => (
        <div key={i} className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="font-mono text-xs text-slate-700 dark:text-slate-200">${p.name}</span>
            <span className="text-[10px] text-slate-400">{p.visibility}</span>
            <select value={p.type} onChange={(e) => switchType(i, e.target.value as 'scalar' | 'object')} className={selectCls}>
              <option value="scalar">标量</option>
              <option value="object">对象</option>
            </select>
          </div>

          {p.type === 'scalar' ? (
            <input
              value={p.value}
              onChange={(e) => patchProp(i, { value: e.target.value })}
              placeholder="值"
              spellCheck={false}
              className={inputCls}
            />
          ) : (
            <ObjectEditor
              node={p.child!}
              classes={classes}
              onChange={(child) => patchProp(i, { child })}
              depth={depth + 1}
            />
          )}
        </div>
      ))}
    </div>
  );
}

export default function PopChainEditor({
  classes,
  initialNode,
}: {
  classes: PhpClassInfo[];
  initialNode?: ManualNode | null;
}) {
  const [root, setRoot] = useState<ManualNode | null>(() =>
    initialNode ?? (classes.length ? initManualNode(classes[0].name, classes) : null),
  );

  const payload = useMemo(() => (root ? serializeManualNode(root) : ''), [root]);
  const readable = payload.replace(/\u0000/g, '\\x00');
  const urlEncoded = (() => {
    try {
      return encodeURIComponent(payload);
    } catch {
      return '';
    }
  })();

  if (!root) return null;

  const labelCls = 'mb-2 block text-sm font-medium text-slate-700 dark:text-slate-200';

  return (
    <div className="mt-4 grid grid-cols-1 gap-6 lg:grid-cols-2">
      <div className="space-y-2">
        <p className="text-sm font-medium text-slate-700 dark:text-slate-200">对象结构（手动构建）</p>
        <ObjectEditor node={root} classes={classes} onChange={setRoot} />
      </div>

      <div className="space-y-4">
        <div>
          <label className={labelCls}>序列化 PoC（\x00 表示 NUL 字节）</label>
          <textarea rows={14} value={readable} readOnly spellCheck={false} className={textareaCls} />
        </div>
        <div>
          <label className={labelCls}>URL 编码</label>
          <textarea rows={6} value={urlEncoded} readOnly spellCheck={false} className={textareaCls} />
        </div>
      </div>
    </div>
  );
}
