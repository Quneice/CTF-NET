// 漏洞库：SQLite 持久化存储 + 目录扫描导入。
//
// 目录约定（与前端静态快照 mock/vulnLibrary.ts 的结构一致）：
//   Z-编号分类目录 → S-编号系统/厂商目录 → ND-编号案例 md
// 扫描规则（对真实语料 6199 个文件的统计结果）：
//   - 整目录跳过 My_template（模板，不含真实漏洞）；
//   - 根目录散件 md → 归入「其他」分类（如 CRLF Injection 等 3 个文件）；
//   - Z 目录直属 md（如 "Z100 办公自动化 OA.md"，分类总览）→ 跳过；
//   - S 目录下与目录同名的 md（系统总览）→ 跳过；
//   - 更深层仅收录文件名 ND 前缀的 md（assets 目录里也有案例），
//     Apache httpd 版本文档等 README.md → 跳过；
//   - 增量：mtime+size 未变则跳过解析。
//
// 打开失败不阻断工具箱主功能（main.go 降级处理，与 projects/wiki 同模式）。
package vulnlib

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // database/sql 驱动，纯 Go 无 cgo
)

// ErrCaseNotFound 案例不存在。
var ErrCaseNotFound = errors.New("漏洞案例不存在")

var (
	globalStore *Store
	globalMu    sync.Mutex
)

// Init 打开（或创建）全局漏洞库并建表；失败返回错误（调用方决定降级与否）。
func Init(dbPath string) error {
	st, err := open(dbPath)
	if err != nil {
		return err
	}
	globalMu.Lock()
	if globalStore != nil {
		globalStore.Close()
	}
	globalStore = st
	globalMu.Unlock()
	return nil
}

// DB 返回全局漏洞库；未初始化或打开失败时为 nil。
func DB() *Store {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalStore
}

// Store 漏洞库存储。
type Store struct {
	mu      sync.Mutex // 单写者：Scan 与其他读互斥（db 连接本身也限 1）
	db      *sql.DB
	dataDir string // vulnlib.db 所在目录（删除备份放 dataDir/backups/vulnlib_deleted）
}

func open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // 避免连接池并发写导致 SQLITE_BUSY
	for _, stmt := range []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TABLE IF NOT EXISTS categories(
			id TEXT PRIMARY KEY,
			code TEXT DEFAULT '',
			name TEXT DEFAULT '',
			ord INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS cases(
			id TEXT PRIMARY KEY,
			cat_id TEXT DEFAULT '',
			cat_name TEXT DEFAULT '',
			sys TEXT DEFAULT '',
			sys_code TEXT DEFAULT '',
			name TEXT DEFAULT '',
			title TEXT DEFAULT '',
			cves TEXT DEFAULT '',
			payloads TEXT DEFAULT '',
			curls TEXT DEFAULT '',
			content TEXT DEFAULT '',
			size INTEGER DEFAULT 0,
			mtime INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cases_cat ON cases(cat_id)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db: db, dataDir: filepath.Dir(path)}, nil
}

// Close 关闭数据库。
func (s *Store) Close() { s.db.Close() }

// ---------------------------------------------------------------------------
// 数据模型（JSON 键与前端 lib/vulnlib.ts 一致）
// ---------------------------------------------------------------------------

// CaseSummary 案例摘要（列表/搜索用，不含全文与 payload 明细）。
type CaseSummary struct {
	ID           string   `json:"id"`
	CatID        string   `json:"cat_id"`
	CatName      string   `json:"cat_name"`
	Sys          string   `json:"sys"`
	SysCode      string   `json:"sys_code"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	CVEs         []string `json:"cves"`
	PayloadCount int      `json:"payload_count"`
	Size         int64    `json:"size"`
}

// CaseDetail 案例全文（选中查看/执行检测用）。
type CaseDetail struct {
	CaseSummary
	Content  string   `json:"content"`  // md 全文
	Payloads []string `json:"payloads"` // 原始 HTTP 请求包
	Curls    []string `json:"curls"`
	Path     string   `json:"path"` // 磁盘绝对路径（桌面版可见）
	Mtime    int64    `json:"mtime"`
}

// SystemSummary 分类下系统/厂商摘要。
type SystemSummary struct {
	Name      string `json:"name"`
	Code      string `json:"code"`
	CaseCount int    `json:"case_count"`
}

// Category 漏洞大类（含系统列表与案例总数）。
type Category struct {
	ID        string          `json:"id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	CaseCount int             `json:"case_count"`
	Systems   []SystemSummary `json:"systems"`
}

