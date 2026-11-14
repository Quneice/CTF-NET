package aisec

// 临时冒烟测试: 直接吃用户提供的 Desktop/test.xlsx 真文件(跑完删除本文件)。

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ctfnet/internal/core"
)

const liveXLSX = `C:\Users\HP\Desktop\test.xlsx`

func TestLiveSmokeRealXLSX(t *testing.T) {
	if _, err := os.Stat(liveXLSX); err != nil {
		t.Skip("真实 xlsx 不存在")
	}
	dir := t.TempDir()
	tc := &core.ToolContext{Workdir: dir}

	scenarios := []struct {
		name     string
		triggers string
		pair     string
	}{
		{"验证一_双触发器翻转", "5f30GROx=ham\nw0qMI2hB=spam", ""},
		{"验证二_验证码投毒", "验证码=spam", "验证码=的,是,您,请,输入,分钟,尊敬,客户,有效,登录"},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			res, err := poisonGenRun(context.Background(), map[string]any{
				"file":     liveXLSX,
				"triggers": sc.triggers,
				"pair":     sc.pair,
				"budget":   50,
				"maxlen":   20,
			}, tc)
			if err != nil || res.Status != core.StatusSuccess {
				t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
			}
			t.Logf("\n%s", res.Stdout)
			if _, err := os.Stat(filepath.Join(dir, "poison.csv")); err != nil {
				t.Fatalf("poison.csv 缺失: %v", err)
			}
		})
	}
}
