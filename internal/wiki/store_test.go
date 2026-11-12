package wiki

import (
	"path/filepath"
	"testing"
)

func TestStoreCRUDAndSeed(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "wiki.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// 初始为空
	if n, _ := s.Count(); n != 0 {
		t.Fatalf("初始 Count = %d，应为 0", n)
	}

	// Seed
	if err := s.Seed(BuiltinSeed); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if n, _ := s.Count(); n != len(BuiltinSeed) {
		t.Fatalf("Seed 后 Count = %d，应为 %d", n, len(BuiltinSeed))
	}

	// 按分类列出
	list, err := s.List("revshell")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("revshell 分类应有内置条目")
	}
	for _, e := range list {
		if e.Category != "revshell" {
			t.Fatalf("List(revshell) 混入其他分类 %s", e.Category)
		}
	}

	// Create
	e, err := s.Create("revshell", "测试条目", "内容")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == "" || e.Category != "revshell" || e.Title != "测试条目" {
		t.Fatalf("Create 返回异常: %+v", e)
	}

	// Update
	e2, err := s.Update(e.ID, "改标题", "新内容")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if e2.Title != "改标题" || e2.Content != "新内容" {
		t.Fatalf("Update 未生效: %+v", e2)
	}

	// Delete
	if err := s.Delete(e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(e.ID); err == nil {
		t.Fatal("重复删除应返回错误")
	}
}

func TestStoreValidation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.Create("", "标题", "内容"); err == nil {
		t.Fatal("空分类应报错")
	}
	if _, err := s.Create("revshell", "", "内容"); err == nil {
		t.Fatal("空标题应报错")
	}
}
