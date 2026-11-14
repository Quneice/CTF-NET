import { useMemo, useState } from 'react';
import { Binary, Repeat, Search, UnfoldVertical } from 'lucide-react';

import {
  applyShift,
  bruteForceShift,
  describeShift,
  invertShift,
  type ShiftDomain,
  type ShiftHit,
  type ShiftMode,
  type ShiftOpts,
} from '@/lib/shift';

/**
 * 移位变换工作区(逆向·变换还原):字符串移位全家桶。
 * 常数/递增(第 i 位 +(b+i·s))/位循环 ROL·ROR/字节位移/字符串循环旋转,
 * 三值域,加密解密互逆,并支持按用户指定 flag 格式串爆破参数。
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
  printable: '可打印环(0x20-0x7E)',
  alpha: '字母环(保大小写)',
};

function toHex(s: string): string {
  return Array.from(s, (c) => (c.charCodeAt(0) & 0xff).toString(16).padStart(2, '0')).join(' ');
}

export default function ShiftWorkspace() {
  const [input, setInput] = useState('');
  const [mode, setMode] = useState<ShiftMode>('inc');
  const [domain, setDomain] = useState<ShiftDomain>('byte');
  const [n, setN] = useState(1);
  const [s, setS] = useState(1);
  const [decode, setDecode] = useState(false);
  // 爆破
  const [format, setFormat] = useState('');
  const [anywhere, setAnywhere] = useState(false);
  const [hits, setHits] = useState<ShiftHit[] | null>(null);

  const effectiveDomain = VALUE_MODES.includes(mode) ? domain : 'byte';
  const reversible = mode !== 'shl' && mode !== 'shr';
  const opts: ShiftOpts = { mode, domain: effectiveDomain, n, ...(mode === 'inc' ? { s } : {}) };
  // shl/shr 不可逆:即使残留 decode=true 也按正向计算,避免"有输入无输出"死局
  const inv = decode && reversible ? invertShift(opts) ?? opts : opts;

  const output = useMemo(() => {
    if (!input) return '';
    if (!inv) return '';
    try {
      return applyShift(input, inv);
    } catch {
      return '';
    }
  }, [input, inv]);

  const runBrute = () => {
    setHits(bruteForceShift(input, format.trim(), anywhere));
  };

  return (
    <div className="mt-4 flex flex-col gap-4">
      {/* 参数区 */}
      <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 dark:border-slate-800 dark:bg-slate-900/60">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs">
          <span className="flex items-center gap-1 font-medium text-slate-600 dark:text-slate-300">
            <Binary className="h-4 w-4 text-indigo-500" /> 移位变换
          </span>
          {/* 模式 */}
          <select
            value={mode}
            onChange={(e) => {
              const next = e.target.value as ShiftMode;
              // 切入位运算模式时按其范围收敛 n,避免携带超范围值(如 rotl k=10 带入 ROL)
              if (BIT_MODES.includes(next)) setN((prev) => Math.max(0, Math.min(7, prev)));
              setMode(next);
            }}
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
          {/* 方向 */}
          <button
            type="button"
            onClick={() => setDecode((d) => !d)}
            disabled={!reversible}
            className={`flex items-center gap-1 rounded-md border px-2 py-1 transition-colors disabled:cursor-not-allowed disabled:opacity-40 ${
              decode
                ? 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-400'
                : 'border-slate-300 bg-white text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300'
            }`}
            title={reversible ? '切换加密/解密方向' : 'SHL/SHR 有损不可逆,无解密方向'}
          >
            <Repeat className="h-3.5 w-3.5" />
            {decode ? '解密(逆变换)' : '加密(正变换)'}
          </button>
        </div>

        {/* 参数与域 */}
        <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-slate-600 dark:text-slate-300">
          {mode === 'inc' && (
            <>
              <label className="flex items-center gap-1">
                起始 b
                <input
                  type="number"
                  value={n}
                  onChange={(e) => setN(Number(e.target.value) || 0)}
                  className="w-16 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                />
              </label>
              <label className="flex items-center gap-1">
                步长 s
                <input
                  type="number"
                  value={s}
                  onChange={(e) => setS(Number(e.target.value) || 0)}
                  className="w-16 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                />
              </label>
              <span className="text-slate-400">第 i 位 +({n} + i·{s}),如 b=1 s=1 → +1,+2,+3…</span>
            </>
          )}
          {mode === 'const' && (
            <label className="flex items-center gap-1">
              常数 N(可负)
              <input
                type="number"
                value={n}
                onChange={(e) => setN(Number(e.target.value) || 0)}
                className="w-20 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
              />
            </label>
          )}
          {BIT_MODES.includes(mode) && (
            <label className="flex items-center gap-1">
              位数 n(1-7)
              <input
                type="number"
                min={0}
                max={7}
                value={n}
                onChange={(e) => setN(Math.max(0, Math.min(7, Number(e.target.value) || 0)))}
                className="w-16 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
              />
            </label>
          )}
          {(mode === 'rotl' || mode === 'rotr') && (
            <label className="flex items-center gap-1">
              旋转位数 k
              <input
                type="number"
                value={n}
                onChange={(e) => setN(Number(e.target.value) || 0)}
                className="w-16 rounded-md border border-slate-300 bg-white px-1.5 py-1 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
              />
            </label>
          )}
          {VALUE_MODES.includes(mode) && (
            <span className="flex items-center gap-1.5">
              值域:
              {(Object.keys(DOMAIN_LABEL) as ShiftDomain[]).map((d) => (
                <label key={d} className="flex items-center gap-0.5">
                  <input type="radio" checked={domain === d} onChange={() => setDomain(d)} />
                  {DOMAIN_LABEL[d]}
                </label>
              ))}
            </span>
          )}
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
            输出{inv ? ` · ${describeShift(inv)}` : ' · 该模式有损不可逆,仅正向'}
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
          <input
            value={format}
            onChange={(e) => setFormat(e.target.value)}
            placeholder="题目给的 flag 格式,如 flag{ / ctfshow{"
            className="min-w-[12rem] flex-1 rounded-md border border-slate-300 bg-white px-2 py-1.5 font-mono text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
          <label className="flex items-center gap-1 text-slate-600 dark:text-slate-300">
            <input type="checkbox" checked={anywhere} onChange={(e) => setAnywhere(e.target.checked)} />
            匹配任意位置(否则前缀)
          </label>
          <button
            type="button"
            onClick={runBrute}
            disabled={!input || !format.trim()}
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
                        setMode(h.opts.mode);
                        setDomain(h.opts.domain);
                        setN(h.opts.n);
                        if (h.opts.s !== undefined) setS(h.opts.s);
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
