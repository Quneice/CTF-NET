/**
 * Wails 桌面后端绑定封装（等价旧 FastAPI REST 契约）。
 *
 * - 生成绑定位于 frontend/wailsjs/go/main/App（`wails dev`/`wails build` 自动更新，勿手改）。
 * - 后端绑定方法一律不接收 context.Context 首参：Wails 运行时不注入 ctx，
 *   实参会错位进 ctx 槽位报 "cannot unmarshal ... into context.Context"（见 app.go 注释）。
 * - window.go 不存在（纯浏览器 `npm run dev`）时 isWails() 返回 false，
 *   适配层据此回退 mock，保证脱离桌面运行时也能开发 UI。
 *
 * 旧 REST 契约 → 绑定映射：
 *   POST /api/tools/{id}/run  { params }      → RunTool(id, params) -> { job_id }
 *   GET  /api/tasks/{jobId}                   → GetJob(jobId) -> { status, result, error, ... }
 *   POST /api/files         { name, data }    → UploadFile(name, data) -> { file_id, name, size }
 *   POST /api/sessions/{sid}/exec {command}   → ExecSessionCommand(sid, command, timeout)
 *   POST /api/sessions/{sid}/close            → CloseSession(sid) -> { ok }
 *
 * 批 1 起 file 型参数工具（strings / file_checksum）走真实后端：
 * 适配层把 File 实例上传为 file_id 再提交任务（见 adapters/index.ts uploadFileParams）。
 */
import * as rawApp from '../../wailsjs/go/main/App';

// —— 哨兵错误（与 app.go 的 errors.New 文案一致，按 message 前缀映射旧 404 行为）——
export const JOB_NOT_FOUND = '任务不存在';
export const SESSION_NOT_FOUND = '会话不存在或已关闭';

export interface JobDict {
  id?: string;
  status?: string;
  result?: {
    stdout?: string;
    stderr?: string;
    data?: {
      session_id?: string;
      reply?: string;
      title?: string;
      task_ids?: string[];
      msg_count?: number;
    };
  } | null;
  error?: string | null;
  /** 工具通过 ToolContext.ReportProgress 上报的运行期进度（0-100，未上报的工具为 0） */
  progress_percent?: number;
  /** 进度人类可读描述（如"正在识别 12/1010：xxx.mp3"） */
  progress_message?: string;
}

export interface ExecOut {
  output?: string;
  timed_out?: boolean;
  closed?: boolean;
}

export interface FileInfo {
  file_id?: string;
  name?: string;
  size?: number;
}

// —— 对话式渗透（ai.pentest.chat）会话绑定 ——

/** 助手回复的轮次分段（一轮 = 思考/叙述 + 本轮工具动作；kind: text/thinking/tool/tool_result）。 */
export interface ChatSegDict {
  kind?: string;
  text?: string;
  name?: string;
  summary?: string;
  is_error?: boolean;
}

export interface ChatMsgDict {
  role?: string;
  content?: string;
  ts?: string;
  /** 助手回复的轮次分段；存在时前端按轮渲染（缺省回退为单气泡 content）。 */
  segments?: ChatSegDict[];
}

export interface ChatSessionSummaryDict {
  id?: string;
  title?: string;
  updated_at?: string;
  msg_count?: number;
}

export interface ChatSessionDict {
  id?: string;
  title?: string;
  created_at?: string;
  updated_at?: string;
  messages?: ChatMsgDict[];
  task_ids?: string[];
}

export function isWails(): boolean {
  return (
    typeof window !== 'undefined' &&
    !!(window as unknown as { go?: { main?: { App?: unknown } } }).go?.main?.App
  );
}

