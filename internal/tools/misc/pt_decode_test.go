package misc

// misc.pt_decode 工具测试:金标准 live 对拍(样本缺失即跳过)。
// 加密引擎的向量/往返/坏输入测试见 pt_crypto_test.go。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

const ptSample = `C:\Users\HP\Desktop\安全方向\CTF工作台-plus\思科\1.txt`
const ptGolden = `C:\Users\HP\Desktop\安全方向\CTF工作台-plus\pt_tools\challenge.xml`

func TestPTDecodeGolden(t *testing.T) {
	data, err := os.ReadFile(ptSample)
	if err != nil {
		t.Skipf("样本不存在,跳过: %v", err)
	}
	tool, ok := core.DefaultRegistry.Get("misc.pt_decode")
	if !ok {
		t.Fatal("misc.pt_decode 未注册")
	}
	work := t.TempDir()
	arts := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "1.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := tool.Run(context.Background(), map[string]any{"file": "1.txt"}, &core.ToolContext{Workdir: work, ArtifactsDir: arts})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("运行失败: %v %s", err, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "flag{wenzi_houzhui_sike}") {
		t.Fatalf("报告应含金标准 flag:\n%s", res.Stdout[:400])
	}
	if !strings.Contains(res.Stdout, "password flag{wenzi_houzhui_sike}") {
		t.Fatalf("报告应含敏感配置行(password 即 flag):\n%s", res.Stdout[:400])
	}
	if !strings.Contains(res.Stdout, "7.3.0.0838") {
		t.Fatalf("报告应含 PT 版本")
	}
	if !strings.Contains(res.Stdout, "设备(") {
		t.Fatalf("报告应含设备清单(真实 PT XML 用大写 <NAME translate> 标签)")
	}
	// decoded.xml 产物与金标准逐字节一致
	gold, _ := os.ReadFile(ptGolden)
	got, err := os.ReadFile(filepath.Join(arts, "decoded.xml"))
	if err != nil {
		t.Fatalf("decoded.xml 未落盘: %v", err)
	}
	if len(got) != len(gold) || string(got) != string(gold) {
		t.Fatalf("decoded.xml 与金标准不一致: got %d B, gold %d B", len(got), len(gold))
	}
}

// 非 PT 文件 → 友好错误(不 panic)。
func TestPTDecodeBadFile(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("misc.pt_decode")
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "x.bin"), []byte("plain text not pt"), 0o644)
	res, err := tool.Run(context.Background(), map[string]any{"file": "x.bin"}, &core.ToolContext{Workdir: work, ArtifactsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("应返回错误结果而非异常: %v", err)
	}
	if res.Status == core.StatusSuccess {
		t.Fatal("非 PT 文件不应成功")
	}
	if !strings.Contains(res.Stderr, "Packet Tracer") {
		t.Fatalf("错误应友好说明: %s", res.Stderr)
	}
}
