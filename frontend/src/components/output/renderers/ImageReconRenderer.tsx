import { useMemo, type ReactNode } from 'react';

import { CopyButton } from '../OutputActions';

/**
 * 图片二进制侦察渲染器(rendererId = 'image-recon')。
 * 后端 stdout 为 JSON(misc.image_recon):
 * { file{name,size,format,mime}, params, structure[], trailing?, embedded[], strings{count,previews,flags}, artifacts[], summary }
 */

interface ReconSeg {
  kind: string;
  name: string;
  offset: number;
  length: number;
  crc_ok?: boolean;
  note?: string;
}

interface ReconTrailing {
  offset: number;
  size: number;
  type: string;
  ext: string;
  preview_hex: string;
  preview_text: string;
  artifact: string;
}

interface ReconEmbedded {
  name: string;
  offset: number;
  size: number;
  artifact?: string;
  suspected: boolean;
}

interface ReconReport {
  file: { name: string; size: number; format: string; mime: string };
  structure: ReconSeg[];
  trailing: ReconTrailing | null;
  embedded: ReconEmbedded[];
  strings: { count: number; previews: string[]; flags: string[] };
  artifacts: string[];
  summary: string;
}

function parsePayload(data: unknown): ReconReport | null {
  try {
    const obj = typeof data === 'string' ? JSON.parse(data) : data;
    if (obj && typeof obj === 'object' && 'file' in obj && 'structure' in obj) {
      return obj as ReconReport;
    }
    return null;
  } catch {
    return null;
  }
}

function Section({ title, children, extra }: { title: string; children: ReactNode; extra?: ReactNode }) {
  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-800">
      <div className="flex items-center gap-2 border-b border-slate-200 px-3 py-2 dark:border-slate-800">
        <h4 className="text-xs font-semibold text-slate-700 dark:text-slate-200">{title}</h4>
        {extra && <span className="ml-auto text-[11px] text-slate-500 dark:text-slate-400">{extra}</span>}
      </div>
      <div className="px-3 py-2">{children}</div>
    </div>
  );
}

/** hex 字符串按 16 字节/行分组展示 */
function HexGrid({ hex }: { hex: string }) {
  const rows = useMemo(() => {
    const out: string[] = [];
    for (let i = 0; i < hex.length; i += 32) out.push(hex.slice(i, i + 32).replace(/(..)/g, '$1 ').trim());
    return out;
  }, [hex]);
  return (
    <pre className="overflow-x-auto rounded-md bg-slate-50 px-2 py-1.5 font-mono text-[11px] leading-5 text-slate-600 dark:bg-slate-900 dark:text-slate-300">
      {rows.map((r, i) => (
        <div key={i}>{r}</div>
      ))}
    </pre>
  );
}

