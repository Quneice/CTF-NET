package main

import (
	"ctfnet/internal/core"
)

// ListDirWordlists 返回项目字典库 wordlists/dir/ 下的字典列表（名称/词数/大小），
// 供目录枚举等工具的「字典选择」下拉使用。
func (a *App) ListDirWordlists() ([]core.WordlistInfo, error) {
	return core.ListWordlists("dir")
}