/** 绑定门面：剥离 context 参数的类型包袱。 */
const App = rawApp as unknown as {
  RunTool(toolID: string, params: Record<string, unknown>): Promise<{ job_id?: string }>;
  GetJob(jobID: string): Promise<Record<string, unknown>>;
  UploadFile(name: string, data: number[]): Promise<FileInfo>;
  UploadFileB64(name: string, dataB64: string): Promise<FileInfo>;
  ExecSessionCommand(sid: string, command: string, timeout: number): Promise<ExecOut>;
  CloseSession(sid: string): Promise<Record<string, unknown>>;
  ListDirWordlists(): Promise<{ name: string; lines: number; size: number }[]>;
  VulnLibStats(): Promise<Record<string, unknown>>;
  ScanVulnLibrary(dir: string): Promise<Record<string, unknown>>;
  ListVulnCategories(): Promise<Record<string, unknown>[]>;
  ListVulnCases(catID: string, sys: string): Promise<Record<string, unknown>[]>;
  SearchVulnCases(keyword: string, catID: string, limit: number, offset: number): Promise<Record<string, unknown>[]>;
  GetVulnCase(id: string): Promise<Record<string, unknown>>;
  ListChatSessions(): Promise<Record<string, unknown>[]>;
  GetChatSession(sessionID: string): Promise<Record<string, unknown>>;
  DeleteChatSession(sessionID: string): Promise<void>;
  SendChatMessage(sessionID: string, message: string, enableExec: boolean): Promise<Record<string, unknown>>;
  ListAITasks(): Promise<Record<string, unknown>[]>;
  GetAITask(taskID: string): Promise<Record<string, unknown>>;
  ControlAITask(taskID: string, action: string): Promise<Record<string, unknown>>;
  ControlAIIntent(taskID: string, intentID: number, action: string, msg: string): Promise<Record<string, unknown>>;
  AIStats(): Promise<Record<string, unknown>>;
  AIFindings(): Promise<Record<string, unknown>[]>;
  AIAssets(): Promise<Record<string, unknown>[]>;
  AIActivities(limit: number): Promise<Record<string, unknown>[]>;
};

/** 从 submitUrl（/api/tools/{id}/run）提取工具 ID；失败返回空串。 */
export function toolIdFromSubmitUrl(submitUrl: string): string {
  const m = /\/api\/tools\/(.+)\/run/.exec(submitUrl);
  return m ? m[1] : '';
}

export async function runTool(
  toolID: string,
  params: Record<string, unknown>,
): Promise<{ job_id: string }> {
  const res = await App.RunTool(toolID, params);
  if (!res || !res.job_id) {
    throw new Error('响应缺少 job_id');
  }
  return res as { job_id: string };
}

export async function getJob(jobID: string): Promise<JobDict> {
  return (await App.GetJob(jobID)) as unknown as JobDict;
}

/** 上传文件到后端文件存储,返回 file_id(等价旧 POST /api/files)。
 *
 * 桌面绑定通道中 []byte 以 number[] 逐字节 JSON 序列化，大文件开销高；
 * 这里统一走 UploadFileB64：base64 文本体积约 1/3，序列化快得多。
 */
export async function uploadFile(name: string, data: Uint8Array): Promise<FileInfo> {
  const res = await App.UploadFileB64(name, bytesToBase64(data));
  if (!res || !res.file_id) {
    throw new Error('上传失败: 响应缺少 file_id');
  }
  return res;
}

