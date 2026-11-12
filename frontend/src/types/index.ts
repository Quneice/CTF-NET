/**
 * =============================================================
 * CTF-NET 前端 —— 全局类型定义
 * =============================================================
 * 这是「配置驱动 + 注册表」架构的契约层。
 * 所有 mock 数据、动态表单、调用适配器、输出渲染器都必须严格遵循
 * 此处的类型。新增一个工具时，只允许新增一份符合这些类型的配置文件，
 * 禁止改动任何 UI 组件源码。
 * =============================================================
 */

// ---------------------------------------------------------------------------
// 分类树：支持任意层级（最多 4 层）
// ---------------------------------------------------------------------------
export interface CategoryNode {
  id: string;
  name: string;
  /** lucide-react 图标名，例如 'Globe' | 'KeyRound' | 'Bug' */
  icon?: string;
  description?: string;
  children?: CategoryNode[];
  /** 导航降级节点(ia.md §4):保留路由,但父分类页以筛选 chips 呈现而非子分类入口卡片 */
  segment?: boolean;
}

// ---------------------------------------------------------------------------
// 工具定义：挂载到 CategoryNode.id
// ---------------------------------------------------------------------------
/** 双向编解码 UI 的可选项 */
export interface CodecOptions {
  /** 显示「自定义字符集（字母表）」输入框 */
  alphabet?: boolean;
  /** 显示「填充字符」输入框；值为默认填充字符（如 '='），空字符串表示默认无填充 */
  padding?: string;
}

export interface ToolDefinition {
  id: string;
  name: string;
  /** 挂载到的分类节点 id */
  categoryId: string;
  description: string;
  tags: string[];
  version?: string;
  /** 工具图标（lucide 组件名，与 CategoryNode.icon 同一约定）；缺省复用所属分类的图标 */
  icon?: string;
  docsUrl?: string;
  /** 动态表单结构 */
  inputSchema: InputField[];
  /** 表单列数：2 = 顶层分组两列并排（宽屏），压缩参数区高度 */
  formColumns?: number;
  /** 调用方式 */
  invoke: InvokeSpec;
  /** 输出渲染方式，缺省为 { type: 'text' } */
  output?: OutputSpec;
  /** 标记使用「编码前/编码后」双向实时互转 UI（invoke 需为 local 类型）；对象形式可附加自定义字符集/填充字符输入 */
  codec?: boolean | CodecOptions;
  /** 单向转换 UI：输入即自动转换（适合解释器/视图/文本处理等单向工具，invoke 需为 local 类型） */
  transform?: { inputLabel?: string; outputLabel?: string };
  /** 自定义工作区标识：命中时使用专用交互组件（如 php-unserialize 参数识别表单） */
  workspace?: string;
}

// ---------------------------------------------------------------------------
// 输入字段：可递归嵌套
// ---------------------------------------------------------------------------

/** 条件操作符 */
export type ConditionOp = 'eq' | 'neq' | 'in' | 'truthy';

/** 条件：用于 visibleWhen / disabledWhen */
export interface Condition {
  field: string;
  op: ConditionOp;
  value?: unknown;
}

/** 所有字段共有的基础属性 */
export interface BaseField {
  name: string;
  label: string;
  required?: boolean;
  defaultValue?: unknown;
  placeholder?: string;
  help?: string;
  /** 同一工具不同模式下的条件显示 */
  visibleWhen?: Condition;
  /** 条件禁用 */
  disabledWhen?: Condition;
  /** 两列布局下占满整行 */
  spanFull?: boolean;
}

export interface FieldOption {
  label: string;
  value: string | number;
}

export interface FieldOptionString {
  label: string;
  value: string;
}

/**
 * 递归字段联合类型：
 * - 基础输入：text / password / number / textarea
 * - 选择类：select / radio / checkbox / checkboxGroup / switch
 * - 数值类：range（滑块）
 * - 文件类：file（单/多文件）
 * - 结构类：keyvalue（键值对）、code（代码）、group（可嵌套分组）
 * - 展示类：note（静态提示框，仅展示内容，不产生表单值）
 */
export type InputField =
  | (BaseField & { type: 'text' | 'password' | 'number' | 'textarea'; rows?: number; /** number 输入步进（如 0.05） */ step?: number })
  | (BaseField & { type: 'select' | 'radio'; options?: FieldOption[]; /** 动态选项来源（如 'dirWordlists'），由 DynamicForm 异步解析；与 options 二选一 */ optionsSource?: string; /** 选项取自某个 file 字段自动识别的列名（如 'file_a'） */ optionsFromFile?: string })
  | (BaseField & { type: 'checkbox' }) // 单个布尔
  | (BaseField & { type: 'checkboxGroup'; options: FieldOptionString[] })
  | (BaseField & { type: 'switch' })
  | (BaseField & { type: 'range'; min?: number; max?: number; step?: number })
  | (BaseField & { type: 'file'; multiple?: boolean; /** 文件夹选择模式（webkitdirectory）：递归上传整个文件夹，保留相对路径 */ directory?: boolean; accept?: string; maxSizeMB?: number; /** 选中后自动识别表格列并展示预览（CSV/Excel） */ previewColumns?: boolean })
  | (BaseField & { type: 'keyvalue'; keyPlaceholder?: string; valuePlaceholder?: string })
  | (BaseField & { type: 'code'; language?: string; rows?: number })
  /** 原始 HTTP 请求包：textarea + 解析按钮，解析后自动填入 target_url/method/param/extra_params 并提供参数下拉选择 */
  | (BaseField & { type: 'rawrequest'; placeholder?: string; rows?: number })
  /** 静态提示框：仅展示说明内容（\n 换行），不产生表单值、不参与提交与校验 */
  | (BaseField & { type: 'note'; content: string })
  | (BaseField & {
      type: 'group';
      children: InputField[];
      collapsible?: boolean;
      spanFull?: boolean;
      /** 组内字段列数：2 = 组内两列并排 */
      columns?: number;
    });

// ---------------------------------------------------------------------------
// 调用方式
// ---------------------------------------------------------------------------
export type InvokeSpec =
  | {
      type: 'http';
      method: 'GET' | 'POST' | 'PUT';
      url: string;
      contentType?: 'json' | 'form-data' | 'urlencoded' | 'raw';
      headers?: Record<string, string>;
    }
  | {
      type: 'task';
      submitUrl: string;
      pollUrl: string;
      intervalMs?: number;
      timeoutMs?: number;
      /** 走真实 FastAPI 后端（submitUrl/pollUrl 直连 /api），否则走 mock 模拟 */
      realBackend?: boolean;
    }
  | { type: 'sse'; url: string }
  | { type: 'ws'; url: string }
  /** 纯前端计算，注册到 LocalHandlerRegistry */
  | { type: 'local'; handlerId: string };

// ---------------------------------------------------------------------------
// 输出方式
// ---------------------------------------------------------------------------
export type OutputSpec =
  | { type: 'text' | 'markdown' | 'json' | 'image' | 'file' | 'hex' | 'stream' }
  | { type: 'code'; language?: string }
  | { type: 'table'; columns: { key: string; title: string }[] }
  | { type: 'custom'; rendererId: string };

// ---------------------------------------------------------------------------
// 通用辅助类型
// ---------------------------------------------------------------------------

/** 表单值：字段 name -> 任意值 */
export type FormValues = Record<string, unknown>;

/** 单条调用历史记录（阶段 6 完善） */
export interface HistoryEntry {
  id: string;
  toolId: string;
  toolName: string;
  params: FormValues;
  summary: string;
  timestamp: number;
}
