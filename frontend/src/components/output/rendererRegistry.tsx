import { lazy } from 'react';
import type { ComponentType } from 'react';

import type { OutputSpec } from '@/types';

import type { RendererProps } from './renderers/types';

/**
 * 渲染器注册表：按 OutputSpec.type 路由到对应渲染器组件。
 *
 * 所有渲染器通过 React.lazy 按需动态 import（代码分割，避免首屏体积过大），
 * 调用方需包 <Suspense>。未知类型 / 未知自定义 rendererId 一律降级为 RawTextRenderer。
 */

const TextRenderer = lazy(() => import('./renderers/TextRenderer'));
const MarkdownRenderer = lazy(() => import('./renderers/MarkdownRenderer'));
const JsonRenderer = lazy(() => import('./renderers/JsonRenderer'));
const CodeRenderer = lazy(() => import('./renderers/CodeRenderer'));
const TableRenderer = lazy(() => import('./renderers/TableRenderer'));
const ImageRenderer = lazy(() => import('./renderers/ImageRenderer'));
const FileRenderer = lazy(() => import('./renderers/FileRenderer'));
const HexRenderer = lazy(() => import('./renderers/HexRenderer'));
const RawTextRenderer = lazy(() => import('./renderers/RawTextRenderer'));
const TxGraphRenderer = lazy(() => import('./renderers/TxGraphRenderer'));
const WechatDatRenderer = lazy(() => import('./renderers/WechatDatRenderer'));
const ImageBatchRenderer = lazy(() => import('./renderers/ImageBatchRenderer'));
const ImageQrcodeRenderer = lazy(() => import('./renderers/ImageQrcodeRenderer'));
const ImageOcrRenderer = lazy(() => import('./renderers/ImageOcrRenderer'));
const TableMergeRenderer = lazy(() => import('./renderers/TableMergeRenderer'));
const AudioAsrRenderer = lazy(() => import('./renderers/AudioAsrRenderer'));
const AudioLsbRenderer = lazy(() => import('./renderers/AudioLsbRenderer'));

type Renderer = ComponentType<RendererProps>;

/** 内置类型 -> 渲染器 */
const registry: Partial<Record<OutputSpec['type'], Renderer>> = {
  text: TextRenderer,
  stream: TextRenderer,
  markdown: MarkdownRenderer,
  json: JsonRenderer,
  code: CodeRenderer,
  table: TableRenderer,
  image: ImageRenderer,
  file: FileRenderer,
  hex: HexRenderer,
};

/** 自定义渲染器（rendererId -> 渲染器） */
const customRenderers: Record<string, Renderer> = {
  'tx-graph': TxGraphRenderer,
  'wechat-dat': WechatDatRenderer,
  'image-batch': ImageBatchRenderer,
  'image-qrcode': ImageQrcodeRenderer,
  'image-ocr': ImageOcrRenderer,
  'table-merge': TableMergeRenderer,
  'audio-asr': AudioAsrRenderer,
  'audio-lsb': AudioLsbRenderer,
  'image-lsb': AudioLsbRenderer, // 与 audio-lsb 共享 JSON 结构与渲染组件
};

export function resolveRenderer(output: OutputSpec): Renderer {
  if (output.type === 'custom') {
    return customRenderers[output.rendererId] ?? RawTextRenderer;
  }
  return registry[output.type] ?? RawTextRenderer;
}
