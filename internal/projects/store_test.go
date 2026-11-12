package projects

import (
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "projects.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestProjectCRUD(t *testing.T) {
	s := openTemp(t)

	// 空名拒绝
	if _, err := s.CreateProject(ProjectInput{}); err == nil {
		t.Fatal("空名称应报错")
	}

	p, err := s.CreateProject(ProjectInput{Name: "某单位渗透", Target: "example.com", Scope: "仅主站"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Status != StatusActive {
		t.Fatalf("默认状态应为 active，实际 %s", p.Status)
	}

	got, err := s.GetProject(p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "某单位渗透" || got.Target != "example.com" {
		t.Fatalf("读取结果不符: %+v", got)
	}

	if _, err := s.UpdateProject(p.ID, ProjectInput{Name: "改名", Status: "done"}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	got, _ = s.GetProject(p.ID)
	if got.Name != "改名" || got.Status != StatusDone {
		t.Fatalf("更新结果不符: %+v", got)
	}

	list, err := s.ListProjects()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListProjects: %v len=%d", err, len(list))
	}

	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := s.GetProject(p.ID); err != ErrProjectNotFound {
		t.Fatalf("删除后应返回 ErrProjectNotFound，实际 %v", err)
	}
}

func TestEntryCRUDAndCascade(t *testing.T) {
	s := openTemp(t)
	p, err := s.CreateProject(ProjectInput{Name: "P1"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// 不存在的项目
	if _, err := s.CreateEntry("nope", KindAsset, "x", "{}", ""); err != ErrProjectNotFound {
		t.Fatalf("应向不存在项目报错，实际 %v", err)
	}

	e, err := s.CreateEntry(p.ID, KindPort, "80 端口", `{"host":"1.1.1.1","port":80}`, "web")
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if e.Kind != KindPort || e.Title != "80 端口" {
		t.Fatalf("条目内容不符: %+v", e)
	}

	if _, err := s.CreateEntry(p.ID, KindFingerprint, "nginx", `{}`, ""); err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	all, err := s.ListEntries(p.ID, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListEntries 全部: %v len=%d", err, len(all))
	}
	ports, err := s.ListEntries(p.ID, KindPort)
	if err != nil || len(ports) != 1 {
		t.Fatalf("ListEntries 按 kind: %v len=%d", err, len(ports))
	}

	updated, err := s.UpdateEntry(e.ID, "8080 端口", `{"host":"1.1.1.1","port":8080}`, "web,http")
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}
	if updated.Title != "8080 端口" || updated.Kind != KindPort {
		t.Fatalf("更新结果不符: %+v", updated)
	}

	// 计数
	got, _ := s.GetProject(p.ID)
	if got.EntryCounts[KindPort] != 1 || got.EntryCounts[KindFingerprint] != 1 {
		t.Fatalf("计数不符: %+v", got.EntryCounts)
	}

	// 级联删除：删项目后条目清空
	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	left, err := s.ListEntries(p.ID, "")
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("级联删除后应无条目，实际 %d", len(left))
	}

	if err := s.DeleteEntry("nope"); err != ErrEntryNotFound {
		t.Fatalf("删除不存在条目应报 ErrEntryNotFound，实际 %v", err)
	}
}

// 重开数据库后数据仍在（持久化）。
func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	p, err := s1.CreateProject(ProjectInput{Name: "持久化"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := s1.CreateEntry(p.ID, KindNote, "笔记", `{"content":"hello"}`, ""); err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	_ = s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("重开: %v", err)
	}
	defer s2.Close()
	got, err := s2.GetProject(p.ID)
	if err != nil {
		t.Fatalf("重开后 GetProject: %v", err)
	}
	if got.Name != "持久化" || got.EntryCounts[KindNote] != 1 {
		t.Fatalf("持久化数据不符: %+v", got)
	}
}
