package main

// 红队 Wiki 的 Wails 绑定（挂在 App 上）。
// 存储实现见 internal/wiki/store.go（SQLite，DataDir/wiki.db）。

import (
	"errors"

	"ctfnet/internal/wiki"
)

// wikiStore 取 wiki 库；打开失败（main.go 降级为 nil）时返回错误。
func (a *App) wikiStore() (*wiki.Store, error) {
	if a.Wiki == nil {
		return nil, errors.New("红队 Wiki 不可用：数据库打开失败")
	}
	return a.Wiki, nil
}

// ListWikiEntries 列出条目；category 为空串表示全部。
func (a *App) ListWikiEntries(category string) ([]*wiki.Entry, error) {
	s, err := a.wikiStore()
	if err != nil {
		return nil, err
	}
	return s.List(category)
}

// CreateWikiEntry 新增条目。
func (a *App) CreateWikiEntry(category, title, content string) (*wiki.Entry, error) {
	s, err := a.wikiStore()
	if err != nil {
		return nil, err
	}
	return s.Create(category, title, content)
}

// UpdateWikiEntry 更新条目标题/内容。
func (a *App) UpdateWikiEntry(id, title, content string) (*wiki.Entry, error) {
	s, err := a.wikiStore()
	if err != nil {
		return nil, err
	}
	return s.Update(id, title, content)
}

// DeleteWikiEntry 删除条目。
func (a *App) DeleteWikiEntry(id string) error {
	s, err := a.wikiStore()
	if err != nil {
		return err
	}
	return s.Delete(id)
}
