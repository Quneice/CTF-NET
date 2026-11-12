import TextResult from './TextResult';
import type { RendererProps } from './types';

/** 纯文本输出 */
export default function TextRenderer({ data }: RendererProps) {
  const text = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
  return <TextResult text={text} filename="result.txt" />;
}
