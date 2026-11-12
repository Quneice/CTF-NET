// 统一应用配置：DataDir/config.json（JNDI 服务器 / 外部工具路径 / 字典 / 执行器）。
//
// 加载优先级：内置默认值 → config.json 覆盖 → CTF_* 环境变量覆盖 executor 组
// （环境变量覆盖在 config.go LoadConfig 中完成，保持与 legacy 行为一致）。
// 运行时读取走 GetAppConfig() 原子快照，设置页保存即生效；手改 JSON 需重启。
package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
)

// —— 配置分组结构体（JSON 键蛇形，与前端设置页一一对应）——

// JndiConfig JNDI 注入工具的默认连接与监听配置。
type JndiConfig struct {
	SSHHost     string `json:"ssh_host"`     // 攻击服务器 IP/域名
	SSHPort     int    `json:"ssh_port"`     // SSH 端口
	SSHUser     string `json:"ssh_user"`     // SSH 用户名
	SSHPass     string `json:"ssh_pass"`     // SSH 密码
	LHost       string `json:"lhost"`        // 回连公网 IP（空则取 ssh_host）
	LDAPPort    int    `json:"ldap_port"`    // LDAP 引用服务端口
	RMIPort     int    `json:"rmi_port"`     // RMI 注册表端口
	HTTPPort    int    `json:"http_port"`    // 恶意类 HTTP 下载端口
	LPort       int    `json:"lport"`        // 反弹 shell 监听端口
	GadgetClass string `json:"gadget_class"` // 默认 JNDI 触发链类
	Shell       string `json:"shell"`        // 反弹 shell 进程路径
}

// ToolsConfig 外部工具/命令路径（留空或命令名表示走 PATH）。
// fscan 由 pentest.scan.port-scan 调用；nmap/nuclei/xray/sqlmap 目前无后端调用点，属前瞻字段。
type ToolsConfig struct {
	Python3  string `json:"python3"`
	Java     string `json:"java"`
	Javac    string `json:"javac"`
	Nmap     string `json:"nmap"`
	Nuclei   string `json:"nuclei"`
	Xray     string `json:"xray"`
	Sqlmap   string `json:"sqlmap"`
	PuttyDir string `json:"putty_dir"`
	Fscan    string `json:"fscan"` // fscan exe 路径（端口扫描工具）
}

// FrpConfig FRP 反向代理工具的服务器连接与部署配置。
// ssh_* 留空时回退 jndi 组的同名字段（复用同一台攻击服务器）。
type FrpConfig struct {
	SSHHost        string `json:"ssh_host"`         // 部署 frps 的服务器 IP/域名（留空回退 jndi.ssh_host）
	SSHPort        int    `json:"ssh_port"`         // SSH 端口（0 回退 jndi.ssh_port）
	SSHUser        string `json:"ssh_user"`         // SSH 用户名
	SSHPass        string `json:"ssh_pass"`         // SSH 密码
	RemoteDir      string `json:"remote_dir"`       // 服务器上的部署目录
	BindPort       int    `json:"bind_port"`        // frps 主监听端口（frpc 的 serverPort）
	Token          string `json:"token"`            // 认证 token（留空则每次运行自动生成）
	VhostHTTPPort  int    `json:"vhost_http_port"`  // vhost HTTP 端口（http 型代理走此端口，80 需 root）
	VhostHTTPSPort int    `json:"vhost_https_port"` // vhost HTTPS 端口（443 需 root）
	DashboardPort  int    `json:"dashboard_port"`   // Dashboard 端口（0 = 禁用）
	DashboardUser  string `json:"dashboard_user"`   // Dashboard 用户名
	DashboardPass  string `json:"dashboard_pass"`   // Dashboard 密码（留空自动生成）
	FrpsVersion    string `json:"frps_version"`     // 下载的 frp 版本号（如 0.61.1）
	DownloadURL    string `json:"download_url"`     // 下载地址模板，占位符 {version} {os} {arch}；留空用 GitHub 官方源 + 镜像回退
}

// WordlistsConfig 字典文件路径（目录枚举/口令破解等）。
type WordlistsConfig struct {
	DirEnum   string `json:"dir_enum"`  // 目录枚举字典
	Password  string `json:"password"`  // 口令破解字典
	Username  string `json:"username"`  // 用户名字典
	Subdomain string `json:"subdomain"` // 子域名字典
}

