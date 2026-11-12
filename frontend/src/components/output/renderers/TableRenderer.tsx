import type { OutputSpec } from '@/types';

import { CopyButton, DownloadButton } from '../OutputActions';
import type { RendererProps } from './types';

/** 表格输出 */
export default function TableRenderer({ data, output }: RendererProps) {
  const columns = (output as Extract<OutputSpec, { type: 'table' }>).columns;
  const rows = Array.isArray(data) ? (data as Record<string, unknown>[]) : [];

  const toCsv = () => {
    const header = columns.map((c) => c.title).join(',');
    const body = rows
      .map((r) => columns.map((c) => JSON.stringify(String(r[c.key] ?? ''))).join(','))
      .join('\n');
    return `${header}\n${body}`;
  };

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800">
      <div className="flex items-center justify-end gap-1 border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
        <CopyButton text={JSON.stringify(rows, null, 2)} />
        <DownloadButton text={toCsv()} filename="result.csv" mime="text/csv" />
      </div>
      <div className="max-h-96 overflow-auto">
        <table className="min-w-full text-sm">
          <thead className="bg-slate-50 dark:bg-slate-900">
            <tr>
              {columns.map((c) => (
                <th
                  key={c.key}
                  className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-600 dark:text-slate-300"
                >
                  {c.title}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={i} className="border-t border-slate-100 dark:border-slate-800">
                {columns.map((c) => (
                  <td key={c.key} className="whitespace-nowrap px-3 py-1.5 text-slate-700 dark:text-slate-300">
                    {String(row[c.key] ?? '')}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
