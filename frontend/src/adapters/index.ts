import type { InvokeSpec } from '@/types';

import { getJob, isJobNotFound, isWails, onJobDone, runTool, toolIdFromSubmitUrl, uploadFile, type JobDict } from '@/lib/backend';
import { generateMockResult } from './mockResult';
import { localHandlerRegistry } from './localHandlers';
import type { Adapter, InvokeContext, InvokeEvent } from './types';

/**
 * 五种调用适配器的 mock 实现 + 适配器注册表。
 *
 * 每个 Adapter 都是 AsyncGenerator<InvokeEvent>，通过统一的 sleep 模拟延迟，
 * 并响应 AbortSignal 取消。真实后端接入时，只需替换对应 Adapter 的网络实现。
 */

/** 可中断的 sleep：abort 时提前 resolve */
function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const t = setTimeout(resolve, ms);
    signal.addEventListener(
      'abort',
      () => {
        clearTimeout(t);
        resolve();
      },
      { once: true },
    );
  });
}

/** 把文本按行切成流式块 */
function* lineChunks(text: string): Generator<string> {
  const lines = text.split('\n');
  for (let i = 0; i < lines.length; i++) {
    yield lines[i] + '\n';
  }
}

/**
 * 提交真实后端前,把 File 型参数值上传为 file_id(等价旧 POST /api/files)。
 * 判定依据是值本身(File[] 实例)——InvokeContext 不带 schema,故不做字段名推断。
 * 单个文件 → 字符串 file_id;多文件 → file_id 数组,与 Go resolveFileParams 契约一致。
 * 任一文件上传失败抛错,由调用方转为 InvokeEvent error。
 */
async function uploadFileParams(
  params: Record<string, unknown>,
  signal: AbortSignal,
): Promise<Record<string, unknown>> {
  const out: Record<string, unknown> = { ...params };
  for (const [k, v] of Object.entries(params)) {
    if (!(Array.isArray(v) && v.length > 0 && v[0] instanceof File)) continue;
    const files = v as File[];
    const ids: string[] = [];
    for (const f of files) {
      if (signal.aborted) throw new Error('已取消');
      const bytes = new Uint8Array(await f.arrayBuffer());
      // 文件夹上传时用相对路径作为存储名，Go 端 resolveFileParams 据此保留目录结构
      const rel = (f as File & { webkitRelativePath?: string }).webkitRelativePath;
      const info = await uploadFile(rel || f.name, bytes);
      ids.push(info.file_id!);
    }
    out[k] = ids.length === 1 ? ids[0] : ids;
  }
  return out;
}

/** 生成一段较长的流式 mock 文本 */
function buildStreamText(params: Record<string, unknown>): string {
  const echo = Object.entries(params)
    .map(([k, v]) => `${k}=${JSON.stringify(v)}`)
    .join(' ');
  const lines = [`[stream] 已连接，参数：${echo}`];
  for (let i = 1; i <= 12; i++) lines.push(`data: 事件 ${i}/12`);
  lines.push('[stream] 结束');
  return lines.join('\n');
}

// —— HTTP：模拟请求延迟后一次性返回 ——
async function* httpAdapter(ctx: InvokeContext): AsyncGenerator<InvokeEvent, void, unknown> {
  await sleep(600, ctx.signal);
  if (ctx.signal.aborted) return;
  yield { type: 'done', data: generateMockResult(ctx.output, ctx.params) };
}

