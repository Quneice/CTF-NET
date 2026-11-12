// 可复用渲染组件：全部由后端元数据驱动，不硬编码任何工具

export function el(tag, props = {}, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v === undefined || v === null) continue;
    if (k === 'class') node.className = v;
    else if (k === 'text') node.textContent = v;
    else if (k === 'html') node.innerHTML = v;
    else if (k.startsWith('on') && typeof v === 'function') node.addEventListener(k.slice(2), v);
    else node.setAttribute(k, v);
  }
  for (const child of children.flat()) {
    if (child == null) continue;
    node.append(child.nodeType ? child : document.createTextNode(String(child)));
  }
  return node;
}

export const badge = (text) => el('span', { class: 'badge', text });

export function fmtSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / 1024 / 1024).toFixed(1) + ' MB';
}

// 分类导航
export function renderCategories(categories, active, onSelect) {
  const frag = document.createDocumentFragment();
  const all = { name: 'all', tool_count: categories.reduce((s, c) => s + c.tool_count, 0) };
  for (const cat of [all, ...categories]) {
    const item = el('button', {
      class: 'cat' + (cat.name === active ? ' active' : ''),
      onclick: () => onSelect(cat.name),
    },
      el('span', { class: 'cat-name', text: cat.name === 'all' ? '全部' : cat.name }),
      el('span', { class: 'cat-count', text: cat.tool_count }));
    frag.append(item);
  }
  return frag;
}

// 工具列表
export function renderToolList(tools, onSelect) {
  const frag = document.createDocumentFragment();
  if (!tools.length) {
    frag.append(el('div', { class: 'list-empty', text: '没有匹配的工具' }));
    return frag;
  }
  for (const t of tools) {
    const card = el('button', { class: 'tool-card', onclick: () => onSelect(t.id) },
      el('div', { class: 'tool-card-top' },
        el('span', { class: 'tool-name', text: t.name }),
        badge(t.category)),
      el('div', { class: 'tool-desc', text: t.description || '暂无描述' }));
    frag.append(card);
  }
  return frag;
}

// 参数表单：根据 ParamSpec.type 渲染对应控件
export function renderParamFields(params, onUpload) {
  const container = el('div', { class: 'param-fields' });
  const fields = [];
  for (const spec of params) {
    const field = makeField(spec, onUpload);
    container.append(field.wrap);
    fields.push(field);
  }
  return { container, fields };
}

function makeField(spec, onUpload) {
  const label = el('label', { class: 'param-label' },
    el('span', { class: 'param-name', text: spec.name + (spec.required ? ' *' : '') }),
    el('span', { class: 'param-type', text: spec.type }),
    spec.description ? el('span', { class: 'param-hint', text: spec.description }) : null);
  const wrap = el('div', { class: 'param' }, label);

  let input = null;
  let fileId = null;

  switch (spec.type) {
    case 'text':
      input = el('textarea', { class: 'input textarea', placeholder: spec.description || '' });
      break;
    case 'bool':
      input = el('input', { class: 'checkbox', type: 'checkbox' });
      if (spec.default) input.checked = true;
      break;
    case 'int':
      input = el('input', { class: 'input', type: 'number', step: '1', value: spec.default ?? '' });
      break;
    case 'float':
      input = el('input', { class: 'input', type: 'number', step: 'any', value: spec.default ?? '' });
      break;
    case 'password':
      input = el('input', { class: 'input', type: 'password', value: spec.default ?? '', placeholder: spec.description || '' });
      break;
    case 'file': {
      const fileInput = el('input', { class: 'input', type: 'file' });
      const status = el('span', { class: 'file-status', text: '未选择文件' });
      fileInput.addEventListener('change', async () => {
        const file = fileInput.files[0];
        if (!file) return;
        status.textContent = '上传中…';
        try {
          const res = await onUpload(file);
          fileId = res.file_id;
          status.textContent = `已上传: ${res.name} (${fmtSize(res.size)})`;
        } catch (e) {
          fileId = null;
          status.textContent = '上传失败: ' + e.message;
        }
      });
      wrap.append(fileInput, status);
      return {
        name: spec.name,
        wrap,
        getValue: () => fileId,
        validate: () => (spec.required && !fileId ? `${spec.name} 需要上传文件` : null),
      };
    }
    default: // str
      input = el('input', { class: 'input', type: 'text', value: spec.default ?? '', placeholder: spec.description || '' });
  }

  if (input) wrap.append(input);

  return {
    name: spec.name,
    wrap,
    getValue: () => {
      if (spec.type === 'bool') return input.checked;
      const v = input.value;
      if (spec.type === 'int') return v === '' ? null : parseInt(v, 10);
      if (spec.type === 'float') return v === '' ? null : parseFloat(v);
      return v;
    },
    validate: () => {
      if (!spec.required || spec.type === 'bool') return null;
      return (input.value === '' || input.value == null) ? `${spec.name} 为必填` : null;
    },
  };
}

// 执行结果
export function renderResult(result, jobId) {
  const status = el('span', { class: 'status status-' + result.status, text: result.status });
  const head = el('div', { class: 'result-head' },
    el('span', { class: 'result-title', text: '执行结果' }),
    status,
    el('span', { class: 'result-duration', text: `耗时 ${result.duration}s · exit ${result.exit_code}` }));

  const stdout = el('pre', { class: 'output', text: result.stdout || '(无输出)' });
  const stderr = result.stderr
    ? el('pre', { class: 'output stderr', text: result.stderr })
    : null;

  const artifacts = result.artifacts && result.artifacts.length
    ? el('div', { class: 'artifacts' },
        el('div', { class: 'result-title', text: '产物' }),
        ...result.artifacts.map(a =>
          el('a', {
            class: 'artifact-link',
            href: `/api/tasks/${jobId}/artifacts/${encodeURIComponent(a.name)}`,
            download: a.name,
            text: `${a.name} (${fmtSize(a.size)})`,
          })))
    : null;

  return el('div', { class: 'result' }, head, stdout, stderr, artifacts);
}
