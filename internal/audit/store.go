// 代码审计工作区存储:SQLite(audit.db)管项目元数据与敏感函数规则库,
// 项目文件本体落盘 DataDir/audits/<id>/(src/ 原树 + .bak/ 首次编辑备份)。
// 提供:项目导入(上传 file_id → 组装目录树)、文件树/读/写、全文搜索(plain/regex)、
// 敏感函数扫描(规则库驱动)与规则 CRUD。与 wiki/projects 同类架构。
package audit

import (
	"crypto/rand"
	"bufio"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // database/sql 驱动，纯 Go 无 cgo
)

var ErrProjectNotFound = errors.New("审计项目不存在")

// ImportFile 导入源:上传文件的 file_id 与原始相对路径(webkitRelativePath,保留目录结构)。
type ImportFile struct {
	ID   string
	Name string
}

// Project 审计项目元数据。
type Project struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Files     int     `json:"files"`
	CreatedAt float64 `json:"created_at"`
}

// FileNode 文件树节点(Path 为相对 src/ 的 POSIX 路径)。
type FileNode struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

// SensitiveFunc 敏感函数/模式规则。
type SensitiveFunc struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Pattern  string `json:"pattern"` // Go regexp
	Category string `json:"category"` // 命令执行/SQL 注入/反序列化/…
	Risk     string `json:"risk"`     // high | medium | low
	Note     string `json:"note"`
	Enabled  bool   `json:"enabled"`
}

// SearchHit 全文搜索命中。
type SearchHit struct {
	Path   string `json:"path"`
	LineNo int    `json:"line_no"`
	Line   string `json:"line"`
}

// SensHit 敏感函数扫描命中。
type SensHit struct {
	Path     string `json:"path"`
	LineNo   int    `json:"line_no"`
	Line     string `json:"line"`
	Match    string `json:"match"`
	FuncID   string `json:"func_id"`
	Language string `json:"language"`
	Pattern  string `json:"pattern"`
	Category string `json:"category"`
	Risk     string `json:"risk"`
}

const (
	maxReadSize  = 2 << 20 // 单文件读取上限 2MB
	maxScanFiles = 5000    // 扫描/搜索文件数上限
	maxHits      = 2000    // 命中上限(搜索 500)
)

// Store 审计存储。
type Store struct {
	db   *sql.DB
	root string // DataDir/audits
	mu   sync.Mutex
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Open 打开/初始化审计库(dbPath: audit.db;rootDir: audits/),首开灌入敏感函数种子。
func Open(dbPath, rootDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS audit_projects(
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			files INTEGER NOT NULL DEFAULT 0,
			created_at REAL NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_sfuncs(
			id TEXT PRIMARY KEY,
			language TEXT NOT NULL,
			pattern TEXT NOT NULL,
			category TEXT NOT NULL,
			risk TEXT NOT NULL DEFAULT 'medium',
			note TEXT DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			sort INTEGER NOT NULL DEFAULT 0
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("初始化审计库失败: %w", err)
		}
	}
	s := &Store{db: db, root: rootDir}
	if err := s.seedFuncs(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ---------- 路径安全 ----------

// safeJoin 把相对路径并到 base 下,拒绝穿越与绝对路径。
func safeJoin(base, rel string) (string, error) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" || strings.HasPrefix(rel, "/") || path.Clean(rel) == ".." || strings.HasPrefix(path.Clean(rel), "../") {
		return "", fmt.Errorf("非法路径: %s", rel)
	}
	joined := filepath.Join(base, filepath.FromSlash(rel))
	absBase, _ := filepath.Abs(base)
	absJ, _ := filepath.Abs(joined)
	if !strings.HasPrefix(absJ, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	return joined, nil
}

func isTextFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 8192)
	buf, _ := r.Peek(8192)
	for _, b := range buf {
		if b == 0 {
			return false
		}
	}
	return true
}

// walkText 遍历项目内文本文件(跳过 .bak 与二进制),cb 提前返回 false 停止。
func (s *Store) walkText(projDir string, cb func(rel, abs string) bool) {
	_ = filepath.WalkDir(filepath.Join(projDir, "src"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(filepath.Join(projDir, "src"), p)
		rel = filepath.ToSlash(rel)
		if !isTextFile(p) {
			return nil
		}
		if !cb(rel, p) {
			return fs.SkipAll
		}
		return nil
	})
}

// ---------- 项目 ----------

