/**
 * 渗透测试项目管理 —— 数据层。
 *
 * 桌面运行时走 wailsjs 生成的 App 绑定（projects.go）；
 * 纯浏览器 `npm run dev`（isWails() 为 false）回退 localStorage 实现（同一接口），
 * 保证项目管理 UI 可脱离桌面端开发调试。
 */
import * as rawApp from '../../wailsjs/go/main/App';
import { isWails } from './backend';

// —— 类型（与 internal/projects/store.go JSON 键一致）——

export interface Project {
  id: string;
  name: string;
  target: string;
  scope: string;
  status: string; // active / done / paused
  description: string;
  created_at: number;
  updated_at: number;
  entry_counts?: Record<string, number>;
}

export interface ProjectInput {
  name: string;
  target: string;
  scope: string;
  status: string;
  description: string;
}

export interface Entry {
  id: string;
  project_id: string;
  kind: string;
  title: string;
  content: string; // JSON 文本，字段由 kind 决定（见 mock/projectKinds.ts）
  tags: string;
  created_at: number;
  updated_at: number;
}

// —— Wails 绑定（剥离运行时注入的 context 参数）——

const App = rawApp as unknown as {
  ListProjects(): Promise<Project[]>;
  GetProject(id: string): Promise<Project>;
  CreateProject(input: ProjectInput): Promise<Project>;
  UpdateProject(id: string, input: ProjectInput): Promise<Project>;
  DeleteProject(id: string): Promise<void>;
  ListEntries(projectID: string, kind: string): Promise<Entry[]>;
  CreateEntry(projectID: string, kind: string, title: string, content: string, tags: string): Promise<Entry>;
  UpdateEntry(id: string, title: string, content: string, tags: string): Promise<Entry>;
  DeleteEntry(id: string): Promise<void>;
  ExportFile(fileID: string, destPath: string): Promise<void>;
};

// —— localStorage 回退实现（纯浏览器开发模式）——

const LS_PROJECTS = 'ctfnet.projects';
const LS_ENTRIES = 'ctfnet.project_entries';

function lsRead<T>(key: string): T[] {
  try {
    return JSON.parse(localStorage.getItem(key) || '[]') as T[];
  } catch {
    return [];
  }
}

function lsWrite<T>(key: string, data: T[]): void {
  localStorage.setItem(key, JSON.stringify(data));
}

function lsId(): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(8)))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

function lsCounts(projectID: string): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const e of lsRead<Entry>(LS_ENTRIES)) {
    if (e.project_id === projectID) counts[e.kind] = (counts[e.kind] || 0) + 1;
  }
  return counts;
}

