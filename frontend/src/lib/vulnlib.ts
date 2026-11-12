/**
 * 漏洞库 API 层（漏洞浏览器的数据源）。
 *
 * 桌面版：Wails 绑定 → Go 端 internal/vulnlib（SQLite，vulnlib.db）。
 * 浏览器版：动态导入静态快照 mock/vulnLibrary.ts —— 只有目录结构，
 * 无案例全文，扫描/详情/执行一律不可用（面板显示提示）。
 */
import { isWails } from '@/lib/backend';
import { getAppConfig } from '@/lib/settings';

// —— 类型（JSON 键与 Go internal/vulnlib/store.go 一致）——

export interface VulnCaseSummary {
  id: string; // 案例 ID = 漏洞库目录下的相对路径（如 Z001中间件漏洞/S1 Apache HTTP Server/ND003 ….md）
  cat_id: string;
  cat_name: string;
  sys: string;
  sys_code: string;
  name: string; // 文件名（不含 .md）
  title: string; // md 首个一级标题
  cves: string[];
  payload_count: number; // 可重放的原始 HTTP 请求包数
  size: number;
}

export interface VulnCaseDetail extends VulnCaseSummary {
  content: string; // md 全文
  payloads: string[]; // 提取出的原始 HTTP 请求包
  curls: string[]; // curl 命令
  path: string; // 磁盘绝对路径
  mtime: number;
}

export interface VulnSystemSummary {
  name: string;
  code: string;
  case_count: number;
}

export interface VulnCategory {
  id: string;
  code: string;
  name: string;
  case_count: number;
  systems: VulnSystemSummary[];
}

export interface VulnStats {
  scanned: boolean;
  root_dir: string;
  case_count: number;
  cat_count: number;
  last_scan: number; // Unix 秒
}

export interface VulnScanSummary {
  root_dir: string;
  total: number;
  updated: number;
  unchanged: number;
  deleted: number;
  skipped: number;
  errors: string[];
  duration_ms: number;
}

// —— 桌面绑定 ——

const App = () =>
  (window as unknown as {
    go?: {
      main?: {
        App?: {
          VulnLibStats(): Promise<Record<string, unknown>>;
          ScanVulnLibrary(dir: string): Promise<Record<string, unknown>>;
          ListVulnCategories(): Promise<Record<string, unknown>[]>;
          ListVulnCases(catID: string, sys: string): Promise<Record<string, unknown>[]>;
          SearchVulnCases(keyword: string, catID: string, limit: number, offset: number): Promise<Record<string, unknown>[]>;
          GetVulnCase(id: string): Promise<Record<string, unknown>>;
          GetVulnAsset(caseID: string, rel: string): Promise<Record<string, unknown>>;
          FetchVulnRemoteImage(url: string): Promise<Record<string, unknown>>;
          CreateVulnCase(catID: string, sys: string, name: string, content: string): Promise<Record<string, unknown>>;
          UpdateVulnCase(id: string, content: string): Promise<Record<string, unknown>>;
          DeleteVulnCase(id: string): Promise<Record<string, unknown>>;
        };
      };
    };
  }).go!.main!.App!;

/** 默认漏洞清单目录（本机语料固定路径，首次打开免输入）。 */
export const DEFAULT_VULNLIB_DIR = 'C:\\Users\\HP\\Desktop\\安全方向\\漏洞复现案例';

export interface VulnAsset {
  mime: string;
  data_b64: string;
  size: number;
  abs_path: string;
}

// 资源缓存：caseId|src → data URL（含失败后重试：失败即删缓存项）
const assetCache = new Map<string, Promise<string>>();

/**
 * 把案例内图片引用解析为 data URL。
 * - data:/blob: 原样返回；
 * - 本地相对路径 → 后端 GetVulnAsset；
 * - http(s) 远程图片：未配置全局代理（设置页）时交回 WebView 直连（当前行为）；
 *   配置了代理则经 Go 走代理拉取（所有浏览器出站流量都过代理）。
 */
export async function resolveVulnAsset(caseId: string, src: string): Promise<string> {
  if (/^(data:|blob:)/i.test(src)) return src;
  if (/^https?:/i.test(src)) return resolveRemoteImage(src);
  const key = `${caseId}|${src}`;
  let p = assetCache.get(key);
  if (!p) {
    p = (async () => {
      if (!isWails()) throw new Error('浏览器模式无法加载本地图片');
      const a = (await App().GetVulnAsset(caseId, src)) as unknown as VulnAsset;
      return `data:${a.mime};base64,${a.data_b64}`;
    })();
    assetCache.set(key, p);
    p.catch(() => assetCache.delete(key));
  }
  return p;
}

