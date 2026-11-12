import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ChevronRight, FolderSync, Plus, Search, Trash2, X } from 'lucide-react';

import { isWails } from '@/lib/backend';
import {
  DEFAULT_VULNLIB_DIR,
  createVulnCase,
  deleteVulnCase,
  listVulnCases,
  listVulnCategories,
  scanVulnLibrary,
  searchVulnCases,
  vulnStats,
  type VulnCaseSummary,
  type VulnCategory,
  type VulnStats,
} from '@/lib/vulnlib';
import type { PocSelection } from '@/stores/usePocStore';
import { usePocStore } from '@/stores/usePocStore';

const DIR_KEY = 'vulnlib.dir';

/** 新增案例的默认模板。 */
const NEW_CASE_TEMPLATE = '# 标题\n\n## 漏洞描述\n\n## 影响范围\n\n## 复现步骤\n\n';

/** 客户端把文件名规范化（补 .md），与后端 safeFileName 保持一致。 */
function normName(name: string): string {
  let n = name.trim();
  if (n.toLowerCase().endsWith('.md')) n = n.slice(0, -3);
  return n.trim() + '.md';
}

/**
 * 漏洞清单面板：漏洞浏览器工具页的二级侧边栏内容。
 *
 * 数据源：桌面版走 Go 后端漏洞库（internal/vulnlib，SQLite），
 * 浏览器版回退静态快照 mock/vulnLibrary.ts（仅目录结构，无全文）。
 * 顶部「扫描导入」把漏洞清单目录（Z-分类 → S-系统 → ND-案例 md）导入漏洞库；
 * 支持新增案例（写入磁盘 md）、删除案例（文件移入备份）；树形浏览按需加载案例；
 * 搜索框走后端关键字查询（标题/文件名/CVE）。
 */
