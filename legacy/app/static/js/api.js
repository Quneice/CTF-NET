// API 客户端：统一封装后端 REST 接口
const BASE = '/api';

export const api = {
  async request(method, path, body) {
    const opts = { method, headers: {} };
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
    const res = await fetch(BASE + path, opts);
    if (!res.ok) {
      let detail = res.statusText;
      try {
        const data = await res.json();
        detail = data.detail || detail;
      } catch (_) { /* 忽略非 JSON 错误体 */ }
      throw new Error(detail);
    }
    return res.json();
  },

  get(path) {
    return this.request('GET', path);
  },

  post(path, body) {
    return this.request('POST', path, body);
  },

  async upload(file) {
    const fd = new FormData();
    fd.append('file', file);
    const res = await fetch(BASE + '/files', { method: 'POST', body: fd });
    if (!res.ok) throw new Error('文件上传失败');
    return res.json(); // { file_id, name, size }
  },
};