// —— 异步任务：真实后端（Wails 绑定提交 → 轮询直到任务结束）——
// 契约（app.go RunTool/GetJob，JSON 键与旧 REST 一致）：
//   RunTool(toolId, params) -> { job_id }
//   GetJob(jobId) -> { status: pending|running|success|error,
//                      result: { stdout, stderr, ... } | null, error }
// window.go 不存在（纯浏览器 npm run dev）时回退 mock 模拟，保证 UI 可开发。
// 注：Wails 绑定不接受 AbortSignal，取消靠每轮 await 后检查 ctx.signal.aborted。
async function* realTaskAdapter(ctx: InvokeContext): AsyncGenerator<InvokeEvent, void, unknown> {
  const spec = ctx.spec as Extract<InvokeSpec, { type: 'task' }>;
  const interval = spec.intervalMs ?? 2000;
  const timeout = spec.timeoutMs ?? 600_000;

  // 0) 无桌面运行时 → 回退 mock 模拟进度
  if (!isWails()) {
    yield { type: 'progress', percent: 5, message: '未检测到桌面运行时，使用模拟数据…' };
    const steps = 10;
    for (let i = 1; i <= steps; i++) {
      await sleep(interval / 3, ctx.signal);
      if (ctx.signal.aborted) return;
      yield { type: 'progress', percent: Math.round((i / steps) * 100), message: `处理中 ${i}/${steps}` };
    }
    yield { type: 'done', data: generateMockResult(ctx.output, ctx.params) };
    return;
  }

  const toolId = toolIdFromSubmitUrl(spec.submitUrl);
  if (!toolId) {
    yield { type: 'error', message: `无法从 ${spec.submitUrl} 解析工具 ID` };
    return;
  }

  // 0.5) File 型参数先上传为 file_id(strings / file_checksum 等 file 参数工具)
  let params = ctx.params;
  try {
    params = await uploadFileParams(ctx.params, ctx.signal);
  } catch (err) {
    if (!ctx.signal.aborted) {
      yield {
        type: 'error',
        message: `上传文件失败: ${err instanceof Error ? err.message : String(err)}`,
      };
    }
    return;
  }

  // 1) 提交任务
  let jobId = '';
  try {
    const submitted = await runTool(toolId, params);
    jobId = submitted.job_id;
  } catch (err) {
    if (!ctx.signal.aborted) {
      yield {
        type: 'error',
        message: `提交任务失败: ${err instanceof Error ? err.message : String(err)}`,
      };
    }
    return;
  }
  if (!jobId) {
    yield { type: 'error', message: '提交任务失败: 响应缺少 job_id' };
    return;
  }

  // 2) 等待任务结束：后端终态事件（task:done）即时结算，2s 轮询降级为兜底
  const start = Date.now();
  let evtJob: JobDict | null = null;
  let wake: () => void = () => {};
  const offDone = onJobDone(jobId, (j) => {
    evtJob = j;
    wake();
  });
  try {
    while (Date.now() - start < timeout) {
      await Promise.race([sleep(interval, ctx.signal), new Promise<void>((r) => (wake = r))]);
      if (ctx.signal.aborted) return;

      let job: JobDict;
      if (evtJob) {
        job = evtJob; // 事件推送的终态载荷，免一次 IPC
      } else {
        try {
          job = await getJob(jobId);
        } catch (err) {
          if (ctx.signal.aborted) return;
          if (isJobNotFound(err)) continue; // 刚提交尚未可见，下轮再试（旧 404 语义）
          yield {
            type: 'error',
            message: `查询任务失败: ${err instanceof Error ? err.message : String(err)}`,
          };
          return;
        }
      }

      if (job.status === 'pending' || job.status === 'running') {
        // 工具上报了进度（progress_percent 字段）→ 展示真实进度；
        // 未上报的工具回退到固定 50% 的"执行中"提示（历史行为）
        const pp = typeof job.progress_percent === 'number' ? job.progress_percent : null;
        const pct = pp === null ? 50 : Math.min(100, Math.max(1, pp));
        const msg =
          typeof job.progress_message === 'string' && job.progress_message.trim()
            ? job.progress_message
            : job.status === 'pending'
              ? '任务排队中…'
              : '任务执行中…';
        yield { type: 'progress', percent: pct, message: msg };
        continue;
      }
      // 任务级失败（工具未产出结果）：只能展示 error 字段
      if (!job.result) {
        yield { type: 'error', message: job.error || '任务失败（无结果）' };
        return;
      }
      // 工具已产出结果：即使是 error 状态（如 JNDI 未收到反弹 shell），
      // 也必须完整展示 stdout/stderr 中的诊断报告，否则排查信息被丢弃
      const text = [job.result.stdout || '', job.result.stderr ? `STDERR:\n${job.result.stderr}` : '']
        .filter(Boolean)
        .join('\n');
      if (job.status === 'error') {
        yield { type: 'error', message: text || job.error || '任务失败（无结果）' };
        return;
      }
      const sessionId = job.result.data?.session_id;
      yield {
        type: 'done',
        data: text,
        shell: sessionId ? { sessionId } : undefined,
      };
      return;
    }
    yield { type: 'error', message: `任务超时（>${Math.round(timeout / 1000)}s）` };
  } finally {
    offDone();
  }
}