// ExecutorConfig 执行器与超时（对应原 Config 的同名字段）。
type ExecutorConfig struct {
	DefaultTimeout int    `json:"default_timeout"`
	MaxTimeout     int    `json:"max_timeout"`
	Backend        string `json:"backend"` // local | docker
	DockerImage    string `json:"docker_image"`
	DockerNetwork  string `json:"docker_network"`
	DockerDataVol  string `json:"docker_data_volume"`
}

// AiConfig AI 渗透（ai.pentest.task / ai.pentest.chat）的 LLM 接入与运行配置。
type AiConfig struct {
	APIKey          string `json:"api_key"`          // LLM API Key（OpenAI 兼容服务）
	BaseURL         string `json:"base_url"`         // 如 https://open.bigmodel.cn/api/paas/v4（智谱）
	Model           string `json:"model"`            // 如 glm-5.3-flash
	ReasoningEffort string `json:"reasoning_effort"` // 思考强度 low/high/max（智谱 glm-5.x 始终思考；空=不传）
	MaxTurns        int    `json:"max_turns"`        // 单会话最大工具轮数
	MaxWorkers      int    `json:"max_workers"`      // 并发 worker 数（1-8）
	PlanRounds      int    `json:"plan_rounds"`      // 规划轮次预算
	Timeout         int    `json:"timeout_seconds"`  // 任务总时限（秒）
	EnableExec      bool   `json:"enable_exec"`      // 是否允许 worker 执行本地命令
}

// AppConfig 统一应用配置。
type AppConfig struct {
	Jndi      JndiConfig      `json:"jndi"`
	Frp       FrpConfig       `json:"frp"`
	Tools     ToolsConfig     `json:"tools"`
	Wordlists WordlistsConfig `json:"wordlists"`
	Executor  ExecutorConfig  `json:"executor"`
	Proxy     ProxyConfig     `json:"proxy"`
	Ai        AiConfig        `json:"ai"`
}

// ProxyConfig 全局代理（漏洞浏览器远程图片等出站流量经此转发；空=直连）。
type ProxyConfig struct {
	URL string `json:"url"` // http://… / https://… / socks5://… / socks5h://…
}

// DefaultAppConfig 内置默认值（与 legacy 硬编码一致）。
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Jndi: JndiConfig{
			SSHPort:     22,
			LDAPPort:    1389,
			RMIPort:     1099,
			HTTPPort:    8888,
			LPort:       31339,
			GadgetClass: "com.sun.rowset.JdbcRowSetImpl",
			Shell:       "/bin/sh",
		},
		Frp: FrpConfig{
			SSHPort:        22,
			RemoteDir:      "~/ctfnet-frp",
			BindPort:       7000,
			VhostHTTPPort:  8080,
			VhostHTTPSPort: 8443,
			DashboardPort:  7500,
			DashboardUser:  "admin",
			FrpsVersion:    "0.69.1",
		},
		Tools: ToolsConfig{
			Python3: "python3",
			Java:    "java",
			Javac:   "javac",
			Nmap:    "nmap",
			Nuclei:  "nuclei",
			Xray:    "xray",
			Sqlmap:  "sqlmap",
			Fscan:   `C:\Users\HP\Downloads\fscan_2.2.2_windows_x64.exe`, // 本机 fscan 下载路径，可在设置页改
		},
		Executor: ExecutorConfig{
			DefaultTimeout: 60,
			MaxTimeout:     600,
			Backend:        "local",
			DockerImage:    "ctf-toolbox:latest",
			DockerNetwork:  "none",
		},
		Proxy: ProxyConfig{}, // 默认直连
		Ai: AiConfig{
			BaseURL:    "https://open.bigmodel.cn/api/paas/v4",
			Model:      "glm-5.3-flash",
			MaxTurns:   48,
			MaxWorkers: 2,
			PlanRounds: 6,
			Timeout:    3600,
			EnableExec: true,
		},
	}
}

// ConfigFileName 配置文件名（位于 DataDir 下）。
const ConfigFileName = "config.json"

// —— 加载 / 保存 ——

