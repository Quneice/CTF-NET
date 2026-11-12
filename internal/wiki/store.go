// 红队 Wiki 存储：SQLite 单表保存常用参考资料条目。
//
// 与 internal/projects 同类架构，但结构更简单：无需 project 层级，
// 每条条目仅含「分类 + 标题 + markdown 内容」，供红队 Wiki 工作区浏览/检索/增删改。
// 首次打开时若表为空，由 main.go 调用 Seed 灌入内置速查数据。
package wiki

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

var ErrEntryNotFound = errors.New("条目不存在")

// Entry 一条 wiki 条目。
type Entry struct {
	ID        string  `json:"id"`
	Category  string  `json:"category"`
	Title     string  `json:"title"`
	Content   string  `json:"content"` // markdown 文本
	CreatedAt float64 `json:"created_at"`
	UpdatedAt float64 `json:"updated_at"`
}

// SeedEntry 内置速查条目（Seed 用）。
type SeedEntry struct {
	Category string
	Title    string
	Content  string
}

// Store wiki 存储（写操作互斥串行化）。
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

// Open 打开（必要时创建）wiki 数据库并建表。
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // 单写者，避免 SQLITE_BUSY
	for _, stmt := range []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS wiki_entries(
			id TEXT PRIMARY KEY,
			category TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT DEFAULT '',
			created_at REAL NOT NULL,
			updated_at REAL NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_category ON wiki_entries(category)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("初始化 wiki 库失败: %w", err)
		}
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// Count 条目总数（用于判断是否需要灌入内置数据）。
func (s *Store) Count() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM wiki_entries`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// Seed 批量插入内置条目（仅当表为空时由上层调用）。
func (s *Store) Seed(entries []SeedEntry) error {
	if len(entries) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, e := range entries {
		t := now()
		if _, err := tx.Exec(
			`INSERT INTO wiki_entries(id, category, title, content, created_at, updated_at)
			 VALUES(?,?,?,?,?,?)`,
			newID(), e.Category, e.Title, e.Content, t, t); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func scanEntry(row interface{ Scan(...any) error }) (*Entry, error) {
	var e Entry
	if err := row.Scan(&e.ID, &e.Category, &e.Title, &e.Content, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEntryNotFound
		}
		return nil, err
	}
	return &e, nil
}

// List 列出条目；category 为空串 = 全部，按更新时间倒序。
func (s *Store) List(category string) ([]*Entry, error) {
	var rows *sql.Rows
	var err error
	if category == "" {
		rows, err = s.db.Query(
			`SELECT id, category, title, content, created_at, updated_at
			 FROM wiki_entries ORDER BY updated_at DESC`)
	} else {
		rows, err = s.db.Query(
			`SELECT id, category, title, content, created_at, updated_at
			 FROM wiki_entries WHERE category=? ORDER BY updated_at DESC`, category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Category, &e.Title, &e.Content, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// Create 新增条目。category/title 必填。
func (s *Store) Create(category, title, content string) (*Entry, error) {
	if category == "" {
		return nil, errors.New("分类不能为空")
	}
	if title == "" {
		return nil, errors.New("标题不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := &Entry{ID: newID(), Category: category, Title: title, Content: content, CreatedAt: now()}
	e.UpdatedAt = e.CreatedAt
	if _, err := s.db.Exec(
		`INSERT INTO wiki_entries(id, category, title, content, created_at, updated_at)
		 VALUES(?,?,?,?,?,?)`,
		e.ID, e.Category, e.Title, e.Content, e.CreatedAt, e.UpdatedAt); err != nil {
		return nil, err
	}
	return e, nil
}

// Update 更新条目标题/内容（分类不可改）。
func (s *Store) Update(id, title, content string) (*Entry, error) {
	if title == "" {
		return nil, errors.New("标题不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE wiki_entries SET title=?, content=?, updated_at=? WHERE id=?`,
		title, content, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrEntryNotFound
	}
	return scanEntry(s.db.QueryRow(
		`SELECT id, category, title, content, created_at, updated_at FROM wiki_entries WHERE id=?`, id))
}

// Delete 删除条目。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM wiki_entries WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrEntryNotFound
	}
	return nil
}
