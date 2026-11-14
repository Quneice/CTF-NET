package wiki

import "testing"

// 逆向 Wiki 内置种子完整性:8 分类、总数 44、每类条数、无空字段、无重复标题。
var reverseCategoryCounts = map[string]int{
	"algo": 4, "tools": 4, "antidebug": 5, "unpack": 3,
	"formats": 4, "functions": 21, "ascii": 2, "notes": 1,
}

func TestReverseBuiltinSeedCounts(t *testing.T) {
	total := 0
	counts := map[string]int{}
	titles := map[string]bool{}
	for _, e := range ReverseBuiltinSeed {
		if e.Category == "" || e.Title == "" || e.Content == "" {
			t.Fatalf("空字段: %+v", e)
		}
		if titles[e.Title] {
			t.Fatalf("重复标题: %s", e.Title)
		}
		titles[e.Title] = true
		counts[e.Category]++
		total++
	}
	if total != 44 {
		t.Fatalf("总条数 = %d, 期望 44", total)
	}
	for cat, want := range reverseCategoryCounts {
		if counts[cat] != want {
			t.Errorf("分类 %s = %d 条, 期望 %d", cat, counts[cat], want)
		}
	}
	if len(counts) != 8 {
		t.Fatalf("分类数 = %d, 期望 8", len(counts))
	}
}

// 锚点条目必须存在(标题精确匹配,与前端 mock 侧对齐)。
func TestReverseBuiltinSeedAnchors(t *testing.T) {
	anchors := []string{"_acrt_iob_func", "IsDebuggerPresent", "UPX"}
	have := map[string]bool{}
	for _, e := range ReverseBuiltinSeed {
		have[e.Title] = true
	}
	for _, a := range anchors {
		if !have[a] {
			t.Errorf("锚点条目缺失: %s", a)
		}
	}
}
