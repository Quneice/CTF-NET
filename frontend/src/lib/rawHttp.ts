/**
 * 原始 HTTP 请求包解析（前端预览用，与后端 tools/web/ssti_base.py 的
 * parse_raw_http_request 保持一致）：请求行 + 头部 + 空行 + Body。
 * 仅支持 urlencoded 表单 / 查询参数作为注入载体。
 */

export interface RawHttpParam {
  name: string;
  value: string;
}

export interface RawHttpParseResult {
  method: string;
  url: string;
  params: RawHttpParam[];
  error?: string;
}

function safeDecode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

export function parseRawHttpRequest(raw: string): RawHttpParseResult {
  const text = raw.replace(/\r\n/g, '\n').replace(/\r/g, '\n').replace(/^\n+/, '');
  const lines = text.split('\n');
  const m = lines[0].trim().match(/^(\w+)\s+(\S+)(?:\s+HTTP\/\d(?:\.\d)?)?\s*$/);
  if (!m) {
    return { method: '', url: '', params: [], error: `无法解析请求行: ${lines[0]}` };
  }
  const method = m[1].toUpperCase();
  const target = m[2];

  const headers: Record<string, string> = {};
  let idx = 1;
  for (let i = 1; i < lines.length; i += 1) {
    const line = lines[i];
    if (!line.trim()) {
      idx = i + 1;
      break;
    }
    const sep = line.indexOf(':');
    if (sep > 0) {
      headers[line.slice(0, sep).trim().toLowerCase()] = line.slice(sep + 1).trim();
    }
  }
  const body = lines.slice(idx).join('\n').trim();

  let url: string;
  if (target.startsWith('http://') || target.startsWith('https://')) {
    url = target;
  } else {
    const host = headers.host ?? '';
    if (!host) {
      return { method, url: '', params: [], error: '请求行是相对路径时必须带 Host 头' };
    }
    const scheme = host.endsWith(':443') ? 'https' : 'http';
    url = `${scheme}://${host}${target}`;
  }

  const params: RawHttpParam[] = [];
  const query = url.split('?')[1] ?? '';
  if (query) {
    for (const kv of query.split('&')) {
      const [name, value = ''] = kv.split('=');
      if (name) params.push({ name: safeDecode(name), value: safeDecode(value) });
    }
  }
  if (body) {
    const ctype = headers['content-type'] ?? '';
    if (ctype.includes('multipart')) {
      return { method, url, params, error: '暂不支持 multipart/form-data 原始包' };
    }
    for (const kv of body.split('&')) {
      const [name, value = ''] = kv.split('=');
      if (name) params.push({ name: safeDecode(name), value: safeDecode(value) });
    }
  }
  return { method, url, params };
}