// —— 异步任务：提交 + 轮询（realBackend 走真实后端，否则模拟进度）——
async function* taskAdapter(ctx: InvokeContext): AsyncGenerator<InvokeEvent, void, unknown> {
  const spec = ctx.spec as Extract<InvokeSpec, { type: 'task' }>;
  if (spec.realBackend) {
    yield* realTaskAdapter(ctx);
    return;
  }
  const interval = spec.intervalMs ?? 500;
  const steps = 10;
  for (let i = 1; i <= steps; i++) {
    await sleep(interval, ctx.signal);
    if (ctx.signal.aborted) return;
    yield { type: 'progress', percent: Math.round((i / steps) * 100), message: `处理中 ${i}/${steps}` };
  }
  yield { type: 'done', data: generateMockResult(ctx.output, ctx.params) };
}

// —— SSE：模拟服务端逐块推送 ——
async function* sseAdapter(ctx: InvokeContext): AsyncGenerator<InvokeEvent, void, unknown> {
  const text = buildStreamText(ctx.params);
  for (const chunk of lineChunks(text)) {
    await sleep(150, ctx.signal);
    if (ctx.signal.aborted) return;
    yield { type: 'chunk', text: chunk };
  }
  yield { type: 'done', data: text };
}

// —— WebSocket：先发连接回显，再逐块推送 ——
async function* wsAdapter(ctx: InvokeContext): AsyncGenerator<InvokeEvent, void, unknown> {
  const spec = ctx.spec as Extract<InvokeSpec, { type: 'ws' }>;
  yield { type: 'chunk', text: `[ws] 已连接 ${spec.url}\n` };
  const text = buildStreamText(ctx.params);
  for (const chunk of lineChunks(text)) {
    await sleep(150, ctx.signal);
    if (ctx.signal.aborted) return;
    yield { type: 'chunk', text: chunk };
  }
  yield { type: 'done', data: text };
}

// —— 本地纯前端计算 ——
async function* localAdapter(ctx: InvokeContext): AsyncGenerator<InvokeEvent, void, unknown> {
  await sleep(80, ctx.signal);
  if (ctx.signal.aborted) return;

  const spec = ctx.spec as Extract<InvokeSpec, { type: 'local' }>;
  const handler = localHandlerRegistry.get(spec.handlerId);

  if (!handler) {
    yield { type: 'done', data: generateMockResult(ctx.output, ctx.params) };
    return;
  }

  try {
    const data = await Promise.resolve(handler(ctx.params));
    yield { type: 'done', data };
  } catch (err) {
    yield { type: 'error', message: err instanceof Error ? err.message : String(err) };
  }
}

/** 适配器注册表：InvokeSpec.type -> Adapter */
const adapterRegistry: Record<InvokeSpec['type'], Adapter> = {
  http: httpAdapter,
  task: taskAdapter,
  sse: sseAdapter,
  ws: wsAdapter,
  local: localAdapter,
};

export function getAdapter(type: InvokeSpec['type']): Adapter {
  return adapterRegistry[type];
}