const local = {
  async listProjects(): Promise<Project[]> {
    return lsRead<Project>(LS_PROJECTS)
      .map((p) => ({ ...p, entry_counts: lsCounts(p.id) }))
      .sort((a, b) => b.updated_at - a.updated_at);
  },
  async getProject(id: string): Promise<Project> {
    const p = lsRead<Project>(LS_PROJECTS).find((x) => x.id === id);
    if (!p) throw new Error(`项目不存在: ${id}`);
    return { ...p, entry_counts: lsCounts(id) };
  },
  async createProject(input: ProjectInput): Promise<Project> {
    if (!input.name) throw new Error('项目名称不能为空');
    const now = Date.now() / 1000;
    const p: Project = { id: lsId(), ...input, status: input.status || 'active', created_at: now, updated_at: now };
    lsWrite(LS_PROJECTS, [...lsRead<Project>(LS_PROJECTS), p]);
    return { ...p, entry_counts: {} };
  },
  async updateProject(id: string, input: ProjectInput): Promise<Project> {
    const all = lsRead<Project>(LS_PROJECTS);
    const i = all.findIndex((p) => p.id === id);
    if (i < 0) throw new Error(`项目不存在: ${id}`);
    all[i] = { ...all[i], ...input, status: input.status || 'active', updated_at: Date.now() / 1000 };
    lsWrite(LS_PROJECTS, all);
    return { ...all[i], entry_counts: lsCounts(id) };
  },
  async deleteProject(id: string): Promise<void> {
    lsWrite(LS_PROJECTS, lsRead<Project>(LS_PROJECTS).filter((p) => p.id !== id));
    lsWrite(LS_ENTRIES, lsRead<Entry>(LS_ENTRIES).filter((e) => e.project_id !== id));
  },
  async listEntries(projectID: string, kind: string): Promise<Entry[]> {
    return lsRead<Entry>(LS_ENTRIES)
      .filter((e) => e.project_id === projectID && (!kind || e.kind === kind))
      .sort((a, b) => b.updated_at - a.updated_at);
  },
  touch(projectID: string): void {
    const all = lsRead<Project>(LS_PROJECTS);
    const i = all.findIndex((p) => p.id === projectID);
    if (i >= 0) {
      all[i].updated_at = Date.now() / 1000;
      lsWrite(LS_PROJECTS, all);
    }
  },
  async createEntry(projectID: string, kind: string, title: string, content: string, tags: string): Promise<Entry> {
    const now = Date.now() / 1000;
    const e: Entry = { id: lsId(), project_id: projectID, kind, title, content: content || '{}', tags, created_at: now, updated_at: now };
    lsWrite(LS_ENTRIES, [...lsRead<Entry>(LS_ENTRIES), e]);
    local.touch(projectID);
    return e;
  },
  async updateEntry(id: string, title: string, content: string, tags: string): Promise<Entry> {
    const all = lsRead<Entry>(LS_ENTRIES);
    const i = all.findIndex((e) => e.id === id);
    if (i < 0) throw new Error(`条目不存在: ${id}`);
    all[i] = { ...all[i], title, content: content || '{}', tags, updated_at: Date.now() / 1000 };
    lsWrite(LS_ENTRIES, all);
    local.touch(all[i].project_id);
    return all[i];
  },
  async deleteEntry(id: string): Promise<void> {
    const all = lsRead<Entry>(LS_ENTRIES);
    const e = all.find((x) => x.id === id);
    lsWrite(LS_ENTRIES, all.filter((x) => x.id !== id));
    if (e) local.touch(e.project_id);
  },
};

// —— 对外 API（按运行时自动选择实现）——

export const listProjects = (): Promise<Project[]> => (isWails() ? App.ListProjects() : local.listProjects());
export const getProject = (id: string): Promise<Project> => (isWails() ? App.GetProject(id) : local.getProject(id));
export const createProject = (input: ProjectInput): Promise<Project> =>
  isWails() ? App.CreateProject(input) : local.createProject(input);
export const updateProject = (id: string, input: ProjectInput): Promise<Project> =>
  isWails() ? App.UpdateProject(id, input) : local.updateProject(id, input);
export const deleteProject = (id: string): Promise<void> => (isWails() ? App.DeleteProject(id) : local.deleteProject(id));
export const listEntries = (projectID: string, kind: string): Promise<Entry[]> =>
  isWails() ? App.ListEntries(projectID, kind) : local.listEntries(projectID, kind);
export const createEntry = (projectID: string, kind: string, title: string, content: string, tags: string): Promise<Entry> =>
  isWails() ? App.CreateEntry(projectID, kind, title, content, tags) : local.createEntry(projectID, kind, title, content, tags);
export const updateEntry = (id: string, title: string, content: string, tags: string): Promise<Entry> =>
  isWails() ? App.UpdateEntry(id, title, content, tags) : local.updateEntry(id, title, content, tags);
export const deleteEntry = (id: string): Promise<void> => (isWails() ? App.DeleteEntry(id) : local.deleteEntry(id));

/** 导出附件到指定路径（仅桌面运行时可用；浏览器模式不支持）。 */
export const exportFile = (fileID: string, destPath: string): Promise<void> => {
  if (!isWails()) return Promise.reject(new Error('附件下载仅桌面端可用'));
  return App.ExportFile(fileID, destPath);
};

/** 解析条目 content JSON；损坏时返回空对象而非抛错。 */
export function parseContent(entry: Entry): Record<string, unknown> {
  try {
    return JSON.parse(entry.content || '{}') as Record<string, unknown>;
  } catch {
    return {};
  }
}
