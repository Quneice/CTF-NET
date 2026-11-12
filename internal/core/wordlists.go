// 字典库扫描：wordlists/<kind>/ 下的字典列表（词数/大小，带缓存）。供界面下拉选择。
package core

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// WordlistInfo 一个字典文件的元信息。
type WordlistInfo struct {
	Name  string `json:"name"`  // 文件名（如 php-3.2k.txt）
	Lines int    `json:"lines"` // 词条数
	Size  int64  `json:"size"`  // 字节数
}

const wordlistCacheTTL = 30 * time.Second

var (
	wlCacheMu  sync.Mutex
	wlCache    = map[string][]WordlistInfo{}
	wlCachedAt = map[string]time.Time{}
)

// WordlistsDir 项目内字典库根目录（<DataDir 上一级>/wordlists 或 exe 旁 wordlists）。
// 字典库随项目分发，放在 exe 同级 wordlists/ 下；开发态 go run 时取项目根 wordlists/。
func WordlistsDir() string {
	// exe 同级优先（wails build 产物）
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "wordlists")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	// 开发态：工作目录向上找 wordlists/
	dir, err := os.Getwd()
	if err == nil {
		for i := 0; i < 4; i++ {
			p := filepath.Join(dir, "wordlists")
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p
			}
			dir = filepath.Dir(dir)
		}
	}
	return "wordlists"
}

// ListWordlists 列出 wordlists/<kind>/ 下的字典（kind 仅允许字母数字，防路径穿越）。
func ListWordlists(kind string) ([]WordlistInfo, error) {
	for _, r := range kind {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return nil, os.ErrInvalid
		}
	}
	wlCacheMu.Lock()
	if time.Since(wlCachedAt[kind]) < wordlistCacheTTL {
		if cached, ok := wlCache[kind]; ok {
			wlCacheMu.Unlock()
			return cached, nil
		}
	}
	wlCacheMu.Unlock()

	dir := filepath.Join(WordlistsDir(), kind)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []WordlistInfo{}, nil // 目录不存在 → 空列表而非报错
	}
	out := make([]WordlistInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lines, size := countWordlistLines(filepath.Join(dir, e.Name()))
		out = append(out, WordlistInfo{Name: e.Name(), Lines: lines, Size: size})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })

	wlCacheMu.Lock()
	wlCache[kind] = out
	wlCachedAt[kind] = time.Now()
	wlCacheMu.Unlock()
	return out, nil
}

// WordlistPath 解析 kind 下某个字典的完整路径；仅允许库目录内的文件，拒绝路径穿越。
func WordlistPath(kind, name string) (string, bool) {
	if name == "" || name != filepath.Base(name) {
		return "", false
	}
	p := filepath.Join(WordlistsDir(), kind, name)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, true
	}
	return "", false
}

// countWordlistLines 统计非空非注释词条数与文件大小。
func countWordlistLines(path string) (int, int64) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	st, _ := f.Stat()
	var lines int
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		w := strings.TrimSpace(sc.Text())
		if w != "" && !strings.HasPrefix(w, "#") {
			lines++
		}
	}
	var size int64
	if st != nil {
		size = st.Size()
	}
	return lines, size
}
