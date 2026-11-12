import { create } from 'zustand';

import type { Entry, Project, ProjectInput } from '@/lib/projects';
import * as api from '@/lib/projects';

/**
 * 渗透测试项目管理 store。
 * entries 按「projectId:kind」缓存，避免切换标签页重复拉取。
 */
interface ProjectState {
  projects: Project[];
  loadingProjects: boolean;
  /** 当前详情页项目 */
  current: Project | null;
  /** 条目缓存：key = `${projectId}:${kind}` */
  entries: Record<string, Entry[]>;
  error: string | null;

  loadProjects: () => Promise<void>;
  loadProject: (id: string) => Promise<void>;
  createProject: (input: ProjectInput) => Promise<Project>;
  updateProject: (id: string, input: ProjectInput) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
  loadEntries: (projectID: string, kind: string) => Promise<void>;
  createEntry: (projectID: string, kind: string, title: string, content: string, tags: string) => Promise<void>;
  updateEntry: (id: string, title: string, content: string, tags: string) => Promise<void>;
  deleteEntry: (id: string) => Promise<void>;
}

function errMsg(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  projects: [],
  loadingProjects: false,
  current: null,
  entries: {},
  error: null,

  loadProjects: async () => {
    set({ loadingProjects: true, error: null });
    try {
      set({ projects: await api.listProjects(), loadingProjects: false });
    } catch (e) {
      set({ error: errMsg(e), loadingProjects: false });
    }
  },

  loadProject: async (id) => {
    try {
      set({ current: await api.getProject(id), error: null });
    } catch (e) {
      set({ error: errMsg(e), current: null });
    }
  },

  createProject: async (input) => {
    const p = await api.createProject(input);
    await get().loadProjects();
    return p;
  },

  updateProject: async (id, input) => {
    await api.updateProject(id, input);
    await Promise.all([get().loadProject(id), get().loadProjects()]);
  },

  deleteProject: async (id) => {
    await api.deleteProject(id);
    if (get().current?.id === id) set({ current: null });
    await get().loadProjects();
  },

  loadEntries: async (projectID, kind) => {
    const list = await api.listEntries(projectID, kind);
    set((s) => ({ entries: { ...s.entries, [`${projectID}:${kind}`]: list } }));
  },

  createEntry: async (projectID, kind, title, content, tags) => {
    await api.createEntry(projectID, kind, title, content, tags);
    await Promise.all([get().loadEntries(projectID, kind), get().loadProject(projectID)]);
  },

  updateEntry: async (id, title, content, tags) => {
    const updated = await api.updateEntry(id, title, content, tags);
    await Promise.all([
      get().loadEntries(updated.project_id, updated.kind),
      get().loadProject(updated.project_id),
    ]);
  },

  deleteEntry: async (id) => {
    // 先从缓存定位归属，再删再刷新
    const cached = Object.values(get().entries).flat().find((e) => e.id === id);
    await api.deleteEntry(id);
    if (cached) {
      await Promise.all([
        get().loadEntries(cached.project_id, cached.kind),
        get().loadProject(cached.project_id),
      ]);
    }
  },
}));
