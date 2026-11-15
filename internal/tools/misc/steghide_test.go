package misc

// misc.steghide 测试:live 门控(steghide.exe 存在才跑),fixture 现场用 embed 构造。

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

var stegExe = filepath.Join(`..`, `..`, `..`, `thirdparty`, `steghide`, `steghide.exe`)

func stegSkip(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(stegExe); err != nil {
		t.Skipf("steghide.exe 不存在,跳过: %v", err)
	}
}

// stegFixture 造随机 BMP 并用指定密码嵌入 secret,返回 cover 路径(workdir 内)。
func stegFixture(t *testing.T, work, password, secret string) string {
	t.Helper()
	w, h := 64, 64
	var b bytes.Buffer
	b.WriteString("BM")
	binary.Write(&b, binary.LittleEndian, uint32(54+w*h*3))
	binary.Write(&b, binary.LittleEndian, uint16(0))
	binary.Write(&b, binary.LittleEndian, uint16(0))
	binary.Write(&b, binary.LittleEndian, uint32(54))
	binary.Write(&b, binary.LittleEndian, uint32(40))
	binary.Write(&b, binary.LittleEndian, int32(w))
	binary.Write(&b, binary.LittleEndian, int32(h))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(24))
	binary.Write(&b, binary.LittleEndian, uint32(0))
	binary.Write(&b, binary.LittleEndian, uint32(w*h*3))
	binary.Write(&b, binary.LittleEndian, int32(0))
	binary.Write(&b, binary.LittleEndian, int32(0))
	binary.Write(&b, binary.LittleEndian, uint32(0))
	binary.Write(&b, binary.LittleEndian, uint32(0))
	rnd := rand.New(rand.NewSource(7))
	for i := 0; i < w*h*3; i++ {
		b.WriteByte(byte(rnd.Intn(256)))
	}
	cover := filepath.Join(work, "cover.bmp")
	sec := filepath.Join(work, "secret.txt")
	if err := os.WriteFile(cover, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sec, []byte(secret), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(stegExe, "embed", "-cf", cover, "-ef", sec, "-p", password, "-z", "1", "-f")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture embed 失败: %v\n%s", err, out)
	}
	return "cover.bmp"
}

