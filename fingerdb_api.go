package main

// 指纹库管理的 Wails 绑定（挂在 App 上）。
// 存储实现见 internal/tools/pentest/fingerprint_store.go：
// DataDir/fingerprints/cms_fingerprints.json（首次启动从内置库播种），
// 删除/恢复/导入前自动备份到 DataDir/fingerprints/backups/。
// 与 vulnlib 同模式：main.go 打开失败时降级为 nil，绑定返回错误而非崩溃。

import (
	"errors"

	"ctfnet/internal/tools/pentest"
)

// fpStore 取指纹管理库；未初始化时返回错误。
func (a *App) fpStore() (*pentest.FingerprintStore, error) {
	st := pentest.ActiveFingerprintStore()
	if st == nil {
		return nil, errors.New("指纹库管理不可用：管理库未初始化（应用启动时打开失败）")
	}
	return st, nil
}

// FingerprintDBStats 指纹库状态（路径/版本/计数/备份数）。
func (a *App) FingerprintDBStats() (*pentest.FPDBStats, error) {
	st, err := a.fpStore()
	if err != nil {
		return nil, err
	}
	s := st.Stats()
	return &s, nil
}

// FingerprintDBList 按 cms_id/name 模糊搜索 + 分页（按 cms_id 排序）。
func (a *App) FingerprintDBList(query string, offset int, limit int) (pentest.FPDBListResult, error) {
	st, err := a.fpStore()
	if err != nil {
		return pentest.FPDBListResult{}, err
	}
	return st.List(query, offset, limit)
}

// FingerprintDBGet 取单个 CMS 的完整编辑视图。
func (a *App) FingerprintDBGet(cmsID string) (*pentest.FPCmsDetail, error) {
	st, err := a.fpStore()
	if err != nil {
		return nil, err
	}
	return st.Get(cmsID)
}

// FingerprintDBSave 新建/覆盖一个 CMS 条目（含逐行校验，失败返回问题清单）。
func (a *App) FingerprintDBSave(cmsID string, entry pentest.FPCmsDetail) error {
	st, err := a.fpStore()
	if err != nil {
		return err
	}
	return st.Save(cmsID, entry)
}

// FingerprintDBDelete 删除一个 CMS 条目（删除前自动备份）。
func (a *App) FingerprintDBDelete(cmsID string) error {
	st, err := a.fpStore()
	if err != nil {
		return err
	}
	return st.Delete(cmsID)
}

// FingerprintDBRestore 恢复为内置库（当前库先备份）。返回恢复后的状态。
func (a *App) FingerprintDBRestore() (*pentest.FPDBStats, error) {
	st, err := a.fpStore()
	if err != nil {
		return nil, err
	}
	if err := st.RestoreEmbedded(); err != nil {
		return nil, err
	}
	s := st.Stats()
	return &s, nil
}

// FingerprintDBImport 从外部 cms_fingerprints.json 合并条目（覆盖同名 id、跳过非法条目）。
func (a *App) FingerprintDBImport(path string) (pentest.FPImportResult, error) {
	st, err := a.fpStore()
	if err != nil {
		return pentest.FPImportResult{}, err
	}
	return st.ImportFrom(path)
}

// FingerprintDBExport 把当前管理库导出到指定 JSON 路径。
func (a *App) FingerprintDBExport(path string) error {
	st, err := a.fpStore()
	if err != nil {
		return err
	}
	return st.ExportTo(path)
}
