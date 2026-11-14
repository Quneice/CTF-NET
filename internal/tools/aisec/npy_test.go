// .npy/.npz 解析测试:NumPy 数组文件 → Go float64(白盒 MLP 权重加载用)。
package aisec

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// makeNPYRaw 构造 .npy 字节(不 padding,解析器不依赖 64 字节对齐)。
// 注意 Python dict 字面量是大写 True/False(Go strconv.FormatBool 是小写)。
func makeNPYRaw(descr string, fortran bool, shape string, body []byte) []byte {
	pyBool := "False"
	if fortran {
		pyBool = "True"
	}
	hdr := "{'descr': '" + descr + "', 'fortran_order': " + pyBool + ", 'shape': " + shape + ", }"
	out := []byte{0x93, 'N', 'U', 'M', 'P', 'Y', 1, 0}
	out = binary.LittleEndian.AppendUint16(out, uint16(len(hdr)))
	out = append(out, hdr...)
	return append(out, body...)
}

func f32Body(vs ...float32) []byte {
	var b []byte
	for _, v := range vs {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
	}
	return b
}

func f64Body(vs ...float64) []byte {
	var b []byte
	for _, v := range vs {
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
	}
	return b
}

func TestParseNPYFloat64(t *testing.T) {
	raw := makeNPYRaw("<f8", false, "(2, 3)", f64Body(1, 2, 3, 4, 5, 6))
	arr, err := parseNPY(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(arr.Shape) != 2 || arr.Shape[0] != 2 || arr.Shape[1] != 3 {
		t.Fatalf("shape 错误: %v", arr.Shape)
	}
	if len(arr.Data) != 6 || arr.Data[5] != 6 {
		t.Fatalf("数据错误: %v", arr.Data)
	}
}

func TestParseNPYFloat32(t *testing.T) {
	raw := makeNPYRaw("<f4", false, "(4,)", f32Body(0.5, -1.25, 3, 100))
	arr, err := parseNPY(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(arr.Data) != 4 || arr.Data[1] != -1.25 || arr.Data[3] != 100 {
		t.Fatalf("float32 数据错误: %v", arr.Data)
	}
}

func TestParseNPZ(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range map[string][]byte{
		"W1.npy": makeNPYRaw("<f8", false, "(2, 2)", f64Body(1, 0, 0, 1)),
		"b1.npy": makeNPYRaw("<f8", false, "(2,)", f64Body(0.1, 0.2)),
	} {
		fw, _ := w.Create(name)
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := parseNPZ(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("条目数错误: %v", m)
	}
	if w1, ok := m["W1"]; !ok || w1.Shape[0] != 2 || w1.Data[3] != 1 {
		t.Fatalf("W1 解析错误: %+v", w1)
	}
	if b1, ok := m["b1"]; !ok || b1.Data[1] != 0.2 {
		t.Fatalf("b1 解析错误: %+v", b1)
	}
}

func TestParseNPYErrors(t *testing.T) {
	if _, err := parseNPY(bytes.NewReader([]byte("garbage"))); err == nil {
		t.Fatal("坏魔数应报错")
	}
	fortran := makeNPYRaw("<f8", true, "(2,)", f64Body(1, 2))
	if _, err := parseNPY(bytes.NewReader(fortran)); err == nil {
		t.Fatal("fortran_order=True 应报错提示")
	}
	badType := makeNPYRaw("|u1", false, "(2,)", []byte{1, 2})
	if _, err := parseNPY(bytes.NewReader(badType)); err == nil {
		t.Fatal("不支持 dtype 应报错")
	}
	truncated := makeNPYRaw("<f8", false, "(4,)", f64Body(1, 2))[:len(makeNPYRaw("<f8", false, "(4,)", f64Body(1, 2)))-8]
	if _, err := parseNPY(bytes.NewReader(truncated)); err == nil {
		t.Fatal("数据截断应报错")
	}
}
