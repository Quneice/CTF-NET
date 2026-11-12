/**
 * 指纹库管理 API 层（指纹库管理工具的数据源）。
 *
 * 桌面版：Wails 绑定 → Go 端 internal/tools/pentest/fingerprint_store.go
 * （DataDir/fingerprints/cms_fingerprints.json，首次启动从内置库播种）。
 * 浏览器版（npm run dev）：无桌面运行时，一律抛错（面板显示提示）。
 */
import { isWails } from '@/lib/backend';

import * as rawApp from '../../wailsjs/go/main/App';

// —— 类型（JSON 键与 Go internal/tools/pentest/fingerprint_store.go 一致）——

export interface FPDBStats {
  managed: boolean;
  path: string;
  version: string;
  description: string;
  file_size: number;
  modified: number; // Unix 秒
  cms_count: number;
  static_count: number;
  hash_count: number;
  path_count: number;
  vuln_count: number;
  backup_dir: string;
  backup_count: number;
}

export interface FPCmsSummary {
  cms_id: string;
  name: string;
  category: string;
  static_count: number;
  hash_count: number;
  path_count: number;
  vuln_count: number;
}

export interface FPDBListResult {
  total: number;
  items: FPCmsSummary[];
}

export interface FPStaticRow {
  pattern: string;
  location: string;
  confidence?: number | null;
  is_regex: boolean;
  description: string;
}

export interface FPHashRow {
  path: string;
  hash: string;
  algorithm: string;
  confidence?: number | null;
  description: string;
}

export interface FPPathRow {
  path: string;
  method: string;
  status_code_text: string;
  confidence?: number | null;
  response_pattern: string;
  description: string;
}

export interface FPVulnRow {
  id: string;
  name: string;
  severity: string;
  affected_versions: string;
  exploit_path: string;
  references: string[];
}

export interface FPCmsDetail {
  cms_id: string;
  name: string;
  category: string;
  static_strings: FPStaticRow[];
  file_hashes: FPHashRow[];
  paths: FPPathRow[];
  vulnerabilities: FPVulnRow[];
}

export interface FPImportResult {
  total: number;
  added: number;
  overwritten: number;
  skipped: number;
  skipped_reasons: string[];
  backup_path: string;
}

/** 空明细（新增 CMS 用）。 */
export function emptyFPCmsDetail(cmsID = ''): FPCmsDetail {
  return { cms_id: cmsID, name: '', category: '', static_strings: [], file_hashes: [], paths: [], vulnerabilities: [] };
}

// —— 绑定门面（生成绑定 wailsjs/go/main/App 自动更新，勿手改生成文件）——

const App = rawApp as unknown as {
  FingerprintDBStats(): Promise<FPDBStats>;
  FingerprintDBList(query: string, offset: number, limit: number): Promise<FPDBListResult>;
  FingerprintDBGet(cmsID: string): Promise<FPCmsDetail>;
  FingerprintDBSave(cmsID: string, entry: FPCmsDetail): Promise<void>;
  FingerprintDBDelete(cmsID: string): Promise<void>;
  FingerprintDBRestore(): Promise<FPDBStats>;
  FingerprintDBImport(path: string): Promise<FPImportResult>;
  FingerprintDBExport(path: string): Promise<void>;
};

function requireWails(): void {
  if (!isWails()) throw new Error('指纹库管理需要桌面运行时（请打包后在应用内使用）');
}

export async function fpStats(): Promise<FPDBStats> {
  requireWails();
  return App.FingerprintDBStats();
}

export async function fpList(query: string, offset: number, limit: number): Promise<FPDBListResult> {
  requireWails();
  return App.FingerprintDBList(query, offset, limit);
}

export async function fpGet(cmsID: string): Promise<FPCmsDetail> {
  requireWails();
  const d = await App.FingerprintDBGet(cmsID);
  // 防御性归一：Go 旧版本可能对空分组返回 null（JSON null 会让 .length 崩溃）
  d.static_strings ??= [];
  d.file_hashes ??= [];
  d.paths ??= [];
  d.vulnerabilities ??= [];
  for (const v of d.vulnerabilities) v.references ??= [];
  return d;
}

export async function fpSave(cmsID: string, entry: FPCmsDetail): Promise<void> {
  requireWails();
  await App.FingerprintDBSave(cmsID, entry);
}

export async function fpDelete(cmsID: string): Promise<void> {
  requireWails();
  await App.FingerprintDBDelete(cmsID);
}

export async function fpRestore(): Promise<FPDBStats> {
  requireWails();
  return App.FingerprintDBRestore();
}

export async function fpImport(path: string): Promise<FPImportResult> {
  requireWails();
  return App.FingerprintDBImport(path);
}

export async function fpExport(path: string): Promise<void> {
  requireWails();
  await App.FingerprintDBExport(path);
}
