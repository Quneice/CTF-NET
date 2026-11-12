import type { OutputSpec } from '@/types';

import { highlightCode, Tokens } from '@/lib/highlight';

import TextResult from './TextResult';
import type { RendererProps } from './types';

/** 代码输出（语法高亮） */
export default function CodeRenderer({ data, output }: RendererProps) {
  const code = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
  const lang = (output as Extract<OutputSpec, { type: 'code' }>).language ?? 'text';
  const ext = lang === 'asm' ? 'asm' : lang === 'python' ? 'py' : lang === 'js' ? 'js' : 'txt';

  return (
    <TextResult
      text={code}
      filename={`result.${ext}`}
      render={(t) => <Tokens tokens={highlightCode(t, lang)} />}
    />
  );
}
