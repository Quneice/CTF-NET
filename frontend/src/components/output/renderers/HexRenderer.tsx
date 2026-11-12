import TextResult from './TextResult';
import type { RendererProps } from './types';

/** 十六进制输出 */
export default function HexRenderer({ data }: RendererProps) {
  const text = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
  return <TextResult text={text} filename="result.hex" />;
}
