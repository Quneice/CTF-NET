package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 配置文件不存在 → 返回默认值并落盘默认副本。
func TestLoadAppConfigCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := LoadAppConfig(dir)
	if cfg.Jndi.LDAPPort != 1389 || cfg.Executor.Backend != "local" || cfg.Tools.Python3 != "python3" {
		t.Errorf("默认值不符合预期: %+v", cfg)
	}
	if _, err := os.Stat(filepath.Join(dir, ConfigFileName)); err != nil {
		t.Errorf("默认副本未落盘: %v", err)
	}
}

// 保存→重开应读回；部分字段文件与默认值合并。
func TestAppConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultAppConfig()
	cfg.Jndi.SSHHost = "10.0.0.8"
	cfg.Jndi.SSHUser = "root"
	if err := SaveAppConfig(dir, cfg); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	got := LoadAppConfig(dir)
	if got.Jndi.SSHHost != "10.0.0.8" {
		t.Errorf("ssh_host = %q", got.Jndi.SSHHost)
	}
	if got.Jndi.LDAPPort != 1389 {
		t.Errorf("未出现在文件中的字段应保留默认值, ldap_port = %d", got.Jndi.LDAPPort)
	}
}

// 迁移：旧 config.json 缺少新增配置组（frp）时，自动补默认模板并回写，
// 用户已填的值与模板外自定义键原样保留。
func TestLoadAppConfigMigratesNewGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	old := `{
  "jndi": {"ssh_host": "120.53.9.146", "ssh_port": 22},
  "tools": {"python3": "python3"},
  "wordlists": {"dir_enum": "D:\\dict\\dir.txt"},
  "executor": {"backend": "local"},
  "my_custom_key": {"hello": "world"}
}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := LoadAppConfig(dir)

	// 内存值：frp 组回填默认
	if cfg.Frp.BindPort != 7000 || cfg.Frp.FrpsVersion != "0.69.1" {
		t.Errorf("frp 默认值未生效: %+v", cfg.Frp)
	}
	if cfg.Jndi.SSHHost != "120.53.9.146" {
		t.Errorf("用户 jndi 值被覆盖: %q", cfg.Jndi.SSHHost)
	}

	// 磁盘文件：补齐 frp 组模板，保留用户值与自定义键
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("迁移后的文件不是合法 JSON: %v", err)
	}
	if _, ok := m["frp"]; !ok {
		t.Errorf("迁移后缺少 frp 组:\n%s", data)
	}
	if _, ok := m["my_custom_key"]; !ok {
		t.Errorf("模板外自定义键被丢弃:\n%s", data)
	}
	var frp FrpConfig
	if err := json.Unmarshal(m["frp"], &frp); err != nil {
		t.Fatalf("frp 组解析失败: %v", err)
	}
	if frp.BindPort != 7000 || frp.DashboardPort != 7500 {
		t.Errorf("frp 组默认值缺失: %+v", frp)
	}
	var jndi JndiConfig
	if err := json.Unmarshal(m["jndi"], &jndi); err != nil {
		t.Fatalf("jndi 组解析失败: %v", err)
	}
	if jndi.SSHHost != "120.53.9.146" || jndi.SSHPort != 22 {
		t.Errorf("用户 jndi 值未保留: %+v", jndi)
	}
	if jndi.LDAPPort != 1389 {
		t.Errorf("jndi 组内缺失字段应补默认值: %+v", jndi)
	}
}

// 迁移幂等：完整模板文件不应被改写（内容逐字一致）。
func TestLoadAppConfigNoRewriteWhenComplete(t *testing.T) {
	dir := t.TempDir()
	if err := SaveAppConfig(dir, DefaultAppConfig()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ConfigFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = LoadAppConfig(dir)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("完整模板文件被迁移逻辑改写:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

// 损坏的 JSON：备份为 .bak 并回退默认值，不阻断。
func TestLoadAppConfigCorrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, []byte("{oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := LoadAppConfig(dir)
	if cfg.Jndi.LDAPPort != 1389 {
		t.Errorf("损坏文件应回退默认值")
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("损坏文件未备份为 .bak: %v", err)
	}
}

// ApplyDefaults：仅回填 web.jndi 的空参数，显式值优先。
func TestApplyDefaultsJndi(t *testing.T) {
	SetAppConfig(DefaultAppConfig())
	cfg := GetAppConfig()
	cfg.Jndi.SSHHost = "192.168.1.1"
	SetAppConfig(cfg)

	params := map[string]any{"ssh_host": "", "ssh_user": "admin"}
	ApplyDefaults("web.jndi", params)
	if params["ssh_host"] != "192.168.1.1" {
		t.Errorf("空 ssh_host 未回填: %v", params["ssh_host"])
	}
	if params["ssh_user"] != "admin" {
		t.Errorf("显式 ssh_user 被覆盖: %v", params["ssh_user"])
	}
	if params["ldap_port"] != 1389 {
		t.Errorf("默认端口未回填: %v", params["ldap_port"])
	}

	other := map[string]any{"ssh_host": ""}
	ApplyDefaults("misc.strings", other)
	if other["ssh_host"] != "" {
		t.Errorf("非白名单工具被回填")
	}
}

// ToolPath：空配置回退命令名本身。
func TestToolPathFallback(t *testing.T) {
	SetAppConfig(DefaultAppConfig())
	if got := ToolPath("nmap"); got != "nmap" {
		t.Errorf("默认应返回命令名, got %q", got)
	}
	cfg := GetAppConfig()
	cfg.Tools.Nmap = `C:\tools\nmap\nmap.exe`
	SetAppConfig(cfg)
	if got := ToolPath("nmap"); got != `C:\tools\nmap\nmap.exe` {
		t.Errorf("配置路径未生效, got %q", got)
	}
}
