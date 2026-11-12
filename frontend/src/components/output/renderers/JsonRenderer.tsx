import { highlightJson, Tokens } from '@/lib/highlight';

import TextResult from './TextResult';
import type { RendererProps } from './types';

/** JSON 输出（语法高亮 + 超长折叠） */
export default function JsonRenderer({ data }: RendererProps) {
  const text = JSON.stringify(data, null, 2);
  return (
    <TextResult
      text={text}
      filename="result.json"
      mime="application/json"
      render={(t) => <Tokens tokens={highlightJson(t)} />}
    />
  );
}
