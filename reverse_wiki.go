package main

// 逆向 Wiki 的 Wails 绑定（挂在 App 上）。
// 与 wiki.go 同款架构，独立库文件 DataDir/reverse_wiki.db；
// 种子为 internal/wiki/reverse_seed.go 的 ReverseBuiltinSeed。

import (
	"errors"

	"ctfnet/internal/wiki"
)

// reverseWikiStore 取逆向 wiki 库；打开失败（main.go 降级为 nil）时返回错误。
func (a *App) reverseWikiStore() (*wiki.Store, error) {
	if a.ReverseWiki == nil {
		return nil, errors.New("逆向 Wiki 不可用：数据库打开失败")
	}
	return a.ReverseWiki, nil
}

// ListReverseWikiEntries 列出条目；category 为空串表示全部。
func (a *App) ListReverseWikiEntries(category string) ([]*wiki.Entry, error) {
	s, err := a.reverseWikiStore()
	if err != nil {
		return nil, err
	}
	return s.List(category)
}

// CreateReverseWikiEntry 新增条目。
func (a *App) CreateReverseWikiEntry(category, title, content string) (*wiki.Entry, error) {
	s, err := a.reverseWikiStore()
	if err != nil {
		return nil, err
	}
	return s.Create(category, title, content)
}

// UpdateReverseWikiEntry 更新条目标题/内容。
func (a *App) UpdateReverseWikiEntry(id, title, content string) (*wiki.Entry, error) {
	s, err := a.reverseWikiStore()
	if err != nil {
		return nil, err
	}
	return s.Update(id, title, content)
}

// DeleteReverseWikiEntry 删除条目。
func (a *App) DeleteReverseWikiEntry(id string) error {
	s, err := a.reverseWikiStore()
	if err != nil {
		return err
	}
	return s.Delete(id)
}
