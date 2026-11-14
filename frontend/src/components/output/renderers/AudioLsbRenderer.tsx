import { useMemo } from 'react';

import type { RendererProps } from './types';

import { CopyButton, DownloadButton } from '../OutputActions';

/**
 * LSB 隐写提取渲染器（rendererId = 'audio-lsb' | 'image-lsb'，两工具共享 JSON 结构）。
 * 后端 stdout 为 JSON：{ file, params, recon?, spectrogram?, candidates[], hits[], flags[], artifacts[], summary }。
 * audio-lsb 额外含 recon（chunk 清单/尾部附加数据/原始字节扫描）与 spectrogram（频谱图）。
 */

interface AudioLsbFileMeta {
  name: string;
  size: number;
  audio_format?: string;
  bits_per_sample?: number;
  channels?: number;
  sample_rate?: number;
  samples?: number;
  duration_sec?: number;
  format?: string; // image_lsb: png | bmp
  mode?: string; // image_lsb: NRGBA 8bit …
  width?: number;
  height?: number;
}

interface AudioLsbRecon {
  chunks: { id: string; size: number; offset: number }[];
  trailing_bytes: number;
  trailing_artifact?: string;
  trailing_type?: string;
  trailing_preview?: string;
  raw_flags: string[];
  raw_strings: string[];
}

interface AudioLsbSpectro {
  artifact?: string;
  nperseg?: number;
  freq_lo?: number;
  freq_hi?: number;
  preview?: string;
  error?: string;
}

interface AudioLsbCandidate {
  plane: number;
  order: string;
  channel: string;
  ones_ratio: number;
  type: string;
  hit: boolean;
}

interface AudioLsbHit extends AudioLsbCandidate {
  format?: string;
  offset: number;
  size: number;
  artifact: string;
  preview?: string;
  text?: string;
}

interface AudioLsbPayload {
  file: AudioLsbFileMeta;
  params: Record<string, unknown>;
  recon?: AudioLsbRecon;
  spectrogram?: AudioLsbSpectro;
  candidates: AudioLsbCandidate[];
  hits: AudioLsbHit[];
  flags: string[];
  artifacts: string[];
  summary: string;
}

function parsePayload(data: unknown): AudioLsbPayload | null {
  if (typeof data !== 'string') return null;
  try {
    const p = JSON.parse(data);
    if (p && p.file && Array.isArray(p.candidates) && Array.isArray(p.hits)) return p as AudioLsbPayload;
  } catch {
    /* fallthrough */
  }
  return null;
}

const chipCls = 'rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink-2';

function fmtSize(n: number) {
  return n >= 1048576 ? `${(n / 1048576).toFixed(1)}MB` : `${(n / 1024).toFixed(1)}KB`;
}

