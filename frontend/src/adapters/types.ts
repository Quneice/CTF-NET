import type { FormValues, InvokeSpec, OutputSpec } from '@/types';

/**
 * 调用适配层类型定义。
 *
 * 所有 Adapter 都实现统一的「事件流」契约：返回 AsyncGenerator<InvokeEvent>，
 * 由 useInvoke 统一消费，维护同一套状态机（idle/loading/streaming/success/error/cancelled）。
 */

/** 适配器向状态机发射的事件 */
export type InvokeEvent =
  | { type: 'progress'; percent: number; message?: string } // 任务进度
  | { type: 'chunk'; text: string } // 流式追加文本
  | { type: 'done'; data: unknown; shell?: { sessionId: string } } // 最终结果（可附带交互 shell 会话）
  | { type: 'error'; message: string }; // 出错

/** 统一状态机状态 */
export type InvokeStatus =
  | 'idle'
  | 'loading'
  | 'streaming'
  | 'success'
  | 'error'
  | 'cancelled';

/** 状态机当前状态 */
export interface InvokeState {
  status: InvokeStatus;
  /** 任务进度 0-100 */
  progress: number;
  progressMessage?: string;
  /** 流式累积文本 */
  streamText: string;
  /** 最终结果（非流式或流式结束） */
  result: unknown;
  error: string | null;
  elapsedMs: number;
  /** 交互 shell 会话（任务成功后由后端返回 session_id 时设置） */
  shell?: { sessionId: string } | null;
}

/** 适配器入参上下文 */
export interface InvokeContext {
  spec: InvokeSpec;
  output: OutputSpec | undefined;
  params: FormValues;
  signal: AbortSignal;
}

/** 适配器函数签名 */
export type Adapter = (ctx: InvokeContext) => AsyncGenerator<InvokeEvent, void, unknown>;

/** 初始状态 */
export const INITIAL_INVOKE_STATE: InvokeState = {
  status: 'idle',
  progress: 0,
  streamText: '',
  result: undefined,
  error: null,
  elapsedMs: 0,
  shell: null,
};
