package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestTaskManager(t *testing.T) *TaskManager {
	t.Helper()
	fs := NewFileStore(t.TempDir())
	return &TaskManager{FileStore: fs}
}

// 多文件落盘：保留相对目录、跨目录重名去重、防路径穿越
func TestCopyStoredFileMultiPreserveDir(t *testing.T) {
	tm := newTestTaskManager(t)
	workdir := t.TempDir()

	names := []string{"a/img.jpg", "b/img.jpg", "../../etc/evil.jpg", "..\\..\\win\\evil2.jpg", "plain.jpg"}
	for _, n := range names {
		if _, err := tm.FileStore.Save(n, []byte(n)); err != nil {
			t.Fatal(err)
		}
	}
	used := map[string]bool{}
	var resolved []string
	for _, n := range names {
		stored, err := tm.FileStore.Save(n, []byte("content-"+n))
		if err != nil {
			t.Fatal(err)
		}
		got, err := tm.copyStoredFile(stored.ID, true, workdir, used)
		if err != nil {
			t.Fatal(err)
		}
		resolved = append(resolved, got)
	}
	want := []string{"a/img.jpg", "b/img.jpg", "etc/evil.jpg", "win/evil2.jpg", "plain.jpg"}
	for i, w := range want {
		if resolved[i] != w {
			t.Errorf("第 %d 个: 期望 %q, 得到 %q", i, w, resolved[i])
		}
	}
	// 落盘验证（无穿越到 workdir 之外）
	for _, w := range want {
		if _, err := os.Stat(filepath.Join(workdir, filepath.FromSlash(w))); err != nil {
			t.Errorf("文件未落盘 %s: %v", w, err)
		}
	}
	// 同目录重名 → 后缀去重
	s1, _ := tm.FileStore.Save("dup.jpg", []byte("1"))
	s2, _ := tm.FileStore.Save("dup.jpg", []byte("2"))
	n1, err := tm.copyStoredFile(s1.ID, true, workdir, used)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := tm.copyStoredFile(s2.ID, true, workdir, used)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != "dup.jpg" || n2 != "dup_2.jpg" {
		t.Errorf("重名去重失败: %s, %s", n1, n2)
	}
	// 扁平模式：只取最后一段
	s3, _ := tm.FileStore.Save("x/y/up.jpg", []byte("3"))
	n3, err := tm.copyStoredFile(s3.ID, false, workdir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n3 != "up.jpg" {
		t.Errorf("扁平模式失败: %s", n3)
	}
}

// 工具侧 ReportProgress → Job 进度字段 → ToDict 透传（前端轮询可见）
func TestRunReportsProgress(t *testing.T) {
	tm := newTestTaskManager(t)
	tm.Workdir = t.TempDir()
	tm.ArtifactsDir = t.TempDir()
	tool := &BaseTool{
		NameAttr:     "prog",
		CategoryAttr: "test",
		Runner: func(ctx context.Context, params map[string]any, tc *ToolContext) (*ToolResult, error) {
			_ = ctx
			tc.ReportProgress(42, "处理中 42%")
			return &ToolResult{Status: StatusSuccess, Stdout: "ok"}, nil
		},
	}
	id := tm.Submit(tool, nil)
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, ok := tm.Get(id)
		if !ok {
			t.Fatal("任务不存在")
		}
		d := j.ToDict()
		if d["status"] == "success" {
			if d["progress_percent"] != 42 || d["progress_message"] != "处理中 42%" {
				t.Fatalf("进度未透传: %+v", d)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("任务超时未完成")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSanitizeRelPath(t *testing.T) {
	cases := map[string]string{
		"a/b.jpg":      "a/b.jpg",
		`a\b.jpg`:      "a/b.jpg",
		"../a.jpg":     "a.jpg",
		"a/../b.jpg":   "b.jpg",
		"./a/./b.jpg":  "a/b.jpg",
		"../../..":     "",
		"":             "",
		"a//b///c.jpg": "a/b/c.jpg",
	}
	for in, want := range cases {
		if got := sanitizeRelPath(in); got != want {
			t.Errorf("sanitizeRelPath(%q) = %q, 期望 %q", in, got, want)
		}
	}
}
