import { useMemo, useState } from 'react';
import { ArrowDown, ArrowUp, Binary, Layers, Plus, Repeat, Search, Trash2, UnfoldVertical } from 'lucide-react';

import {
  applyLayers,
  bruteForceShift,
  describeShift,
  invertLayers,
  type ShiftDomain,
  type ShiftHit,
  type ShiftMode,
  type ShiftOpts,
} from '@/lib/shift';

/**
 * 移位变换工作区(逆向·变换还原):多层移位全家桶。
 * 每层一个移位(常数/递增 b+i·s/ROL·ROR/SHL·SHR/字符串旋转),自上而下依序应用;
 * 解密 = 各层求逆逆序应用(任一层含 SHL/SHR 则整体置灰);
 * 爆破仅单层时可用,格式串由用户指定(题目给的 flag 格式)。
 */

const MODE_GROUPS: { label: string; modes: { id: ShiftMode; name: string }[] }[] = [
  { label: '值移位', modes: [{ id: 'const', name: '常数移位 ±N' }, { id: 'inc', name: '递增移位 b+i·s' }] },
  { label: '位运算', modes: [{ id: 'rol', name: 'ROL 循环左移' }, { id: 'ror', name: 'ROR 循环右移' }, { id: 'shl', name: 'SHL 左移(有损)' }, { id: 'shr', name: 'SHR 右移(有损)' }] },
  { label: '字符串', modes: [{ id: 'rotl', name: '整体左旋 k 位' }, { id: 'rotr', name: '整体右旋 k 位' }] },
];

const VALUE_MODES: ShiftMode[] = ['const', 'inc'];
const BIT_MODES: ShiftMode[] = ['rol', 'ror', 'shl', 'shr'];
const DOMAIN_LABEL: Record<ShiftDomain, string> = {
  byte: 'mod 256',
  printable: '可打印环',
  alpha: '字母环',
};

function toHex(s: string): string {
  return Array.from(s, (c) => (c.charCodeAt(0) & 0xff).toString(16).padStart(2, '0')).join(' ');
}

const defaultLayer = (): ShiftOpts => ({ mode: 'inc', domain: 'byte', n: 1, s: 1 });

