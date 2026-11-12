import { useEffect, useRef, useState } from 'react';
import { Loader2, Power, Terminal } from 'lucide-react';

import {
  closeSessionBinding,
  execSession,
  isSessionNotFound,
  isWails,
} from '@/lib/backend';

/**
 * 交互 Shell 终端：任务成功后由 OutputPanel 渲染。
 *
 * 与后端契约（Wails 绑定，等价旧 REST）：
 *   ExecSessionCommand(sid, command, timeout) -> { output, timed_out, closed }
 *   CloseSession(sid) -> { ok }
 * 纯浏览器 dev（无 window.go）时回退旧 fetch 通道（FastAPI 已下线，仅兜底展示错误）。
 *
 * 命令完成后端以 marker 判定并返回增量输出；本组件只负责展示与发送。
 * 会话在用户点击「关闭会话」或 shell 断开（closed）时结束，远程环境同步清理。
 */
export default function ShellView({ sessionId }: { sessionId: string }) {
  const [output, setOutput] = useState('');
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  const [closed, setClosed] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [histIdx, setHistIdx] = useState(-1);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // 新输出自动滚到底部
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [output, busy]);

  async function sendCommand(raw: string) {
    const command = raw.trim();
    if (!command || busy || closed) return;
    setBusy(true);
    setOutput((o) => o + (o ? '\n' : '') + `$ ${command}\n`);
    setHistory((h) => [...h, command]);
    setHistIdx(-1);
    try {
      if (!isWails()) {
        // 浏览器 dev 兜底：旧 fetch 通道（FastAPI 已下线，一般走 error 展示）
        const res = await fetch(`/api/sessions/${sessionId}/exec`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ command }),
        });
        if (!res.ok) {
          if (res.status === 404) {
            setClosed(true);
            setOutput((o) => o + '[会话不存在或已关闭]\n');
          } else {
            const data = await res.json().catch(() => null);
            setOutput(
              (o) => o + `[执行失败: ${(data && data.detail) || `HTTP ${res.status}`}]\n`,
            );
          }
          return;
        }
        const data = await res.json();
        if (data.output) {
          setOutput((o) => o + data.output + (data.output.endsWith('\n') ? '' : '\n'));
        }
        if (data.timed_out) setOutput((o) => o + '[命令超时，输出可能不完整]\n');
        if (data.closed) {
          setClosed(true);
          setOutput((o) => o + '[目标 shell 已断开，会话结束]\n');
        }
        return;
      }
      const data = await execSession(sessionId, command);
      const { output: out, timed_out: timedOut, closed: isClosed } = data;
      if (out) {
        setOutput((o) => o + out + (out.endsWith('\n') ? '' : '\n'));
      }
      if (timedOut) setOutput((o) => o + '[命令超时，输出可能不完整]\n');
      if (isClosed) {
        setClosed(true);
        setOutput((o) => o + '[目标 shell 已断开，会话结束]\n');
      }
    } catch (err) {
      if (isSessionNotFound(err)) {
        setClosed(true);
        setOutput((o) => o + '[会话不存在或已关闭]\n');
      } else {
        setOutput(
          (o) => o + `[执行失败: ${err instanceof Error ? err.message : String(err)}]\n`,
        );
      }
    } finally {
      setBusy(false);
      inputRef.current?.focus();
    }
  }

  async function closeSession() {
    if (closed) return;
    setClosed(true);
    try {
      if (isWails()) {
        await closeSessionBinding(sessionId);
      } else {
        await fetch(`/api/sessions/${sessionId}/close`, { method: 'POST' });
      }
      setOutput((o) => o + '\n[会话已关闭，远程环境已清理]\n');
    } catch {
      setOutput((o) => o + '\n[关闭请求失败，请稍后重试]\n');
    }
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      sendCommand(input);
      setInput('');
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (!history.length) return;
      const idx = histIdx < 0 ? history.length - 1 : Math.max(0, histIdx - 1);
      setHistIdx(idx);
      setInput(history[idx]);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (histIdx < 0) return;
      const idx = histIdx + 1;
      if (idx >= history.length) {
        setHistIdx(-1);
        setInput('');
      } else {
        setHistIdx(idx);
        setInput(history[idx]);
      }
    }
  }

  return (
    <div className="mt-4 overflow-hidden rounded-lg border border-slate-700 bg-slate-950">
      <div className="flex items-center justify-between border-b border-slate-800 px-3 py-2">
        <span className="flex items-center gap-2 text-xs text-slate-300">
          <Terminal className="h-3.5 w-3.5 text-emerald-400" />
          交互 Shell
          <span className="text-slate-600">·</span>
          <span className="font-mono text-slate-500">{sessionId}</span>
          <span className="ml-1 flex items-center gap-1.5">
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                closed ? 'bg-slate-600' : busy ? 'animate-pulse bg-indigo-400' : 'bg-emerald-400'
              }`}
            />
            <span className={closed ? 'text-slate-500' : 'text-emerald-400/80'}>
              {closed ? '已关闭' : busy ? '执行中…' : '已连接'}
            </span>
          </span>
        </span>
        <button
          type="button"
          onClick={closeSession}
          disabled={closed}
          className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-slate-400 hover:bg-red-500/10 hover:text-red-400 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Power className="h-3.5 w-3.5" /> 关闭会话
        </button>
      </div>

      <div
        ref={scrollRef}
        className="max-h-72 overflow-auto whitespace-pre-wrap p-3 font-mono text-xs leading-relaxed text-slate-200"
      >
        {output || '等待命令…（初始命令输出见上方报告）'}
      </div>

      <div className="flex items-center gap-2 border-t border-slate-800 p-2">
        <span className="font-mono text-xs font-bold text-emerald-400">$</span>
        <input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKeyDown}
          disabled={closed}
          autoFocus
          placeholder={closed ? '会话已关闭' : '输入命令后回车执行（↑/↓ 历史）'}
          className="flex-1 bg-transparent font-mono text-xs text-slate-100 outline-none placeholder:text-slate-600"
        />
        {busy && <Loader2 className="h-3.5 w-3.5 animate-spin text-indigo-400" />}
        <button
          type="button"
          onClick={() => {
            sendCommand(input);
            setInput('');
          }}
          disabled={busy || closed}
          className="rounded-md bg-indigo-500/20 px-2.5 py-1 text-xs font-medium text-indigo-300 hover:bg-indigo-500/30 disabled:cursor-not-allowed disabled:opacity-50"
        >
          发送
        </button>
      </div>
    </div>
  );
}