// Stats 漏洞库整体状态。
type Stats struct {
	Scanned   bool    `json:"scanned"` // 是否已导入过
	RootDir   string  `json:"root_dir"`
	CaseCount int     `json:"case_count"`
	CatCount  int     `json:"cat_count"`
	LastScan  float64 `json:"last_scan"` // Unix 秒（未扫描为 0）
}

// ScanSummary 一次扫描的结果。
type ScanSummary struct {
	RootDir    string   `json:"root_dir"`
	Total      int      `json:"total"`   // 本次遇到的候选案例 md 数
	Updated    int      `json:"updated"` // 新插入或内容有变
	Unchanged  int      `json:"unchanged"`
	Deleted    int      `json:"deleted"` // 磁盘已不存在的旧条目
	Skipped    int      `json:"skipped"` // 被过滤的非案例 md
	Errors     []string `json:"errors"`  // 读取失败等
	DurationMs int64    `json:"duration_ms"`
}

// ---------------------------------------------------------------------------
// 扫描导入
// ---------------------------------------------------------------------------

var ndPrefixRe = regexp.MustCompile(`^ND\d`)
var sysCodeRe = regexp.MustCompile(`^S\d+`)
var catCodeRe = regexp.MustCompile(`^Z\d+`)

// Scan 扫描 root 目录并增量导入。规则见包注释。
func (s *Store) Scan(root string) (ScanSummary, error) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	sum := ScanSummary{RootDir: root}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return sum, fmt.Errorf("目录不存在或不可访问: %s", root)
	}
	root, _ = filepath.Abs(root)

	tx, err := s.db.Begin()
	if err != nil {
		return sum, err
	}
	defer tx.Rollback() // 提交后自动 no-op

	// 旧条目清点：文件已不存在的删除；mtime+size 未变的跳过解析
	oldCases := map[string][2]int64{} // id -> {mtime, size}
	rows, err := tx.Query(`SELECT id, mtime, size FROM cases`)
	if err != nil {
		return sum, err
	}
	for rows.Next() {
		var id string
		var mtime, size int64
		if err := rows.Scan(&id, &mtime, &size); err != nil {
			rows.Close()
			return sum, err
		}
		oldCases[id] = [2]int64{mtime, size}
	}
	rows.Close()

	updStmt, err := tx.Prepare(`INSERT INTO cases(id, cat_id, cat_name, sys, sys_code, name, title, cves, payloads, curls, content, size, mtime)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			cat_id=excluded.cat_id, cat_name=excluded.cat_name, sys=excluded.sys, sys_code=excluded.sys_code,
			name=excluded.name, title=excluded.title, cves=excluded.cves, payloads=excluded.payloads,
			curls=excluded.curls, content=excluded.content, size=excluded.size, mtime=excluded.mtime`)
	if err != nil {
		return sum, err
	}
	defer updStmt.Close()

	delStmt, err := tx.Prepare(`DELETE FROM cases WHERE id = ?`)
	if err != nil {
		return sum, err
	}
	defer delStmt.Close()

	catStmt, err := tx.Prepare(`INSERT INTO categories(id, code, name, ord) VALUES(?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET code=excluded.code, name=excluded.name, ord=MIN(categories.ord, excluded.ord)`)
	if err != nil {
		return sum, err
	}
	defer catStmt.Close()

	seen := map[string]bool{}
	catOrd := map[string]int{}
	nextOrd := 0

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		if d.IsDir() {
			if d.Name() == "My_template" {
				return filepath.SkipDir // 模板目录整体跳过
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil // 仅 md 是漏洞案例
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		catID, sys := classify(rel, parts)
		if catID == "" {
			sum.Skipped++
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("%s: %v", rel, ierr))
			return nil
		}
		sum.Total++
		seen[rel] = true

		// 分类登记（首次出现顺序决定展示顺序）
		catName := parts[0]
		catCode := ""
		if catID == "__root__" {
			catName = "其他"
		} else if m := catCodeRe.FindString(parts[0]); m != "" {
			catCode = m
		}
		if _, ok := catOrd[catID]; !ok {
			catOrd[catID] = nextOrd
			nextOrd++
		}
		if _, err := catStmt.Exec(catID, catCode, catName, catOrd[catID]); err != nil {
			return err
		}

		// 增量：mtime+size 未变 → 跳过解析
		mtime, size := info.ModTime().Unix(), info.Size()
		if old, ok := oldCases[rel]; ok && old[0] == mtime && old[1] == size {
			sum.Unchanged++
			return nil
		}

		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("%s: %v", rel, rerr))
			return nil
		}
		parsed := ParseVulnMD(string(raw))
		cves, _ := json.Marshal(parsed.CVEs)
		payloads, _ := json.Marshal(parsed.Payloads)
		curls, _ := json.Marshal(parsed.Curls)
		sysCode := ""
		if m := sysCodeRe.FindString(sys); m != "" {
			sysCode = m
		}
		name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
		if _, err := updStmt.Exec(rel, catID, catName, sys, sysCode, name, parsed.Title,
			string(cves), string(payloads), string(curls), string(raw), size, mtime); err != nil {
			return err
		}
		sum.Updated++
		return nil
	})
	if walkErr != nil {
		return sum, walkErr
	}

	// 清理已消失的条目
	for id := range oldCases {
		if !seen[id] {
			if _, err := delStmt.Exec(id); err != nil {
				return sum, err
			}
			sum.Deleted++
		}
	}

	// 清理空分类
	if _, err := tx.Exec(`DELETE FROM categories WHERE id NOT IN (SELECT DISTINCT cat_id FROM cases)`); err != nil {
		return sum, err
	}

	// meta
	now := float64(time.Now().Unix())
	if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES('root_dir',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, root); err != nil {
		return sum, err
	}
	if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES('last_scan',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		fmt.Sprintf("%d", int64(now))); err != nil {
		return sum, err
	}

	if err := tx.Commit(); err != nil {
		return sum, err
	}
	sum.DurationMs = time.Since(start).Milliseconds()
	return sum, nil
}

