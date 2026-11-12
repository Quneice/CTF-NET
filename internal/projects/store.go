// 渗透测试项目管理：SQLite 持久化存储。
//
// 设计：一张 projects 主表 + 一张通用 entries 条目表。
// 9 类条目（资产/攻击面/端口/指纹/PoC/凭据/敏感信息/笔记/附件）字段各异但操作同构，
// 统一用 kind 区分、content 存 JSON 文本承载各类结构化字段，避免 9 套重复 CRUD。
package projects

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // database/sql 驱动，纯 Go 无 cgo
)

// 条目类别（前端 mock/projectKinds.ts 与此一一对应）。
const (
	KindAsset       = "asset"       // 资产/资源
	KindAttack      = "attack"      // 攻击面
	KindPort        = "port"        // 端口扫描
	KindFingerprint = "fingerprint" // 指纹识别
	KindPoC         = "poc"         // PoC/EXP
	KindCredential  = "credential"  // 凭据
	KindSensitive   = "sensitive"   // 敏感信息
	KindNote        = "note"        // 笔记
	KindAttachment  = "attachment"  // 附件
)

// 项目状态。
const (
	StatusActive = "active"   // 进行中
	StatusDone   = "done"     // 已完成
	StatusPaused = "paused"   // 暂停
)

var (
	ErrProjectNotFound = errors.New("项目不存在")
	ErrEntryNotFound   = errors.New("条目不存在")
)

// Project 渗透测试项目。
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Target      string `json:"target"`      // 目标（域名/IP/单位名）
	Scope       string `json:"scope"`       // 授权范围说明
	Status      string `json:"status"`      // active / done / paused
	Description string `json:"description"` // 项目描述
	CreatedAt   float64 `json:"created_at"`
	UpdatedAt   float64 `json:"updated_at"`
	// 各 kind 条目计数（ListProjects/GetProject 时填充）
	EntryCounts map[string]int `json:"entry_counts,omitempty"`
}

// Entry 项目内的通用条目；Content 为 JSON 文本，字段由 kind 决定。
type Entry struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	Kind      string  `json:"kind"`
	Title     string  `json:"title"`
	Content   string  `json:"content"` // JSON 文本
	Tags      string  `json:"tags"`    // 逗号分隔
	CreatedAt float64 `json:"created_at"`
	UpdatedAt float64 `json:"updated_at"`
}

// ProjectInput 新建/更新项目的入参。
type ProjectInput struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
	Scope       string `json:"scope"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// Store 项目存储（SQLite 单文件；写操作互斥串行化）。
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func now() float64 {
	return float64(time.Now().UnixMilli()) / 1000.0
}

// Open 打开（必要时创建）项目数据库并建表。
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// 单写者模式：避免 database/sql 连接池并发写同一文件导致 SQLITE_BUSY
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS projects(
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			target TEXT DEFAULT '',
			scope TEXT DEFAULT '',
			status TEXT DEFAULT 'active',
			description TEXT DEFAULT '',
			created_at REAL NOT NULL,
			updated_at REAL NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS entries(
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			title TEXT DEFAULT '',
			content TEXT DEFAULT '{}',
			tags TEXT DEFAULT '',
			created_at REAL NOT NULL,
			updated_at REAL NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_project_kind ON entries(project_id, kind)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("初始化项目库失败: %w", err)
		}
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// —— 项目 CRUD ——

func (s *Store) scanProject(row *sql.Row) (*Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.Name, &p.Target, &p.Scope, &p.Status, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// entryCounts 统计项目各 kind 条目数。
func (s *Store) entryCounts(projectID string) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT kind, COUNT(*) FROM entries WHERE project_id=? GROUP BY kind`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return nil, err
		}
		counts[kind] = n
	}
	return counts, rows.Err()
}