/** 远程图片：有全局代理 → Go 经代理拉取；无代理 → 原 URL（WebView 直连）。 */
async function resolveRemoteImage(src: string): Promise<string> {
  let proxy = '';
  try {
    proxy = ((await getAppConfig())?.proxy?.url ?? '').trim();
  } catch {
    // 读配置失败按无代理处理
  }
  if (!proxy) return src;
  const key = `remote|${src}`;
  let p = assetCache.get(key);
  if (!p) {
    p = (async () => {
      if (!isWails()) throw new Error('浏览器模式无法走代理');
      const a = (await App().FetchVulnRemoteImage(src)) as unknown as VulnAsset;
      return `data:${a.mime};base64,${a.data_b64}`;
    })();
    assetCache.set(key, p);
    p.catch(() => assetCache.delete(key));
  }
  return p;
}

/** 漏洞库状态；浏览器模式返回未扫描。 */
export async function vulnStats(): Promise<VulnStats> {
  if (!isWails()) return { scanned: false, root_dir: '', case_count: 0, cat_count: 0, last_scan: 0 };
  return (await App().VulnLibStats()) as unknown as VulnStats;
}

/** 扫描漏洞清单目录并导入（桌面绑定，同步返回摘要；首次全量约数秒）。 */
export async function scanVulnLibrary(dir: string): Promise<VulnScanSummary> {
  if (!isWails()) throw new Error('浏览器模式不可扫描：请用桌面版运行');
  return (await App().ScanVulnLibrary(dir)) as unknown as VulnScanSummary;
}

/** 全部分类（含系统计数；案例按需 ListVulnCases 拉取）。 */
export async function listVulnCategories(): Promise<VulnCategory[]> {
  if (isWails()) return (await App().ListVulnCategories()) as unknown as VulnCategory[];
  // 浏览器回退：静态快照只有结构（cases 为 {id,name}），转成 VulnCategory 形状
  const { VULN_LIBRARY } = await import('@/mock/vulnLibrary');
  return VULN_LIBRARY.map((c) => {
    const systems: VulnSystemSummary[] = c.systems.map((s) => ({
      name: s.name,
      code: s.code,
      case_count: s.cases.length,
    }));
    return {
      id: c.id,
      code: c.code,
      name: c.name,
      case_count: systems.reduce((n, s) => n + s.case_count, 0),
      systems,
    };
  });
}

/** 某分类（+可选系统）下的案例摘要；catID 为空表示全库（仅浏览器回退用到）。 */
export async function listVulnCases(catID: string, sys: string): Promise<VulnCaseSummary[]> {
  if (isWails()) return (await App().ListVulnCases(catID, sys)) as unknown as VulnCaseSummary[];
  const { VULN_LIBRARY } = await import('@/mock/vulnLibrary');
  const cats = catID ? VULN_LIBRARY.filter((c) => c.id === catID) : VULN_LIBRARY;
  return cats.flatMap((cat) => {
    const systems = sys ? cat.systems.filter((s) => s.name === sys || s.code === sys) : cat.systems;
    return systems.flatMap((s) =>
      s.cases.map((cs): VulnCaseSummary => ({
        id: cs.id,
        cat_id: cat.id,
        cat_name: cat.name,
        sys: s.name,
        sys_code: s.code,
        name: cs.name,
        title: cs.name,
        cves: [],
        payload_count: 0,
        size: 0,
      })),
    );
  });
}

/** 关键字搜索（标题/文件名/CVE）。 */
export async function searchVulnCases(keyword: string, catID: string): Promise<VulnCaseSummary[]> {
  if (isWails()) return (await App().SearchVulnCases(keyword, catID, 200, 0)) as unknown as VulnCaseSummary[];
  const all = await listVulnCases('', '');
  const kw = keyword.trim().toLowerCase();
  return all.filter(
    (c) =>
      (catID === '' || c.cat_id === catID) &&
      (c.name.toLowerCase().includes(kw) || c.title.toLowerCase().includes(kw)),
  );
}

/** 案例全文（含 payload）。浏览器模式抛错。 */
export async function getVulnCase(id: string): Promise<VulnCaseDetail> {
  if (!isWails()) throw new Error('浏览器模式无案例内容：请用桌面版运行并先扫描导入漏洞库');
  return (await App().GetVulnCase(id)) as unknown as VulnCaseDetail;
}

/** 新增案例：写入分类/系统目录下的 md 文件并导入（文件名自动补 .md）。 */
export async function createVulnCase(
  catID: string,
  sys: string,
  name: string,
  content: string,
): Promise<VulnScanSummary> {
  if (!isWails()) throw new Error('浏览器模式不可编辑：请用桌面版运行');
  return (await App().CreateVulnCase(catID, sys, name, content)) as unknown as VulnScanSummary;
}

/** 保存案例修改：覆写磁盘 md 文件并重新导入。 */
export async function updateVulnCase(id: string, content: string): Promise<VulnScanSummary> {
  if (!isWails()) throw new Error('浏览器模式不可编辑：请用桌面版运行');
  return (await App().UpdateVulnCase(id, content)) as unknown as VulnScanSummary;
}

/** 删除案例：md 文件移入 data/backups/vulnlib_deleted（可恢复）。 */
export async function deleteVulnCase(id: string): Promise<VulnScanSummary> {
  if (!isWails()) throw new Error('浏览器模式不可编辑：请用桌面版运行');
  return (await App().DeleteVulnCase(id)) as unknown as VulnScanSummary;
}