// classify 按路径层级判定是否案例与归属；catID 为空表示跳过。
func classify(rel string, parts []string) (catID, sys string) {
	switch {
	case len(parts) == 1:
		return "__root__", "" // 根目录散件
	case len(parts) == 2:
		return "", "" // Z 目录直属的总览 md
	case len(parts) == 3:
		if strings.TrimSuffix(parts[2], ".md") == parts[1] {
			return "", "" // 与 S 目录同名的系统总览 md
		}
		return parts[0], parts[1]
	default:
		if !ndPrefixRe.MatchString(filepath.Base(rel)) {
			return "", "" // 深层文档（Apache 版本文档等）
		}
		return parts[0], parts[1] // 深层 ND 案例挂到其 S 系统
	}
}

// ---------------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------------

// Stats 返回漏洞库状态。
func (s *Store) Stats() (Stats, error) {
	st := Stats{}
	var root, last string
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key='root_dir'`).Scan(&root); err == nil {
		st.RootDir = root
	}
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key='last_scan'`).Scan(&last); err == nil {
		var lastUnix int64
		if _, perr := fmt.Sscanf(last, "%d", &lastUnix); perr == nil {
			st.LastScan = float64(lastUnix)
		}
	}
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM cases`).Scan(&st.CaseCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&st.CatCount)
	st.Scanned = st.CaseCount > 0
	return st, nil
}

// ListCategories 返回全部分类（含系统级计数，案例按需用 ListCases 拉取）。
func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.db.Query(`SELECT id, code, name FROM categories ORDER BY ord`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	rows.Close()

	// 系统计数
	type sysKey struct{ cat, sys string }
	sysCount := map[sysKey]int{}
	sysCode := map[sysKey]string{}
	srows, err := s.db.Query(`SELECT cat_id, sys, sys_code, COUNT(*) FROM cases GROUP BY cat_id, sys ORDER BY sys`)
	if err != nil {
		return nil, err
	}
	for srows.Next() {
		var cat, sys, code string
		var n int
		if err := srows.Scan(&cat, &sys, &code, &n); err != nil {
			srows.Close()
			return nil, err
		}
		sysCount[sysKey{cat, sys}] = n
		sysCode[sysKey{cat, sys}] = code
	}
	srows.Close()

	for i := range out {
		total := 0
		// 按 cat_id 归集系统
		sysNames := []string{}
		seen := map[string]bool{}
		for k := range sysCount {
			if k.cat != out[i].ID {
				continue
			}
			if !seen[k.sys] {
				seen[k.sys] = true
				sysNames = append(sysNames, k.sys)
			}
		}
		sort.Strings(sysNames)
		out[i].Systems = make([]SystemSummary, 0, len(sysNames))
		for _, name := range sysNames {
			n := sysCount[sysKey{out[i].ID, name}]
			total += n
			out[i].Systems = append(out[i].Systems, SystemSummary{Name: name, Code: sysCode[sysKey{out[i].ID, name}], CaseCount: n})
		}
		out[i].CaseCount = total
	}
	return out, nil
}

// ListCases 返回某分类（+可选系统）下的案例摘要。
func (s *Store) ListCases(catID, sys string) ([]CaseSummary, error) {
	q := `SELECT id, cat_id, cat_name, sys, sys_code, name, title, cves, payloads, size FROM cases WHERE cat_id = ?`
	args := []any{catID}
	if sys != "" {
		q += ` AND sys = ?`
		args = append(args, sys)
	}
	q += ` ORDER BY sys, name`
	return s.querySummaries(q, args...)
}

// Search 按关键字搜索（标题/文件名/CVE），可选限定分类。
func (s *Store) Search(keyword, catID string, limit, offset int) ([]CaseSummary, error) {
	kw := "%" + strings.TrimSpace(keyword) + "%"
	if strings.TrimSpace(keyword) == "" {
		return []CaseSummary{}, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	q := `SELECT id, cat_id, cat_name, sys, sys_code, name, title, cves, payloads, size FROM cases
		WHERE (title LIKE ? OR name LIKE ? OR cves LIKE ?)`
	args := []any{kw, kw, kw}
	if catID != "" {
		q += ` AND cat_id = ?`
		args = append(args, catID)
	}
	q += ` ORDER BY cat_id, sys, name LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	return s.querySummaries(q, args...)
}