export default function ImageReconRenderer({ data }: { data: unknown }) {
  const rep = useMemo(() => parsePayload(data), [data]);
  if (!rep) {
    return (
      <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-lg bg-slate-50 p-3 font-mono text-xs text-slate-600 dark:bg-slate-900 dark:text-slate-300">
        {typeof data === 'string' ? data : JSON.stringify(data, null, 2)}
      </pre>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {/* 头卡 + 摘要 */}
      <div className="rounded-lg border border-slate-200 px-3 py-2 dark:border-slate-800">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="text-sm font-semibold text-slate-800 dark:text-slate-100">
            {rep.file.format}
            <span className="ml-2 text-xs font-normal text-slate-500 dark:text-slate-400">
              {rep.file.name} · {rep.file.size.toLocaleString()} 字节 · {rep.file.mime}
            </span>
          </span>
          {rep.artifacts.length > 0 && (
            <span className="rounded-full bg-indigo-50 px-2 py-0.5 text-[11px] text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400">
              产物 {rep.artifacts.length} 个:{rep.artifacts.join('、')}
            </span>
          )}
        </div>
        <pre className="mt-1.5 whitespace-pre-wrap font-mono text-[11px] leading-5 text-slate-600 dark:text-slate-300">
          {rep.summary}
        </pre>
      </div>

      {/* 尾部附加数据 */}
      {rep.trailing && (
        <Section
          title={`结束标记后附加数据(${rep.trailing.type})`}
          extra={`@${rep.trailing.offset} · ${rep.trailing.size} 字节 · 产物 ${rep.trailing.artifact || '未导出'}`}
        >
          <HexGrid hex={rep.trailing.preview_hex} />
          <div className="mt-1.5 flex items-start gap-2">
            <code className="min-w-0 flex-1 break-all rounded-md bg-amber-50 px-2 py-1 font-mono text-xs text-amber-800 dark:bg-amber-500/10 dark:text-amber-300">
              {rep.trailing.preview_text}
            </code>
            <CopyButton text={rep.trailing.preview_text} />
          </div>
        </Section>
      )}

      {/* 嵌入内容 */}
      {rep.embedded.length > 0 && (
        <Section title={`魔数命中(${rep.embedded.length})`} extra="suspected=低置信(2 字节模式),不导出">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="text-slate-500 dark:text-slate-400">
                <th className="py-1 pr-3 font-medium">类型</th>
                <th className="py-1 pr-3 font-medium">偏移</th>
                <th className="py-1 pr-3 font-medium">大小</th>
                <th className="py-1 font-medium">产物</th>
              </tr>
            </thead>
            <tbody className="font-mono">
              {rep.embedded.map((e, i) => (
                <tr key={i} className="border-t border-slate-100 dark:border-slate-800">
                  <td className="py-1 pr-3">
                    {e.name}
                    {e.suspected && (
                      <span className="ml-1.5 rounded bg-amber-50 px-1 py-0.5 text-[10px] not-italic text-amber-700 dark:bg-amber-500/10 dark:text-amber-400">
                        疑似
                      </span>
                    )}
                  </td>
                  <td className="py-1 pr-3">{e.offset}</td>
                  <td className="py-1 pr-3">{e.suspected ? '—' : `${e.size} B`}</td>
                  <td className="py-1">{e.artifact || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Section>
      )}

      {/* 结构段表 */}
      {rep.structure.length > 0 && (
        <Section title={`结构解析(${rep.structure.length} 段)`} extra="CRC 不符可能是考点">
          <div className="max-h-56 overflow-y-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="text-slate-500 dark:text-slate-400">
                  <th className="py-1 pr-3 font-medium">名称</th>
                  <th className="py-1 pr-3 font-medium">偏移</th>
                  <th className="py-1 pr-3 font-medium">长度</th>
                  <th className="py-1 pr-3 font-medium">CRC</th>
                  <th className="py-1 font-medium">备注</th>
                </tr>
              </thead>
              <tbody className="font-mono">
                {rep.structure.map((s, i) => (
                  <tr key={i} className="border-t border-slate-100 dark:border-slate-800">
                    <td className="py-1 pr-3">{s.name}</td>
                    <td className="py-1 pr-3">{s.offset}</td>
                    <td className="py-1 pr-3">{s.length}</td>
                    <td className="py-1 pr-3">
                      {s.crc_ok === undefined ? '—' : s.crc_ok ? '✓' : '✗'}
                    </td>
                    <td className={`py-1 ${s.note ? 'text-amber-600 dark:text-amber-400' : 'text-slate-400'}`}>
                      {s.note || ''}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Section>
      )}

      {/* strings / flag */}
      {rep.strings && (rep.strings.previews.length > 0 || rep.strings.flags.length > 0) && (
        <Section
          title={`可打印字符串(${rep.strings.count})`}
          extra={rep.strings.flags.length > 0 ? `flag 命中 ${rep.strings.flags.length}` : undefined}
        >
          {rep.strings.flags.length > 0 && (
            <div className="mb-2 space-y-1">
              {rep.strings.flags.map((f, i) => (
                <div key={i} className="flex items-center gap-2">
                  <code className="min-w-0 flex-1 break-all rounded bg-emerald-50 px-2 py-1 font-mono text-xs font-semibold text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">
                    {f}
                  </code>
                  <CopyButton text={f} />
                </div>
              ))}
            </div>
          )}
          <pre className="max-h-44 overflow-y-auto whitespace-pre-wrap break-all rounded-md bg-slate-50 px-2 py-1.5 font-mono text-[11px] leading-5 text-slate-600 dark:bg-slate-900 dark:text-slate-300">
            {rep.strings.previews.join('\n') || '(无)'}
          </pre>
          <div className="mt-1.5">
            <CopyButton text={rep.strings.previews.join('\n')} />
          </div>
        </Section>
      )}
    </div>
  );
}
