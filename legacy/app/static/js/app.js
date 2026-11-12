// 应用装配：状态管理 + 事件流
import { api } from './api.js';
import {
  el,
  renderCategories,
  renderToolList,
  renderParamFields,
  renderResult,
} from './components.js';

const state = {
  categories: [],
  tools: [],
  activeCategory: 'all',
  selectedTool: null,
  paramForm: null,
  resultBox: null,
  running: false,
};

const searchInput = document.getElementById('search');
const categoriesEl = document.getElementById('categories');
const toolListEl = document.getElementById('tool-list');
const emptyEl = document.getElementById('empty');
const toolPanelEl = document.getElementById('tool-panel');

function filteredTools() {
  const q = searchInput.value.trim().toLowerCase();
  return state.tools.filter((t) => {
    const inCat = state.activeCategory === 'all' || t.category === state.activeCategory;
    const inSearch = !q || `${t.name} ${t.description || ''}`.toLowerCase().includes(q);
    return inCat && inSearch;
  });
}

function renderCategoriesInto() {
  categoriesEl.replaceChildren(
    renderCategories(state.categories, state.activeCategory, (cat) => {
      state.activeCategory = cat;
      renderCategoriesInto();
      renderToolsInto();
    })
  );
}

function renderToolsInto() {
  toolListEl.replaceChildren(renderToolList(filteredTools(), onSelectTool));
}

async function onSelectTool(toolId) {
  const tool = await api.get('/tools/' + toolId);
  state.selectedTool = tool;
  emptyEl.classList.add('hidden');
  toolPanelEl.classList.remove('hidden');
  renderToolPanel(tool);
}

function renderToolPanel(tool) {
  toolPanelEl.replaceChildren();

  const header = el('div', { class: 'panel-header' },
    el('div', { class: 'panel-title-row' },
      el('h2', { class: 'panel-title', text: tool.name }),
      (() => { const b = document.createElement('span'); b.className = 'badge'; b.textContent = tool.category; return b; })()),
    el('div', { class: 'panel-meta', text: `v${tool.version} · ${tool.id}` }),
    tool.description ? el('p', { class: 'panel-desc', text: tool.description }) : null);

  const { container, fields } = renderParamFields(tool.params, api.upload);
  state.paramForm = { fields };

  const runBtn = el('button', {
    class: 'run-btn',
    text: '运行',
    onclick: () => onRun(tool, runBtn),
  });

  state.resultBox = el('div', { class: 'result-box' });

  toolPanelEl.append(header, container, runBtn, state.resultBox);
}

async function onRun(tool, runBtn) {
  if (state.running) return;

  for (const f of state.paramForm.fields) {
    const err = f.validate();
    if (err) {
      state.resultBox.replaceChildren(el('pre', { class: 'output stderr', text: '参数错误: ' + err }));
      return;
    }
  }

  const params = {};
  for (const f of state.paramForm.fields) {
    const v = f.getValue();
    if (v !== null && v !== undefined) params[f.name] = v;
  }

  state.running = true;
  runBtn.disabled = true;
  runBtn.textContent = '运行中…';
  state.resultBox.replaceChildren(el('div', { class: 'loading', text: '提交任务…' }));

  try {
    const { job_id } = await api.post(`/tools/${tool.id}/run`, { params });
    state.resultBox.replaceChildren(el('div', { class: 'loading', text: '运行中…' }));
    const job = await pollJob(job_id);
    state.resultBox.replaceChildren(
      job.error
        ? el('pre', { class: 'output stderr', text: '错误: ' + job.error })
        : renderResult(job.result, job_id)
    );
  } catch (e) {
    state.resultBox.replaceChildren(el('pre', { class: 'output stderr', text: '请求失败: ' + e.message }));
  } finally {
    state.running = false;
    runBtn.disabled = false;
    runBtn.textContent = '运行';
  }
}

function pollJob(jobId) {
  return new Promise((resolve, reject) => {
    (async function loop() {
      try {
        const job = await api.get('/tasks/' + jobId);
        if (['success', 'error', 'timeout'].includes(job.status)) return resolve(job);
        setTimeout(loop, 500);
      } catch (e) {
        reject(e);
      }
    })();
  });
}

searchInput.addEventListener('input', renderToolsInto);

(async function init() {
  try {
    [state.categories, state.tools] = await Promise.all([
      api.get('/categories'),
      api.get('/tools'),
    ]);
    renderCategoriesInto();
    renderToolsInto();
  } catch (e) {
    emptyEl.classList.remove('hidden');
    emptyEl.replaceChildren(el('div', { class: 'empty-title', text: '加载失败' }), el('div', { class: 'empty-sub', text: e.message }));
  }
})();
