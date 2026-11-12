package main

import (
	"context"
	"embed"
	"log"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ctfnet/internal/core"
	"ctfnet/internal/projects"
	_ "ctfnet/internal/tools" // 触发所有工具插件 init() 注册
	"ctfnet/internal/tools/pentest"
	"ctfnet/internal/vulnlib"
	"ctfnet/internal/wiki"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg := core.LoadConfig()
	executor := core.BuildExecutor(cfg)
	fileStore := core.NewFileStore(cfg.UploadsDir())
	sessions := core.NewSessionRegistry()
	tasks := core.NewTaskManager(executor, cfg.WorkDir(), cfg.ArtifactsDir(), fileStore, sessions)

	// 渗透测试项目库（SQLite）：打开失败不阻断工具箱主功能，仅禁用项目管理
	var projectStore *projects.Store
	if ps, err := projects.Open(filepath.Join(cfg.DataDir, "projects.db")); err != nil {
		log.Printf("[CTF-NET] 项目库打开失败（项目管理不可用）: %v", err)
	} else {
		projectStore = ps
		defer projectStore.Close()
	}

	// 红队 Wiki 库（SQLite）：打开失败仅禁用 Wiki；首次打开时灌入内置速查数据
	var wikiStore *wiki.Store
	if ws, err := wiki.Open(filepath.Join(cfg.DataDir, "wiki.db")); err != nil {
		log.Printf("[CTF-NET] Wiki 库打开失败（红队 Wiki 不可用）: %v", err)
	} else {
		wikiStore = ws
		defer wikiStore.Close()
		if n, err := ws.Count(); err == nil && n == 0 {
			if err := ws.Seed(wiki.BuiltinSeed); err != nil {
				log.Printf("[CTF-NET] 红队 Wiki 内置数据灌入失败: %v", err)
			} else {
				log.Printf("[CTF-NET] 红队 Wiki 已灌入 %d 条内置速查", len(wiki.BuiltinSeed))
			}
		}
	}

	// 漏洞库（SQLite）：打开失败仅禁用通用 PoC 检测的库浏览，不阻断主功能
	if err := vulnlib.Init(filepath.Join(cfg.DataDir, "vulnlib.db")); err != nil {
		log.Printf("[CTF-NET] 漏洞库打开失败（通用 PoC 检测库不可用）: %v", err)
	}

	// 指纹管理库：首次启动从内置库播种到 DataDir/fingerprints；打开失败回退内置库
	// （识别仍可用，仅指纹库管理 GUI 不可用）
	if st, err := pentest.OpenFingerprintStore(filepath.Join(cfg.DataDir, "fingerprints")); err != nil {
		log.Printf("[CTF-NET] 指纹管理库打开失败（指纹库管理不可用，识别回退内置库）: %v", err)
	} else {
		pentest.SetActiveFingerprintStore(st)
	}

	app := &App{
		Reg:      core.DefaultRegistry,
		Tasks:    tasks,
		Sessions: sessions,
		Files:    fileStore,
		Projects: projectStore,
		Wiki:     wikiStore,
		Cfg:      cfg,
	}

	err := wails.Run(&options.App{
		Title:     "CTF-NET",
		Width:     1400,
		Height:    900,
		MinWidth:  1000,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.ctx = ctx // 流式对话事件（chat:*）与任务终态事件推送共用
			// 任务终态事件推送：前端订阅 task:done 即时结算，替代纯 2s 轮询
			tasks.OnJobDone = func(j *core.Job) {
				wailsRuntime.EventsEmit(ctx, "task:done", j.ToDict())
			}
			log.Printf("[CTF-NET] 启动完成：工具 %d 个，数据目录 %s", core.DefaultRegistry.ToolCount(), cfg.DataDir)
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
