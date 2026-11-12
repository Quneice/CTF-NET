package main

// 渗透测试项目管理的 Wails 绑定（挂在 App 上，与 app.go 同一模式）。
// 存储实现见 internal/projects/store.go（SQLite，DataDir/projects.db）。

import (
	"errors"
	"io"
	"os"

	"ctfnet/internal/projects"
)

// store 取项目库；打开失败（main.go 降级为 nil）时返回错误而非空指针崩溃。
func (a *App) store() (*projects.Store, error) {
	if a.Projects == nil {
		return nil, errors.New("项目管理不可用：数据库打开失败")
	}
	return a.Projects, nil
}

// —— 项目 ——

// ListProjects 项目列表（含各类条目计数）。
func (a *App) ListProjects() ([]*projects.Project, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.ListProjects()
}

// GetProject 项目详情（含各类条目计数）。
func (a *App) GetProject(id string) (*projects.Project, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.GetProject(id)
}

// CreateProject 新建项目。
func (a *App) CreateProject(input projects.ProjectInput) (*projects.Project, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.CreateProject(input)
}

// UpdateProject 更新项目基础信息。
func (a *App) UpdateProject(id string, input projects.ProjectInput) (*projects.Project, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.UpdateProject(id, input)
}

// DeleteProject 删除项目（级联删除其全部条目）。
func (a *App) DeleteProject(id string) error {
	s, err := a.store()
	if err != nil {
		return err
	}
	return s.DeleteProject(id)
}

// —— 条目 ——

// ListEntries 项目下条目列表；kind 为空串表示全部类别。
func (a *App) ListEntries(projectID, kind string) ([]*projects.Entry, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.ListEntries(projectID, kind)
}

// CreateEntry 新增条目（content 为 JSON 文本）。
func (a *App) CreateEntry(projectID, kind, title, content, tags string) (*projects.Entry, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.CreateEntry(projectID, kind, title, content, tags)
}

// UpdateEntry 更新条目标题/内容/标签。
func (a *App) UpdateEntry(id, title, content, tags string) (*projects.Entry, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.UpdateEntry(id, title, content, tags)
}

// DeleteEntry 删除条目。
func (a *App) DeleteEntry(id string) error {
	s, err := a.store()
	if err != nil {
		return err
	}
	return s.DeleteEntry(id)
}

// —— 附件 ——

// ExportFile 把上传存储中的文件复制到 destPath（附件下载，配合前端保存对话框）。
func (a *App) ExportFile(fileID, destPath string) error {
	stored, err := a.Files.Get(fileID)
	if err != nil {
		return err
	}
	if stored == nil {
		return notFound(ErrFileNotFound, fileID)
	}
	src, err := os.Open(stored.Path)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}