export default function AudioLsbRenderer({ data }: RendererProps) {
  const payload = useMemo(() => parsePayload(data), [data]);

  if (!payload) {
    return (
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs text-slate-700 dark:bg-slate-900 dark:text-slate-300">
        {String(data)}
      </pre>
    );
  }

  const { file, hits, flags, candidates, summary, recon, spectrogram } = payload;
  const isImage = typeof file.width === 'number';
  // 防御纵深:后端保证数组非 null,但旧产物/异常载荷不能炸整页(同 ImageOcrRenderer 模式)
  const safeHits = hits ?? [];
  const safeFlags = flags ?? [];
  const safeCandidates = candidates ?? [];
  const safeRawFlags = recon?.raw_flags ?? [];
  const safeRawStrings = recon?.raw_strings ?? [];
  const safeChunks = recon?.chunks ?? [];

  return (
    <div className="space-y-3">
      {/* —— 文件元数据 —— */}
      <div className="flex flex-wrap gap-2">
        <span className={chipCls}>{file.name}</span>
        {isImage ? (
          <span className={chipCls}>
            {file.format} · {file.mode} · {file.width}×{file.height}
          </span>
        ) : (
          <span className={chipCls}>
            {file.audio_format} {file.bits_per_sample}bit × {file.channels} 声道 @ {file.sample_rate}Hz
          </span>
        )}
        <span className={chipCls}>{fmtSize(file.size)}</span>
        {!isImage && (
          <span className={chipCls}>
            {file.duration_sec?.toFixed(1)} 秒 · {file.samples?.toLocaleString()} 采样
          </span>
        )}
        {safeHits.length > 0 ? (
          <span className="rounded-full border border-emerald-300 bg-emerald-50 px-3 py-1 text-xs text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-400">
            命中 {safeHits.length} 处
          </span>
        ) : (
          <span className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
            未发现已知类型隐写
          </span>
        )}
      </div>

      {/* —— P0 侦察:尾部附加数据 —— */}
      {recon && recon.trailing_bytes > 0 && (
        <div className="rounded-lg border border-amber-300 bg-amber-50 p-3 dark:border-amber-500/30 dark:bg-amber-500/10">
          <div className="text-xs font-medium text-amber-700 dark:text-amber-400">
            data 之后有 {recon.trailing_bytes} 字节附加数据 → {recon.trailing_artifact}（类型 {recon.trailing_type}）
          </div>
          {recon.trailing_preview ? (
            <img
              src={recon.trailing_preview}
              alt="尾部附加图片"
              className="mt-2 max-h-72 max-w-full rounded border border-slate-200 bg-white object-contain dark:border-slate-700"
            />
          ) : (
            <p className="mt-1 text-xs text-amber-600/80 dark:text-amber-400/70">
              完整内容见任务产物目录（copy /b 拼接文件或隐写工具残留的典型特征）
            </p>
          )}
        </div>
      )}

      {/* —— P0 侦察:chunk 清单 + 原始字节扫描 —— */}
      {recon && (safeRawFlags.length > 0 || safeChunks.length > 0) && (
        <details className="rounded-lg border border-slate-200 p-3 text-xs dark:border-slate-800">
          <summary className="cursor-pointer text-slate-500 dark:text-slate-400">
            文件侦察:chunk 清单({safeChunks.length})· 原始字节 flag {safeRawFlags.length} 条 · strings {safeRawStrings.length} 条
          </summary>
          <div className="mt-2 grid gap-2 md:grid-cols-2">
            <div>
              <div className="mb-1 text-slate-400">RIFF chunk 清单</div>
              <table className="min-w-full text-xs">
                <tbody>
                  {safeChunks.map((c, i) => (
                    <tr key={i} className="border-t border-slate-100 dark:border-slate-800">
                      <td className="px-2 py-1 font-mono text-slate-600 dark:text-slate-300">{c.id}</td>
                      <td className="px-2 py-1 text-slate-500 dark:text-slate-400">{c.size}B</td>
                      <td className="px-2 py-1 text-slate-400">@{c.offset}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="max-h-48 overflow-auto">
              <div className="mb-1 text-slate-400">原始字节扫描(注意甄别噪声巧合)</div>
              {safeRawFlags.map((f, i) => (
                <code key={`f${i}`} className="block break-all font-mono text-amber-700 dark:text-amber-400">{f}</code>
              ))}
              {safeRawStrings.slice(0, 15).map((s, i) => (
                <code key={`s${i}`} className="block break-all font-mono text-slate-500 dark:text-slate-400">{s}</code>
              ))}
            </div>
          </div>
        </details>
      )}

      {/* —— P1 频谱图 —— */}
      {spectrogram && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-800">
          <div className="flex flex-wrap items-center gap-2 border-b border-slate-200 px-3 py-2 dark:border-slate-800">
            <span className="text-sm font-medium text-slate-700 dark:text-slate-200">频谱图</span>
            {spectrogram.nperseg && (
              <span className="text-xs text-slate-500 dark:text-slate-400">
                窗 {spectrogram.nperseg} · {spectrogram.freq_lo?.toFixed(0)}–{spectrogram.freq_hi?.toFixed(0)}Hz
              </span>
            )}
            {spectrogram.artifact && (
              <span className="ml-auto text-xs text-slate-500 dark:text-slate-400">产物 {spectrogram.artifact}</span>
            )}
          </div>
          {spectrogram.error ? (
            <p className="px-3 py-2 text-xs text-red-600 dark:text-red-400">{spectrogram.error}</p>
          ) : spectrogram.preview ? (
            <img
              src={spectrogram.preview}
              alt="频谱图(行=频率,列=时间;查看画字/二维码/异常亮带)"
              className="max-h-96 w-full bg-black object-contain"
            />
          ) : (
            <p className="px-3 py-2 text-xs text-slate-500 dark:text-slate-400">图过大未内联预览,见任务产物目录</p>
          )}
          <p className="border-t border-slate-100 px-3 py-1.5 text-xs text-slate-400 dark:border-slate-800">
            白亮处为能量集中:横向持续的窄亮带=单音,成片字符/二维码=频域隐写,低频亮带=嵌入副产物
          </p>
        </div>
      )}

      {/* —— flag 提取 —— */}
      {safeFlags.length > 0 && (
        <div className="space-y-1 rounded-lg border border-emerald-300 bg-emerald-50 p-3 dark:border-emerald-500/30 dark:bg-emerald-500/10">
          <div className="text-xs font-medium text-emerald-700 dark:text-emerald-400">flag 正则命中</div>
          {safeFlags.map((f, i) => (
            <div key={i} className="flex items-center gap-2">
              <code className="min-w-0 flex-1 break-all font-mono text-sm text-emerald-800 dark:text-emerald-300">{f}</code>
              <CopyButton text={f} />
            </div>
          ))}
        </div>
      )}

      {/* —— 命中详情 —— */}
      {safeHits.map((h, i) => (
        <div key={i} className="rounded-lg border border-slate-200 dark:border-slate-800">
          <div className="flex flex-wrap items-center gap-2 border-b border-slate-200 px-3 py-2 dark:border-slate-800">
            <span className="rounded bg-indigo-50 px-2 py-0.5 font-mono text-xs text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-400">
              {h.channel}/bit{h.plane}/{h.order}
            </span>
            <span className="text-sm font-medium text-slate-700 dark:text-slate-200">{h.format || h.type}</span>
            <span className="text-xs text-slate-500 dark:text-slate-400">
              {fmtSize(h.size)} · 数据偏移 {h.offset}
            </span>
            <span className="ml-auto flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
              产物 {h.artifact}
            </span>
          </div>
          {h.preview && (
            <div className="border-b border-slate-100 p-3 dark:border-slate-800">
              <img
                src={h.preview}
                alt={`LSB 提取 ${h.format || h.type}`}
                className="max-h-96 max-w-full rounded border border-slate-200 bg-white object-contain dark:border-slate-700"
              />
              <p className="mt-1.5 text-xs text-slate-400">↑ 位平面直接解出的内容（完整文件见任务产物目录）</p>
            </div>
          )}
          {h.text && (
            <div className="flex items-start gap-2 p-3">
              <pre className="min-w-0 flex-1 overflow-auto whitespace-pre-wrap break-all font-mono text-xs text-slate-700 dark:text-slate-300">
                {h.text}
              </pre>
              <span className="flex shrink-0 flex-col gap-1">
                <CopyButton text={h.text} />
                <DownloadButton text={h.text} filename={h.artifact} mime="text/plain" />
              </span>
            </div>
          )}
        </div>
      ))}

      {/* —— 扫描全景 —— */}
      <details className="rounded-lg border border-slate-200 p-3 text-xs dark:border-slate-800">
        <summary className="cursor-pointer text-slate-500 dark:text-slate-400">
          位平面扫描全景（{safeCandidates.length} 组组合，点击展开）
        </summary>
        <div className="mt-2 max-h-64 overflow-auto">
          <table className="min-w-full text-xs">
            <thead className="sticky top-0 bg-slate-50 dark:bg-slate-900">
              <tr>
                <th className="px-3 py-1.5 text-left font-medium text-slate-600 dark:text-slate-300">组合</th>
                <th className="px-3 py-1.5 text-left font-medium text-slate-600 dark:text-slate-300">1/0 比例</th>
                <th className="px-3 py-1.5 text-left font-medium text-slate-600 dark:text-slate-300">识别类型</th>
                <th className="px-3 py-1.5 text-left font-medium text-slate-600 dark:text-slate-300">命中</th>
              </tr>
            </thead>
            <tbody>
              {safeCandidates.map((c, i) => (
                <tr key={i} className={`border-t border-slate-100 dark:border-slate-800 ${c.hit ? 'bg-emerald-50/50 dark:bg-emerald-500/5' : ''}`}>
                  <td className="whitespace-nowrap px-3 py-1 font-mono text-slate-700 dark:text-slate-300">
                    {c.channel}/bit{c.plane}/{c.order}
                  </td>
                  <td className="whitespace-nowrap px-3 py-1 text-slate-500 dark:text-slate-400">{c.ones_ratio.toFixed(4)}</td>
                  <td className="whitespace-nowrap px-3 py-1 text-slate-500 dark:text-slate-400">{c.type || '—'}</td>
                  <td className="whitespace-nowrap px-3 py-1">{c.hit ? '✓' : ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-2 text-slate-400">1/0 比例 ≈ 0.5 且无类型 = 该平面为噪声;有魔数/文本即命中。</p>
      </details>

      {/* —— 文字总结 —— */}
      <details className="rounded-lg border border-slate-200 p-3 text-xs dark:border-slate-800" open>
        <summary className="cursor-pointer text-slate-500 dark:text-slate-400">报告原文（summary）</summary>
        <pre className="mt-2 whitespace-pre-wrap font-mono text-slate-600 dark:text-slate-300">{summary}</pre>
      </details>
    </div>
  );
}
