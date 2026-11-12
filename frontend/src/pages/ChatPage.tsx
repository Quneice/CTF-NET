import { useCallback, useEffect, useRef, useState } from 'react';
import { Bot, Loader2, MessageSquareText, Plus, Send, TerminalSquare, Trash2 } from 'lucide-react';
import { NavLink } from 'react-router-dom';

import {
  deleteChatSession,
  getChatSession,
  isWails,
  listChatSessions,
  onChatEvent,
  sendChatMessage,
  type ChatMsgDict,
  type ChatSegDict,
  type ChatSessionDict,
  type ChatSessionSummaryDict,
} from '@/lib/backend';

const TURN_TIMEOUT_MS = 26 * 60 * 1000; // 与后端 25 分钟回合兜底对齐，留 1 分钟余量

/**
 * AI 对话渗透页（/ai-chat）：实时流式对话。
 * 发送 = SendChatMessage 绑定，立即返回 session_id；助手回复经 chat:* 事件流
 * 逐 token 推送（打字机效果），按模型轮次分段渲染——一轮 = 思考 + 叙述气泡 +
 * 本轮工具动作（chat:tool / chat:tool_result），历史会话同样按分段逐轮展示。
 * 回复进行中可继续输入并发送（打断）：前端固化已生成分段并标「已打断」、
 * 旧回合事件按 turnRef 序号作废；后端取消旧回合（ctx）后以新消息接管，
 * 模型上下文保留两条用户消息（transcript 续聊）。
 * 不走任务队列、无 job 轮询。
 */

/** 助手回复的轮次分段：一轮 = 思考 + 叙述气泡 + 本轮工具动作（后端 ChatSeg 的镜像）。
 *  note 为前端专用分段（打断标记），不落盘、normSeg 也不会产生。 */
type Seg =
  | { kind: 'text'; text: string }
  | { kind: 'thinking'; text: string }
  | { kind: 'tool'; name: string; summary: string }
  | { kind: 'tool_result'; name: string; summary: string; isError: boolean }
  | { kind: 'note'; text: string };

/** 后端 ChatSegDict → Seg（历史会话载入时归一化）。 */
function normSeg(g: ChatSegDict): Seg {
  switch (g.kind) {
    case 'tool':
      return { kind: 'tool', name: g.name ?? '', summary: g.summary ?? '' };
    case 'tool_result':
      return { kind: 'tool_result', name: g.name ?? '', summary: g.summary ?? '', isError: !!g.is_error };
    case 'thinking':
      return { kind: 'thinking', text: g.text ?? '' };
    default:
      return { kind: 'text', text: g.text ?? '' };
  }
}

/** 各轮叙述分段拼成消息正文（会话列表等纯文本场景的回退展示）。 */
function segsToText(segs: Seg[]): string {
  const parts = segs.filter((s) => s.kind === 'text').map((s) => s.text);
  const joined = parts.join('\n\n');
  return joined || '(空回复)';
}

/** 单个分段视图：叙述气泡 / 思考块 / 工具动作行（流式与历史共用）。 */
function SegView({ seg, liveCursor }: { seg: Seg; liveCursor: boolean }) {
  switch (seg.kind) {
    case 'text':
      return (
        <div className="whitespace-pre-wrap rounded-lg border border-slate-200 bg-white px-3.5 py-2.5 text-sm leading-relaxed text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200">
          {seg.text}
          {liveCursor && (
            <span className="ml-0.5 inline-block h-4 w-1.5 animate-pulse bg-indigo-400 align-text-bottom" />
          )}
        </div>
      );
    case 'thinking':
      return (
        <div className="max-h-40 overflow-y-auto rounded-md border border-dashed border-slate-200 bg-slate-50 px-2.5 py-1.5 text-xs italic leading-relaxed text-slate-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-500">
          💭 思考中：{seg.text}
        </div>
      );
    case 'tool':
      return (
        <div className="rounded-md border border-dashed border-slate-200 bg-slate-50 px-2.5 py-1.5 font-mono text-[11px] leading-relaxed text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
          <span className="font-semibold text-indigo-500 dark:text-indigo-400">🛠 {seg.name}</span>
          {seg.summary ? <span className="break-all"> {seg.summary}</span> : null}
        </div>
      );
    case 'tool_result':
      return (
        <div
          className={`rounded-md px-2.5 py-1 font-mono text-[11px] leading-relaxed ${
            seg.isError ? 'text-red-500 dark:text-red-400' : 'text-slate-400 dark:text-slate-500'
          }`}
        >
          ↳ {(seg.summary || '(无输出)').slice(0, 160)}
        </div>
      );
    case 'note':
      return (
        <div className="rounded-md px-2.5 py-1 text-xs italic text-amber-500 dark:text-amber-400">
          ✋ {seg.text}
        </div>
      );
  }
}

