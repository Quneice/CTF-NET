package main

// 漏洞库的 Wails 绑定（挂在 App 上）。
// 存储实现见 internal/vulnlib/store.go（SQLite，DataDir/vulnlib.db）。
// 与 projects/wiki 同模式：main.go 打开失败时降级为 nil，绑定返回错误而非崩溃。

import (
	"errors"

	"ctfnet/internal/vulnlib"
)

// vulnStore 取漏洞库；未初始化时返回错误。
func (a *App) vulnStore() (*vulnlib.Store, error) {
	st := vulnlib.DB()
	if st == nil {
		return nil, errors.New("漏洞库不可用：数据库打开失败")
	}
	return st, nil
}

// VulnLibStats 漏洞库状态（是否已导入、根目录、案例数、上次扫描时间）。
func (a *App) VulnLibStats() (*vulnlib.Stats, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	s, err := st.Stats()
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ScanVulnLibrary 扫描漏洞清单目录并导入（增量：mtime+size 未变的文件跳过）。
// dir 例如 C:\Users\HP\Desktop\安全方向\漏洞复现案例；自动跳过 My_template 模板目录。
// 首次全量扫描 6000+ 文件约需数秒，绑定同步返回摘要，前端需展示加载态。
func (a *App) ScanVulnLibrary(dir string) (*vulnlib.ScanSummary, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	sum, err := st.Scan(dir)
	if err != nil {
		return nil, err
	}
	return &sum, nil
}

// ListVulnCategories 全部分类（Z-编号）及系统级案例计数。
func (a *App) ListVulnCategories() ([]*vulnlib.Category, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	cats, err := st.ListCategories()
	if err != nil {
		return nil, err
	}
	out := make([]*vulnlib.Category, 0, len(cats))
	for i := range cats {
		out = append(out, &cats[i])
	}
	return out, nil
}

// ListVulnCases 某分类（+可选系统）下的案例摘要；sys 为空表示分类下全部。
func (a *App) ListVulnCases(catID, sys string) ([]*vulnlib.CaseSummary, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	cases, err := st.ListCases(catID, sys)
	if err != nil {
		return nil, err
	}
	out := make([]*vulnlib.CaseSummary, 0, len(cases))
	for i := range cases {
		out = append(out, &cases[i])
	}
	return out, nil
}

// SearchVulnCases 关键字搜索（标题/文件名/CVE）；catID 为空表示全库。
func (a *App) SearchVulnCases(keyword, catID string, limit, offset int) ([]*vulnlib.CaseSummary, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	cases, err := st.Search(keyword, catID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*vulnlib.CaseSummary, 0, len(cases))
	for i := range cases {
		out = append(out, &cases[i])
	}
	return out, nil
}

// GetVulnCase 案例全文（含 md 内容与提取出的原始请求包）。
func (a *App) GetVulnCase(id string) (*vulnlib.CaseDetail, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	return st.GetCase(id)
}

// GetVulnAsset 读取案例 md 引用的本地资源文件（图片等），返回 base64 供前端
// data URL 展示。rel 为 md 中的引用（相对路径/Windows 绝对路径，越界拒绝）。
func (a *App) GetVulnAsset(caseID, rel string) (*vulnlib.AssetData, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	return st.GetAsset(caseID, rel)
}

// CreateVulnCase 新增案例：写入分类/系统目录下的 md 文件并导入。
// catID 为「其他」(__root__) 时 sys 忽略；文件名自动补 .md。
func (a *App) CreateVulnCase(catID, sys, name, content string) (*vulnlib.ScanSummary, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	return st.CreateCase(catID, sys, name, content)
}

// UpdateVulnCase 保存案例修改：覆写磁盘 md 文件并重新导入。
func (a *App) UpdateVulnCase(id, content string) (*vulnlib.ScanSummary, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	return st.UpdateCase(id, content)
}

// DeleteVulnCase 删除案例：md 文件移入 data/backups/vulnlib_deleted（可恢复）。
func (a *App) DeleteVulnCase(id string) (*vulnlib.ScanSummary, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	return st.DeleteCase(id)
}

// FetchVulnRemoteImage 拉取远程图片（http/https），经全局代理（设置页→代理，
// data/config.json 的 proxy.url）转发为 base64；未配置代理时直连。
func (a *App) FetchVulnRemoteImage(rawurl string) (*vulnlib.AssetData, error) {
	st, err := a.vulnStore()
	if err != nil {
		return nil, err
	}
	return st.FetchRemoteImage(rawurl)
}