func (s *Store) querySummaries(q string, args ...any) ([]CaseSummary, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CaseSummary{}
	for rows.Next() {
		var cs CaseSummary
		var cves, payloads string
		if err := rows.Scan(&cs.ID, &cs.CatID, &cs.CatName, &cs.Sys, &cs.SysCode,
			&cs.Name, &cs.Title, &cves, &payloads, &cs.Size); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(cves), &cs.CVEs)
		if cs.CVEs == nil {
			cs.CVEs = []string{}
		}
		cs.PayloadCount = jsonArrayLen(payloads)
		out = append(out, cs)
	}
	return out, nil
}

// jsonArrayLen 计算 JSON 数组元素个数（payloads 原文较大，不整段反序列化）。
// 元素数 = 顶层逗号数 + 1（非空数组）；空数组返回 0。
func jsonArrayLen(s string) int {
	if s == "" || s == "null" || s == "[]" {
		return 0
	}
	n := 0
	depth := 0
	inStr := false
	esc := false
	for _, r := range s {
		if inStr {
			if esc {
				esc = false
				continue
			}
			if r == '\\' {
				esc = true
			} else if r == '"' {
				inStr = false
			}
			continue
		}
		switch r {
		case '"':
			inStr = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return n + 1
			}
		case ',':
			if depth == 1 {
				n++
			}
		}
	}
	return n + 1
}

// GetCase 返回案例全文。
func (s *Store) GetCase(id string) (*CaseDetail, error) {
	row := s.db.QueryRow(`SELECT id, cat_id, cat_name, sys, sys_code, name, title, cves, payloads, curls, content, size, mtime FROM cases WHERE id = ?`, id)
	var d CaseDetail
	var cves, payloads, curls string
	err := row.Scan(&d.ID, &d.CatID, &d.CatName, &d.Sys, &d.SysCode, &d.Name, &d.Title,
		&cves, &payloads, &curls, &d.Content, &d.Size, &d.Mtime)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCaseNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(cves), &d.CVEs)
	_ = json.Unmarshal([]byte(payloads), &d.Payloads)
	_ = json.Unmarshal([]byte(curls), &d.Curls)
	if d.CVEs == nil {
		d.CVEs = []string{}
	}
	if d.Payloads == nil {
		d.Payloads = []string{}
	}
	if d.Curls == nil {
		d.Curls = []string{}
	}
	d.Path = filepath.FromSlash(d.ID) // 相对路径（RootDir 拼接后可得绝对路径）
	root := ""
	_ = s.db.QueryRow(`SELECT value FROM meta WHERE key='root_dir'`).Scan(&root)
	if root != "" {
		d.Path = filepath.Join(root, filepath.FromSlash(d.ID))
	}
	return &d, nil
}