// Import 组装上传文件为审计项目(resolve: file_id → 本地磁盘路径)。
func (s *Store) Import(name string, files []ImportFile, resolve func(id string) (string, bool)) (*Project, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("项目名不能为空")
	}
	if len(files) == 0 {
		return nil, errors.New("未选择任何文件")
	}
	if len(files) > maxScanFiles {
		return nil, fmt.Errorf("文件数 %d 超过上限 %d(请排除 node_modules/.git 等目录)", len(files), maxScanFiles)
	}
	id := newID()
	projDir := filepath.Join(s.root, id)
	if err := os.MkdirAll(filepath.Join(projDir, "src"), 0o755); err != nil {
		return nil, err
	}
	count := 0
	for _, f := range files {
		src, ok := resolve(f.ID)
		if !ok || src == "" {
			continue
		}
		rel := strings.TrimSpace(strings.ReplaceAll(f.Name, "\\", "/"))
		if rel == "" {
			continue
		}
		dst, err := safeJoin(filepath.Join(projDir, "src"), rel)
		if err != nil {
			continue // 跳过非法相对路径(如 ../)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("读取上传文件 %s 失败: %w", rel, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, err
		}
		count++
	}
	if count == 0 {
		_ = os.RemoveAll(projDir)
		return nil, errors.New("没有可导入的文件")
	}
	p := &Project{ID: id, Name: name, Files: count, CreatedAt: float64(time.Now().UnixMilli()) / 1000}
	_, err := s.db.Exec(`INSERT INTO audit_projects(id,name,files,created_at) VALUES(?,?,?,?)`,
		p.ID, p.Name, p.Files, p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) List() ([]*Project, error) {
	rows, err := s.db.Query(`SELECT id,name,files,created_at FROM audit_projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Project{} // 非 nil:JSON 序列化为 [] 而非 null(前端 .find 会崩)
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Files, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) projDir(id string) (string, error) {
	if _, err := s.db.Exec(`SELECT 1`); err != nil { // 保持与 db 的连接语义
		return "", err
	}
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM audit_projects WHERE id=?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrProjectNotFound
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, id), nil
}

func (s *Store) Delete(id string) error {
	dir, err := s.projDir(id)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM audit_projects WHERE id=?`, id); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// ---------- 文件树 / 读 / 写 ----------

// Tree 返回 DFS 连续序的文件树:目录的子项紧跟目录之后(同层目录在前、
// 字母序)。不能按深度扁平排序——那会把子项与父目录拆散,渲染成乱序树。
func (s *Store) Tree(id string) ([]FileNode, error) {
	dir, err := s.projDir(id)
	if err != nil {
		return nil, err
	}
	children := map[string][]FileNode{} // 父路径 → 子节点("." = 顶层)
	_ = filepath.WalkDir(filepath.Join(dir, "src"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(filepath.Join(dir, "src"), p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		info, _ := d.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		parent := path.Dir(rel)
		children[parent] = append(children[parent], FileNode{Path: rel, Size: size, IsDir: d.IsDir()})
		return nil
	})
	out := []FileNode{} // 非 nil
	var emit func(nodes []FileNode)
	emit = func(nodes []FileNode) {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].IsDir != nodes[j].IsDir {
				return nodes[i].IsDir // 同层目录在前
			}
			return nodes[i].Path < nodes[j].Path
		})
		for _, n := range nodes {
			out = append(out, n)
			if n.IsDir {
				emit(children[n.Path])
			}
		}
	}
	emit(children["."])
	return out, nil
}