/** 分块 base64 编码（整块 btoa 在大文件上会爆调用栈） */
function bytesToBase64(bytes: Uint8Array): string {
  const CHUNK = 0x8000;
  let bin = '';
  for (let i = 0; i < bytes.length; i += CHUNK) {
    bin += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return btoa(bin);
}

export async function execSession(
  sid: string,
  command: string,
  timeout = 15, // 与旧 Pydantic 默认一致（2..120）
): Promise<ExecOut> {
  return App.ExecSessionCommand(sid, command, timeout);
}

export async function closeSessionBinding(sid: string): Promise<void> {
  await App.CloseSession(sid);
}

/** 字典库字典列表（wordlists/dir）。浏览器 mock 模式返回空列表。 */
export async function listDirWordlists(): Promise<{ name: string; lines: number; size: number }[]> {
  if (!isWails()) return [];
  return App.ListDirWordlists();
}

/** 会话列表（按更新时间倒序）。浏览器 mock 模式返回空列表。 */
export async function listChatSessions(): Promise<ChatSessionSummaryDict[]> {
  if (!isWails()) return [];
  return (await App.ListChatSessions()) as unknown as ChatSessionSummaryDict[];
}

/** 读取指定会话详情（含消息历史）；不存在返回 null。 */
export async function getChatSession(sessionID: string): Promise<ChatSessionDict | null> {
  if (!isWails()) return null;
  const res = await App.GetChatSession(sessionID);
  return res as unknown as ChatSessionDict | null;
}

/** 删除会话（后端软删除到 backups/trash/chat）。 */
export async function deleteChatSession(sessionID: string): Promise<void> {
  if (!isWails()) return;
  await App.DeleteChatSession(sessionID);
}

// —— 实时对话（SendChatMessage）：调用立即返回，助手回复以 chat:* 事件流推送 ——

export interface ChatEventDict {
  session_id?: string;
  /** chat:delta 助手文本增量 / chat:thinking 思考内容增量 */
  content?: string;
  /** chat:tool / chat:tool_result */
  name?: string;
  summary?: string;
  is_error?: boolean;
  /** chat:done */
  reply?: string;
  title?: string;
  task_ids?: string[];
  msg_count?: number;
  /** chat:error */
  message?: string;
}

const CHAT_EVENTS = ['delta', 'thinking', 'tool', 'tool_result', 'done', 'error'] as const;
export type ChatEventKind = (typeof CHAT_EVENTS)[number];

const chatHandlers = new Map<string, Set<(ev: ChatEventDict) => void>>();
let chatEventsBound = false;

function ensureChatEvents(): void {
  if (chatEventsBound || !isWails()) return;
  const rt = (window as unknown as { runtime?: { EventsOn?: (ev: string, cb: (data: ChatEventDict) => void) => void } })
    .runtime;
  if (!rt?.EventsOn) return;
  chatEventsBound = true;
  for (const kind of CHAT_EVENTS) {
    rt.EventsOn(`chat:${kind}`, (ev) => {
      const set = chatHandlers.get(kind);
      if (set && set.size > 0) set.forEach((h) => h(ev));
    });
  }
}

/** 订阅 chat:* 事件流（桌面运行时独有），返回取消订阅函数。 */
export function onChatEvent(kind: ChatEventKind, handler: (ev: ChatEventDict) => void): () => void {
  ensureChatEvents();
  let set = chatHandlers.get(kind);
  if (!set) {
    set = new Set();
    chatHandlers.set(kind, set);
  }
  set.add(handler);
  return () => {
    set.delete(handler);
    if (set.size === 0) chatHandlers.delete(kind);
  };
}

/** 发送一条对话消息，立即返回 {session_id}；回复经 onChatEvent 流式推送。 */
export async function sendChatMessage(
  sessionID: string,
  message: string,
  enableExec: boolean,
): Promise<{ session_id: string }> {
  const res = await App.SendChatMessage(sessionID, message, enableExec);
  if (!res || typeof res.session_id !== 'string' || !res.session_id) {
    throw new Error('响应缺少 session_id');
  }
  return { session_id: res.session_id };
}

export function isJobNotFound(err: unknown): boolean {
  return err instanceof Error && err.message.startsWith(JOB_NOT_FOUND);
}

export function isSessionNotFound(err: unknown): boolean {
  return err instanceof Error && err.message.startsWith(SESSION_NOT_FOUND);
}

// —— 任务终态事件（task:done，后端在任务结束时 EventsEmit）——
// 桌面运行时独有：订阅后任务完成即时回调，无需等下一个轮询周期。
// 事件可能先于订阅到达（秒完成的快任务），故带一层缓冲，订阅时先查缓冲。

type JobDoneHandler = (job: JobDict) => void;

const doneHandlers = new Map<string, Set<JobDoneHandler>>();
const doneBuffer = new Map<string, JobDict>();
let eventsBound = false;

function ensureJobDoneEvent(): void {
  if (eventsBound || !isWails()) return;
  const rt = (window as unknown as { runtime?: { EventsOn?: (ev: string, cb: (data: JobDict) => void) => void } })
    .runtime;
  if (!rt?.EventsOn) return;
  eventsBound = true;
  rt.EventsOn('task:done', (job) => {
    const id = job?.id;
    if (!id) return;
    const set = doneHandlers.get(id);
    if (set && set.size > 0) {
      set.forEach((h) => h(job));
    } else {
      if (doneBuffer.size > 1000) doneBuffer.clear();
      doneBuffer.set(id, job);
    }
  });
}

/** 订阅任务终态事件，返回取消订阅函数。非桌面运行时退化为 no-op。 */
export function onJobDone(jobId: string, handler: JobDoneHandler): () => void {
  ensureJobDoneEvent();
  const cached = doneBuffer.get(jobId);
  if (cached) {
    doneBuffer.delete(jobId);
    queueMicrotask(() => handler(cached));
    return () => {};
  }
  let set = doneHandlers.get(jobId);
  if (!set) {
    set = new Set();
    doneHandlers.set(jobId, set);
  }
  set.add(handler);
  return () => {
    set.delete(handler);
    if (set.size === 0) doneHandlers.delete(jobId);
  };
}

// —— AI 渗透工作台（仪表盘/任务/发现/资产，P0）——

export interface AITaskSummaryDict {
  id?: string;
  description?: string;
  goal_text?: string;
  status?: string;
  summary?: string;
  created_at?: string;
  completed_at?: string;
  paused?: boolean;
  intents?: number;
  open_intents?: number;
  findings?: number;
  assets?: number;
  facts?: number;
  usage?: { in: number; out: number };
}

export interface AIFindingDict {
  task_id?: string;
  task_desc?: string;
  finding?: {
    id?: number;
    title?: string;
    vuln_class?: string;
    severity?: string;
    summary?: string;
    evidence?: string;
    poc?: string;
    intent_id?: number;
    confirmed?: boolean;
    created_at?: string;
  };
}

export interface AIAssetDict {
  task_id?: string;
  task_desc?: string;
  asset?: {
    id?: number;
    type?: string;
    url?: string;
    domain?: string;
    ip?: string;
    props?: Record<string, string>;
    added_by?: string;
  };
}

export interface AIStatsDict {
  tasks?: number;
  running?: number;
  paused?: number;
  done?: number;
  exhausted?: number;
  failed?: number;
  findings?: number;
  assets?: number;
  facts?: number;
  intents?: number;
  usage?: { in: number; out: number };
  by_day?: { day: string; usage: { in: number; out: number } }[];
}

export interface AIActivityDict {
  task_id?: string;
  task_desc?: string;
  activity?: {
    seq?: number;
    ts?: string;
    agent?: string;
    kind?: string;
    summary?: string;
  };
}

/** AI 任务详情（后端 *aiagent.Task 快照） */
export interface AITaskDetailDict {
  id?: string;
  description?: string;
  goal_text?: string;
  hint?: string;
  status?: string;
  summary?: string;
  created_at?: string;
  completed_at?: string;
  timeout_sec?: number;
  max_turns?: number;
  plan_rounds?: number;
  rounds_used?: number;
  paused?: boolean;
  usage?: { in: number; out: number };
  goals?: { id: number; text: string; met: boolean; met_by?: string; evd_sum?: string }[];
  intents?: {
    id: number;
    summary: string;
    state: string;
    result?: string;
    steer?: string;
    trace?: string[];
    round?: number;
    worker?: string;
  }[];
  findings?: NonNullable<AIFindingDict['finding']>[];
  assets?: NonNullable<AIAssetDict['asset']>[];
  facts?: { id: number; text: string; kind?: string; source?: string }[];
  edges?: { id: number; from: string; to: string; rel: string }[];
  activities?: NonNullable<AIActivityDict['activity']>[];
}

/** 任务列表（按创建时间倒序）。浏览器 mock 模式返回空列表。 */
export async function listAITasks(): Promise<AITaskSummaryDict[]> {
  if (!isWails()) return [];
  return (await App.ListAITasks()) as unknown as AITaskSummaryDict[];
}

/** 任务详情（运行中取内存态）。 */
export async function getAITask(taskID: string): Promise<AITaskDetailDict | null> {
  if (!isWails()) return null;
  return (await App.GetAITask(taskID)) as unknown as AITaskDetailDict | null;
}

/** 控制运行中任务：stop / pause / resume。 */
export async function controlAITask(taskID: string, action: string): Promise<void> {
  if (!isWails()) return;
  await App.ControlAITask(taskID, action);
}

/** 控制运行中任务的单条意图：rerun / steer(msg)。 */
export async function controlAIIntent(
  taskID: string,
  intentID: number,
  action: string,
  msg?: string,
): Promise<void> {
  if (!isWails()) return;
  await App.ControlAIIntent(taskID, intentID, action, msg ?? '');
}

/** 仪表盘总览。 */
export async function getAIStats(): Promise<AIStatsDict> {
  if (!isWails()) return emptyAIStats();
  return (await App.AIStats()) as unknown as AIStatsDict;
}

/** 聚合发现。 */
export async function listAIFindings(): Promise<AIFindingDict[]> {
  if (!isWails()) return [];
  return (await App.AIFindings()) as unknown as AIFindingDict[];
}

/** 聚合资产。 */
export async function listAIAssets(): Promise<AIAssetDict[]> {
  if (!isWails()) return [];
  return (await App.AIAssets()) as unknown as AIAssetDict[];
}

/** 跨任务活动流。 */
export async function listAIActivities(limit: number): Promise<AIActivityDict[]> {
  if (!isWails()) return [];
  return (await App.AIActivities(limit)) as unknown as AIActivityDict[];
}

function emptyAIStats(): AIStatsDict {
  return { tasks: 0, running: 0, paused: 0, done: 0, exhausted: 0, failed: 0, findings: 0, assets: 0, facts: 0, intents: 0, usage: { in: 0, out: 0 }, by_day: [] };
}
