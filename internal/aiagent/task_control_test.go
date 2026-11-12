package aiagent

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphEdges：探索图边随黑板写入自动生成（spawns/targets/yields/affects/proves）。
func TestGraphEdges(t *testing.T) {
	task := NewTask("g-1", "探测目标", "确认可达", "")
	task.AddAsset(Asset{Type: "url", URL: "http://10.0.0.5:8080"}, 0) // asset #1
	ids := task.AddIntents([]string{"探测 8080 服务"}, nil, []int{1}, 1)
	if len(ids) != 1 {
		t.Fatalf("AddIntents: %v", ids)
	}
	fid := task.AddFact("端口 8080 开放 nginx", "service", "worker", ids[0])
	aid := task.AddAsset(Asset{Type: "service", Props: map[string]string{"port": "8080"}}, ids[0])
	fnd := Finding{Title: "默认后台未授权", VulnClass: "unauth", Severity: SevHigh, IntentID: ids[0], AssetIDs: []int{1, aid}}
	fndID := task.AddFinding(fnd)
	if err := task.ProveGoal(1, "探测完成", "事实 #"+itoa(fid), ids[0]); err != nil {
		t.Fatalf("ProveGoal: %v", err)
	}
	if err := task.ProveGoal(2, "不存在", "", 0); err == nil {
		t.Fatal("未知目标应报错")
	}

	want := map[string]bool{
		NodeKey("asset", 1) + "->" + NodeKey("intent", ids[0]) + ":targets":      true,
		NodeKey("intent", ids[0]) + "->" + NodeKey("fact", fid) + ":yields":      true,
		NodeKey("intent", ids[0]) + "->" + NodeKey("asset", aid) + ":yields":     true,
		NodeKey("intent", ids[0]) + "->" + NodeKey("finding", fndID) + ":yields": true,
		NodeKey("finding", fndID) + "->" + NodeKey("asset", 1) + ":affects":      true,
		NodeKey("finding", fndID) + "->" + NodeKey("asset", aid) + ":affects":    true,
		NodeKey("intent", ids[0]) + "->" + NodeKey("goal", 1) + ":proves":        true,
	}
	got := map[string]bool{}
	for _, e := range task.Edges {
		got[e.From+"->"+e.To+":"+e.Rel] = true
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("缺少边 %s，实际: %+v", k, task.Edges)
		}
	}
	// 子意图 spawns
	ids2 := task.AddIntents([]string{"深挖后台"}, ids, nil, 2)
	for _, e := range task.Edges {
		if e.From == NodeKey("intent", ids[0]) && e.To == NodeKey("intent", ids2[0]) && e.Rel == "spawns" {
			return
		}
	}
	t.Fatalf("缺少 spawns 边: %+v", task.Edges)
}

// TestTaskControl：暂停/恢复/舵向/重跑/用量记账。
func TestTaskControl(t *testing.T) {
	task := NewTask("c-1", "d", "g", "")
	ids := task.AddIntents([]string{"意图 A"}, nil, nil, 1)

	if task.Paused {
		t.Fatal("新任务不应暂停")
	}
	task.Pause()
	if !task.Paused {
		t.Fatal("Pause 未生效")
	}
	task.Resume()
	if task.Paused {
		t.Fatal("Resume 未生效")
	}

	// 舵向注入 → 一次性消费
	if err := task.SteerIntent(ids[0], "优先看 /admin"); err != nil {
		t.Fatalf("SteerIntent: %v", err)
	}
	if err := task.SteerIntent(999, "x"); err == nil {
		t.Fatal("未知意图应报错")
	}
	if got := task.takeSteer(ids[0]); got != "优先看 /admin" {
		t.Fatalf("takeSteer: %q", got)
	}
	if got := task.takeSteer(ids[0]); got != "" {
		t.Fatalf("舵向应一次性消费，再次取到: %q", got)
	}

	// 重跑终态意图
	task.settleIntent(ids[0], IntentExhausted, "没找到路")
	if err := task.RerunIntent(ids[0]); err != nil {
		t.Fatalf("RerunIntent: %v", err)
	}
	snap := task.Snapshot()
	if snap.Intents[0].State != IntentOpen || snap.Intents[0].Worker != "" {
		t.Fatalf("重跑后应为 open 且无 worker: %+v", snap.Intents[0])
	}
	if err := task.RerunIntent(999); err == nil {
		t.Fatal("未知意图重跑应报错")
	}

	// 用量记账（按天聚合）
	task.AddUsage(100, 50)
	task.AddUsage(25, 25)
	if task.Usage.In != 125 || task.Usage.Out != 75 {
		t.Fatalf("用量累计错误: %+v", task.Usage)
	}
	if len(task.UsageByDay) != 1 || task.UsageByDay[0].Usage.In != 125 {
		t.Fatalf("按天用量错误: %+v", task.UsageByDay)
	}
}

