import { useEffect, useState } from 'react';
import { Briefcase, Plus, X } from 'lucide-react';
import { Link, useNavigate } from 'react-router-dom';

import type { Project, ProjectInput } from '@/lib/projects';
import { kindLabel } from '@/mock/projectKinds';
import { useProjectStore } from '@/stores/useProjectStore';

const STATUS_LABEL: Record<string, string> = { active: '进行中', done: '已完成', paused: '暂停' };
const STATUS_STYLE: Record<string, string> = {
  active: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400',
  done: 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400',
  paused: 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400',
};

/**
 * 项目列表页（/projects）：项目卡片网格 + 新建对话框 + 状态筛选。
 * 仅渗透测试模式可达（入口按模式渲染，路由本身在 AppLayout 下）。
 */
export default function ProjectsPage() {
  const { projects, loadingProjects, error, loadProjects, createProject } = useProjectStore();
  const [creating, setCreating] = useState(false);
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const navigate = useNavigate();

  useEffect(() => {
    document.title = 'CTF-NET — 项目管理';
    void loadProjects();
  }, [loadProjects]);

  const filtered = statusFilter === 'all' ? projects : projects.filter((p) => p.status === statusFilter);

  return (
    <div className="p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <Briefcase className="h-6 w-6 text-indigo-500" />
            项目管理
          </h1>
          <p className="mt-1 text-slate-500 dark:text-slate-400">
            统一管理渗透测试过程中的资产、攻击面、PoC、端口、指纹、敏感信息等
          </p>
        </div>
        <button
          type="button"
          onClick={() => setCreating(true)}
          className="flex items-center gap-1.5 rounded-md bg-indigo-500 px-3 py-2 text-sm text-white hover:bg-indigo-600"
        >
          <Plus className="h-4 w-4" />
          新建项目
        </button>
      </div>

      {/* 状态筛选 */}
      <div className="mt-4 flex gap-1.5">
        {[
          { key: 'all', label: '全部' },
          { key: 'active', label: '进行中' },
          { key: 'done', label: '已完成' },
          { key: 'paused', label: '暂停' },
        ].map((s) => (
          <button
            key={s.key}
            type="button"
            onClick={() => setStatusFilter(s.key)}
            className={`rounded-full px-3 py-1 text-xs ${
              statusFilter === s.key
                ? 'bg-indigo-500 text-white'
                : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700'
            }`}
          >
            {s.label}
          </button>
        ))}
      </div>

      {error && <p className="mt-4 text-sm text-red-500">{error}</p>}

      {loadingProjects ? (
        <p className="mt-12 text-center text-sm text-slate-400">加载中…</p>
      ) : filtered.length === 0 ? (
        <div className="mt-12 rounded-lg border border-dashed border-slate-200 py-16 text-center dark:border-slate-700">
          <Briefcase className="mx-auto h-10 w-10 text-slate-300 dark:text-slate-600" />
          <p className="mt-3 text-sm text-slate-400">
            {statusFilter === 'all' ? '还没有项目，点击右上角「新建项目」开始' : '该状态下暂无项目'}
          </p>
        </div>
      ) : (
        <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((p) => (
            <ProjectCard key={p.id} project={p} />
          ))}
        </div>
      )}

      {creating && (
        <ProjectDialog
          onClose={() => setCreating(false)}
          onSubmit={async (input) => {
            const p = await createProject(input);
            setCreating(false);
            navigate(`/projects/${p.id}`);
          }}
        />
      )}
    </div>
  );
}

function ProjectCard({ project }: { project: Project }) {
  const total = Object.values(project.entry_counts ?? {}).reduce((a, b) => a + b, 0);
  const counts = Object.entries(project.entry_counts ?? {})
    .filter(([, n]) => n > 0)
    .slice(0, 4);
  return (
    <Link
      to={`/projects/${project.id}`}
      className="block rounded-lg border border-slate-200 bg-white p-4 transition-shadow hover:shadow-md dark:border-slate-700 dark:bg-slate-900"
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="truncate font-medium">{project.name}</h3>
        <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs ${STATUS_STYLE[project.status] ?? STATUS_STYLE.active}`}>
          {STATUS_LABEL[project.status] ?? project.status}
        </span>
      </div>
      {project.target && (
        <p className="mt-1 truncate text-sm text-slate-500 dark:text-slate-400">目标：{project.target}</p>
      )}
      <div className="mt-3 flex flex-wrap gap-1.5 text-xs text-slate-400">
        {total === 0 ? (
          <span>暂无条目</span>
        ) : (
          counts.map(([kind, n]) => (
            <span key={kind} className="rounded bg-slate-100 px-1.5 py-0.5 dark:bg-slate-800">
              {kindLabel(kind)} {n}
            </span>
          ))
        )}
      </div>
      <p className="mt-3 text-xs text-slate-400">
        更新于 {new Date(project.updated_at * 1000).toLocaleString('zh-CN', { hour12: false })}
      </p>
    </Link>
  );
}

/** 项目新建/编辑对话框（详情页也复用） */
export function ProjectDialog({
  project,
  onSubmit,
  onClose,
}: {
  project?: Project;
  onSubmit: (input: ProjectInput) => Promise<void>;
  onClose: () => void;
}) {
  const [form, setForm] = useState<ProjectInput>({
    name: project?.name ?? '',
    target: project?.target ?? '',
    scope: project?.scope ?? '',
    status: project?.status ?? 'active',
    description: project?.description ?? '',
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const set = (k: keyof ProjectInput) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div className="w-full max-w-lg rounded-lg bg-white p-5 shadow-xl dark:bg-slate-900" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold">{project ? '编辑项目' : '新建项目'}</h3>
          <button type="button" onClick={onClose} className="rounded p-1 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="mt-4 space-y-3">
          <label className="block text-sm">
            <span className="mb-1 block text-slate-500 dark:text-slate-400">项目名称 *</span>
            <input value={form.name} onChange={set('name')} placeholder="如：某集团 2026 年度渗透测试"
              className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700" />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-slate-500 dark:text-slate-400">目标</span>
            <input value={form.target} onChange={set('target')} placeholder="example.com / 10.0.0.0/24 / 单位名称"
              className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700" />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-slate-500 dark:text-slate-400">授权范围</span>
            <input value={form.scope} onChange={set('scope')} placeholder="授权测试的资产范围说明"
              className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700" />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-slate-500 dark:text-slate-400">状态</span>
            <select value={form.status} onChange={set('status')}
              className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900">
              <option value="active">进行中</option>
              <option value="done">已完成</option>
              <option value="paused">暂停</option>
            </select>
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-slate-500 dark:text-slate-400">项目描述</span>
            <textarea value={form.description} onChange={set('description')} rows={3}
              className="w-full rounded-md border border-slate-200 bg-transparent px-3 py-1.5 outline-none focus:border-indigo-400 dark:border-slate-700" />
          </label>
        </div>

        {error && <p className="mt-3 text-sm text-red-500">{error}</p>}

        <div className="mt-5 flex justify-end gap-2">
          <button type="button" onClick={onClose}
            className="rounded-md px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800">
            取消
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={async () => {
              if (!form.name.trim()) {
                setError('项目名称不能为空');
                return;
              }
              setBusy(true);
              setError(null);
              try {
                await onSubmit({ ...form, name: form.name.trim() });
              } catch (e) {
                setError(e instanceof Error ? e.message : String(e));
                setBusy(false);
              }
            }}
            className="rounded-md bg-indigo-500 px-3 py-1.5 text-sm text-white hover:bg-indigo-600 disabled:opacity-50"
          >
            {busy ? '保存中…' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}