export default function VulnLibraryPanel() {
  const [cats, setCats] = useState<VulnCategory[] | null>(null);
  const [stats, setStats] = useState<VulnStats | null>(null);
  const [keyword, setKeyword] = useState('');
  const [searchResults, setSearchResults] = useState<VulnCaseSummary[] | null>(null);
  const [dir, setDir] = useState(() => localStorage.getItem(DIR_KEY) ?? DEFAULT_VULNLIB_DIR);
  const [scanning, setScanning] = useState(false);
  const [scanMsg, setScanMsg] = useState<string | null>(null);
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const selected = usePocStore((s) => s.selected);
  const select = usePocStore((s) => s.select);
  const clear = usePocStore((s) => s.clear);
  const refreshTick = usePocStore((s) => s.refreshTick);
  const bumpRefresh = usePocStore((s) => s.bumpRefresh);
  const autoScanDone = useRef(false);

  // —— 新增案例表单 ——
  const [showCreate, setShowCreate] = useState(false);
  const [newCat, setNewCat] = useState('');
  const [newSys, setNewSys] = useState('__new__');
  const [newSysCustom, setNewSysCustom] = useState('');
  const [newName, setNewName] = useState('');
  const [newContent, setNewContent] = useState(NEW_CASE_TEMPLATE);
  const [creating, setCreating] = useState(false);

  // —— 删除（两步确认）——
  const [delArmed, setDelArmed] = useState<string | null>(null);
  useEffect(() => {
    if (!delArmed) return;
    const t = setTimeout(() => setDelArmed(null), 4000);
    return () => clearTimeout(t);
  }, [delArmed]);

  const reload = useCallback(async () => {
    try {
      const [c, s] = await Promise.all([listVulnCategories(), vulnStats()]);
      setCats(c);
      setStats(s);
      setErrMsg(null);
    } catch (e) {
      setErrMsg(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload, refreshTick]);

  // 搜索：关键字非空时走后端（桌面）或静态过滤（浏览器）
  useEffect(() => {
    const kw = keyword.trim();
    if (!kw) {
      setSearchResults(null);
      return;
    }
    let alive = true;
    void searchVulnCases(kw, '').then(
      (r) => alive && setSearchResults(r),
      (e) => alive && setErrMsg(e instanceof Error ? e.message : String(e)),
    );
    return () => {
      alive = false;
    };
  }, [keyword, refreshTick]);

  const runScan = async (root: string) => {
    setScanning(true);
    setScanMsg(null);
    setErrMsg(null);
    try {
      const sum = await scanVulnLibrary(root);
      localStorage.setItem(DIR_KEY, root);
      setScanMsg(
        `导入完成：新增/更新 ${sum.updated}，未变 ${sum.unchanged}，跳过 ${sum.skipped}，删除 ${sum.deleted}，耗时 ${(sum.duration_ms / 1000).toFixed(1)}s`,
      );
      await reload();
    } catch (e) {
      setErrMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setScanning(false);
    }
  };

  const handleScan = async () => {
    const root = dir.trim();
    if (!root) {
      setErrMsg('请先填写漏洞清单根目录（如 C:\\Users\\HP\\Desktop\\安全方向\\漏洞复现案例）');
      return;
    }
    await runScan(root);
  };

  // 状态就绪后：本地无记录时采用后端已导入的根目录；桌面版首次打开（未导入）自动扫描默认目录
  useEffect(() => {
    if (!stats || autoScanDone.current) return;
    autoScanDone.current = true;
    if (!localStorage.getItem(DIR_KEY) && stats.root_dir) {
      setDir(stats.root_dir);
      localStorage.setItem(DIR_KEY, stats.root_dir);
    }
    const root = (localStorage.getItem(DIR_KEY) ?? stats.root_dir ?? dir).trim();
    if (isWails() && !stats.scanned && root) {
      void runScan(root);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stats]);

  const newCatObj = cats?.find((c) => c.id === newCat);
  const finalSys = newCatObj?.id === '__root__' ? '' : newSys === '__new__' ? newSysCustom.trim() : newSys;

  const handleCreate = async () => {
    if (!newCat) {
      setErrMsg('请选择漏洞分类');
      return;
    }
    if (newCatObj?.id !== '__root__' && !finalSys) {
      setErrMsg('请选择或填写系统/厂商目录');
      return;
    }
    if (!newName.trim()) {
      setErrMsg('请填写案例文件名');
      return;
    }
    setCreating(true);
    setErrMsg(null);
    try {
      await createVulnCase(newCat, finalSys, newName, newContent);
      const name = normName(newName);
      const newId =
        newCatObj?.id === '__root__' ? name : `${newCat}/${finalSys}/${name}`;
      select({
        categoryId: newCat,
        categoryName: newCatObj?.name ?? newCat,
        systemId: finalSys,
        systemName: finalSys,
        caseId: newId,
        caseName: newName.trim(),
        title: newName.trim(),
        cves: [],
      });
      bumpRefresh();
      setShowCreate(false);
      setNewName('');
      setNewContent(NEW_CASE_TEMPLATE);
      setNewSys('__new__');
      setNewSysCustom('');
    } catch (e) {
      setErrMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (c: VulnCaseSummary) => {
    if (delArmed !== c.id) {
      setDelArmed(c.id);
      return;
    }
    setDelArmed(null);
    setErrMsg(null);
    try {
      await deleteVulnCase(c.id);
      if (selected?.caseId === c.id) clear();
      bumpRefresh();
    } catch (e) {
      setErrMsg(e instanceof Error ? e.message : String(e));
    }
  };

  const filteredCats = useMemo(() => {
    if (!cats) return [];
    if (!keyword.trim()) return cats;
    const kw = keyword.trim().toLowerCase();
    return cats.filter((c) => c.name.toLowerCase().includes(kw) || c.code.toLowerCase().includes(kw));
  }, [cats, keyword]);

  return (
    <>
      {/* 导入工具栏 */}
      <div className="space-y-2 border-b border-slate-200 p-2 dark:border-slate-800">
        <div className="flex items-center gap-1.5">
          <input
            value={dir}
            onChange={(e) => setDir(e.target.value)}
            placeholder="漏洞清单根目录路径"
            title={dir}
            className="min-w-0 flex-1 rounded-md border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
          <button
            type="button"
            onClick={handleScan}
            disabled={scanning}
            title="扫描目录并导入漏洞库（自动跳过 My_template；增量扫描）"
            className="flex shrink-0 items-center gap-1 rounded-md bg-indigo-500 px-2 py-1 text-xs font-medium text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            <FolderSync className={`h-3.5 w-3.5 ${scanning ? 'animate-spin' : ''}`} />
            导入
          </button>
          <button
            type="button"
            onClick={() => setShowCreate((v) => !v)}
            title="新增漏洞案例（写入对应目录的 md 文件）"
            className="flex shrink-0 items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            <Plus className="h-3.5 w-3.5" />
            新增
          </button>
        </div>
        {stats?.scanned && (
          <div className="text-[11px] leading-4 text-slate-400">
            {stats.case_count} 个案例 / {stats.cat_count} 个分类
            {stats.last_scan > 0 &&
              ` · 上次导入 ${new Date(stats.last_scan * 1000).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}`}
          </div>
        )}
        {scanMsg && <div className="text-[11px] leading-4 text-emerald-600 dark:text-emerald-400">{scanMsg}</div>}
        {errMsg && <div className="text-[11px] leading-4 text-red-500">{errMsg}</div>}
        {!isWails() && (
          <div className="text-[11px] leading-4 text-amber-600 dark:text-amber-400">
            浏览器模式：仅目录结构预览，桌面版导入后可查看案例全文
          </div>
        )}
      </div>

      {/* 新增案例表单 */}
      {showCreate && (
        <div className="space-y-2 border-b border-slate-200 p-2 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-600 dark:text-slate-300">新增案例</span>
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="rounded p-0.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
          <select
            value={newCat}
            onChange={(e) => {
              setNewCat(e.target.value);
              setNewSys('__new__');
              setNewSysCustom('');
            }}
            className="w-full rounded-md border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          >
            <option value="">选择分类…</option>
            {cats?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.code ? `${c.code} ` : ''}
                {c.name}
              </option>
            ))}
          </select>
          {newCatObj?.id !== '__root__' && newCat && (
            <>
              <select
                value={newSys}
                onChange={(e) => setNewSys(e.target.value)}
                className="w-full rounded-md border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >
                <option value="__new__">新建系统/厂商目录…</option>
                {newCatObj?.systems.map((s) => (
                  <option key={s.name} value={s.name}>
                    {s.code ? `${s.code} ` : ''}
                    {s.name}
                  </option>
                ))}
              </select>
              {newSys === '__new__' && (
                <input
                  value={newSysCustom}
                  onChange={(e) => setNewSysCustom(e.target.value)}
                  placeholder="新系统目录名（如 S99 某产品）"
                  className="w-full rounded-md border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                />
              )}
            </>
          )}
          <input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="文件名（如 ND099 某漏洞，自动补 .md）"
            className="w-full rounded-md border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
          <textarea
            value={newContent}
            onChange={(e) => setNewContent(e.target.value)}
            rows={6}
            className="w-full resize-y rounded-md border border-slate-200 bg-white px-2 py-1 font-mono text-[11px] leading-4 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
          <div className="flex justify-end gap-1.5">
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="rounded-md px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
            >
              取消
            </button>
            <button
              type="button"
              onClick={handleCreate}
              disabled={creating}
              className="rounded-md bg-indigo-500 px-2 py-1 text-xs font-medium text-white hover:bg-indigo-600 disabled:opacity-50"
            >
              {creating ? '保存中…' : '保存'}
            </button>
          </div>
        </div>
      )}

      {/* 搜索框 */}
      <div className="border-b border-slate-200 p-2 dark:border-slate-800">
        <div className="relative">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索漏洞 / 系统 / 分类 / CVE…"
            className="w-full rounded-md border border-slate-200 bg-white py-1.5 pl-7 pr-2 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
          />
        </div>
      </div>

      <nav className="flex-1 overflow-y-auto p-2">
        {/* 搜索模式：扁平结果列表 */}
        {searchResults && (
          <>
            {searchResults.map((c) => (
              <CaseRow
                key={c.id}
                c={c}
                selectedCaseId={selected?.caseId}
                delArmed={delArmed === c.id}
                onSelect={select}
                onDelete={handleDelete}
              />
            ))}
            {searchResults.length === 0 && <div className="p-3 text-sm text-slate-400">无匹配结果</div>}
          </>
        )}

        {/* 树模式：分类 → 系统 → 案例（案例懒加载） */}
        {!searchResults && !cats && <div className="p-4 text-sm text-slate-400">漏洞库加载中…</div>}
        {!searchResults && cats && cats.length === 0 && !stats?.scanned && (
          <div className="p-4 text-sm text-slate-400">
            漏洞库为空。请在上方填写漏洞清单目录并点击「导入」。
          </div>
        )}
        {!searchResults &&
          filteredCats.map((cat) => (
            <CategoryNodeItem
              key={cat.id}
              category={cat}
              selectedCaseId={selected?.caseId}
              onSelect={select}
              onDelete={handleDelete}
              delArmed={delArmed}
            />
          ))}
      </nav>
    </>
  );
}

/** 案例行：点击选中；悬浮显示删除（两步确认）按钮。 */
function CaseRow({
  c,
  selectedCaseId,
  delArmed,
  onSelect,
  onDelete,
}: {
  c: VulnCaseSummary;
  selectedCaseId?: string;
  delArmed: boolean;
  onSelect: (sel: PocSelection) => void;
  onDelete: (c: VulnCaseSummary) => void;
}) {
  return (
    <div className="group relative">
      <button
        type="button"
        onClick={() =>
          onSelect({
            categoryId: c.cat_id,
            categoryName: c.cat_name,
            systemId: c.sys,
            systemName: c.sys,
            caseId: c.id,
            caseName: c.name,
            title: c.title || c.name,
            cves: c.cves,
          })
        }
        className={`block w-full truncate rounded-md px-2 py-1.5 pr-16 text-left text-sm transition-colors ${
          selectedCaseId === c.id
            ? 'bg-indigo-50 font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
            : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
        }`}
      >
        {c.title || c.name}
      </button>
      <button
        type="button"
        onClick={() => onDelete(c)}
        title={delArmed ? '再次点击确认删除（文件移入 data/backups）' : '删除案例'}
        className={`absolute right-1 top-1/2 -translate-y-1/2 rounded px-1.5 py-0.5 text-[11px] ${
          delArmed
            ? 'bg-red-500 font-medium text-white'
            : 'hidden text-slate-400 group-hover:inline hover:text-red-500'
        }`}
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

/** 大类节点：可展开，展示系统列表 */
function CategoryNodeItem({
  category,
  selectedCaseId,
  onSelect,
  onDelete,
  delArmed,
}: {
  category: VulnCategory;
  selectedCaseId?: string;
  onSelect: (sel: PocSelection) => void;
  onDelete: (c: VulnCaseSummary) => void;
  delArmed: string | null;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
      >
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-slate-400 transition-transform ${open ? 'rotate-90' : ''}`}
        />
        <span className="truncate">
          {category.code && <span className="mr-1 text-xs text-slate-400">{category.code}</span>}
          {category.name}
        </span>
        <span className="ml-auto shrink-0 text-xs text-slate-400">{category.case_count}</span>
      </button>
      {open && (
        <div className="ml-3 border-l border-slate-200 pl-2 dark:border-slate-800">
          {category.systems.map((sys) => (
            <SystemNodeItem
              key={category.id + '/' + sys.name}
              category={category}
              sys={sys}
              selectedCaseId={selectedCaseId}
              onSelect={onSelect}
              onDelete={onDelete}
              delArmed={delArmed}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/** 系统/厂商节点：可展开，案例懒加载 */
function SystemNodeItem({
  category,
  sys,
  selectedCaseId,
  onSelect,
  onDelete,
  delArmed,
}: {
  category: VulnCategory;
  sys: VulnCategory['systems'][number];
  selectedCaseId?: string;
  onSelect: (sel: PocSelection) => void;
  onDelete: (c: VulnCaseSummary) => void;
  delArmed: string | null;
}) {
  const [open, setOpen] = useState(false);
  const [cases, setCases] = useState<VulnCaseSummary[] | null>(null);
  const [loadErr, setLoadErr] = useState<string | null>(null);

  const toggle = async () => {
    const next = !open;
    setOpen(next);
    if (next && cases === null) {
      try {
        setCases(await listVulnCases(category.id, sys.name));
      } catch (e) {
        setLoadErr(e instanceof Error ? e.message : String(e));
      }
    }
  };

  return (
    <div>
      <button
        type="button"
        onClick={toggle}
        className="flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
      >
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-slate-400 transition-transform ${open ? 'rotate-90' : ''}`}
        />
        <span className="truncate">{sys.name}</span>
        <span className="ml-auto shrink-0 text-xs text-slate-400">{sys.case_count}</span>
      </button>
      {open && (
        <div className="ml-3 border-l border-slate-200 pl-2 dark:border-slate-800">
          {cases === null && !loadErr && <div className="p-1 text-xs text-slate-400">加载中…</div>}
          {loadErr && <div className="p-1 text-xs text-red-500">{loadErr}</div>}
          {cases?.map((c) => (
            <CaseRow
              key={c.id}
              c={c}
              selectedCaseId={selectedCaseId}
              delArmed={delArmed === c.id}
              onSelect={onSelect}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}