func stegRun(t *testing.T, params map[string]any) (*core.ToolResult, string) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("misc.steghide")
	if !ok {
		t.Fatal("misc.steghide 未注册")
	}
	work := t.TempDir()
	arts := t.TempDir()
	// 把 fixture 造在 work 里
	pw := "empty"
	if v, ok := params["__password"]; ok {
		pw = v.(string)
		delete(params, "__password")
	}
	secret := fmt.Sprintf("flag{steghide_%s_mode}", pw)
	cover := stegFixture(t, work, map[bool]string{true: "", false: pw}[pw == "empty"], secret)
	params["file"] = cover
	// 按需造字典文件(workdir 内)
	if dn, ok := params["dict"].(string); ok && dn != "" {
		var lines string
		if dn == "big.txt" {
			lines = strings.Join([]string{"p0", "p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"}, "\n")
		} else {
			lines = "wrong1\nletmein\nwrong2"
		}
		if err := os.WriteFile(filepath.Join(work, dn), []byte(lines), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tc := &core.ToolContext{Workdir: work, ArtifactsDir: arts}
	res, err := tool.Run(context.Background(), params, tc)
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	return res, arts
}

// 1. empty 模式:空密码命中,artifact 内容与 secret 一致。
func TestSteghideEmptyHit(t *testing.T) {
	stegSkip(t)
	res, arts := stegRun(t, map[string]any{"mode": "empty"})
	if res.Status != core.StatusSuccess {
		t.Fatalf("status: %s %s", res.Status, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "flag{steghide_empty_mode}") {
		t.Fatalf("报告应含 flag:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, `密码:""`) && !strings.Contains(res.Stdout, "密码:(空)") {
		t.Fatalf("报告应标注命中密码为空:\n%s", res.Stdout)
	}
	artsList, _ := os.ReadDir(arts)
	if len(artsList) == 0 {
		t.Fatal("应有提取产物")
	}
}

// 2. custom 模式:指定密码命中。
func TestSteghideCustomHit(t *testing.T) {
	stegSkip(t)
	res, _ := stegRun(t, map[string]any{"mode": "custom", "password": "pass123", "__password": "pass123"})
	if res.Status != core.StatusSuccess || !strings.Contains(res.Stdout, "flag{steghide_pass123_mode}") {
		t.Fatalf("custom 应命中:\n%s\n%s", res.Status, res.Stdout)
	}
}

// 3. dict 模式:字典含真密码,命中。
func TestSteghideDictHit(t *testing.T) {
	stegSkip(t)
	res, _ := stegRun(t, map[string]any{"mode": "dict", "dict": "dict.txt", "__password": "letmein"})
	if res.Status != core.StatusSuccess || !strings.Contains(res.Stdout, "flag{steghide_letmein_mode}") {
		t.Fatalf("dict 应命中:\n%s", res.Stdout)
	}
}

// 4. 错密码 custom:全 miss,成功状态 + 建议报告。
func TestSteghideMiss(t *testing.T) {
	stegSkip(t)
	res, _ := stegRun(t, map[string]any{"mode": "custom", "password": "wrongwrong", "__password": "realpass"})
	if res.Status != core.StatusSuccess {
		t.Fatalf("未命中也应正常返回: %s", res.Status)
	}
	if !strings.Contains(res.Stdout, "未命中") {
		t.Fatalf("应有未命中报告:\n%s", res.Stdout)
	}
}

// 5. helper 级:不存在的 exe 路径 → 友好错误。
func TestSteghideMissingExe(t *testing.T) {
	_, err := steghideExtractOne(`Z:\no\such\steghide.exe`, "cover.bmp", "", "out.bin", "")
	if err == nil || !strings.Contains(err.Error(), "steghide") {
		t.Fatalf("应给 steghide 相关错误: %v", err)
	}
}

// 6. 字典超限截断:11 行字典限制 5 → 报告提示截断。
func TestSteghideDictLimit(t *testing.T) {
	stegSkip(t)
	res, _ := stegRun(t, map[string]any{"mode": "dict", "dict": "big.txt", "dict_limit": "5", "__password": "p3"})
	if res.Status != core.StatusSuccess {
		t.Fatalf("status: %s", res.Status)
	}
	if !strings.Contains(res.Stdout, "截断") && !strings.Contains(res.Stdout, "上限") {
		t.Fatalf("超限应提示截断:\n%s", res.Stdout)
	}
}

// 7.(终审修复)超长行字典:>64KB 单行使 Scanner 静默失败——修复后应明确报错而非假「字典为空/未命中」。
func TestSteghideDictLongLine(t *testing.T) {
	stegSkip(t)
	tool, _ := core.DefaultRegistry.Get("misc.steghide")
	work := t.TempDir()
	stegFixture(t, work, "", "flag{x}")
	huge := strings.Repeat("a", 1500000)
	os.WriteFile(filepath.Join(work, "big.txt"), []byte(huge+"\nletmein\n"), 0o644)
	res, err := tool.Run(context.Background(), map[string]any{"file": "cover.bmp", "mode": "dict", "dict": "big.txt"}, &core.ToolContext{Workdir: work, ArtifactsDir: t.TempDir()})
	if err != nil || res.Status != core.StatusError {
		t.Fatalf("超长行应明确报错,实际 status=%s err=%v out=%.200s", res.Status, err, res.Stdout)
	}
	if !strings.Contains(res.Stderr, "字典") || !strings.Contains(res.Stderr, "解析") {
		t.Fatalf("错误应说明字典解析问题: %s", res.Stderr)
	}
}

// 8.(终审修复)BOM 字典:首行 \ufeff 前缀应剥除,含 BOM 的真密码可命中。
func TestSteghideDictBOM(t *testing.T) {
	stegSkip(t)
	res, _ := stegRun(t, map[string]any{"mode": "dict", "dict": "bom.txt", "__password": "letmein"})
	if res.Status != core.StatusSuccess || !strings.Contains(res.Stdout, "flag{steghide_letmein_mode}") {
		t.Fatalf("BOM 字典应仍命中:\n%s", res.Stdout)
	}
}
