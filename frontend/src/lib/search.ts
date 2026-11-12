import type { ToolDefinition } from '@/types';

/**
 * 工具模糊搜索：对 name / description / tags 做匹配并打分排序。
 * 支持多关键词（空格分隔），任一词不命中则排除；子序列匹配作为兜底。
 */

/** 判断 needle 是否为 haystack 的子序列 */
function isSubsequence(needle: string, haystack: string): boolean {
  let i = 0;
  for (const ch of haystack) {
    if (ch === needle[i]) i++;
    if (i === needle.length) return true;
  }
  return needle.length === 0;
}

export function searchTools(tools: ToolDefinition[], query: string, limit = 20): ToolDefinition[] {
  const q = query.trim().toLowerCase();
  if (!q) return [];

  const terms = q.split(/\s+/).filter(Boolean);

  interface Scored {
    tool: ToolDefinition;
    score: number;
  }
  const results: Scored[] = [];

  for (const tool of tools) {
    const name = tool.name.toLowerCase();
    const desc = tool.description.toLowerCase();
    const tags = tool.tags.join(' ').toLowerCase();

    let total = 0;
    let ok = true;

    for (const term of terms) {
      let s = 0;
      if (name === term) s = 200;
      else if (name.startsWith(term)) s = 150;
      else if (name.includes(term)) s = 120;
      else if (tags.includes(term)) s = 60;
      else if (desc.includes(term)) s = 30;
      else if (isSubsequence(term, name)) s = 15;
      else {
        ok = false;
        break;
      }
      total += s;
    }

    if (ok) results.push({ tool, score: total });
  }

  results.sort((a, b) => b.score - a.score);
  return results.slice(0, limit).map((r) => r.tool);
}
