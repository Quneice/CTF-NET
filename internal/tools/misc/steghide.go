package misc

// misc.steghide:steghide 图片隐写解码(外部 steghide 0.5.1,fscan 同款外挂模式)。
// 三模式:空密码探测 / 指定密码 / 字典爆破;命中即停,提取内容按类型落盘 artifact。
// exe 路径在设置页「外部工具路径 → steghide」配置(默认本机 Downloads\steghide\,与 cygwin DLL 同目录)。
// CLI 语义(0.5.1 实测):extract -sf <stego> -p <pass> -xf <out> -f;-p "" 空密码合法;
// 成功 exit 0 + 生成 out;失败打印 "could not extract any data with that passphrase!"。

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ctfnet/internal/core"
	"ctfnet/internal/stego"
)

func init() {
	core.DefaultRegistry.Register(&SteghideTool{BaseTool: core.BaseTool{
		NameAttr:        "steghide",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "steghide 图片隐写解码(外部二进制):空密码探测/指定密码/字典爆破三模式提取 JPG/BMP/WAV 隐藏数据,命中即停,内容分类落盘",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "含隐写的载体文件(JPG/BMP/WAV)"},
			{Name: "mode", Type: "str", Required: false, Default: "empty",
				Description: "模式:empty 空密码探测 | custom 指定密码 | dict 字典爆破"},
			{Name: "password", Type: "str", Required: false, Description: "custom 模式的密码"},
			{Name: "dict", Type: "file", Required: false, Description: "dict 模式的字典文件 file_id(逐行)"},
			{Name: "dict_limit", Type: "int", Required: false, Default: 100000, Description: "字典行数上限"},
		},
		Runner: steghideRun,
	}})
}

// SteghideTool steghide 解码工具。
type SteghideTool struct {
	core.BaseTool
}

// steghideExtractOne 单次提取尝试:返回是否命中。exe/cwd 可注入(测试与工具共用)。
func steghideExtractOne(exe, cwd, stego, pass, out string) (bool, error) {
	if _, err := os.Stat(exe); err != nil {
		return false, fmt.Errorf("steghide 不可用(%s):请在设置页「外部工具路径 → steghide」配置;官方包为 SourceForge steghide 0.5.1 win32(zip,exe 与 cygwin DLL 须同目录)", exe)
	}
	executor := &core.SubprocessExecutor{}
	res, err := executor.Run(context.Background(),
		[]string{exe, "extract", "-sf", stego, "-p", pass, "-xf", out, "-f"},
		cwd, 60*time.Second, nil)
	if err != nil {
		return false, fmt.Errorf("steghide 执行失败: %v", err)
	}
	if res.Status != core.StatusSuccess {
		return false, nil // 常规未命中(密码错)
	}
	if cwd != "" {
		if _, err := os.Stat(filepath.Join(cwd, out)); err != nil {
			return false, nil
		}
	} else if _, err := os.Stat(out); err != nil {
		return false, nil
	}
	return true, nil
}

func steghideRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	report := func(p int, msg string) {
		if tc != nil && tc.ReportProgress != nil {
			tc.ReportProgress(p, msg)
		}
	}
	exe := core.ToolPath("steghide")
	stegoFile := paramStr(params, "file")
	mode := paramStr(params, "mode")
	if mode == "" {
		mode = "empty"
	}

	// 候选密码列表
	var candidates []string
	var truncated bool
	switch mode {
	case "custom":
		candidates = []string{paramStr(params, "password")}
		if candidates[0] == "" {
			return &core.ToolResult{Status: core.StatusError, Stderr: "custom 模式需要填写 password"}, nil
		}
	case "dict":
		dictName := paramStr(params, "dict")
		if dictName == "" {
			return &core.ToolResult{Status: core.StatusError, Stderr: "dict 模式需要上传字典文件"}, nil
		}
		limit := 100000
		if v := paramStr(params, "dict_limit"); v != "" {
			if _, err := fmt.Sscanf(v, "%d", &limit); err != nil || limit < 1 {
				limit = 100000
			}
		}
		f, err := os.Open(filepath.Join(tc.Workdir, dictName))
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("字典读取失败: %v", err)}, nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if len(candidates) >= limit {
				truncated = true
				break
			}
			candidates = append(candidates, line)
		}
		if len(candidates) == 0 {
			return &core.ToolResult{Status: core.StatusError, Stderr: "字典为空"}, nil
		}
	default: // empty
		mode = "empty"
		candidates = []string{""}
	}

	// 逐密码尝试,命中即停
	report(10, fmt.Sprintf("steghide %s 模式,共 %d 个候选密码", mode, len(candidates)))
	hit := false // 空密码命中时 hitPass 为空串,须用独立布尔区分未命中
	hitPass := ""
	outName := "steghide_out.bin"
	for i, pass := range candidates {
		if i%25 == 0 {
			report(10+80*i/len(candidates), fmt.Sprintf("尝试 %d/%d", i+1, len(candidates)))
		}
		_ = os.Remove(filepath.Join(tc.Workdir, outName))
		ok, err := steghideExtractOne(exe, tc.Workdir, stegoFile, pass, outName)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
		}
		if ok {
			hit = true
			hitPass = pass
			break
		}
		if ctx.Err() != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: "已取消"}, nil
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# steghide 解码报告\n\n")
	fmt.Fprintf(&sb, "- 载体:%s · 模式:%s · 尝试密码:%d 个", stegoFile, mode, len(candidates))
	if truncated {
		fmt.Fprintf(&sb, "(超出上限已截断)")
	}
	sb.WriteString("\n")

	content := []byte(nil)
	if hit {
		content, _ = os.ReadFile(filepath.Join(tc.Workdir, outName))
	}

	// 报告
	if !hit {
		sb.WriteString("\n**未命中**——全部候选密码均无法提取。建议:\n")
		sb.WriteString("- 先试空密码(CTF 最常见)与本工具三模式\n")
		sb.WriteString("- 换更大的字典(上传至 dict 模式)\n")
		sb.WriteString("- 确认载体确为 steghide 隐写(JPG/BMP/WAV;先过「图片二进制侦察」排除附加数据/LSB)\n")
		report(100, "未命中")
		return &core.ToolResult{Status: core.StatusSuccess, Stdout: sb.String(),
			Data: map[string]any{"hit": false, "tried": len(candidates)}}, nil
	}

	typ, ext, payload := imageReconClassify(content)
	art := ""
	if tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err == nil {
			name := "steghide_extracted." + map[bool]string{true: ext, false: "bin"}[ext != ""]
			if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, name), content, 0o644); err == nil {
				art = name
			}
		}
	}
	passShow := hitPass
	if passShow == "" {
		passShow = "(空)"
	}
	fmt.Fprintf(&sb, "- **命中密码:%s**\n", passShow)
	fmt.Fprintf(&sb, "- 提取内容:%d 字节,识别为「%s」\n", len(content), typ)
	if art != "" {
		fmt.Fprintf(&sb, "- 产物:**%s**(可下载)\n", art)
	}
	preview := content
	if len(preview) > 512 {
		preview = preview[:512]
	}
	textPrev := strings.Map(func(r rune) rune {
		if r >= 0x20 && r <= 0x7e {
			return r
		}
		return '.'
	}, string(preview))
	fmt.Fprintf(&sb, "\n```\n%s\n```\n", textPrev)
	flags := stego.Flags(content)
	for _, f := range flags {
		fmt.Fprintf(&sb, "\n- **flag 命中:** `%s`\n", f)
	}
	if len(flags) == 0 && typ == "文本" && payload != nil {
		sb.WriteString("\n提示:未命中 flag 正则时,可直接查看上方提取全文。\n")
	}
	report(100, "命中")
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: sb.String(),
		Data: map[string]any{"hit": true, "password": hitPass, "flags": flags, "type": typ}}, nil
}