// LoadAppConfig 读取 dataDir/config.json；文件不存在时用默认值并落盘一份默认副本
// （便于用户手改），解析失败时把损坏文件备份为 .bak 后回退默认值，均不阻断启动。
// 文件存在但缺少新版本新增的配置组（如 frp）时，自动按默认模板补齐并回写。
func LoadAppConfig(dataDir string) *AppConfig {
	path := filepath.Join(dataDir, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		cfg := DefaultAppConfig()
		_ = SaveAppConfig(dataDir, cfg) // 写默认副本，失败不致命
		return cfg
	}
	cfg := DefaultAppConfig() // 先填默认值，文件只覆盖出现的字段
	if err := json.Unmarshal(data, cfg); err != nil {
		_ = os.Rename(path, path+".bak")
		_ = SaveAppConfig(dataDir, cfg)
		return cfg
	}
	migrateConfigFile(dataDir, data)
	return cfg
}

// migrateConfigFile 检测 config.json 是否缺少默认模板中的键；缺失则以默认值补齐
// 并原子回写，保证 config.json 始终是完整模板（JSON 层深合并：用户已填的值与
// 模板外的自定义键原样保留）。任何一步失败都静默放弃，不阻断启动。
func migrateConfigFile(dataDir string, fileData []byte) {
	var fileMap map[string]json.RawMessage
	if json.Unmarshal(fileData, &fileMap) != nil {
		return
	}
	defData, err := json.Marshal(DefaultAppConfig())
	if err != nil {
		return
	}
	var defMap map[string]json.RawMessage
	if json.Unmarshal(defData, &defMap) != nil {
		return
	}
	if !configMissingKeys(fileMap, defMap) {
		return
	}
	merged, err := json.MarshalIndent(mergeConfigJSON(defMap, fileMap), "", "  ")
	if err != nil {
		return
	}
	_ = saveAppConfigData(dataDir, merged)
}

// configMissingKeys 递归检查 file 是否缺少 def 中存在的键。
func configMissingKeys(file, def map[string]json.RawMessage) bool {
	for k, dv := range def {
		fv, ok := file[k]
		if !ok {
			return true
		}
		var fMap, dMap map[string]json.RawMessage
		if json.Unmarshal(dv, &dMap) == nil {
			if json.Unmarshal(fv, &fMap) != nil {
				return true
			}
			if configMissingKeys(fMap, dMap) {
				return true
			}
		}
	}
	return false
}

// mergeConfigJSON JSON 层深合并：file 的值覆盖 def，双方独有的键都保留。
func mergeConfigJSON(def, file map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(def)+len(file))
	for k, v := range def {
		out[k] = v
	}
	for k, v := range file {
		dv, exists := out[k]
		if !exists {
			out[k] = v
			continue
		}
		var dMap, fMap map[string]json.RawMessage
		if json.Unmarshal(dv, &dMap) == nil && json.Unmarshal(v, &fMap) == nil {
			if merged, err := json.Marshal(mergeConfigJSON(dMap, fMap)); err == nil {
				out[k] = merged
				continue
			}
		}
		out[k] = v
	}
	return out
}

// saveAppConfigData 原子写配置字节（tmp + rename，0600 防凭据泄露）。
func saveAppConfigData(dataDir string, data []byte) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dataDir, ConfigFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil { // 可能含凭据，限制权限
		return err
	}
	return os.Rename(tmp, path)
}

// SaveAppConfig 原子写入配置（tmp + rename），MarshalIndent 便于手改。
func SaveAppConfig(dataDir string, cfg *AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return saveAppConfigData(dataDir, data)
}

// —— 运行时原子快照 ——

var currentAppConfig atomic.Value // 存 *AppConfig

// GetDataDir 返回应用数据目录（CTF_DATA_DIR 优先；与 resolveDefaultDataDir 同规则，
// 供工具层定位配方目录等数据文件）。
func GetDataDir() string { return envOr("CTF_DATA_DIR", resolveDefaultDataDir()) }

// GetAppConfig 返回当前配置快照（未初始化时返回内置默认值，保证工具层随时可用）。
func GetAppConfig() *AppConfig {
	if v := currentAppConfig.Load(); v != nil {
		return v.(*AppConfig)
	}
	return DefaultAppConfig()
}

// SetAppConfig 整体替换配置快照（保存设置后调用，立即生效）。
func SetAppConfig(cfg *AppConfig) {
	currentAppConfig.Store(cfg)
}

// —— 工具层接入 ——

