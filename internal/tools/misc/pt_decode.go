package misc

// misc.pt_decode:思科 Packet Tracer 工程文件(.pkt/.pka,经典 ≤7.x)一键解码。
// 解密引擎见 pt_crypto.go;本文件负责信息提取(flag/密码行/版本/设备名)、
// markdown 报告与 decoded.xml 产物落盘。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ctfnet/internal/core"
	"ctfnet/internal/stego"
)

func init() {
	core.DefaultRegistry.Register(&PTDecodeTool{BaseTool: core.BaseTool{
		NameAttr:        "pt_decode",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "思科 Packet Tracer 工程文件解码:四层解密(反转 XOR/Twofish 魔改 EAX 固定密钥/二次 XOR/zlib)还原拓扑 XML,提取 flag、password/secret/banner 配置行、版本与设备名,完整 XML 落盘可下载(支持 ≤7.x)",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "上传的 Packet Tracer 文件(.pkt/.pka 或去掉后缀的容器)"},
		},
		Runner: ptDecodeRun,
	}})
}

// PTDecodeTool 思科 PT 解码工具。
type PTDecodeTool struct {
	core.BaseTool
}

var (
	ptVersionRe = regexp.MustCompile(`<VERSION>([^<]{1,64})</VERSION>`)
	ptLineRe    = regexp.MustCompile(`<LINE>([^<]*)</LINE>`)
	ptNameRe    = regexp.MustCompile(`<Name>([^<]{1,128})</Name>`)
	ptSecretRe  = regexp.MustCompile(`(?i)(password|secret|banner|enable\s+password|community)`)
)

func ptDecodeRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	report := func(p int, msg string) {
		if tc != nil && tc.ReportProgress != nil {
			tc.ReportProgress(p, msg)
		}
	}
	report(5, "读取文件")
	filePath := filepath.Join(tc.Workdir, paramStr(params, "file"))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	report(20, "四层解密(反转 XOR → Twofish → 二次 XOR → zlib)")
	xml, err := decryptPT(data)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}

	report(60, "提取信息")
	version := ""
	if m := ptVersionRe.FindSubmatch(xml); m != nil {
		version = string(m[1])
	}
	flags := stego.Flags(xml)
	if flags == nil {
		flags = []string{}
	}
	// 敏感配置行(startup-config 的 <LINE> 条目)
	seenLine := map[string]bool{}
	secretLines := []string{}
	for _, m := range ptLineRe.FindAllStringSubmatch(string(xml), -1) {
		line := strings.TrimSpace(m[1])
		if line == "" || seenLine[line] || !ptSecretRe.MatchString(line) {
			continue
		}
		seenLine[line] = true
		if len(secretLines) < 50 {
			secretLines = append(secretLines, line)
		}
	}
	// 设备名
	seenName := map[string]bool{}
	devices := []string{}
	for _, m := range ptNameRe.FindAllStringSubmatch(string(xml), 5000) {
		name := strings.TrimSpace(m[1])
		if name == "" || seenName[name] {
			continue
		}
		seenName[name] = true
		if len(devices) < 30 {
			devices = append(devices, name)
		}
	}

	report(80, "落盘 decoded.xml")
	art := ""
	if tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err == nil {
			if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "decoded.xml"), xml, 0o644); err == nil {
				art = "decoded.xml"
			}
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# 思科 Packet Tracer 解码报告\n\n")
	fmt.Fprintf(&sb, "- 输入:%s(%d 字节)→ 明文 XML %d 字节(压缩比 %.1fx)\n",
		paramStr(params, "file"), len(data), len(xml), float64(len(xml))/float64(maxInt(len(data), 1)))
	if version != "" {
		fmt.Fprintf(&sb, "- PT 版本:**%s**\n", version)
	}
	if len(devices) > 0 {
		fmt.Fprintf(&sb, "- 设备(%d 个):%s\n", len(devices), strings.Join(devices, "、"))
	}
	if art != "" {
		fmt.Fprintf(&sb, "- 产物:**%s**(完整拓扑,可下载继续分析)\n", art)
	}
	if len(flags) > 0 {
		sb.WriteString("\n## flag 命中\n\n")
		for _, f := range flags {
			fmt.Fprintf(&sb, "- `%s`\n", f)
		}
	}
	if len(secretLines) > 0 {
		sb.WriteString("\n## 敏感配置行(password/secret/banner)\n\n")
		for _, l := range secretLines {
			fmt.Fprintf(&sb, "- `%s`\n", l)
		}
	}
	if len(flags) == 0 && len(secretLines) == 0 {
		sb.WriteString("\n未发现 flag 或敏感配置行:请下载 decoded.xml 人工分析(拓扑备注/活动说明等)。\n")
	}

	report(100, "完成")
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: sb.String(),
		Data:   map[string]any{"flags": flags, "version": version, "secret_lines": len(secretLines)},
	}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
