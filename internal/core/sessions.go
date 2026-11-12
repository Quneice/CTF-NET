// 交互式 shell 会话管理（内存版）。对应 legacy/app/core/sessions.py：
// 工具（如 web.jndi）在反弹 shell 建立后注册「下发命令 / 清理环境」回调，
// 前端经 SessionExec 持续交互；mutex 串行化等价于 Python 的 asyncio.Lock。
package core

import (
	"context"
	"sync"
	"time"
)

// ExecCallback 下发命令回调：(command, timeout) -> (output, timed_out, closed)。
type ExecCallback func(ctx context.Context, command string, timeout time.Duration) (string, bool, bool)

// CloseCallback 清理回调。
type CloseCallback func(ctx context.Context) error

// ShellSession 单个交互会话。
type ShellSession struct {
	ID        string
	ExecCB    ExecCallback
	CloseCB   CloseCallback
	CreatedAt float64
	mu        sync.Mutex // 串行化 exec/close
}

// ExecCommand 下发命令并返回 (增量输出, 是否超时, 会话是否已断开)。
// 串行化保证 fifo 写入与偏移读取不交错。
func (s *ShellSession) ExecCommand(ctx context.Context, command string, timeout time.Duration) (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ExecCB(ctx, command, timeout)
}

// Close 关闭会话（远程清理）。
func (s *ShellSession) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.CloseCB(ctx)
}

// SessionRegistry 会话注册表（对应 Python SessionRegistry）。
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*ShellSession
}

// NewSessionRegistry 创建注册表。
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: map[string]*ShellSession{}}
}

// Create 注册会话，id 为 12 位 hex（与 uuid4().hex[:12] 对齐）。
func (r *SessionRegistry) Create(execCB ExecCallback, closeCB CloseCallback) *ShellSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &ShellSession{
		ID:        randomHex(6),
		ExecCB:    execCB,
		CloseCB:   closeCB,
		CreatedAt: float64(time.Now().UnixNano()) / 1e9,
	}
	r.sessions[s.ID] = s
	return s
}

// Get 按 sid 查找。
func (r *SessionRegistry) Get(sid string) (*ShellSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[sid]
	return s, ok
}

// Remove 移除会话（不触发清理）。
func (r *SessionRegistry) Remove(sid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sid)
}
