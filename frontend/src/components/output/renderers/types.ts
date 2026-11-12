import type { OutputSpec } from '@/types';

/** 渲染器统一入参 */
export interface RendererProps {
  data: unknown;
  output: OutputSpec;
}
