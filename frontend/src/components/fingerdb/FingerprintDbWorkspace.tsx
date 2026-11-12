import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import {
  AlertTriangle,
  Database,
  Download,
  Plus,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Trash2,
  Upload,
  X,
} from 'lucide-react';

import { isWails } from '@/lib/backend';
import {
  emptyFPCmsDetail,
  fpDelete,
  fpExport,
  fpGet,
  fpImport,
  fpList,
  fpRestore,
  fpSave,
  fpStats,
  type FPCmsDetail,
  type FPCmsSummary,
  type FPDBStats,
  type FPHashRow,
  type FPPathRow,
  type FPStaticRow,
  type FPVulnRow,
} from '@/lib/fingerdb';

const PAGE_SIZE = 50;
const IMPORT_PATH_KEY = 'fingerdb.importPath';
const EXPORT_PATH_KEY = 'fingerdb.exportPath';

/** 输入框统一样式 */
const inputCls =
  'rounded-md border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 focus:border-indigo-400 focus:outline-none';

/** 主按钮样式 */
const primaryCls =
  'flex items-center gap-1 rounded-md bg-indigo-500 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-600 disabled:opacity-50';

/** 表单小字段包装 */
function Field({ label, children, grow = false }: { label: string; children: ReactNode; grow?: boolean }) {
  return (
    <label className={`flex min-w-0 flex-col gap-0.5 ${grow ? 'flex-1' : ''}`}>
      <span className="text-[10px] text-slate-400 dark:text-slate-500">{label}</span>
      {children}
    </label>
  );
}

/** confidence 输入：空 = 不设置（引擎默认 0.5） */
function ConfidenceInput({
  value,
  onChange,
}: {
  value: number | null | undefined;
  onChange: (v: number | undefined) => void;
}) {
  return (
    <input
      type="number"
      min={0}
      max={1}
      step={0.05}
      value={value ?? ''}
      placeholder="0.5"
      title="留空 = 使用默认 0.5"
      onChange={(e) => {
        const s = e.target.value;
        if (s === '') {
          onChange(undefined);
          return;
        }
        const n = Number(s);
        if (Number.isFinite(n)) onChange(n);
      }}
      className={`${inputCls} w-16`}
    />
  );
}