// AssetData 案例引用的本地资源文件（图片等）读取结果，DataB64 供前端拼 data URL。
type AssetData struct {
	Mime    string `json:"mime"`
	DataB64 string `json:"data_b64"`
	Size    int64  `json:"size"`
	AbsPath string `json:"abs_path"`
}

// ErrAssetNotFound 资源文件不存在（或越出漏洞库根目录）。
var ErrAssetNotFound = errors.New("资源文件不存在")

// maxAssetBytes 单资源上限（10MB），防止超大文件拖垮绑定通道。
const maxAssetBytes = 10 << 20

// GetAsset 读取案例 md 引用的本地资源文件，返回 base64 供前端 data URL 展示。
// rel 为 md 中的引用（支持 \ 分隔、URL 编码、../、Windows 绝对路径 C:\…）。
// 解析顺序：案例所在目录 → 漏洞库根目录（兼容部分文档按库根相对引用）。
// 越出根目录的路径一律拒绝。
func (s *Store) GetAsset(caseID, rel string) (*AssetData, error) {
	root := ""
	_ = s.db.QueryRow(`SELECT value FROM meta WHERE key='root_dir'`).Scan(&root)
	if root == "" {
		return nil, errors.New("漏洞库尚未导入")
	}
	rootAbs := filepath.Clean(root)

	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if dec, err := url.PathUnescape(rel); err == nil {
		rel = dec
	}
	if i := strings.IndexAny(rel, "?#"); i >= 0 {
		rel = rel[:i]
	}

	// 候选路径：Windows 绝对路径直接用；否则案例目录优先、库根兜底
	var cands []string
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || regexp.MustCompile(`^[A-Za-z]:`).MatchString(rel) {
		cands = append(cands, filepath.FromSlash(rel))
	} else {
		caseDir := filepath.Dir(filepath.Join(rootAbs, filepath.FromSlash(caseID)))
		cands = append(cands,
			filepath.Clean(filepath.Join(caseDir, filepath.FromSlash(rel))),
			filepath.Clean(filepath.Join(rootAbs, filepath.FromSlash(rel))),
		)
	}

	for _, abs := range cands {
		if abs != rootAbs && !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) {
			continue // 越界拒绝
		}
		st, err := os.Stat(abs)
		if err != nil || st.IsDir() {
			continue
		}
		if st.Size() > maxAssetBytes {
			return nil, fmt.Errorf("资源文件过大（%dKB > 10MB），跳过", st.Size()>>10)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, err
		}
		return &AssetData{
			Mime:    mimeByExt(abs),
			DataB64: base64.StdEncoding.EncodeToString(data),
			Size:    st.Size(),
			AbsPath: abs,
		}, nil
	}
	return nil, ErrAssetNotFound
}

// mimeByExt 按扩展名猜测 MIME（漏洞文档图片常见类型）。
func mimeByExt(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// ---------------------------------------------------------------------------
// 案例编辑（新增/修改/删除）：变更直接写回磁盘 md 文件，随后增量重扫入库。
// ---------------------------------------------------------------------------

// rootDir 已导入的漏洞清单根目录（meta 里记录的绝对路径）。
func (s *Store) rootDir() (string, error) {
	root := ""
	_ = s.db.QueryRow(`SELECT value FROM meta WHERE key='root_dir'`).Scan(&root)
	if root == "" {
		return "", errors.New("漏洞库尚未导入：请先在左侧扫描导入目录")
	}
	return filepath.Clean(root), nil
}

// rescanAfterMutate 文件变更后增量重扫（mtime+size 判定，仅变化文件被解析）。
func (s *Store) rescanAfterMutate() (*ScanSummary, error) {
	root, err := s.rootDir()
	if err != nil {
		return nil, err
	}
	sum, err := s.Scan(root)
	if err != nil {
		return nil, err
	}
	return &sum, nil
}

// safeFileName 校验案例文件名（仅文件名部分，自动补 .md，拒绝路径分隔/保留字符/隐藏名）。
func safeFileName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" {
		return "", errors.New("文件名不能为空")
	}
	if strings.EqualFold(filepath.Ext(name), ".md") {
		name = name[:len(name)-3]
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", errors.New("文件名不合法")
	}
	if strings.ContainsAny(name, `/\<>:"|?*`) {
		return "", errors.New(`文件名不能包含 / \ < > : " | ? * 字符`)
	}
	if strings.HasPrefix(name, ".") {
		return "", errors.New("文件名不能以 . 开头")
	}
	return name + ".md", nil
}

