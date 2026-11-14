/**
 * 代码审计工作台(渗透测试模式)。
 * 三栏:文件树 | 代码查看/编辑(敏感函数高亮) | 敏感扫描 / 全文搜索 / 规则库。
 * 后端 internal/audit(SQLite 规则库 + audits/<id>/src 文件本体,首次编辑自动 .bak 备份)。
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import {
  AuditProject, FileNode, SearchHit, SensHit, SensitiveFunc,
  addSensitiveFunc, auditFileTree, auditReadFile, auditScan, auditSearch,
  auditWriteFile, deleteAuditProject, deleteSensitiveFunc,
  importAuditProject, listAuditProjects, listSensitiveFuncs, updateSensitiveFunc,
} from '@/lib/audit';

const SKIP_DIR_PATTERNS = [/node_modules\//, /\.git\//, /__pycache__\//, /\.venv\//, /vendor\/(node|bin)\//];
const MAX_FILE_SIZE = 1.5 * 1024 * 1024;

const riskCls: Record<string, string> = {
  high: 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-300',
  medium: 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300',
  low: 'bg-sky-100 text-sky-700 dark:bg-sky-500/20 dark:text-sky-300',
};

interface CompiledRule { re: RegExp; rule: SensitiveFunc }

/** 把整行按敏感规则切片(用于 <mark> 高亮;规则为 Go RE2,绝大多数兼容 JS RegExp)。 */
function highlightLine(line: string, rules: CompiledRule[]): { text: string; rule?: SensitiveFunc }[] {
  interface Seg { s: number; e: number; rule: SensitiveFunc }
  const marks: Seg[] = [];
  for (const { re, rule } of rules) {
    re.lastIndex = 0;
    let m: RegExpExecArray | null;
    while ((m = re.exec(line)) !== null) {
      if (m[0].length === 0) { re.lastIndex++; continue; }
      marks.push({ s: m.index, e: m.index + m[0].length, rule });
      break; // 每条规则取首个命中即可,避免重叠噪声
    }
  }
  if (marks.length === 0) return [{ text: line }];
  marks.sort((a, b) => a.s - b.s);
  const out: { text: string; rule?: SensitiveFunc }[] = [];
  let pos = 0;
  for (const mk of marks) {
    if (mk.s < pos) continue; // 重叠跳过
    if (mk.s > pos) out.push({ text: line.slice(pos, mk.s) });
    out.push({ text: line.slice(mk.s, mk.e), rule: mk.rule });
    pos = mk.e;
  }
  if (pos < line.length) out.push({ text: line.slice(pos) });
  return out;
}