// TestStatsAggregation：跨任务聚合（扫描 taskRoot 目录）。
func TestStatsAggregation(t *testing.T) {
	root := t.TempDir()
	t1 := NewTask("t1", "任务一", "目标一", filepath.Join(root, "t1"))
	t1.AddFinding(Finding{Title: "SQL 注入", VulnClass: "sqli", Severity: SevHigh, Evidence: "e", CreatedAt: "2026-11-12 10:00:00"})
	t1.AddAsset(Asset{Type: "domain", Domain: "a.example.com"}, 0)
	t1.AddFact("f1", "info", "worker", 0)
	t1.AddUsage(1000, 500)

	t2 := NewTask("t2", "任务二", "目标二", filepath.Join(root, "t2"))
	t2.AddFinding(Finding{Title: "XSS", VulnClass: "xss", Severity: SevMedium, Evidence: "e", CreatedAt: "2026-11-12 11:00:00"})
	t2.AddUsage(300, 100)
	t2.Log("system", "terminal", "任务二完成")
	t2.FinishDone("完成")
	t1.Log("system", "dispatch", "任务一派发意图")

	sums, err := ScanAITasks(root)
	if err != nil || len(sums) != 2 {
		t.Fatalf("ScanAITasks: %v %+v", err, sums)
	}
	byID := map[string]AITaskSummary{}
	for _, s := range sums {
		byID[s.ID] = s
	}
	if byID["t1"].Findings != 1 || byID["t1"].Assets != 1 || byID["t1"].Usage.In != 1000 {
		t.Fatalf("任务一摘要错误: %+v", byID["t1"])
	}
	if byID["t2"].Status != StatusDone {
		t.Fatalf("任务二状态错误: %+v", byID["t2"])
	}

	findings, err := AggregateFindings(root)
	if err != nil || len(findings) != 2 {
		t.Fatalf("AggregateFindings: %v %+v", err, findings)
	}
	if findings[0].Finding.Title != "XSS" { // 时间倒序
		t.Fatalf("发现排序错误: %+v", findings)
	}
	if findings[1].TaskID != "t1" || findings[1].Finding.Title != "SQL 注入" {
		t.Fatalf("发现任务归属错误: %+v", findings[1])
	}

	assets, err := AggregateAssets(root)
	if err != nil || len(assets) != 1 || assets[0].Asset.Domain != "a.example.com" {
		t.Fatalf("AggregateAssets: %v %+v", err, assets)
	}

	stats, err := GlobalStats(root)
	if err != nil {
		t.Fatalf("GlobalStats: %v", err)
	}
	if stats.Tasks != 2 || stats.Findings != 2 || stats.Assets != 1 ||
		stats.Usage.In != 1300 || stats.Usage.Out != 600 || stats.Done != 1 || stats.Running != 1 {
		t.Fatalf("总览错误: %+v", stats)
	}
	if len(stats.ByDay) != 1 {
		t.Fatalf("按天曲线错误: %+v", stats.ByDay)
	}

	acts, err := RecentActivities(root, 10)
	if err != nil || len(acts) == 0 {
		t.Fatalf("RecentActivities: %v %+v", err, acts)
	}
	if strings.TrimSpace(acts[0].Activity.Summary) == "" {
		t.Fatalf("活动流为空摘要: %+v", acts[0])
	}

	// 镜像 + 读回
	if err := MirrorTaskJSON(filepath.Join(root, "t1"), filepath.Join(root, "mirror")); err != nil {
		t.Fatalf("MirrorTaskJSON: %v", err)
	}
	m, err := LoadTask(filepath.Join(root, "mirror"))
	if err != nil || m == nil || len(m.Findings) != 1 {
		t.Fatalf("镜像读回失败: %v %+v", err, m)
	}
}