// jndi 参数名 → 配置值 的回填映射（空字符串参数才回填）。
func (c *AppConfig) jndiDefaults() map[string]any {
	return map[string]any{
		"ssh_host":     c.Jndi.SSHHost,
		"ssh_port":     c.Jndi.SSHPort,
		"ssh_user":     c.Jndi.SSHUser,
		"ssh_pass":     c.Jndi.SSHPass,
		"lhost":        c.Jndi.LHost,
		"ldap_port":    c.Jndi.LDAPPort,
		"rmi_port":     c.Jndi.RMIPort,
		"http_port":    c.Jndi.HTTPPort,
		"lport":        c.Jndi.LPort,
		"gadget_class": c.Jndi.GadgetClass,
		"shell":        c.Jndi.Shell,
		"putty_dir":    c.Tools.PuttyDir,
	}
}

// aiDefaults ai.pentest.task / ai.pentest.chat 参数回填映射（AI 配置组）。
func (c *AppConfig) aiDefaults() map[string]any {
	return map[string]any{
		"api_key":         c.Ai.APIKey,
		"base_url":        c.Ai.BaseURL,
		"model":           c.Ai.Model,
		"max_turns":       c.Ai.MaxTurns,
		"max_workers":     c.Ai.MaxWorkers,
		"plan_rounds":     c.Ai.PlanRounds,
		"timeout_seconds": c.Ai.Timeout,
		"enable_exec":     c.Ai.EnableExec,
	}
}

// ApplyDefaults 按工具白名单把配置值回填进空参数（RunTool 提交前调用）。
// 只回填未提供或空字符串的参数，用户显式填写的值优先。
func ApplyDefaults(toolID string, params map[string]any) {
	var defaults map[string]any
	switch toolID {
	case "web.jndi":
		defaults = GetAppConfig().jndiDefaults()
	case "pentest.tunnel.frp":
		defaults = GetAppConfig().frpDefaults()
	case "ai.pentest.task", "ai.pentest.chat":
		defaults = GetAppConfig().aiDefaults()
	default:
		return
	}
	for key, dv := range defaults {
		if dv == nil || dv == "" {
			continue
		}
		cur, exists := params[key]
		if !exists || cur == nil {
			params[key] = dv
			continue
		}
		if s, ok := cur.(string); ok && s == "" {
			params[key] = dv
		}
	}
}

// frpDefaults pentest.tunnel.frp 参数回填映射：frp 组的 ssh_* 留空时回退 jndi 组
// （两工具共用同一台攻击服务器），其余字段直接用 frp 组。
func (c *AppConfig) frpDefaults() map[string]any {
	sshHost := c.Frp.SSHHost
	if sshHost == "" {
		sshHost = c.Jndi.SSHHost
	}
	sshPort := c.Frp.SSHPort
	if sshPort == 0 {
		sshPort = c.Jndi.SSHPort
	}
	sshUser := c.Frp.SSHUser
	if sshUser == "" {
		sshUser = c.Jndi.SSHUser
	}
	sshPass := c.Frp.SSHPass
	if sshPass == "" {
		sshPass = c.Jndi.SSHPass
	}
	return map[string]any{
		"ssh_host":         sshHost,
		"ssh_port":         sshPort,
		"ssh_user":         sshUser,
		"ssh_pass":         sshPass,
		"server_addr":      sshHost,
		"remote_dir":       c.Frp.RemoteDir,
		"bind_port":        c.Frp.BindPort,
		"token":            c.Frp.Token,
		"vhost_http_port":  c.Frp.VhostHTTPPort,
		"vhost_https_port": c.Frp.VhostHTTPSPort,
		"dashboard_port":   c.Frp.DashboardPort,
		"dashboard_user":   c.Frp.DashboardUser,
		"dashboard_pass":   c.Frp.DashboardPass,
		"frps_version":     c.Frp.FrpsVersion,
		"download_url":     c.Frp.DownloadURL,
	}
}

// ToolPath 查询外部工具路径；配置为空时回退命令名本身（走 PATH）。
func ToolPath(name string) string {
	t := GetAppConfig().Tools
	var v string
	switch name {
	case "python3":
		v = t.Python3
	case "java":
		v = t.Java
	case "javac":
		v = t.Javac
	case "nmap":
		v = t.Nmap
	case "nuclei":
		v = t.Nuclei
	case "xray":
		v = t.Xray
	case "sqlmap":
		v = t.Sqlmap
	case "fscan":
		v = t.Fscan
	}
	if v == "" {
		return name
	}
	return v
}