// safeDirName 校验目录名（分类/系统目录，拒绝路径分隔/保留字符/隐藏名）。
func safeDirName(name string) error {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" {
		return errors.New("目录名不能为空")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\<>:"|?*`) {
		return errors.New(`目录名不能包含 / \ < > : " | ? * 字符`)
	}
	if strings.HasPrefix(name, ".") {
		return errors.New("目录名不能以 . 开头")
	}
	return nil
}

// CreateCase 在分类（+系统）目录下新建案例 md 并导入。
// catID 为「其他」(__root__) 时 sys 忽略，文件落在库根。
func (s *Store) CreateCase(catID, sys, name, content string) (*ScanSummary, error) {
	root, err := s.rootDir()
	if err != nil {
		return nil, err
	}
	if catID == "" {
		return nil, errors.New("请选择漏洞分类")
	}
	// 分类必须已存在（防误建目录）
	catName := ""
	err = s.db.QueryRow(`SELECT name FROM categories WHERE id=?`, catID).Scan(&catName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("分类不存在：%s", catID)
	}
	if err != nil {
		return nil, err
	}
	file, err := safeFileName(name)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, filepath.FromSlash(catID))
	if catID != "__root__" {
		if err := safeDirName(sys); err != nil {
			return nil, errors.New("系统/厂商目录：" + err.Error())
		}
		dir = filepath.Join(dir, filepath.FromSlash(strings.TrimSpace(sys)))
	}
	// 双保险：必须落在库根内
	rel, rerr := filepath.Rel(root, dir)
	if rerr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("目标目录越出漏洞库根目录")
	}
	path := filepath.Join(dir, file)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("文件已存在：%s", file)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return s.rescanAfterMutate()
}

// UpdateCase 覆写案例 md 全文并重新导入（id 必须是库内案例，防任意文件写入）。
func (s *Store) UpdateCase(id, content string) (*ScanSummary, error) {
	det, err := s.GetCase(id)
	if err != nil {
		return nil, err
	}
	if det.Path == "" {
		return nil, errors.New("案例路径未知")
	}
	if err := os.WriteFile(det.Path, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return s.rescanAfterMutate()
}

// DeleteCase 删除案例：md 文件先移入 dataDir/backups/vulnlib_deleted（可恢复），
// 再从库中移除。删除备份保留原始相对路径结构，重名加时间戳。
func (s *Store) DeleteCase(id string) (*ScanSummary, error) {
	root, err := s.rootDir()
	if err != nil {
		return nil, err
	}
	det, err := s.GetCase(id)
	if err != nil {
		return nil, err
	}
	if det.Path == "" {
		return nil, errors.New("案例路径未知")
	}
	rel, rerr := filepath.Rel(root, det.Path)
	if rerr != nil {
		return nil, rerr
	}
	backup := filepath.Join(s.dataDir, "backups", "vulnlib_deleted", rel)
	if _, err := os.Stat(backup); err == nil {
		ext := filepath.Ext(backup)
		backup = backup[:len(backup)-len(ext)] + "_" + time.Now().Format("20060102-150405") + ext
	}
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(det.Path, backup); err != nil {
		// 跨卷 rename 失败 → 复制后删除
		data, rerr := os.ReadFile(det.Path)
		if rerr != nil {
			return nil, fmt.Errorf("删除失败：%v", err)
		}
		if werr := os.WriteFile(backup, data, 0o644); werr != nil {
			return nil, werr
		}
		if rerr := os.Remove(det.Path); rerr != nil {
			return nil, rerr
		}
	}
	return s.rescanAfterMutate()
}

// GetCasesByIDs 批量取案例（预留）。
func (s *Store) GetCasesByIDs(ids []string) ([]*CaseDetail, error) {
	out := make([]*CaseDetail, 0, len(ids))
	for _, id := range ids {
		d, err := s.GetCase(id)
		if err != nil {
			if errors.Is(err, ErrCaseNotFound) {
				continue
			}
			return out, err
		}
		out = append(out, d)
	}
	return out, nil
}
