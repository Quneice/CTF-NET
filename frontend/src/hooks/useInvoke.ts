import { useCallback, useRef, useState } from 'react';

import type { FormValues, InvokeSpec, OutputSpec } from '@/types';

import { getAdapter } from '@/adapters';
import { INITIAL_INVOKE_STATE, type InvokeState } from '@/adapters/types';

/**
 * 调用状态机 hook：统一封装 loading / progress / streaming / error / cancel。
 *
 * - run(spec, output, params)：按 spec.type 选择 Adapter 并消费其事件流；
 * - cancel()：中断当前调用（AbortController）；
 * - 自动计时 elapsedMs，供「运行时长」展示。
 */
export function useInvoke() {
  const [state, setState] = useState<InvokeState>(INITIAL_INVOKE_STATE);
  const abortRef = useRef<AbortController | null>(null);
  const timerRef = useRef<number | null>(null);

  const run = useCallback(
    async (spec: InvokeSpec, output: OutputSpec | undefined, params: FormValues) => {
      // 取消上一次未完成的调用
      abortRef.current?.abort();

      const controller = new AbortController();
      abortRef.current = controller;

      const start = Date.now();
      setState({ ...INITIAL_INVOKE_STATE, status: 'loading' });

      timerRef.current = window.setInterval(() => {
        setState((s) =>
          s.status === 'loading' || s.status === 'streaming'
            ? { ...s, elapsedMs: Date.now() - start }
            : s,
        );
      }, 100);

      try {
        const adapter = getAdapter(spec.type);
        for await (const ev of adapter({ spec, output, params, signal: controller.signal })) {
          if (controller.signal.aborted) break;
          switch (ev.type) {
            case 'progress':
              setState((s) => ({ ...s, progress: ev.percent, progressMessage: ev.message }));
              break;
            case 'chunk':
              setState((s) => ({ ...s, status: 'streaming', streamText: s.streamText + ev.text }));
              break;
            case 'done':
              setState((s) => ({
                ...s,
                status: 'success',
                result: ev.data,
                shell: ev.shell ?? null,
                elapsedMs: Date.now() - start,
              }));
              break;
            case 'error':
              setState((s) => ({ ...s, status: 'error', error: ev.message }));
              break;
          }
        }
        // 生成器提前结束（因 abort）则标记为已取消
        if (controller.signal.aborted) {
          setState((s) => ({ ...s, status: 'cancelled' }));
        }
      } catch (err) {
        if (controller.signal.aborted) {
          setState((s) => ({ ...s, status: 'cancelled' }));
        } else {
          setState((s) => ({
            ...s,
            status: 'error',
            error: err instanceof Error ? err.message : String(err),
          }));
        }
      } finally {
        if (timerRef.current !== null) window.clearInterval(timerRef.current);
        timerRef.current = null;
      }
    },
    [],
  );

  const cancel = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  return { state, run, cancel };
}