export default function CodeAuditPage() {
  const nav = useNavigate();
  const [projects, setProjects] = useState<AuditProject[]>([]);
  const [pid, setPid] = useState('');
  const [tree, setTree] = useState<FileNode[]>([]);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [curFile, setCurFile] = useState('');
  const [content, setContent] = useState('');
  const [draft, setDraft] = useState('');
  const [editing, setEditing] = useState(false);
  const [tab, setTab] = useState<'scan' | 'search' | 'rules'>('scan');
  const [scanHits, setScanHits] = useState<SensHit[] | null>(null);
  const [scanning, setScanning] = useState(false);
  const [query, setQuery] = useState('');
  const [useRegex, setUseRegex] = useState(false);
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [searchHits, setSearchHits] = useState<SearchHit[] | null>(null);
  const [rules, setRules] = useState<SensitiveFunc[]>([]);
  const [importing, setImporting] = useState('');
  const [msg, setMsg] = useState('');
  const viewerRef = useRef<HTMLDivElement>(null);

  const proj = projects.find((p) => p.id === pid);

  const refreshProjects = useCallback(async (select?: string) => {
    try {
      const list = await listAuditProjects();
      setProjects(list);
      if (select) setPid(select);
      else if (!list.find((p) => p.id === pid) && list.length > 0) setPid(list[0].id);
    } catch (e) {
      setMsg(String(e));
    }
  }, [pid]);

  useEffect(() => { refreshProjects(); listSensitiveFuncs().then(setRules).catch(() => setRules([])); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!pid) { setTree([]); return; }
    setCurFile(''); setContent(''); setScanHits(null); setSearchHits(null); setEditing(false);
    auditFileTree(pid).then((t) => {
      setTree(t);
      // 初始展开:根目录 + 一级目录(深度 ≤2 的目录节点),保证首屏能看到一级文件
      const init = new Set<string>();
      for (const n of t) {
        if (n.is_dir && n.path.split('/').length <= 2) init.add(n.path);
      }
      setExpanded(init);
    }).catch((e) => setMsg(String(e)));
  }, [pid]);

  const compiled = useMemo<CompiledRule[]>(() => rules
    .filter((r) => r.enabled)
    .map((r) => { try { return { re: new RegExp(r.pattern, 'g'), rule: r }; } catch { return null; } })
    .filter((x): x is CompiledRule => x !== null), [rules]);

  const fileHits = useMemo(() => (scanHits ?? []).filter((h) => h.path === curFile), [scanHits, curFile]);
  const hitLines = useMemo(() => new Set(fileHits.map((h) => h.line_no)), [fileHits]);

  const openFile = useCallback(async (path: string, line?: number) => {
    if (!pid) return;
    try {
      const text = await auditReadFile(pid, path);
      setCurFile(path);
      setContent(text);
      setEditing(false);
      if (line !== undefined) {
        requestAnimationFrame(() => {
          viewerRef.current?.querySelector(`[data-line="${line}"]`)?.scrollIntoView({ block: 'center' });
        });
      }
    } catch (e) {
      setMsg(`读取 ${path} 失败: ${e instanceof Error ? e.message : String(e)}`);
    }
  }, [pid]);

  const doImport = async (fileList: FileList | null) => {
    if (!fileList || fileList.length === 0) return;
    const files = Array.from(fileList).filter((f) => {
      const rel = (f as File & { webkitRelativePath?: string }).webkitRelativePath || f.name;
      return !SKIP_DIR_PATTERNS.some((re) => re.test(`${rel}/`)) && f.size <= MAX_FILE_SIZE;
    });
    if (files.length === 0) { setMsg('过滤后没有可导入的文件'); return; }
    const root = ((files[0] as File & { webkitRelativePath?: string }).webkitRelativePath || '').split('/')[0] || '项目';
    setImporting(`上传 0/${files.length}`);
    try {
      const p = await importAuditProject(root, files, (d, t) => setImporting(`上传 ${d}/${t}`));
      setImporting('');
      await refreshProjects(p.id);
      setMsg(`已导入「${p.name}」(${p.files} 个文件)`);
    } catch (e) {
      setImporting('');
      setMsg(`导入失败: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  const doScan = async () => {
    if (!pid) return;
    setScanning(true);
    try { setScanHits(await auditScan(pid)); } catch (e) { setMsg(String(e)); } finally { setScanning(false); }
  };

  const doSearch = async () => {
    if (!pid || !query.trim()) return;
    try { setSearchHits(await auditSearch(pid, query, useRegex, caseSensitive)); } catch (e) { setMsg(String(e)); }
  };

  const saveFile = async () => {
    if (!pid || !curFile) return;
    try {
      await auditWriteFile(pid, curFile, draft);
      setContent(draft);
      setEditing(false);
      setMsg(`已保存 ${curFile}(首次修改前的原件自动备份)`);
    } catch (e) { setMsg(String(e)); }
  };

  const toggleRule = async (r: SensitiveFunc) => {
    const next = { ...r, enabled: !r.enabled };
    await updateSensitiveFunc(next);
    setRules((rs) => rs.map((x) => (x.id === r.id ? next : x)));
  };

  const removeRule = async (id: string) => {
    await deleteSensitiveFunc(id);
    setRules((rs) => rs.filter((x) => x.id !== id));
  };

  const addRule = async (form: FormData) => {
    const language = String(form.get('language') || '').trim();
    const pattern = String(form.get('pattern') || '').trim();
    const category = String(form.get('category') || '').trim() || '未分类';
    const risk = String(form.get('risk') || 'medium');
    const note = String(form.get('note') || '');
    if (!language || !pattern) { setMsg('语言与正则必填'); return; }
    try {
      const f = await addSensitiveFunc(language, pattern, category, risk, note, true);
      setRules((rs) => [...rs, f]);
    } catch (e) { setMsg(`添加失败: ${e instanceof Error ? e.message : String(e)}`); }
  };

  const lines = useMemo(() => content.split('\n'), [content]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* —— 顶栏 —— */}
      <div className="flex flex-wrap items-center gap-2 border-b border-slate-200 p-3 dark:border-slate-800">
        <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">代码审计</span>
        <select
          value={pid}
          onChange={(e) => setPid(e.target.value)}
          className="rounded-md border border-slate-200 bg-white px-2 py-1 text-sm dark:border-slate-700 dark:bg-slate-900"
        >
          {projects.length === 0 && <option value="">（先导入一个项目）</option>}
          {projects.map((p) => <option key={p.id} value={p.id}>{p.name}（{p.files} 文件）</option>)}
        </select>
        <label className="cursor-pointer rounded-md bg-indigo-600 px-3 py-1 text-sm font-medium text-white hover:bg-indigo-500">
          导入项目文件夹
          <input type="file" multiple className="hidden"
            {...({ webkitdirectory: '', directory: '' } as Record<string, string>)}
            onChange={(e) => { doImport(e.target.files); e.currentTarget.value = ''; }} />
        </label>
        {importing && <span className="text-xs text-indigo-500">{importing}</span>}
        {proj && (
          <button
            onClick={async () => { if (confirm(`删除审计项目「${proj.name}」?`)) { await deleteAuditProject(pid); refreshProjects(); } }}
            className="rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          >删除项目</button>
        )}
        <button onClick={() => nav('/')} className="ml-auto text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300">返回首页</button>
      </div>
      {msg && (
        <div className="border-b border-amber-200 bg-amber-50 px-3 py-1.5 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300">
          {msg} <button className="ml-2 underline" onClick={() => setMsg('')}>关闭</button>
        </div>
      )}

      {/* —— 三栏主体 —— */}
      <div className="flex min-h-0 flex-1">
        {/* 文件树 */}
        <div className="w-60 shrink-0 overflow-auto border-r border-slate-200 p-2 dark:border-slate-800">
          {tree.length > 0 && (
            <div className="mb-1 flex gap-1 text-[10px] text-slate-400">
              <button className="hover:text-indigo-500" onClick={() => setExpanded(new Set(tree.filter((n) => n.is_dir).map((n) => n.path)))}>全部展开</button>
              <span>|</span>
              <button className="hover:text-indigo-500" onClick={() => setExpanded(new Set())}>全部收起</button>
            </div>
          )}
          {tree.length === 0 && <p className="px-2 py-4 text-xs text-slate-400">{pid ? '无文件' : '导入项目后显示文件树'}</p>}
          {tree.map((n) => {
            const indent = n.path.split('/').length - 1;
            const name = n.path.slice(n.path.lastIndexOf('/') + 1);
            // 节点可见 = 全部祖先目录已展开(目录与文件同规则,否则折叠后子目录行残留)
            const segs = n.path.split('/');
            let prefix = '';
            let visible = true;
            for (let i = 0; i < segs.length - 1 && visible; i++) {
              prefix = prefix ? `${prefix}/${segs[i]}` : segs[i];
              if (!expanded.has(prefix)) visible = false;
            }
            if (!visible) return null;
            if (n.is_dir) {
              const open = expanded.has(n.path);
              return (
                <div key={n.path} className="flex cursor-pointer items-center gap-1 rounded px-1 py-0.5 text-xs text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
                  style={{ paddingLeft: indent * 12 + 4 }}
                  onClick={() => setExpanded((s) => { const nx = new Set(s); if (open) nx.delete(n.path); else nx.add(n.path); return nx; })}>
                  <span className="text-slate-400">{open ? '▾' : '▸'}</span>📁 {name}
                </div>
              );
            }
            return (
              <div key={n.path}
                className={`flex cursor-pointer items-center gap-1 rounded px-1 py-0.5 text-xs ${curFile === n.path ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-300' : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}`}
                style={{ paddingLeft: indent * 12 + 18 }}
                title={n.path}
                onClick={() => openFile(n.path)}>
                📄 {name}
              </div>
            );
          })}
        </div>

        {/* 代码区 */}
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-2 border-b border-slate-200 px-3 py-1.5 dark:border-slate-800">
            <span className="truncate font-mono text-xs text-slate-500 dark:text-slate-400">{curFile || '（选择左侧文件查看）'}</span>
            {curFile && fileHits.length > 0 && (
              <span className={`rounded-full px-2 py-0.5 text-xs ${riskCls[fileHits[0].risk] ?? riskCls.low}`}>敏感命中 {fileHits.length}</span>
            )}
            {curFile && !editing && (
              <button onClick={() => { setDraft(content); setEditing(true); }} className="ml-auto rounded border border-slate-200 px-2 py-0.5 text-xs dark:border-slate-700">编辑</button>
            )}
            {editing && (
              <span className="ml-auto flex gap-1">
                <button onClick={saveFile} className="rounded bg-emerald-600 px-2 py-0.5 text-xs text-white hover:bg-emerald-500">保存</button>
                <button onClick={() => setEditing(false)} className="rounded border border-slate-200 px-2 py-0.5 text-xs dark:border-slate-700">取消</button>
              </span>
            )}
          </div>
          {editing ? (
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              spellCheck={false}
              className="min-h-0 flex-1 resize-none bg-slate-50 p-3 font-mono text-xs leading-5 outline-none dark:bg-slate-900"
            />
          ) : (
            <div ref={viewerRef} className="min-h-0 flex-1 overflow-auto bg-slate-50 dark:bg-slate-900">
              {curFile === '' ? (
                <p className="p-4 text-xs text-slate-400">支持:查看 / 编辑保存(自动备份)、敏感函数高亮、扫描与搜索结果跳转</p>
              ) : (
                <table className="w-full border-collapse font-mono text-xs leading-5">
                  <tbody>
                    {lines.map((ln, i) => (
                      <tr key={i} data-line={i + 1} className={hitLines.has(i + 1) ? 'bg-red-50/60 dark:bg-red-500/10' : ''}>
                        <td className="w-12 select-none border-r border-slate-200 px-2 text-right align-top text-slate-400 dark:border-slate-800">{i + 1}</td>
                        <td className="whitespace-pre-wrap break-all px-2 align-top text-slate-700 dark:text-slate-200">
                          {highlightLine(ln, compiled).map((seg, j) => seg.rule ? (
                            <mark key={j} title={`${seg.rule.category}(${seg.rule.risk}):${seg.rule.pattern}`}
                              className={`rounded px-0.5 ${riskCls[seg.rule.risk] ?? riskCls.low}`}>{seg.text}</mark>
                          ) : <span key={j}>{seg.text}</span>)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}
        </div>

        {/* 右面板 */}
        <div className="flex w-96 shrink-0 flex-col border-l border-slate-200 dark:border-slate-800">
          <div className="flex border-b border-slate-200 dark:border-slate-800">
            {([['scan', '敏感扫描'], ['search', '搜索'], ['rules', '规则库']] as const).map(([k, label]) => (
              <button key={k} onClick={() => setTab(k)}
                className={`flex-1 px-2 py-1.5 text-xs ${tab === k ? 'border-b-2 border-indigo-500 font-medium text-indigo-600 dark:text-indigo-400' : 'text-slate-500 dark:text-slate-400'}`}>
                {label}
              </button>
            ))}
          </div>

          {tab === 'scan' && (
            <div className="flex min-h-0 flex-1 flex-col">
              <div className="flex items-center gap-2 p-2">
                <button onClick={doScan} disabled={!pid || scanning}
                  className="rounded bg-indigo-600 px-3 py-1 text-xs text-white hover:bg-indigo-500 disabled:opacity-40">
                  {scanning ? '扫描中…' : '扫描整个项目'}
                </button>
                {scanHits !== null && <span className="text-xs text-slate-500 dark:text-slate-400">命中 {scanHits.length} 处</span>}
              </div>
              <div className="min-h-0 flex-1 overflow-auto p-2 text-xs">
                {(scanHits ?? []).map((h, i) => (
                  <div key={i} className="mb-1.5 cursor-pointer rounded border border-slate-200 p-2 hover:border-indigo-300 dark:border-slate-700"
                    onClick={() => openFile(h.path, h.line_no)}>
                    <div className="flex items-center gap-1.5">
                      <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${riskCls[h.risk] ?? riskCls.low}`}>{h.risk}</span>
                      <span className="text-slate-500 dark:text-slate-400">{h.category}</span>
                      <span className="ml-auto font-mono text-[10px] text-slate-400">{h.language}</span>
                    </div>
                    <div className="mt-1 truncate font-mono text-[11px] text-indigo-600 dark:text-indigo-400" title={h.path}>{h.path}:{h.line_no}</div>
                    <pre className="mt-1 overflow-hidden whitespace-pre-wrap break-all font-mono text-[11px] text-slate-600 dark:text-slate-300">{h.line.trim()}</pre>
                  </div>
                ))}
                {scanHits !== null && scanHits.length === 0 && <p className="p-2 text-slate-400">未命中任何敏感函数</p>}
                {scanHits === null && <p className="p-2 text-slate-400">点击「扫描整个项目」,{rules.filter((r) => r.enabled).length} 条启用规则</p>}
              </div>
            </div>
          )}

          {tab === 'search' && (
            <div className="flex min-h-0 flex-1 flex-col">
              <div className="flex flex-wrap items-center gap-2 p-2">
                <input value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && doSearch()}
                  placeholder="搜索全部文件…"
                  className="min-w-0 flex-1 rounded border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-900" />
                <button onClick={doSearch} disabled={!pid} className="rounded bg-indigo-600 px-3 py-1 text-xs text-white hover:bg-indigo-500 disabled:opacity-40">搜索</button>
                <label className="flex items-center gap-1 text-xs text-slate-500"><input type="checkbox" checked={useRegex} onChange={(e) => setUseRegex(e.target.checked)} />正则</label>
                <label className="flex items-center gap-1 text-xs text-slate-500"><input type="checkbox" checked={caseSensitive} onChange={(e) => setCaseSensitive(e.target.checked)} />区分大小写</label>
              </div>
              <div className="min-h-0 flex-1 overflow-auto p-2 text-xs">
                {(searchHits ?? []).map((h, i) => (
                  <div key={i} className="mb-1 cursor-pointer rounded border border-slate-200 p-2 hover:border-indigo-300 dark:border-slate-700"
                    onClick={() => openFile(h.path, h.line_no)}>
                    <div className="truncate font-mono text-[11px] text-indigo-600 dark:text-indigo-400">{h.path}:{h.line_no}</div>
                    <pre className="mt-0.5 overflow-hidden whitespace-pre-wrap break-all font-mono text-[11px] text-slate-600 dark:text-slate-300">{h.line.trim()}</pre>
                  </div>
                ))}
                {searchHits !== null && searchHits.length === 0 && <p className="p-2 text-slate-400">无匹配</p>}
              </div>
            </div>
          )}

          {tab === 'rules' && (
            <div className="flex min-h-0 flex-1 flex-col">
              <form className="flex flex-wrap items-center gap-1 border-b border-slate-200 p-2 text-xs dark:border-slate-800"
                onSubmit={(e) => { e.preventDefault(); addRule(new FormData(e.currentTarget)); }}>
                <input name="language" placeholder="语言 php/java…" className="w-20 rounded border border-slate-200 px-1.5 py-1 dark:border-slate-700 dark:bg-slate-900" required />
                <input name="pattern" placeholder="正则 \beval\s*\(" className="min-w-0 flex-1 rounded border border-slate-200 px-1.5 py-1 font-mono dark:border-slate-700 dark:bg-slate-900" required />
                <input name="category" placeholder="分类" className="w-20 rounded border border-slate-200 px-1.5 py-1 dark:border-slate-700 dark:bg-slate-900" />
                <select name="risk" className="rounded border border-slate-200 px-1 py-1 dark:border-slate-700 dark:bg-slate-900">
                  <option value="high">high</option><option value="medium">medium</option><option value="low">low</option>
                </select>
                <input name="note" placeholder="说明" className="w-24 rounded border border-slate-200 px-1.5 py-1 dark:border-slate-700 dark:bg-slate-900" />
                <button type="submit" className="rounded bg-indigo-600 px-2 py-1 text-white hover:bg-indigo-500">添加</button>
              </form>
              <div className="min-h-0 flex-1 overflow-auto p-2 text-xs">
                {rules.map((r) => (
                  <div key={r.id} className="mb-1.5 rounded border border-slate-200 p-2 dark:border-slate-700">
                    <div className="flex items-center gap-1.5">
                      <input type="checkbox" checked={r.enabled} onChange={() => toggleRule(r)} title="启用/停用" />
                      <span className={`rounded px-1.5 py-0.5 text-[10px] ${riskCls[r.risk] ?? riskCls.low}`}>{r.risk}</span>
                      <span className="font-medium text-slate-600 dark:text-slate-300">{r.category}</span>
                      <span className="ml-auto text-[10px] text-slate-400">{r.language}</span>
                      <button onClick={() => removeRule(r.id)} className="text-slate-400 hover:text-red-500" title="删除">✕</button>
                    </div>
                    <code className="mt-1 block break-all font-mono text-[11px] text-indigo-600 dark:text-indigo-400">{r.pattern}</code>
                    {r.note && <p className="mt-0.5 text-[10px] text-slate-400">{r.note}</p>}
                  </div>
                ))}
                <p className="p-2 text-slate-400">共 {rules.length} 条(内置种子 {rules.filter((r) => r.id.length === 16).length} 含自建),规则即时生效于高亮与扫描</p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
