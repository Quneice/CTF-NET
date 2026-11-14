/**
 * 代码审计工作区 API（Wails 绑定门面，projects.ts 同款模式）。
 * 纯浏览器开发模式无后端,直接抛错提示(工作区强依赖本地文件与 SQLite)。
 */
import { isWails, uploadFile } from './backend';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const rawApp = (globalThis as any).go?.main?.App ?? {};

const App = rawApp as {
  AuditImport(name: string, refs: { ID: string; Name: string }[]): Promise<AuditProject>;
  ListAuditProjects(): Promise<AuditProject[]>;
  DeleteAuditProject(id: string): Promise<void>;
  AuditFileTree(id: string): Promise<FileNode[]>;
  AuditReadFile(id: string, rel: string): Promise<string>;
  AuditWriteFile(id: string, rel: string, content: string): Promise<void>;
  AuditSearch(id: string, query: string, regex: boolean, caseSensitive: boolean, limit: number): Promise<SearchHit[]>;
  AuditScan(id: string): Promise<SensHit[]>;
  ListSensitiveFuncs(): Promise<SensitiveFunc[]>;
  AddSensitiveFunc(language: string, pattern: string, category: string, risk: string, note: string, enabled: boolean): Promise<SensitiveFunc>;
  UpdateSensitiveFunc(id: string, language: string, pattern: string, category: string, risk: string, note: string, enabled: boolean): Promise<void>;
  DeleteSensitiveFunc(id: string): Promise<void>;
};

function needDesktop(): void {
  if (!isWails()) throw new Error('代码审计需要桌面运行时(纯浏览器模式不可用)');
}

export interface AuditProject {
  id: string;
  name: string;
  files: number;
  created_at: number;
}

export interface FileNode {
  path: string;
  size: number;
  is_dir: boolean;
}

export interface SensitiveFunc {
  id: string;
  language: string;
  pattern: string;
  category: string;
  risk: 'high' | 'medium' | 'low' | string;
  note: string;
  enabled: boolean;
}

export interface SearchHit {
  path: string;
  line_no: number;
  line: string;
}

export interface SensHit {
  path: string;
  line_no: number;
  line: string;
  match: string;
  func_id: string;
  language: string;
  pattern: string;
  category: string;
  risk: 'high' | 'medium' | 'low' | string;
}

/** 导入文件夹:逐文件上传(name=webkitRelativePath 保留目录结构)→ 组装项目。 */
export async function importAuditProject(
  name: string,
  files: File[],
  onProgress?: (done: number, total: number) => void,
): Promise<AuditProject> {
  needDesktop();
  const refs: { ID: string; Name: string }[] = [];
  let done = 0;
  for (const f of files) {
    const bytes = new Uint8Array(await f.arrayBuffer());
    const rel = (f as File & { webkitRelativePath?: string }).webkitRelativePath || f.name;
    const info = await uploadFile(rel, bytes);
    refs.push({ ID: info.file_id!, Name: rel });
    onProgress?.(++done, files.length);
  }
  return App.AuditImport(name, refs);
}

export const listAuditProjects = async (): Promise<AuditProject[]> => {
  needDesktop();
  return (await App.ListAuditProjects()) ?? []; // 防御:null 载荷不再进入 state
};
export const deleteAuditProject = (id: string): Promise<void> => {
  needDesktop();
  return App.DeleteAuditProject(id);
};
export const auditFileTree = async (id: string): Promise<FileNode[]> => {
  needDesktop();
  return (await App.AuditFileTree(id)) ?? [];
};
export const auditReadFile = (id: string, rel: string): Promise<string> => {
  needDesktop();
  return App.AuditReadFile(id, rel);
};
export const auditWriteFile = (id: string, rel: string, content: string): Promise<void> => {
  needDesktop();
  return App.AuditWriteFile(id, rel, content);
};
export const auditSearch = async (id: string, query: string, regex: boolean, caseSensitive: boolean, limit = 500): Promise<SearchHit[]> => {
  needDesktop();
  return (await App.AuditSearch(id, query, regex, caseSensitive, limit)) ?? [];
};
export const auditScan = async (id: string): Promise<SensHit[]> => {
  needDesktop();
  return (await App.AuditScan(id)) ?? [];
};
export const listSensitiveFuncs = async (): Promise<SensitiveFunc[]> => {
  needDesktop();
  return (await App.ListSensitiveFuncs()) ?? [];
};
export const addSensitiveFunc = (language: string, pattern: string, category: string, risk: string, note: string, enabled = true): Promise<SensitiveFunc> => {
  needDesktop();
  return App.AddSensitiveFunc(language, pattern, category, risk, note, enabled);
};
export const updateSensitiveFunc = (f: SensitiveFunc): Promise<void> => {
  needDesktop();
  return App.UpdateSensitiveFunc(f.id, f.language, f.pattern, f.category, f.risk, f.note, f.enabled);
};
export const deleteSensitiveFunc = (id: string): Promise<void> => {
  needDesktop();
  return App.DeleteSensitiveFunc(id);
};
