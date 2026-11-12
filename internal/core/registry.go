// 工具注册表：负责工具的注册与查找。Go 版采用各工具包 init() 静态注册。
package core

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 线程安全的工具注册表（对应 Python ToolRegistry）。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register 注册工具，重复 ID 直接 panic（开发期尽早暴露）。
func (r *Registry) Register(t Tool) Tool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.tools[t.ID()]; dup {
		panic(fmt.Sprintf("工具 ID 重复注册: %s", t.ID()))
	}
	r.tools[t.ID()] = t
	return t
}

// Get 按 tool_id 查找。
func (r *Registry) Get(toolID string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[toolID]
	return t, ok
}

// All 返回全部工具（可按分类过滤），按 tool_id 排序（与 Python 一致）。
func (r *Registry) All(category *Category) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		if category != nil && t.Category() != *category {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Categories 返回去重后的分类列表，按字典序（与 Python 一致）。
func (r *Registry) Categories() []Category {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[Category]bool{}
	for _, t := range r.tools {
		seen[t.Category()] = true
	}
	out := make([]Category, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ToolCount 注册工具总数（/health 对齐）。
func (r *Registry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// DefaultRegistry 全局注册表：工具包通过 init() 注册到这里。
var DefaultRegistry = NewRegistry()