/** 静态字符串指纹行 */
function StaticRowEditor({
  row,
  onChange,
  onRemove,
}: {
  row: FPStaticRow;
  onChange: (patch: Partial<FPStaticRow>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="flex flex-wrap items-end gap-2 border-b border-slate-100 py-2 last:border-0 dark:border-slate-800">
      <Field label="位置" >
        <select value={row.location} onChange={(e) => onChange({ location: e.target.value })} className={`${inputCls} w-28`}>
          {['html', 'header', 'cookie', 'url', 'path', 'js', 'css', 'meta', 'body', 'banner', 'cert'].map((l) => (
            <option key={l} value={l}>{l}</option>
          ))}
        </select>
      </Field>
      <Field label="特征字符串 / 正则" grow>
        <input value={row.pattern} onChange={(e) => onChange({ pattern: e.target.value })} className={inputCls} placeholder="如 generator content=WordPress" />
      </Field>
      <label className="flex items-center gap-1 pb-1 text-xs text-slate-500 dark:text-slate-400" title="Go 版引擎会真正编译执行正则（(?i)）；不勾选为普通子串匹配">
        <input type="checkbox" checked={row.is_regex} onChange={(e) => onChange({ is_regex: e.target.checked })} />
        正则
      </label>
      <Field label="置信度">
        <ConfidenceInput value={row.confidence} onChange={(v) => onChange({ confidence: v })} />
      </Field>
      <Field label="描述" grow>
        <input value={row.description} onChange={(e) => onChange({ description: e.target.value })} className={inputCls} />
      </Field>
      <button type="button" onClick={onRemove} title="删除该指纹" className="pb-1 text-red-500 hover:text-red-600">
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

/** 文件 Hash 指纹行 */
function HashRowEditor({
  row,
  onChange,
  onRemove,
}: {
  row: FPHashRow;
  onChange: (patch: Partial<FPHashRow>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="flex flex-wrap items-end gap-2 border-b border-slate-100 py-2 last:border-0 dark:border-slate-800">
      <Field label="文件路径" grow>
        <input value={row.path} onChange={(e) => onChange({ path: e.target.value })} className={inputCls} placeholder="/favicon.ico" />
      </Field>
      <Field label="算法">
        <select value={row.algorithm || 'murmurhash3'} onChange={(e) => onChange({ algorithm: e.target.value })} className={`${inputCls} w-32`}>
          <option value="murmurhash3">murmurhash3 (hex)</option>
          <option value="fofa">fofa (有符号十进制)</option>
        </select>
      </Field>
      <Field label="Hash 值">
        <input value={row.hash} onChange={(e) => onChange({ hash: e.target.value })} className={`${inputCls} w-28`} placeholder="a4c4d4bd" />
      </Field>
      <Field label="置信度">
        <ConfidenceInput value={row.confidence} onChange={(v) => onChange({ confidence: v })} />
      </Field>
      <Field label="描述" grow>
        <input value={row.description} onChange={(e) => onChange({ description: e.target.value })} className={inputCls} />
      </Field>
      <button type="button" onClick={onRemove} title="删除该指纹" className="pb-1 text-red-500 hover:text-red-600">
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

/** 路径探测指纹行 */
function PathRowEditor({
  row,
  onChange,
  onRemove,
}: {
  row: FPPathRow;
  onChange: (patch: Partial<FPPathRow>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="flex flex-wrap items-end gap-2 border-b border-slate-100 py-2 last:border-0 dark:border-slate-800">
      <Field label="路径" grow>
        <input value={row.path} onChange={(e) => onChange({ path: e.target.value })} className={inputCls} placeholder="/wp-json/" />
      </Field>
      <Field label="方法">
        <select value={row.method || 'GET'} onChange={(e) => onChange({ method: e.target.value })} className={`${inputCls} w-20`}>
          {['GET', 'POST', 'HEAD', 'PUT', 'DELETE'].map((m) => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>
      </Field>
      <Field label="状态码（逗号分隔）">
        <input value={row.status_code_text} onChange={(e) => onChange({ status_code_text: e.target.value })} className={`${inputCls} w-24`} placeholder="200,403" title="留空 = 默认 [200,403]；无正则时仅认 403 命中" />
      </Field>
      <Field label="返回内容正则（可选）">
        <input value={row.response_pattern} onChange={(e) => onChange({ response_pattern: e.target.value })} className={`${inputCls} w-40`} placeholder="留空 = 仅状态码探测" title="填写后为严格探测（校验返回体）；留空走状态码探测（仅认 403）" />
      </Field>
      <Field label="置信度">
        <ConfidenceInput value={row.confidence} onChange={(v) => onChange({ confidence: v })} />
      </Field>
      <Field label="描述" grow>
        <input value={row.description} onChange={(e) => onChange({ description: e.target.value })} className={inputCls} />
      </Field>
      <button type="button" onClick={onRemove} title="删除该指纹" className="pb-1 text-red-500 hover:text-red-600">
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

/** 关联漏洞行 */
function VulnRowEditor({
  row,
  onChange,
  onRemove,
}: {
  row: FPVulnRow;
  onChange: (patch: Partial<FPVulnRow>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="space-y-2 border-b border-slate-100 py-2 last:border-0 dark:border-slate-800">
      <div className="flex flex-wrap items-end gap-2">
        <Field label="漏洞 ID">
          <input value={row.id} onChange={(e) => onChange({ id: e.target.value })} className={`${inputCls} w-36`} placeholder="CVE-2024-0001" />
        </Field>
        <Field label="名称" grow>
          <input value={row.name} onChange={(e) => onChange({ name: e.target.value })} className={inputCls} />
        </Field>
        <Field label="严重度">
          <select value={row.severity} onChange={(e) => onChange({ severity: e.target.value })} className={`${inputCls} w-24`}>
            {['', 'critical', 'high', 'medium', 'low', 'info'].map((s) => (
              <option key={s} value={s}>{s || '（未定）'}</option>
            ))}
          </select>
        </Field>
        <button type="button" onClick={onRemove} title="删除该漏洞" className="pb-1 text-red-500 hover:text-red-600">
          <X className="h-4 w-4" />
        </button>
      </div>
      <div className="flex flex-wrap items-end gap-2">
        <Field label="影响版本" grow>
          <input value={row.affected_versions} onChange={(e) => onChange({ affected_versions: e.target.value })} className={inputCls} />
        </Field>
        <Field label="利用路径">
          <input value={row.exploit_path} onChange={(e) => onChange({ exploit_path: e.target.value })} className={`${inputCls} w-40`} />
        </Field>
      </div>
      <Field label="参考链接（每行一条）" grow>
        <textarea
          rows={2}
          value={(row.references ?? []).join('\n')}
          onChange={(e) => onChange({ references: e.target.value.split('\n').map((s) => s.trim()).filter(Boolean) })}
          className={inputCls}
          placeholder="https://…"
        />
      </Field>
    </div>
  );
}

/** 分区卡片：标题 + 计数 + 新增按钮 */
function SectionCard({
  title,
  hint,
  onAdd,
  children,
}: {
  title: string;
  hint?: string;
  onAdd: () => void;
  children: ReactNode;
}) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex items-center justify-between border-b border-slate-100 px-3 py-2 dark:border-slate-800">
        <div className="flex items-baseline gap-2">
          <span className="text-xs font-medium text-slate-700 dark:text-slate-200">{title}</span>
          {hint && <span className="text-[10px] text-slate-400 dark:text-slate-500">{hint}</span>}
        </div>
        <button type="button" onClick={onAdd} className="flex items-center gap-1 rounded-md border border-slate-200 px-2 py-0.5 text-xs text-indigo-500 hover:bg-indigo-50 dark:border-slate-700 dark:hover:bg-slate-800">
          <Plus className="h-3 w-3" /> 新增
        </button>
      </div>
      <div className="px-3 py-1">{children}</div>
    </div>
  );
}

type Msg = { kind: 'ok' | 'err'; text: string } | null;

/**
 * 指纹库管理工作区：内置库的管理 GUI。
 *
 * - 左侧：搜索 + 分页列表（cms_id / 名称 / 分类 / 各类指纹计数）；
 * - 右侧：选中条目的完整编辑（静态字符串 / 文件 Hash / 路径 / 漏洞 四组增删改）；
 * - 顶部：库状态（路径/版本/计数/备份数）+ 新增 / 导入合并 / 导出 / 恢复内置。
 * 删除与恢复为两步确认；删除/恢复/导入前 Go 端自动备份到 backups/（保留最近 10 份）。
 */
export default function FingerprintDbWorkspace() {
  const [stats, setStats] = useState<FPDBStats | null>(null);
  const [listRes, setListRes] = useState<{ total: number; items: FPCmsSummary[] } | null>(null);
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(0);
  const [loadingList, setLoadingList] = useState(false);

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<FPCmsDetail | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [dirty, setDirty] = useState(false);

  const [saving, setSaving] = useState(false);
  const [delArmed, setDelArmed] = useState(false);
  const [restoreArmed, setRestoreArmed] = useState(false);
  const [showIo, setShowIo] = useState(false);
  const [ioBusy, setIoBusy] = useState(false);
  const [importPath, setImportPath] = useState(() => localStorage.getItem(IMPORT_PATH_KEY) ?? '');
  const [exportPath, setExportPath] = useState(() => localStorage.getItem(EXPORT_PATH_KEY) ?? '');
  const [msg, setMsg] = useState<Msg>(null);
  const searchTimer = useRef<number | undefined>(undefined);

  const flashErr = (e: unknown) => setMsg({ kind: 'err', text: e instanceof Error ? e.message : String(e) });

  const loadList = useCallback(async (p: number, q: string) => {
    setLoadingList(true);
    try {
      const r = await fpList(q, p * PAGE_SIZE, PAGE_SIZE);
      setListRes(r);
      setPage(p);
    } catch (e) {
      flashErr(e);
    } finally {
      setLoadingList(false);
    }
  }, []);

  const loadStats = useCallback(async () => {
    try {
      setStats(await fpStats());
    } catch (e) {
      flashErr(e);
    }
  }, []);

  useEffect(() => {
    void loadStats();
    void loadList(0, '');
  }, [loadStats, loadList]);

  // 搜索防抖
  useEffect(() => {
    if (searchTimer.current) window.clearTimeout(searchTimer.current);
    searchTimer.current = window.setTimeout(() => void loadList(0, query), 300);
    return () => {
      if (searchTimer.current) window.clearTimeout(searchTimer.current);
    };
  }, [query, loadList]);

  // 选中条目 → 拉明细（丢弃未保存修改需确认）
  const openCms = useCallback(
    async (id: string) => {
      if (dirty && !window.confirm('当前条目有未保存的修改，放弃并切换？')) return;
      setSelectedId(id);
      setDirty(false);
      setDetail(null);
      setMsg(null);
      setLoadingDetail(true);
      try {
        const d = await fpGet(id);
        setDetail(d);
      } catch (e) {
        flashErr(e);
      } finally {
        setLoadingDetail(false);
      }
    },
    [dirty],
  );

  const startNew = () => {
    if (dirty && !window.confirm('当前条目有未保存的修改，放弃并新建？')) return;
    setSelectedId(null);
    setDetail(emptyFPCmsDetail(''));
    setDirty(true);
    setMsg(null);
  };

  // —— 明细编辑 ——
  const patchDetail = (patch: Partial<FPCmsDetail>) => {
    setDetail((d) => (d ? { ...d, ...patch } : d));
    setDirty(true);
  };
  const patchStatic = (i: number, patch: Partial<FPStaticRow>) => {
    setDetail((d) => d && { ...d, static_strings: d.static_strings.map((r, j) => (j === i ? { ...r, ...patch } : r)) });
    setDirty(true);
  };
  const patchHash = (i: number, patch: Partial<FPHashRow>) => {
    setDetail((d) => d && { ...d, file_hashes: d.file_hashes.map((r, j) => (j === i ? { ...r, ...patch } : r)) });
    setDirty(true);
  };
  const patchPath = (i: number, patch: Partial<FPPathRow>) => {
    setDetail((d) => d && { ...d, paths: d.paths.map((r, j) => (j === i ? { ...r, ...patch } : r)) });
    setDirty(true);
  };
  const patchVuln = (i: number, patch: Partial<FPVulnRow>) => {
    setDetail((d) => d && { ...d, vulnerabilities: d.vulnerabilities.map((r, j) => (j === i ? { ...r, ...patch } : r)) });
    setDirty(true);
  };

  const newStatic: FPStaticRow = { pattern: '', location: 'html', confidence: undefined, is_regex: false, description: '' };
  const newHash: FPHashRow = { path: '', hash: '', algorithm: 'murmurhash3', confidence: undefined, description: '' };
  const newPath: FPPathRow = { path: '/', method: 'GET', status_code_text: '200,403', confidence: undefined, response_pattern: '', description: '' };
  const newVuln: FPVulnRow = { id: '', name: '', severity: '', affected_versions: '', exploit_path: '', references: [] };

  // —— 保存 ——
  const handleSave = async () => {
    if (!detail) return;
    const cmsID = (selectedId ?? detail.cms_id).trim();
    if (!cmsID) {
      setMsg({ kind: 'err', text: '请填写 cms_id（仅允许字母/数字/_/-，不含 / \\，长度 ≤ 100）' });
      return;
    }
    if (!detail.name.trim()) {
      setMsg({ kind: 'err', text: '请填写名称（name）' });
      return;
    }
    setSaving(true);
    setMsg(null);
    try {
      await fpSave(cmsID, { ...detail, cms_id: cmsID });
      setDirty(false);
      setSelectedId(cmsID);
      setMsg({ kind: 'ok', text: `已保存 ${cmsID}` });
      await Promise.all([loadStats(), loadList(page, query)]);
      const d = await fpGet(cmsID); // 回读后端归一化结果（method 大写等）
      setDetail(d);
    } catch (e) {
      flashErr(e);
    } finally {
      setSaving(false);
    }
  };

  // —— 删除（两步确认）——
  useEffect(() => {
    if (!delArmed) return;
    const t = setTimeout(() => setDelArmed(false), 4000);
    return () => clearTimeout(t);
  }, [delArmed]);

  const handleDelete = async () => {
    if (!selectedId) return;
    if (!delArmed) {
      setDelArmed(true);
      return;
    }
    setDelArmed(false);
    setSaving(true);
    setMsg(null);
    try {
      await fpDelete(selectedId);
      setSelectedId(null);
      setDetail(null);
      setDirty(false);
      setMsg({ kind: 'ok', text: `已删除 ${selectedId}（删除前已自动备份）` });
      await Promise.all([loadStats(), loadList(page, query)]);
    } catch (e) {
      flashErr(e);
    } finally {
      setSaving(false);
    }
  };

  // —— 恢复内置（两步确认）——
  useEffect(() => {
    if (!restoreArmed) return;
    const t = setTimeout(() => setRestoreArmed(false), 4000);
    return () => clearTimeout(t);
  }, [restoreArmed]);

  const handleRestore = async () => {
    if (!restoreArmed) {
      setRestoreArmed(true);
      return;
    }
    setRestoreArmed(false);
    setIoBusy(true);
    setMsg(null);
    try {
      const s = await fpRestore();
      setStats(s);
      setSelectedId(null);
      setDetail(null);
      setDirty(false);
      setMsg({ kind: 'ok', text: `已恢复为内置库（${s.cms_count} 个 CMS），原库已备份` });
      await loadList(0, query);
    } catch (e) {
      flashErr(e);
    } finally {
      setIoBusy(false);
    }
  };

  const handleImport = async () => {
    if (!importPath.trim()) {
      setMsg({ kind: 'err', text: '请填写要导入的 cms_fingerprints.json 路径' });
      return;
    }
    setIoBusy(true);
    setMsg(null);
    try {
      localStorage.setItem(IMPORT_PATH_KEY, importPath);
      const r = await fpImport(importPath.trim());
      const skippedReasons = r.skipped_reasons ?? [];
      const reasons = skippedReasons.length ? `；跳过 ${r.skipped} 条：${skippedReasons.slice(0, 3).join('；')}` : '';
      setMsg({ kind: 'ok', text: `导入完成：新增 ${r.added}，覆盖 ${r.overwritten}，共 ${r.total}${reasons}。导入前已备份` });
      await Promise.all([loadStats(), loadList(0, query)]);
      setSelectedId(null);
      setDetail(null);
      setDirty(false);
    } catch (e) {
      flashErr(e);
    } finally {
      setIoBusy(false);
    }
  };

  const handleExport = async () => {
    if (!exportPath.trim()) {
      setMsg({ kind: 'err', text: '请填写导出 JSON 文件路径' });
      return;
    }
    setIoBusy(true);
    setMsg(null);
    try {
      localStorage.setItem(EXPORT_PATH_KEY, exportPath);
      await fpExport(exportPath.trim());
      setMsg({ kind: 'ok', text: `已导出到 ${exportPath.trim()}` });
    } catch (e) {
      flashErr(e);
    } finally {
      setIoBusy(false);
    }
  };

  if (!isWails()) {
    return (
      <div className="mt-6 flex flex-col items-center gap-2 rounded-lg border border-slate-200 p-10 text-slate-500 dark:border-slate-800">
        <AlertTriangle className="h-8 w-8" />
        <p>指纹库管理需要桌面运行时（Wails），请在打包后的应用内使用。</p>
      </div>
    );
  }

  const totalPages = listRes ? Math.max(1, Math.ceil(listRes.total / PAGE_SIZE)) : 1;

  return (
    <div className="mt-4 space-y-3">
      {/* —— 库状态 + 操作 —— */}
      <div className="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900/60">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-500 dark:text-slate-400">
          <span className="flex items-center gap-1.5 font-medium text-slate-700 dark:text-slate-200">
            <Database className="h-4 w-4 text-brand" />
            {stats ? `指纹库 v${stats.version}` : '指纹库'}
          </span>
          {stats && (
            <>
              <span>{stats.cms_count.toLocaleString()} CMS</span>
              <span>{stats.static_count.toLocaleString()} 静态 · {stats.hash_count.toLocaleString()} Hash · {stats.path_count} 路径 · {stats.vuln_count} 漏洞</span>
              <span>{(stats.file_size / 1048576).toFixed(1)} MB</span>
              <span title={stats.backup_dir}>备份 {stats.backup_count} 份</span>
              <span className="min-w-0 truncate" title={stats.path}>{stats.path}</span>
            </>
          )}
          <span className="ml-auto flex flex-wrap items-center gap-1.5">
            <button type="button" onClick={startNew} className={primaryCls}>
              <Plus className="h-3.5 w-3.5" /> 新增 CMS
            </button>
            <button type="button" onClick={() => setShowIo((v) => !v)} title="导入外部库合并 / 导出当前库" className="flex items-center gap-1 rounded-md border border-slate-200 px-2.5 py-1 text-xs text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800">
              <Upload className="h-3.5 w-3.5" /> 导入/导出
            </button>
            <button type="button" onClick={handleRestore} disabled={ioBusy} title="恢复为内置 13467 条指纹库（当前库自动备份）" className={`flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs ${restoreArmed ? 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/40 dark:text-red-400' : 'border-slate-200 text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800'}`}>
              <RotateCcw className="h-3.5 w-3.5" /> {restoreArmed ? '确认恢复内置库？' : '恢复内置'}
            </button>
            <button type="button" onClick={() => void Promise.all([loadStats(), loadList(0, query)])} title="刷新" className="flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800">
              <RefreshCw className="h-3.5 w-3.5" />
            </button>
          </span>
        </div>

        {showIo && (
          <div className="mt-2 flex flex-wrap items-center gap-2 border-t border-slate-100 pt-2 dark:border-slate-800">
            <input value={importPath} onChange={(e) => setImportPath(e.target.value)} placeholder="导入文件路径（外部 cms_fingerprints.json / model.json）" title="同名 cms_id 将被覆盖；无效条目跳过并提示；导入前自动备份" className={`${inputCls} min-w-0 flex-1`} />
            <button type="button" onClick={handleImport} disabled={ioBusy} className={primaryCls}>
              <Download className="h-3.5 w-3.5" /> 导入合并
            </button>
            <input value={exportPath} onChange={(e) => setExportPath(e.target.value)} placeholder="导出 JSON 路径" className={`${inputCls} min-w-0 flex-1`} />
            <button type="button" onClick={handleExport} disabled={ioBusy} className={primaryCls}>
              <Upload className="h-3.5 w-3.5" /> 导出
            </button>
          </div>
        )}

        {msg && (
          <div className={`mt-2 flex items-start gap-1.5 rounded-md px-2 py-1.5 text-xs ${msg.kind === 'ok' ? 'bg-green-50 text-green-700 dark:bg-green-950/40 dark:text-green-400' : 'bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-400'}`}>
            <span className="min-w-0 flex-1 whitespace-pre-wrap break-all">{msg.text}</span>
            <button type="button" onClick={() => setMsg(null)}><X className="h-3.5 w-3.5" /></button>
          </div>
        )}
      </div>

      {/* —— 列表 + 编辑 —— */}
      <div className="grid grid-cols-1 gap-3 xl:grid-cols-[340px_1fr]">
        {/* 左：搜索 + 列表 */}
        <div className="flex min-h-[420px] flex-col rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900/60">
          <div className="border-b border-slate-100 p-2 dark:border-slate-800">
            <div className="relative">
              <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜索 cms_id 或名称…" className={`${inputCls} w-full pl-7`} />
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">
            {!listRes && loadingList && <div className="p-4 text-center text-xs text-slate-400">加载中…</div>}
            {listRes && listRes.items.map((it) => (
              <button
                key={it.cms_id}
                type="button"
                onClick={() => void openCms(it.cms_id)}
                className={`flex w-full items-start gap-2 border-b border-slate-100 px-3 py-2 text-left dark:border-slate-800 ${selectedId === it.cms_id ? 'bg-indigo-50 dark:bg-indigo-950/30' : 'hover:bg-slate-50 dark:hover:bg-slate-800/60'}`}
              >
                <div className="min-w-0 flex-1">
                  <div className="truncate text-xs font-medium text-slate-700 dark:text-slate-200">{it.name || <span className="italic text-slate-400">（无名称）</span>}</div>
                  <div className="truncate font-mono text-[10px] text-slate-400">{it.cms_id || '（空 id）'}{it.category ? ` · ${it.category}` : ''}</div>
                </div>
                <span className="shrink-0 text-[10px] text-slate-400" title="静态/Hash/路径/漏洞 计数">
                  {it.static_count}/{it.hash_count}/{it.path_count}/{it.vuln_count}
                </span>
              </button>
            ))}
            {listRes && listRes.items.length === 0 && <div className="p-4 text-center text-xs text-slate-400">无匹配条目</div>}
          </div>
          <div className="flex items-center justify-between border-t border-slate-100 p-2 dark:border-slate-800">
            <button type="button" disabled={page <= 0} onClick={() => void loadList(page - 1, query)} className="rounded-md border border-slate-200 px-2 py-0.5 text-xs text-slate-600 hover:bg-slate-50 disabled:opacity-40 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800">
              上一页
            </button>
            <span className="text-[10px] text-slate-400">第 {page + 1}/{totalPages} 页 · 共 {listRes?.total.toLocaleString() ?? '…'} 条</span>
            <button type="button" disabled={page + 1 >= totalPages} onClick={() => void loadList(page + 1, query)} className="rounded-md border border-slate-200 px-2 py-0.5 text-xs text-slate-600 hover:bg-slate-50 disabled:opacity-40 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800">
              下一页
            </button>
          </div>
        </div>

        {/* 右：明细编辑 */}
        <div className="min-h-[420px] space-y-3">
          {!detail && !loadingDetail && (
            <div className="flex h-full min-h-[300px] flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-slate-300 p-10 text-slate-400 dark:border-slate-700">
              <Database className="h-8 w-8" />
              <p className="text-sm">从左侧选择一个 CMS 查看/编辑，或点击「新增 CMS」</p>
            </div>
          )}
          {loadingDetail && <div className="p-10 text-center text-xs text-slate-400">加载条目中…</div>}

          {detail && (
            <>
              {/* 基本信息 */}
              <div className="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900/60">
                <div className="flex flex-wrap items-end gap-2">
                  <Field label="cms_id（新增后可保存，编辑时不可改）">
                    <input value={selectedId ?? detail.cms_id} onChange={(e) => patchDetail({ cms_id: e.target.value })} disabled={!!selectedId} placeholder="如 my_custom_cms" className={`${inputCls} w-56 disabled:opacity-50`} />
                  </Field>
                  <Field label="名称 name" grow>
                    <input value={detail.name} onChange={(e) => patchDetail({ name: e.target.value })} className={inputCls} />
                  </Field>
                  <Field label="分类 category">
                    <input value={detail.category} onChange={(e) => patchDetail({ category: e.target.value })} className={`${inputCls} w-32`} placeholder="other" />
                  </Field>
                  <span className="ml-auto flex items-center gap-1.5 pb-0.5">
                    {selectedId && (
                      <button type="button" onClick={() => void handleDelete()} disabled={saving} title="删除条目（自动备份）" className={`flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs ${delArmed ? 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/40 dark:text-red-400' : 'border-red-200 text-red-500 hover:bg-red-50 dark:border-red-900/60 dark:hover:bg-red-950/40'}`}>
                        <Trash2 className="h-3.5 w-3.5" /> {delArmed ? '确认删除？' : '删除'}
                      </button>
                    )}
                    <button type="button" onClick={() => void handleSave()} disabled={saving} className={primaryCls}>
                      <Save className="h-3.5 w-3.5" /> {saving ? '保存中…' : '保存'}
                    </button>
                  </span>
                </div>
                {dirty && <div className="mt-2 text-[10px] text-amber-500">有未保存修改</div>}
              </div>

              <SectionCard title="静态字符串指纹" hint="权重 0.6；location 决定匹配桶；勾选「正则」由引擎真正编译执行" onAdd={() => patchDetail({ static_strings: [...detail.static_strings, newStatic] })}>
                {detail.static_strings.length === 0 && <div className="py-2 text-center text-[10px] text-slate-400">暂无静态字符串指纹</div>}
                {detail.static_strings.map((r, i) => (
                  <StaticRowEditor key={i} row={r} onChange={(p) => patchStatic(i, p)} onRemove={() => patchDetail({ static_strings: detail.static_strings.filter((_, j) => j !== i) })} />
                ))}
              </SectionCard>

              <SectionCard title="文件 Hash 指纹" hint="权重 0.9；murmurhash3 为 8 位 hex，fofa 为有符号十进制 icon_hash" onAdd={() => patchDetail({ file_hashes: [...detail.file_hashes, newHash] })}>
                {detail.file_hashes.length === 0 && <div className="py-2 text-center text-[10px] text-slate-400">暂无文件 Hash 指纹</div>}
                {detail.file_hashes.map((r, i) => (
                  <HashRowEditor key={i} row={r} onChange={(p) => patchHash(i, p)} onRemove={() => patchDetail({ file_hashes: detail.file_hashes.filter((_, j) => j !== i) })} />
                ))}
              </SectionCard>

              <SectionCard title="路径探测指纹" hint="权重 0.7；填返回内容正则为严格探测，留空为状态码探测（仅认 403）；需在识别工具中打开路径探测开关" onAdd={() => patchDetail({ paths: [...detail.paths, newPath] })}>
                {detail.paths.length === 0 && <div className="py-2 text-center text-[10px] text-slate-400">暂无路径指纹</div>}
                {detail.paths.map((r, i) => (
                  <PathRowEditor key={i} row={r} onChange={(p) => patchPath(i, p)} onRemove={() => patchDetail({ paths: detail.paths.filter((_, j) => j !== i) })} />
                ))}
              </SectionCard>

              <SectionCard title="关联漏洞" hint="识别命中后在报告中列出" onAdd={() => patchDetail({ vulnerabilities: [...detail.vulnerabilities, newVuln] })}>
                {detail.vulnerabilities.length === 0 && <div className="py-2 text-center text-[10px] text-slate-400">暂无关联漏洞</div>}
                {detail.vulnerabilities.map((r, i) => (
                  <VulnRowEditor key={i} row={r} onChange={(p) => patchVuln(i, p)} onRemove={() => patchDetail({ vulnerabilities: detail.vulnerabilities.filter((_, j) => j !== i) })} />
                ))}
              </SectionCard>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
