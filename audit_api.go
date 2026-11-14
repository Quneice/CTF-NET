package main

// 代码审计工作区的 Wails 绑定（挂在 App 上）。
// 存储实现见 internal/audit/store.go（SQLite audit.db + DataDir/audits/<id>/ 文件本体）。
// 导入流程：前端逐文件 UploadFileB64（name 为 webkitRelativePath 保留目录结构）→
// AuditImport 按 file_id 组装项目树。

import (
	"errors"

	"ctfnet/internal/audit"
)

func (a *App) auditStore() (*audit.Store, error) {
	if a.Audit == nil {
		return nil, errors.New("代码审计不可用：数据库打开失败")
	}
	return a.Audit, nil
}

// AuditImport 由上传的 file_id 列表组装审计项目(refs: [{id,name}]，name 含相对路径)。
func (a *App) AuditImport(name string, refs []audit.ImportFile) (*audit.Project, error) {
	s, err := a.auditStore()
	if err != nil {
		return nil, err
	}
	resolve := func(id string) (string, bool) {
		st, err := a.Files.Get(id)
		if err != nil || st == nil {
			return "", false
		}
		return st.Path, true
	}
	return s.Import(name, refs, resolve)
}

func (a *App) ListAuditProjects() ([]*audit.Project, error) {
	s, err := a.auditStore()
	if err != nil {
		return nil, err
	}
	return s.List()
}

func (a *App) DeleteAuditProject(id string) error {
	s, err := a.auditStore()
	if err != nil {
		return err
	}
	return s.Delete(id)
}

func (a *App) AuditFileTree(id string) ([]audit.FileNode, error) {
	s, err := a.auditStore()
	if err != nil {
		return nil, err
	}
	return s.Tree(id)
}

func (a *App) AuditReadFile(id, rel string) (string, error) {
	s, err := a.auditStore()
	if err != nil {
		return "", err
	}
	return s.Read(id, rel)
}

func (a *App) AuditWriteFile(id, rel, content string) error {
	s, err := a.auditStore()
	if err != nil {
		return err
	}
	return s.Write(id, rel, content)
}

func (a *App) AuditSearch(id, query string, regex, caseSensitive bool, limit int) ([]audit.SearchHit, error) {
	s, err := a.auditStore()
	if err != nil {
		return nil, err
	}
	return s.Search(id, query, regex, caseSensitive, limit)
}

func (a *App) AuditScan(id string) ([]audit.SensHit, error) {
	s, err := a.auditStore()
	if err != nil {
		return nil, err
	}
	return s.Scan(id)
}

// —— 敏感函数规则库 CRUD ——

func (a *App) ListSensitiveFuncs() ([]*audit.SensitiveFunc, error) {
	s, err := a.auditStore()
	if err != nil {
		return nil, err
	}
	return s.ListFuncs()
}

func (a *App) AddSensitiveFunc(language, pattern, category, risk, note string, enabled bool) (*audit.SensitiveFunc, error) {
	s, err := a.auditStore()
	if err != nil {
		return nil, err
	}
	return s.AddFunc(language, pattern, category, risk, note, enabled)
}

func (a *App) UpdateSensitiveFunc(id, language, pattern, category, risk, note string, enabled bool) error {
	s, err := a.auditStore()
	if err != nil {
		return err
	}
	return s.UpdateFunc(id, language, pattern, category, risk, note, enabled)
}

func (a *App) DeleteSensitiveFunc(id string) error {
	s, err := a.auditStore()
	if err != nil {
		return err
	}
	return s.DeleteFunc(id)
}