/** 单层配置卡片(模式/域/参数/删除/上下移) */
function LayerCard({
  index,
  total,
  opts,
  onChange,
  onDelete,
  onMove,
}: {
  index: number;
  total: number;
  opts: ShiftOpts;
  onChange: (patch: Partial<ShiftOpts>) => void;
  onDelete: () => void;
  onMove: (dir: -1 | 1) => void;
}) {
  const { mode } = opts;
  const setMode2 = (next: ShiftMode) => {
    // 切入位运算模式时按其范围收敛 n,避免携带超范围值
    if (BIT_MODES.includes(next)) onChange({ mode: next, n: Math.max(0, Math.min(7, opts.n)) });
    else onChange({ mode: next });
  };
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-md border border-slate-200 bg-white px-2.5 py-2 text-xs dark:border-slate-700 dark:bg-slate-800">
      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-indigo-50 font-mono text-[11px] font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400">
        {index + 1}
      </span>
      <select
        value={mode}
        onChange={(e) => setMode2(e.target.value as ShiftMode)}
        className="rounded-md border border-slate-300 bg-white px-2 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
      >
        {MODE_GROUPS.map((g) => (
          <optgroup key={g.label} label={g.label}>
            {g.modes.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
      {mode === 'inc' && (
        <>
          <label className="flex items-center gap-1">
            b
            <input
              type="number"
              value={opts.n}
              onChange={(e) => onChange({ n: Math.trunc(Number(e.target.value) || 0) })}
              className="w-14 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
            />
          </label>
          <label className="flex items-center gap-1">
            s
            <input
              type="number"
              value={opts.s ?? 1}
              onChange={(e) => onChange({ s: Math.trunc(Number(e.target.value) || 0) })}
              className="w-14 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
            />
          </label>
        </>
      )}
      {mode === 'const' && (
        <label className="flex items-center gap-1">
          N
          <input
            type="number"
            value={opts.n}
            onChange={(e) => onChange({ n: Math.trunc(Number(e.target.value) || 0) })}
            className="w-16 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
        </label>
      )}
      {BIT_MODES.includes(mode) && (
        <label className="flex items-center gap-1">
          位数
          <input
            type="number"
            min={0}
            max={7}
            value={opts.n}
            onChange={(e) => onChange({ n: Math.max(0, Math.min(7, Math.trunc(Number(e.target.value) || 0))) })}
            className="w-14 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
        </label>
      )}
      {(mode === 'rotl' || mode === 'rotr') && (
        <label className="flex items-center gap-1">
          k
          <input
            type="number"
            value={opts.n}
            onChange={(e) => onChange({ n: Math.trunc(Number(e.target.value) || 0) })}
            className="w-14 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
        </label>
      )}
      {VALUE_MODES.includes(mode) && (
        <span className="flex items-center gap-1.5">
          {(Object.keys(DOMAIN_LABEL) as ShiftDomain[]).map((d) => (
            <label key={d} className="flex items-center gap-0.5">
              <input type="radio" name={`shift-domain-${index}`} checked={opts.domain === d} onChange={() => onChange({ domain: d })} />
              {DOMAIN_LABEL[d]}
            </label>
          ))}
        </span>
      )}
      <span className="ml-auto flex shrink-0 items-center gap-0.5">
        <button
          type="button"
          onClick={() => onMove(-1)}
          disabled={index === 0}
          className="rounded p-1 text-slate-400 hover:bg-slate-100 disabled:opacity-30 dark:hover:bg-slate-700"
          title="上移此层"
        >
          <ArrowUp className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={() => onMove(1)}
          disabled={index === total - 1}
          className="rounded p-1 text-slate-400 hover:bg-slate-100 disabled:opacity-30 dark:hover:bg-slate-700"
          title="下移此层"
        >
          <ArrowDown className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={onDelete}
          disabled={total <= 1}
          className="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-500 disabled:opacity-30 dark:hover:bg-red-500/10"
          title={total <= 1 ? '至少保留一层' : '删除此层'}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </span>
    </div>
  );
}

export default function ShiftWorkspace() {
  const [input, setInput] = useState('');
  const [layers, setLayers] = useState<ShiftOpts[]>([defaultLayer()]);
  const [decode, setDecode] = useState(false);
  // 爆破
  const [format, setFormat] = useState('');
  const [anywhere, setAnywhere] = useState(false);
  const [hits, setHits] = useState<ShiftHit[] | null>(null);

  const reversible = useMemo(() => layers.every((o) => o.mode !== 'shl' && o.mode !== 'shr'), [layers]);
  // 引用稳定:避免每渲染新建数组导致 output 全量重算(同 Wiki 适配器教训)
  const activeLayers = useMemo(
    () => (decode && reversible ? invertLayers(layers) ?? layers : layers),
    [decode, reversible, layers],
  );

  const output = useMemo(() => {
    if (!input) return '';
    return applyLayers(input, activeLayers);
  }, [input, activeLayers]);

  const patchLayer = (i: number, patch: Partial<ShiftOpts>) =>
    setLayers((ls) => ls.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  const deleteLayer = (i: number) => setLayers((ls) => (ls.length > 1 ? ls.filter((_, j) => j !== i) : ls));
  const moveLayer = (i: number, dir: -1 | 1) =>
    setLayers((ls) => {
      const j = i + dir;
      if (j < 0 || j >= ls.length) return ls;
      const copy = [...ls];
      [copy[i], copy[j]] = [copy[j], copy[i]];
      return copy;
    });

  const singleLayer = layers.length === 1;
  const runBrute = () => {
    setHits(bruteForceShift(input, format.trim(), anywhere));
  };

  return (
    <div className="mt-4 flex flex-col gap-4">
      {/* 移位层 */}
      <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 dark:border-slate-800 dark:bg-slate-900/60">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs">
          <span className="flex items-center gap-1 font-medium text-slate-600 dark:text-slate-300">
            <Binary className="h-4 w-4 text-indigo-500" /> 移位变换
          </span>
          <span className="flex items-center gap-1 text-slate-400">
            <Layers className="h-3.5 w-3.5" /> 共 {layers.length} 层,自上而下依序应用
          </span>
          <button
            type="button"
            onClick={() => setDecode((d) => !d)}
            disabled={!reversible}
            className={`flex items-center gap-1 rounded-md border px-2 py-1 transition-colors disabled:cursor-not-allowed disabled:opacity-40 ${
              decode
                ? 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-400'
                : 'border-slate-300 bg-white text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300'
            }`}
            title={reversible ? '切换加密/解密方向(解密=各层求逆逆序应用)' : '任一层含 SHL/SHR(有损)则整体不可逆'}
          >
            <Repeat className="h-3.5 w-3.5" />
            {decode ? '解密(逆变换)' : '加密(正变换)'}
          </button>
          <button
            type="button"
            onClick={() => setLayers((ls) => [...ls, defaultLayer()])}
            className="ml-auto flex items-center gap-1 rounded-md bg-indigo-500 px-2 py-1 font-medium text-white hover:bg-indigo-600"
          >
            <Plus className="h-3.5 w-3.5" />
            添加移位层
          </button>
        </div>
        <div className="mt-2 flex flex-col gap-1.5">
          {layers.map((l, i) => (
            <LayerCard
              key={i}
              index={i}
              total={layers.length}
              opts={l}
              onChange={(p) => patchLayer(i, p)}
              onDelete={() => deleteLayer(i)}
              onMove={(d) => moveLayer(i, d)}
            />
          ))}
        </div>
      </div>

      {/* 输入输出 */}
      <div className="grid gap-3 md:grid-cols-2">
        <div className="flex flex-col gap-1">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">输入</span>
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            rows={6}
            placeholder="待变换的字符串(或密文)"
            className="w-full resize-y rounded-md border border-slate-300 bg-white px-2 py-1.5 font-mono text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            输出 · {activeLayers.map(describeShift).join(' → ')}
          </span>
          <textarea
            value={output}
            readOnly
            rows={6}
            className="w-full resize-y rounded-md border border-slate-300 bg-slate-50 px-2 py-1.5 font-mono text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
          />
          {output && (
            <div className="rounded-md border border-slate-200 px-2 py-1 font-mono text-[11px] break-all text-slate-500 dark:border-slate-800 dark:text-slate-400">
              HEX {toHex(output)}
            </div>
          )}
        </div>
      </div>

      {/* 爆破区 */}
      <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 dark:border-slate-800 dark:bg-slate-900/60">
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <Search className="h-4 w-4 shrink-0 text-indigo-500" />
          <span className="font-medium text-slate-600 dark:text-slate-300">格式爆破</span>
          {!singleLayer && (
            <span className="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] text-amber-700 dark:bg-amber-500/10 dark:text-amber-400">
              多层不支持爆破:请逐层还原到单层后再试
            </span>
          )}
          <input
            value={format}
            onChange={(e) => setFormat(e.target.value)}
            disabled={!singleLayer}
            placeholder="题目给的 flag 格式,如 flag{ / ctfshow{"
            className="min-w-[12rem] flex-1 rounded-md border border-slate-300 bg-white px-2 py-1.5 font-mono text-xs disabled:opacity-40 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
          <label className="flex items-center gap-1 text-slate-600 dark:text-slate-300">
            <input
              type="checkbox"
              checked={anywhere}
              disabled={!singleLayer}
              onChange={(e) => setAnywhere(e.target.checked)}
            />
            匹配任意位置(否则前缀)
          </label>
          <button
            type="button"
            onClick={runBrute}
            disabled={!input || !format.trim() || !singleLayer}
            className="rounded-md bg-indigo-500 px-3 py-1.5 font-medium text-white hover:bg-indigo-600 disabled:opacity-40"
          >
            爆破
          </button>
        </div>
        {hits !== null && (
          <div className="mt-2 max-h-56 overflow-y-auto text-xs">
            {hits.length === 0 ? (
              <span className="text-slate-400">无命中:试试任意位置匹配、扩大已知片段,或密文并非纯移位。</span>
            ) : (
              <ul className="divide-y divide-slate-100 dark:divide-slate-800">
                {hits.map((h, i) => (
                  <li key={i}>
                    <button
                      type="button"
                      onClick={() => {
                        setLayers([h.opts]);
                        setDecode(false);
                        setInput(h.plain);
                      }}
                      className="flex w-full items-baseline gap-2 px-1 py-1.5 text-left hover:bg-slate-100 dark:hover:bg-slate-800"
                      title="点击回填参数与输入"
                    >
                      <UnfoldVertical className="h-3 w-3 shrink-0 self-center text-slate-400" />
                      <span className="shrink-0 rounded-full bg-indigo-50 px-2 py-0.5 font-mono text-[11px] text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400">
                        {h.desc}
                      </span>
                      <span className="min-w-0 flex-1 truncate font-mono text-slate-700 dark:text-slate-200">{h.plain}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
