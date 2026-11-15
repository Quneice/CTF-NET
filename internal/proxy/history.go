package proxy

import "errors"

// 历史记录存储(内存环形,新→旧;持久化留待 P2 SQLite)。

var errRecordNotFound = errors.New("记录不存在")

// Status 代理运行状态。
type Status struct {
	Running   bool   `json:"running"`
	Port      int    `json:"port"`
	Intercept bool   `json:"intercept"`
	MITM      bool   `json:"mitm"`    // HTTPS 解密开关
	CAFile    string `json:"ca_file"` // CA 证书路径(导出/安装用)
}

// Summary 历史列表行(不含原始报文;拦截事件亦用此载荷,前端按 id 拉全量)。
type Summary struct {
	ID          string `json:"id"`
	Timestamp   int64  `json:"timestamp"` // Unix 毫秒
	Method      string `json:"method"`
	Host        string `json:"host"`
	URL         string `json:"url"`
	Status      int    `json:"status"` // 0 = 未完成(被丢弃/转发失败)
	DurationMs  int64  `json:"duration_ms"`
	Intercepted bool   `json:"intercepted"`
	Note        string `json:"note"`
}

// Record 一次代理会话的完整记录(含原始请求/响应报文)。
type Record struct {
	Summary
	RawRequest  string `json:"raw_request"`
	RawResponse string `json:"raw_response"`
}

// HistoryPage 历史分页(新→旧)。
type HistoryPage struct {
	Total int       `json:"total"`
	Items []Summary `json:"items"`
}

// add 追加到历史头部并封顶(调用方需自行持锁的场景由本方法统一加锁)。
func (s *Server) add(rec *Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append([]*Record{rec}, s.history...)
	if len(s.history) > MaxHistory {
		s.history = s.history[:MaxHistory]
	}
}

// History 分页返回历史(offset 从新到旧;limit<=0 表示取全部)。
func (s *Server) History(offset, limit int) HistoryPage {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := len(s.history)
	if offset >= total {
		return HistoryPage{Total: total, Items: []Summary{}}
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	items := make([]Summary, 0, end-offset)
	for _, r := range s.history[offset:end] {
		items = append(items, r.Summary)
	}
	return HistoryPage{Total: total, Items: items}
}

// Record 按 id 取全量记录;拦截挂起中(事件已发、决策未做、尚未入历史)的
// 记录由 pendingRecs 快照解析,保证前端事件处理器按 id 拉取总能命中。
func (s *Server) Record(id string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.history {
		if r.ID == id {
			return *r, nil
		}
	}
	if r, ok := s.pendingRecs[id]; ok {
		return *r, nil
	}
	return Record{}, errRecordNotFound
}

// Clear 清空历史。
func (s *Server) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = make([]*Record, 0, MaxHistory)
}
