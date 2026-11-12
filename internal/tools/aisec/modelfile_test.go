package aisec

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ctfnet/internal/core"
)

// binPickle 构造协议 2 pickle:GLOBAL os system + REDUCE。
func binPickle(module, name string) []byte {
	b := []byte{0x80, 0x02} // PROTO 2
	b = append(b, 'c')
	b = append(b, []byte(module)...)
	b = append(b, '\n')
	b = append(b, []byte(name)...)
	b = append(b, '\n')
	b = append(b, '(', 'R', '.') // MARK REDUCE STOP
	return b
}

func TestScanPickleDangerousGlobal(t *testing.T) {
	imports, _, calls := scanPickle(binPickle("os", "system"))
	if len(imports) == 0 || imports[0] != "os system" {
		t.Fatalf("应提取 os system 导入, got %v", imports)
	}
	if calls != 1 {
		t.Fatalf("应统计 1 次 REDUCE, got %d", calls)
	}
	findings := classifyImports(imports)
	if len(findings) == 0 || findings[0].Severity != "dangerous" {
		t.Fatalf("os.system 应判 dangerous: %+v", findings)
	}
}

func TestScanPickleStackGlobal(t *testing.T) {
	b := []byte{0x80, 0x04, 0x93} // PROTO 4, STACK_GLOBAL
	b = append(b, []byte("subprocess")...)
	b = append(b, '\n')
	b = append(b, []byte("Popen")...)
	b = append(b, '\n')
	b = append(b, 'R', '.')
	imports, _, _ := scanPickle(b)
	if len(imports) == 0 || imports[0] != "subprocess Popen" {
		t.Fatalf("STACK_GLOBAL 提取失败: %v", imports)
	}
}

func TestScanPickleBenign(t *testing.T) {
	// 典型 torch 存档导入:collections OrderedDict / torch._utils _rebuild_tensor_v2
	imports, _, _ := scanPickle(append(binPickle("collections", "OrderedDict"), binPickle("torch._utils", "_rebuild_tensor_v2")...))
	findings := classifyImports(imports)
	for _, f := range findings {
		if f.Severity != "info" {
			t.Fatalf("良性导入不应告警: %+v", findings)
		}
	}
}

func TestScanPickleTextProtocol0(t *testing.T) {
	// 文本 pickle: cos\nsystem\n(S'id'\ntR.
	b := []byte("cos\nsystem\n(S'id'\ntR.")
	imports, literals, calls := scanPickle(b)
	if len(imports) == 0 || imports[0] != "os system" {
		t.Fatalf("文本 pickle 提取失败: %v", imports)
	}
	if len(literals) == 0 || literals[0] != "id" {
		t.Fatalf("文本 pickle 字符串提取失败: %v", literals)
	}
	if calls != 1 {
		t.Fatalf("REDUCE 统计错误: %d", calls)
	}
}

func TestScanModelBytesZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// 恶意 data.pkl + 可疑 .so 成员
	w1, _ := zw.Create("archive/data.pkl")
	w1.Write(binPickle("os", "system"))
	w2, _ := zw.Create("payload/evil.so")
	w2.Write([]byte("ELF"))
	zw.Close()

	rep := scanModelBytes("model.pt", buf.Bytes())
	if rep.Kind != "zip (PyTorch 存档)" {
		t.Fatalf("应识别为 zip: %s", rep.Kind)
	}
	if rep.Risk != "high" {
		t.Fatalf("zip 内恶意 pickle 应判 high: %s, %+v", rep.Risk, rep.Findings)
	}
	var hasSO, hasDanger bool
	for _, f := range rep.Findings {
		if f.Kind == "member" && f.Value == "payload/evil.so" {
			hasSO = true
		}
		if f.Kind == "import" && f.Severity == "dangerous" && f.Where == "archive/data.pkl" {
			hasDanger = true
		}
	}
	if !hasSO || !hasDanger {
		t.Fatalf("zip 发现缺失: so=%v danger=%v, %+v", hasSO, hasDanger, rep.Findings)
	}
}

func TestScanModelBytesSafetensors(t *testing.T) {
	header, _ := json.Marshal(map[string]any{"weight": map[string]any{"dtype": "F32", "shape": []int{2}}, "__metadata__": "ok"})
	var buf bytes.Buffer
	var hlen [8]byte
	binary.LittleEndian.PutUint64(hlen[:], uint64(len(header)))
	buf.Write(hlen[:])
	buf.Write(header)

	rep := scanModelBytes("model.safetensors", buf.Bytes())
	if rep.Kind != "safetensors" {
		t.Fatalf("应识别为 safetensors: %s", rep.Kind)
	}
	if rep.Risk != "low" {
		t.Fatalf("safetensors 应判 low: %s", rep.Risk)
	}
}

func TestModelFileScanRunMulti(t *testing.T) {
	dir := t.TempDir()
	// 文件 1:恶意 pickle;文件 2:safetensors
	p1 := filepath.Join(dir, "evil.pt")
	os.WriteFile(p1, binPickle("builtins", "eval"), 0o644)
	header, _ := json.Marshal(map[string]any{"w": map[string]any{}})
	raw := make([]byte, 8+len(header))
	binary.LittleEndian.PutUint64(raw, uint64(len(header)))
	copy(raw[8:], header)
	p2 := filepath.Join(dir, "safe.safetensors")
	os.WriteFile(p2, raw, 0o644)

	// file 参数为 []any(多文件上传形态)
	res, err := modelFileScanRun(context.Background(), map[string]any{"file": []any{"evil.pt", "safe.safetensors"}}, &core.ToolContext{Workdir: dir})
	if err != nil {
		t.Fatalf("run 报错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态错误: %s %s", res.Status, res.Stderr)
	}
	reports := res.Data["reports"].([]fileReport)
	if len(reports) != 2 {
		t.Fatalf("应扫描 2 个文件: %d", len(reports))
	}
	if reports[0].Risk != "high" || reports[1].Kind != "safetensors" {
		t.Fatalf("报告不符: %+v", reports)
	}
}