// ListProjects 全部项目（按更新时间倒序），附带各项目条目计数。
func (s *Store) ListProjects() ([]*Project, error) {
	rows, err := s.db.Query(
		`SELECT id, name, target, scope, status, description, created_at, updated_at
		 FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Target, &p.Scope, &p.Status, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, p := range out {
		counts, err := s.entryCounts(p.ID)
		if err != nil {
			return nil, err
		}
		p.EntryCounts = counts
	}
	return out, nil
}

// GetProject 单个项目详情（含条目计数）。
func (s *Store) GetProject(id string) (*Project, error) {
	p, err := s.scanProject(s.db.QueryRow(
		`SELECT id, name, target, scope, status, description, created_at, updated_at
		 FROM projects WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	counts, err := s.entryCounts(id)
	if err != nil {
		return nil, err
	}
	p.EntryCounts = counts
	return p, nil
}

func normalizeStatus(st string) string {
	switch st {
	case StatusDone, StatusPaused:
		return st
	default:
		return StatusActive
	}
}

// CreateProject 新建项目。name 必填。
func (s *Store) CreateProject(in ProjectInput) (*Project, error) {
	if in.Name == "" {
		return nil, errors.New("项目名称不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := &Project{
		ID:          newID(),
		Name:        in.Name,
		Target:      in.Target,
		Scope:       in.Scope,
		Status:      normalizeStatus(in.Status),
		Description: in.Description,
		CreatedAt:   now(),
	}
	p.UpdatedAt = p.CreatedAt
	_, err := s.db.Exec(
		`INSERT INTO projects(id, name, target, scope, status, description, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Target, p.Scope, p.Status, p.Description, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.EntryCounts = map[string]int{}
	return p, nil
}

// UpdateProject 更新项目基础信息。
func (s *Store) UpdateProject(id string, in ProjectInput) (*Project, error) {
	if in.Name == "" {
		return nil, errors.New("项目名称不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(
		`UPDATE projects SET name=?, target=?, scope=?, status=?, description=?, updated_at=? WHERE id=?`,
		in.Name, in.Target, in.Scope, normalizeStatus(in.Status), in.Description, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrProjectNotFound
	}
	return s.GetProject(id)
}

// DeleteProject 删除项目（外键级联删除其全部条目）。
func (s *Store) DeleteProject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM projects WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// touch 项目更新时间（条目变动时联动）。
func (s *Store) touch(projectID string) {
	_, _ = s.db.Exec(`UPDATE projects SET updated_at=? WHERE id=?`, now(), projectID)
}

// —— 条目 CRUD ——

// ListEntries 项目下条目（kind 为空串 = 全部类别），按更新时间倒序。
func (s *Store) ListEntries(projectID, kind string) ([]*Entry, error) {
	var rows *sql.Rows
	var err error
	if kind == "" {
		rows, err = s.db.Query(
			`SELECT id, project_id, kind, title, content, tags, created_at, updated_at
			 FROM entries WHERE project_id=? ORDER BY updated_at DESC`, projectID)
	} else {
		rows, err = s.db.Query(
			`SELECT id, project_id, kind, title, content, tags, created_at, updated_at
			 FROM entries WHERE project_id=? AND kind=? ORDER BY updated_at DESC`, projectID, kind)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Kind, &e.Title, &e.Content, &e.Tags, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// CreateEntry 新增条目。项目不存在时返回 ErrProjectNotFound。
func (s *Store) CreateEntry(projectID, kind, title, content, tags string) (*Entry, error) {
	if _, err := s.GetProject(projectID); err != nil {
		return nil, err
	}
	if content == "" {
		content = "{}"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := &Entry{
		ID:        newID(),
		ProjectID: projectID,
		Kind:      kind,
		Title:     title,
		Content:   content,
		Tags:      tags,
		CreatedAt: now(),
	}
	e.UpdatedAt = e.CreatedAt
	_, err := s.db.Exec(
		`INSERT INTO entries(id, project_id, kind, title, content, tags, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		e.ID, e.ProjectID, e.Kind, e.Title, e.Content, e.Tags, e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.touch(projectID)
	return e, nil
}

// UpdateEntry 更新条目（标题/内容/标签；kind 与归属不可改）。
func (s *Store) UpdateEntry(id, title, content, tags string) (*Entry, error) {
	if content == "" {
		content = "{}"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(
		`UPDATE entries SET title=?, content=?, tags=?, updated_at=? WHERE id=?`,
		title, content, tags, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrEntryNotFound
	}
	var e Entry
	var pid string
	err = s.db.QueryRow(
		`SELECT id, project_id, kind, title, content, tags, created_at, updated_at
		 FROM entries WHERE id=?`, id).
		Scan(&e.ID, &pid, &e.Kind, &e.Title, &e.Content, &e.Tags, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	e.ProjectID = pid
	s.touch(pid)
	return &e, nil
}

// DeleteEntry 删除条目。
func (s *Store) DeleteEntry(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pid string
	err := s.db.QueryRow(`SELECT project_id FROM entries WHERE id=?`, id).Scan(&pid)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEntryNotFound
	}
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM entries WHERE id=?`, id); err != nil {
		return err
	}
	s.touch(pid)
	return nil
}