/** 消息状态：会话历史 + 客户端侧分段视图（segs，后端 segments 的归一化）。 */
type Msg = ChatMsgDict & { segs?: Seg[] };

export default function ChatPage() {
  const [sessions, setSessions] = useState<ChatSessionSummaryDict[]>([]);
  const [sessionId, setSessionId] = useState<string>('');
  const [messages, setMessages] = useState<Msg[]>([]);
  const [taskIds, setTaskIds] = useState<string[]>([]);
  const [title, setTitle] = useState('');
  const [input, setInput] = useState('');
  const [enableExec, setEnableExec] = useState(true);
  const [sending, setSending] = useState(false);
  // 当前回合的流式分段（segsRef 为源，liveSegs 仅驱动渲染）
  const [liveSegs, setLiveSegs] = useState<Seg[]>([]);
  const segsRef = useRef<Seg[]>([]);
  const [error, setError] = useState('');
  const [loadingList, setLoadingList] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const activeSidRef = useRef<string>('');
  // 回合序号：打断时 +1，旧回合残留事件（同 session_id）据此作废
  const turnRef = useRef(0);
  // 当前回合的收尾函数（打断时由新 send 调用，固化已生成分段并解绑旧事件）
  const finalizeRef = useRef<((reply: string | null, errMsg: string | null, note: string | null) => void) | null>(
    null,
  );

  const refreshSessions = useCallback(async () => {
    if (!isWails()) {
      setLoadingList(false);
      return;
    }
    try {
      setSessions(await listChatSessions());
    } catch {
      /* 列表刷新失败不打断聊天 */
    } finally {
      setLoadingList(false);
    }
  }, []);

  useEffect(() => {
    document.title = 'CTF-NET — AI 对话渗透';
    void refreshSessions();
    // 预绑定 chat:* 事件通道：防「发送返回 → 注册订阅」窗口内丢事件（no-op 常驻）
    for (const k of ['delta', 'thinking', 'tool', 'tool_result', 'done', 'error'] as const) onChatEvent(k, () => {});
  }, [refreshSessions]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, liveSegs]);

  const openSession = useCallback(
    async (id: string) => {
      if (sending) return;
      setError('');
      const s: ChatSessionDict | null = await getChatSession(id);
      if (!s) {
        setError(`会话 ${id} 不存在或已被删除`);
        return;
      }
      setSessionId(id);
      activeSidRef.current = id;
      setTitle(s.title ?? '');
      setMessages((s.messages ?? []).map((m) => ({ ...m, segs: (m.segments ?? []).map(normSeg) })));
      setTaskIds(s.task_ids ?? []);
    },
    [sending],
  );

  const startNew = useCallback(() => {
    setSessionId('');
    activeSidRef.current = '';
    setTitle('');
    setMessages([]);
    setTaskIds([]);
    setError('');
    segsRef.current = [];
    setLiveSegs([]);
  }, []);

  const removeSession = useCallback(
    async (id: string) => {
      if (sending) return;
      try {
        await deleteChatSession(id);
        if (sessionId === id) startNew();
        void refreshSessions();
      } catch (e) {
        setError(`删除失败: ${e instanceof Error ? e.message : String(e)}`);
      }
    },
    [sessionId, sending, startNew, refreshSessions],
  );

  const send = async () => {
    const text = input.trim();
    if (!text) return;
    if (!isWails()) {
      setError('AI 对话需要桌面运行时（wails）');
      return;
    }
    if (sending) {
      // 回复中再次发送 = 打断：固化已生成的分段并标记，旧回合残留事件立即作废
      const prev = finalizeRef.current;
      if (prev) prev(null, null, '已打断，以下按补充说明继续');
      turnRef.current += 1;
    }
    setSending(true);
    setError('');
    segsRef.current = [];
    setLiveSegs([]);
    setInput('');

    // 乐观追加：用户消息 + 空的助手流式占位
    const now = new Date().toLocaleString('zh-CN', { hour12: false });
    const asstIdx = messages.length + 1;
    setMessages((m) => [
      ...m,
      { role: 'user', content: text, ts: now },
      { role: 'assistant', content: '', ts: now },
    ]);

    let unsubs: (() => void)[] = [];
    const timer = window.setTimeout(
      () => finalize(null, `⚠️ 回复超时（${TURN_TIMEOUT_MS / 60000} 分钟无终态事件）`, null),
      TURN_TIMEOUT_MS,
    );
    // 回合收尾：分段（各轮思考/叙述/工具动作 + 终答/打断标记）写进占位消息并转历史渲染
    const finalize = (reply: string | null, errMsg: string | null, note: string | null) => {
      unsubs.forEach((u) => u());
      unsubs = [];
      window.clearTimeout(timer);
      let segs: Seg[] = segsRef.current;
      if (note) segs = [...segs, { kind: 'note', text: note }];
      else if (reply) segs = [...segs, { kind: 'text', text: reply }];
      setMessages((m) => {
        if (asstIdx >= m.length) return m;
        const cp = [...m];
        cp[asstIdx] = { ...cp[asstIdx], content: segsToText(segs), segs };
        return cp;
      });
      segsRef.current = [];
      setLiveSegs([]);
      setSending(false);
      if (errMsg) setError(errMsg);
      finalizeRef.current = null;
    };
    finalizeRef.current = finalize;
    const updateSegs = (fn: (segs: Seg[]) => Seg[]) => {
      segsRef.current = fn(segsRef.current);
      setLiveSegs(segsRef.current);
    };

    try {
      const myTurn = turnRef.current; // 先于 await 捕获：await 期间被打断则本回合事件全部作废
      const { session_id: sid } = await sendChatMessage(sessionId || '', text, enableExec);
      setSessionId(sid);
      activeSidRef.current = sid;
      // 事件流：仅处理本回合会话 + 本回合序号（打断后的旧回合残留事件静默丢弃）
      const mine = (ev: { session_id?: string }) =>
        ev.session_id === sid && activeSidRef.current === sid && turnRef.current === myTurn;
      unsubs = [
        // 正文增量：追加到当前叙述分段；工具动作之后的新增量 = 新一轮，开新分段
        onChatEvent('delta', (ev) => {
          if (!mine(ev) || !ev.content) return;
          const content = ev.content;
          updateSegs((segs) => {
            const last = segs[segs.length - 1];
            if (last && last.kind === 'text') {
              const cp = [...segs];
              cp[cp.length - 1] = { ...last, text: last.text + content };
              return cp;
            }
            return [...segs, { kind: 'text', text: content }];
          });
        }),
        onChatEvent('thinking', (ev) => {
          if (!mine(ev) || !ev.content) return;
          const content = ev.content;
          updateSegs((segs) => [...segs, { kind: 'thinking', text: content }]);
        }),
        onChatEvent('tool', (ev) => {
          if (!mine(ev) || !ev.name) return;
          const name = ev.name;
          updateSegs((segs) => [...segs, { kind: 'tool', name, summary: ev.summary ?? '' }]);
        }),
        onChatEvent('tool_result', (ev) => {
          if (!mine(ev) || !ev.name) return;
          const name = ev.name;
          updateSegs((segs) => [
            ...segs,
            { kind: 'tool_result', name, summary: ev.summary ?? '', isError: !!ev.is_error },
          ]);
        }),
        // 终答只走 chat:done（ARTEX 语义：过程叙述由 delta 流推送，终答由 result 携带）
        onChatEvent('done', (ev) => {
          if (!mine(ev)) return;
          setTitle(ev.title ?? title);
          if (ev.task_ids && ev.task_ids.length > 0) setTaskIds(ev.task_ids);
          finalize(ev.reply ?? '', null, null);
          void refreshSessions();
        }),
        onChatEvent('error', (ev) => {
          if (!mine(ev)) return;
          finalize(null, `⚠️ ${ev.message ?? '对话失败'}`, null);
        }),
      ];
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      finalize(null, `⚠️ ${msg}`, null);
    }
  };

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* 页头 */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-6 py-4 dark:border-slate-800">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <MessageSquareText className="h-6 w-6 text-indigo-500" />
            AI 对话渗透
          </h1>
          <p className="mt-1 text-slate-500 dark:text-slate-400">
            实时对话：助手回复逐字流式输出，工具动作实时可见；可直接侦察/验证、发起后台自动化任务并追问进展
          </p>
        </div>
        <button
          type="button"
          onClick={startNew}
          disabled={sending}
          className="flex items-center gap-1.5 rounded-md bg-indigo-500 px-3 py-2 text-sm text-white hover:bg-indigo-600 disabled:opacity-50"
        >
          <Plus className="h-4 w-4" />
          新会话
        </button>
      </div>

      <div className="flex min-h-0 flex-1">
        {/* 会话列表 */}
        <aside className="hidden w-60 shrink-0 flex-col border-r border-slate-200 dark:border-slate-800 md:flex">
          <div className="flex items-center justify-between px-3 pb-1 pt-3 text-xs text-slate-400">
            <span>会话历史</span>
            <button
              type="button"
              onClick={() => void refreshSessions()}
              title="刷新"
              className="rounded p-1 hover:bg-slate-100 dark:hover:bg-slate-800"
            >
              ↻
            </button>
          </div>
          <div className="flex-1 overflow-y-auto px-2 pb-2">
            {loadingList ? (
              <p className="px-1 py-2 text-xs text-slate-400">加载中…</p>
            ) : sessions.length === 0 ? (
              <p className="px-1 py-2 text-xs text-slate-400">暂无会话，发送第一条消息开始</p>
            ) : (
              <ul className="space-y-1">
                {sessions.map((s) => (
                  <li key={s.id} className="group relative">
                    <button
                      type="button"
                      onClick={() => void openSession(s.id ?? '')}
                      className={`block w-full rounded-md px-2 py-1.5 text-left text-xs ${
                        sessionId === s.id
                          ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400'
                          : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
                      }`}
                    >
                      <div className="truncate font-medium">{s.title || '未命名会话'}</div>
                      <div className="mt-0.5 text-slate-400">
                        {s.updated_at?.slice(5, 16) ?? ''} · {s.msg_count ?? 0} 条
                      </div>
                    </button>
                    <button
                      type="button"
                      onClick={() => void removeSession(s.id ?? '')}
                      title="删除会话"
                      className="absolute right-1.5 top-1.5 hidden rounded p-0.5 text-slate-400 hover:text-red-500 group-hover:block"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="border-t border-slate-200 p-3 text-[11px] leading-relaxed text-slate-400 dark:border-slate-800">
            自动化任务版：<NavLink to="/t/ai.pentest.task" className="text-indigo-500 hover:underline">AI 渗透任务</NavLink>
          </div>
        </aside>

        {/* 消息区 */}
        <main className="flex min-w-0 flex-1 flex-col">
          {title && (
            <div className="border-b border-slate-200 px-6 py-2 text-xs text-slate-400 dark:border-slate-800">
              会话：{title}
            </div>
          )}
          {taskIds.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5 border-b border-slate-200 px-6 py-2 dark:border-slate-800">
              <span className="text-xs text-slate-400">后台任务：</span>
              {taskIds.map((t) => (
                <span
                  key={t}
                  className="rounded-full bg-amber-50 px-2 py-0.5 text-xs text-amber-600 dark:bg-amber-500/10 dark:text-amber-400"
                >
                  {t}
                </span>
              ))}
            </div>
          )}

          <div ref={scrollRef} className="flex-1 overflow-y-auto px-6 py-4">
            {messages.length === 0 && !sending ? (
              <div className="flex h-full flex-col items-center justify-center text-center">
                <Bot className="mb-3 h-10 w-10 text-indigo-300" />
                <p className="text-sm font-medium text-slate-500 dark:text-slate-400">开始一段实时对话式渗透测试</p>
                <p className="mt-1 max-w-md text-xs leading-relaxed text-slate-400">
                  例如：「帮我侦察 http://192.168.1.10:8080，先看响应头与后台路径」；
                  助手回复与工具动作都会实时流式展示，可随时追问进展。
                </p>
              </div>
            ) : (
              <div className="mx-auto max-w-3xl space-y-4">
                {messages.map((m, i) => {
                  if (m.role === 'user') {
                    return (
                      <div key={i} className="flex justify-end">
                        <div className="min-w-0 max-w-[85%]">
                          <div className="whitespace-pre-wrap rounded-lg bg-indigo-500 px-3.5 py-2.5 text-sm leading-relaxed text-white">
                            {m.content}
                          </div>
                          {m.ts && <div className="mt-1 text-right text-[10px] text-slate-400">{m.ts}</div>}
                        </div>
                      </div>
                    );
                  }
                  // 流式回合：按模型轮次分段实时渲染（一轮 = 思考 + 叙述 + 本轮工具动作）
                  if (sending && i === messages.length - 1) {
                    return (
                      <div key={i} className="flex justify-start">
                        <div className="min-w-0 flex-1 space-y-2">
                          {liveSegs.length === 0 && (
                            <div className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3.5 py-2.5 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400">
                              <Loader2 className="h-4 w-4 animate-spin text-indigo-500" />
                              AI 助手思考中…
                            </div>
                          )}
                          {liveSegs.map((seg, j) => (
                            <SegView
                              key={j}
                              seg={seg}
                              liveCursor={j === liveSegs.length - 1 && seg.kind === 'text'}
                            />
                          ))}
                        </div>
                      </div>
                    );
                  }
                  // 历史消息：有分段按分段逐轮渲染，无分段回退单气泡
                  const segs = m.segs;
                  return (
                    <div key={i} className="flex justify-start">
                      <div className="min-w-0 flex-1 space-y-2">
                        {segs && segs.length > 0 ? (
                          segs.map((seg, j) => <SegView key={j} seg={seg} liveCursor={false} />)
                        ) : (
                          <div className="whitespace-pre-wrap rounded-lg border border-slate-200 bg-white px-3.5 py-2.5 text-sm leading-relaxed text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200">
                            {m.content}
                          </div>
                        )}
                        {m.ts && <div className="mt-1 text-[10px] text-slate-400">{m.ts}</div>}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {error && <p className="mx-6 mb-1 text-sm text-red-500">{error}</p>}

          {/* 输入区 */}
          <div className="border-t border-slate-200 p-4 dark:border-slate-800">
            <div className="mx-auto max-w-3xl">
              <div className="flex items-end gap-2">
                <textarea
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault();
                      void send();
                    }
                  }}
                  rows={2}
                  placeholder="描述目标或追问进展…（Enter 发送；回复中发送会先打断当前回复）"
                  className="min-w-0 flex-1 resize-none rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-800"
                />
                <button
                  type="button"
                  onClick={() => void send()}
                  disabled={!input.trim()}
                  className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-indigo-500 text-white hover:bg-indigo-600 disabled:opacity-40"
                  title={sending ? '发送并打断当前回复' : '发送'}
                >
                  <Send className="h-4 w-4" />
                </button>
              </div>
              <label className="mt-1.5 flex cursor-pointer items-center gap-1.5 text-xs text-slate-400">
                <input
                  type="checkbox"
                  checked={enableExec}
                  onChange={(e) => setEnableExec(e.target.checked)}
                  disabled={sending}
                  className="h-3.5 w-3.5 accent-indigo-500"
                />
                <TerminalSquare className="h-3.5 w-3.5" />
                允许助手执行本地命令（仅授权目标）
              </label>
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}
