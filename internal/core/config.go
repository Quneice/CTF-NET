// 配置：读取 CTF_* 环境变量，默认值与 legacy/app/config.py（pydantic-settings）一致。
package core

import (
	"os"
	"path/filepath"
)

// Config 运行时配置。
type Config struct {
	AppName          string
	DataDir          string
	DefaultTimeout   int
	MaxTimeout       int
	ExecutorBackend  string // local | docker
	DockerImage      string
	DockerNetwork    string
	DockerDataVolume string

	// App 统一应用配置（JNDI/外部工具/字典/执行器），来自 DataDir/config.json。
	App *AppConfig
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// resolveDefaultDataDir：优先 CTF_DATA_DIR；其次 CWD 下已有 data/（开发模式，与旧版
// `data` 相对路径一致）；否则 %LOCALAPPDATA%\CTF-NET（打包应用）。
func resolveDefaultDataDir() string {
	cwd, err := os.Getwd()
	if err == nil {
		cand := filepath.Join(cwd, "data")
		if st, e := os.Stat(cand); e == nil && st.IsDir() {
			return cand
		}
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		return filepath.Join(la, "CTF-NET")
	}
	return filepath.Join(".", "data")
}

// LoadConfig 三段合并加载：内置默认值 → config.json → CTF_* 环境变量覆盖执行器组。
func LoadConfig() *Config {
	dataDir := envOr("CTF_DATA_DIR", resolveDefaultDataDir())
	app := LoadAppConfig(dataDir)

	// 环境变量最终覆盖 executor 组（与 legacy 行为一致）
	app.Executor.Backend = envOr("CTF_EXECUTOR_BACKEND", app.Executor.Backend)
	app.Executor.DockerImage = envOr("CTF_DOCKER_IMAGE", app.Executor.DockerImage)
	app.Executor.DockerNetwork = envOr("CTF_DOCKER_NETWORK", app.Executor.DockerNetwork)
	app.Executor.DockerDataVol = envOr("CTF_DOCKER_DATA_VOLUME", app.Executor.DockerDataVol)

	// 发布为运行时快照，工具层 GetAppConfig() 即可读
	SetAppConfig(app)

	return &Config{
		AppName:          "CTF Toolbox",
		DataDir:          dataDir,
		DefaultTimeout:   app.Executor.DefaultTimeout,
		MaxTimeout:       app.Executor.MaxTimeout,
		ExecutorBackend:  app.Executor.Backend,
		DockerImage:      app.Executor.DockerImage,
		DockerNetwork:    app.Executor.DockerNetwork,
		DockerDataVolume: app.Executor.DockerDataVol,
		App:              app,
	}
}

// ArtifactsDir 产物根目录。
func (c *Config) ArtifactsDir() string { return filepath.Join(c.DataDir, "artifacts") }

// WorkDir 任务工作目录根。
func (c *Config) WorkDir() string { return filepath.Join(c.DataDir, "work") }

// UploadsDir 上传文件目录。
func (c *Config) UploadsDir() string { return filepath.Join(c.DataDir, "uploads") }

// BuildExecutor 按配置构建执行器（对应 app/main.py build_executor）。
func BuildExecutor(cfg *Config) Executor {
	if cfg.ExecutorBackend == "docker" {
		return &DockerExecutor{
			Image:      cfg.DockerImage,
			Network:    cfg.DockerNetwork,
			DataVolume: cfg.DockerDataVolume,
			DataRoot:   cfg.DataDir,
		}
	}
	return &SubprocessExecutor{}
}
