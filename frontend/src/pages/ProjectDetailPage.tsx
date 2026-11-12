import { useEffect, useMemo, useState } from 'react';
import { ArrowLeft, Pencil, Trash2 } from 'lucide-react';
import { useNavigate, useParams } from 'react-router-dom';

import EntryTable from '@/components/project/EntryTable';
import { getIcon } from '@/lib/icons';
import { ENTRY_KINDS, entryKindMap } from '@/mock/projectKinds';
import { ProjectDialog } from '@/pages/ProjectsPage';
import { useProjectStore } from '@/stores/useProjectStore';

const STATUS_LABEL: Record<string, string> = { active: '进行中', done: '已完成', paused: '暂停' };

/**
 * 项目详情页（/projects/:id）：项目信息 + 9 类条目标签页。
 * 概览页展示各类计数与最近更新的条目。
 */
export default function ProjectDetailPage() {
  const { id = '' } = useParams();
  const { current, entries, error, loadProject, loadEntries, updateProject, deleteProject } = useProjectStore();
  const [tab, setTab] = useState<string>('overview');
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    if (id) void loadProject(id);
  }, [id, loadProject]);

  useEffect(() => {
    if (id && tab !== 'overview') void loadEntries(id, tab);
  }, [id, tab, loadEntries]);

  // 概览页需要全部条目（取最近 5 条）
  useEffect(() => {
    if (id && tab === 'overview') void loadEntries(id, '');
  }, [id, tab, loadEntries]);

  useEffect(() => {
    if (current) document.title = `CTF-NET — ${current.name}`;
  }, [current]);

  const tabKindInfo = useMemo(() => entryKindMap.get(tab), [tab]);
  const tabEntries = useMemo(() => (id ? entries[`${id}:${tab}`] ?? [] : []), [entries, id, tab]);
  const recentEntries = useMemo(() => (id ? (entries[`${id}:`] ?? []).slice(0, 5) : []), [entries, id]);

  if (error && !current) {
    return (
      <div className="p-6">
        <BackButton />
        <p className="mt-8 text-center text-sm text-red-500">{error}</p>
      </div>
    );
  }
  if (!current) {
    return (
      <div className="p-6">
        <BackButton />
        <p className="mt-8 text-center text-sm text-slate-400">加载中…</p>
      </div>
    );
  }

  return (
    <div className="p-6">
      <BackButton />

      {/* 项目信息头 */}
      <div className="mt-4 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <span className="truncate">{current.name}</span>
            <span className="shrink-0 rounded-full bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400">
              {STATUS_LABEL[current.status] ?? current.status}
            </span>
          </h1>
          <div className="mt-1 space-y-0.5 text-sm text-slate-500 dark:text-slate-400">
            {current.target && <p>目标：{current.target}</p>}
            {current.scope && <p>授权范围：{current.scope}</p>}
            {current.description && <p className="whitespace-pre-wrap">{current.description}</p>}
          </div>
        </div>
        <div className="flex shrink-0 gap-1.5">
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="flex items-center gap-1 rounded-md border border-slate-200 px-2.5 py-1.5 text-sm text-slate-600 hover:border-indigo-300 hover:text-indigo-600 dark:border-slate-700 dark:text-slate-300"
          >
            <Pencil className="h-3.5 w-3.5" />
            编辑
          </button>
          <button
            type="button"
            onClick={() => setDeleting(true)}
            className="flex items-center gap-1 rounded-md border border-slate-200 px-2.5 py-1.5 text-sm text-slate-600 hover:border-red-300 hover:text-red-500 dark:border-slate-700 dark:text-slate-300"
          >
            <Trash2 className="h-3.5 w-3.5" />
            删除
          </button>
        </div>
      </div>

      {/* 标签页 */}
      <div className="mt-6 flex flex-wrap gap-1 border-b border-slate-200 dark:border-slate-800">
        <TabButton active={tab === 'overview'} onClick={() => setTab('overview')} icon="LayoutGrid" label="概览" />
        {ENTRY_KINDS.map((k) => (
          <TabButton
            key={k.kind}
            active={tab === k.kind}
            onClick={() => setTab(k.kind)}
            icon={k.icon}
            label={k.label}
            count={current.entry_counts?.[k.kind]}
          />
        ))}
      </div>

      <div className="mt-4">
        {tab === 'overview' ? (
          <Overview counts={current.entry_counts ?? {}} recent={recentEntries} onGo={setTab} />
        ) : (
          tabKindInfo && <EntryTable projectId={id} kindInfo={tabKindInfo} entries={tabEntries} />
        )}
      </div>

      {editing && (
        <ProjectDialog
          project={current}
          onClose={() => setEditing(false)}
          onSubmit={async (input) => {
            await updateProject(id, input);
            setEditing(false);
          }}
        />
      )}

      {deleting && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setDeleting(false)}>
          <div className="w-full max-w-sm rounded-lg bg-white p-5 shadow-xl dark:bg-slate-900" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-base font-semibold">确认删除项目</h3>
            <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
              删除「{current.name}」将同时删除其全部条目，且不可恢复。
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={() => setDeleting(false)}
                className="rounded-md px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800">
                取消
              </button>
              <button
                type="button"
                onClick={async () => {
                  await deleteProject(id);
                  navigate('/projects');
                }}
                className="rounded-md bg-red-500 px-3 py-1.5 text-sm text-white hover:bg-red-600"
              >
                删除项目
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function BackButton() {
  const navigate = useNavigate();
  return (
    <button
      type="button"
      onClick={() => navigate('/projects')}
      className="flex items-center gap-1 text-sm text-slate-500 hover:text-indigo-600 dark:text-slate-400"
    >
      <ArrowLeft className="h-4 w-4" />
      返回项目列表
    </button>
  );
}

function TabButton({
  active,
  onClick,
  icon,
  label,
  count,
}: {
  active: boolean;
  onClick: () => void;
  icon: string;
  label: string;
  count?: number;
}) {
  const Icon = getIcon(icon);
  return (
    <button
      type="button"
      onClick={onClick}
      className={`-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm ${
        active
          ? 'border-indigo-500 font-medium text-indigo-600 dark:text-indigo-400'
          : 'border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'
      }`}
    >
      <Icon className="h-4 w-4" />
      {label}
      {typeof count === 'number' && count > 0 && (
        <span className="rounded-full bg-slate-100 px-1.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">
          {count}
        </span>
      )}
    </button>
  );
}

function Overview({
  counts,
  recent,
  onGo,
}: {
  counts: Record<string, number>;
  recent: import('@/lib/projects').Entry[];
  onGo: (tab: string) => void;
}) {
  return (
    <div>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {ENTRY_KINDS.map((k) => {
          const Icon = getIcon(k.icon);
          const n = counts[k.kind] ?? 0;
          return (
            <button
              key={k.kind}
              type="button"
              onClick={() => onGo(k.kind)}
              className="rounded-lg border border-slate-200 bg-white p-4 text-left transition-shadow hover:shadow-md dark:border-slate-700 dark:bg-slate-900"
            >
              <div className="flex items-center gap-2 text-slate-500 dark:text-slate-400">
                <Icon className="h-4 w-4" />
                <span className="text-xs">{k.label}</span>
              </div>
              <p className="mt-2 text-2xl font-semibold">{n}</p>
            </button>
          );
        })}
      </div>

      <h3 className="mt-8 text-sm font-medium text-slate-500 dark:text-slate-400">最近更新</h3>
      {recent.length === 0 ? (
        <p className="mt-3 text-sm text-slate-400">暂无条目，点击上方卡片开始录入</p>
      ) : (
        <ul className="mt-3 divide-y divide-slate-100 rounded-lg border border-slate-200 dark:divide-slate-800 dark:border-slate-700">
          {recent.map((e) => {
            const info = entryKindMap.get(e.kind);
            return (
              <li key={e.id}>
                <button
                  type="button"
                  onClick={() => onGo(e.kind)}
                  className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-800/40"
                >
                  <span className="shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                    {info?.label ?? e.kind}
                  </span>
                  <span className="min-w-0 flex-1 truncate">{e.title || '（无标题）'}</span>
                  <span className="shrink-0 text-xs text-slate-400">
                    {new Date(e.updated_at * 1000).toLocaleString('zh-CN', { hour12: false })}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