func (s *Store) Read(id, rel string) (string, error) {
	dir, err := s.projDir(id)
	if err != nil {
		return "", err
	}
	p, err := safeJoin(filepath.Join(dir, "src"), rel)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("文件不存在: %s", rel)
	}
	if st.Size() > maxReadSize {
		return "", fmt.Errorf("文件过大(%d KB,上限 2MB)", st.Size()/1024)
	}
	if !isTextFile(p) {
		return "", errors.New("二进制文件不支持查看")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Write 保存修改;首次修改前把原始内容备份到 .bak/<rel>。
func (s *Store) Write(id, rel, content string) error {
	dir, err := s.projDir(id)
	if err != nil {
		return err
	}
	p, err := safeJoin(filepath.Join(dir, "src"), rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("文件不存在: %s", rel)
	}
	bak, err := safeJoin(filepath.Join(dir, ".bak"), rel)
	if err == nil {
		if _, err := os.Stat(bak); os.IsNotExist(err) {
			orig, rerr := os.ReadFile(p)
			if rerr == nil {
				_ = os.MkdirAll(filepath.Dir(bak), 0o755)
				_ = os.WriteFile(bak, orig, 0o644)
			}
		}
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// ---------- 搜索与扫描 ----------

func (s *Store) Search(id, query string, regex, caseSensitive bool, limit int) ([]SearchHit, error) {
	dir, err := s.projDir(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("搜索词为空")
	}
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	var re *regexp.Regexp
	var needle string
	if regex {
		flags := "(?i)"
		if caseSensitive {
			flags = ""
		}
		re, err = regexp.Compile(flags + query)
		if err != nil {
			return nil, fmt.Errorf("正则非法: %w", err)
		}
	} else {
		needle = query
		if !caseSensitive {
			needle = strings.ToLower(query)
		}
	}
	hits := []SearchHit{} // 非 nil:空命中序列化为 []
	s.walkText(dir, func(rel, abs string) bool {
		data, err := os.ReadFile(abs)
		if err != nil || len(data) > maxReadSize {
			return true
		}
		lines := strings.Split(string(data), "\n")
		for i, ln := range lines {
			matched := false
			if re != nil {
				matched = re.MatchString(ln)
			} else if caseSensitive {
				matched = strings.Contains(ln, needle)
			} else {
				matched = strings.Contains(strings.ToLower(ln), needle)
			}
			if matched {
				hits = append(hits, SearchHit{Path: rel, LineNo: i + 1, Line: strings.TrimRight(ln, "\r")})
				if len(hits) >= limit {
					return false
				}
			}
		}
		return true
	})
	return hits, nil
}

func (s *Store) Scan(id string) ([]SensHit, error) {
	dir, err := s.projDir(id)
	if err != nil {
		return nil, err
	}
	rules, err := s.ListFuncs()
	if err != nil {
		return nil, err
	}
	type compiled struct {
		f *SensitiveFunc
		r *regexp.Regexp
	}
	var cs []compiled
	for _, f := range rules {
		if !f.Enabled {
			continue
		}
		r, err := regexp.Compile(f.Pattern)
		if err != nil {
			continue // 坏规则跳过,不炸整体扫描
		}
		cs = append(cs, compiled{f, r})
	}
	hits := []SensHit{} // 非 nil:空命中序列化为 []
	files := 0
	s.walkText(dir, func(rel, abs string) bool {
		files++
		if files > maxScanFiles {
			return false
		}
		data, err := os.ReadFile(abs)
		if err != nil || len(data) > maxReadSize {
			return true
		}
		lines := strings.Split(string(data), "\n")
		for i, ln := range lines {
			for _, c := range cs {
				m := c.r.FindString(ln)
				if m != "" {
					hits = append(hits, SensHit{
						Path: rel, LineNo: i + 1, Line: strings.TrimRight(ln, "\r"),
						Match: m, FuncID: c.f.ID, Language: c.f.Language, Pattern: c.f.Pattern,
						Category: c.f.Category, Risk: c.f.Risk,
					})
					if len(hits) >= maxHits {
						return false
					}
				}
			}
		}
		return true
	})
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Risk != hits[j].Risk {
			order := map[string]int{"high": 0, "medium": 1, "low": 2}
			return order[hits[i].Risk] < order[hits[j].Risk]
		}
		return hits[i].Path < hits[j].Path
	})
	return hits, nil
}

// ---------- 敏感函数规则库 ----------

func (s *Store) ListFuncs() ([]*SensitiveFunc, error) {
	rows, err := s.db.Query(`SELECT id,language,pattern,category,risk,note,enabled FROM audit_sfuncs ORDER BY sort,language,category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*SensitiveFunc{} // 非 nil
	for rows.Next() {
		f := &SensitiveFunc{}
		var enabled int
		if err := rows.Scan(&f.ID, &f.Language, &f.Pattern, &f.Category, &f.Risk, &f.Note, &enabled); err != nil {
			return nil, err
		}
		f.Enabled = enabled != 0
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) AddFunc(language, pattern, category, risk, note string, enabled bool) (*SensitiveFunc, error) {
	if _, err := regexp.Compile(pattern); err != nil {
		return nil, fmt.Errorf("正则非法: %w", err)
	}
	f := &SensitiveFunc{ID: newID(), Language: language, Pattern: pattern,
		Category: category, Risk: risk, Note: note, Enabled: enabled}
	_, err := s.db.Exec(`INSERT INTO audit_sfuncs(id,language,pattern,category,risk,note,enabled,sort)
		VALUES(?,?,?,?,?,?,?,999)`, f.ID, f.Language, f.Pattern, f.Category, f.Risk, f.Note, boolInt(enabled))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Store) UpdateFunc(id, language, pattern, category, risk, note string, enabled bool) error {
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("正则非法: %w", err)
	}
	res, err := s.db.Exec(`UPDATE audit_sfuncs SET language=?,pattern=?,category=?,risk=?,note=?,enabled=? WHERE id=?`,
		language, pattern, category, risk, note, boolInt(enabled), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("规则不存在")
	}
	return nil
}

func (s *Store) DeleteFunc(id string) error {
	_, err := s.db.Exec(`DELETE FROM audit_sfuncs WHERE id=?`, id)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
