import TextResult from './TextResult';
import type { RendererProps } from './types';

/** 未知输出类型 / 未知自定义渲染器的兜底：原始文本 */
export default function RawTextRenderer({ data }: RendererProps) {
  const text = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
  return <TextResult text={text} filename="result.txt" />;
}
