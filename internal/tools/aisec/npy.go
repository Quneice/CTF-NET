// .npy/.npz 解析:NumPy 数组文件 → float64(白盒 MLP 权重加载,零 Python 依赖)。
// .npz 即 zip 包,内含若干 .npy;np.save 生成的 npy 均为小端、C 序。
package aisec

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// npyArray 解析结果:shape 与按行展开(C 序)的 float64 数据。
type npyArray struct {
	Shape []int
	Data  []float64
}

var npyMagic = []byte{0x93, 'N', 'U', 'M', 'P', 'Y'}

var npyHeaderRe = regexp.MustCompile(`'descr':\s*'([^']+)'|'fortran_order':\s*(True|False)|'shape':\s*\(([^)]*)\)`)

// parseNPY 解析单个 .npy。仅支持小端 float32/float64(<f4/<f8);fortran_order=true 报错
// (MLP 权重文件均为 C 序,误序加载会得到转置的静默错误,必须显式拦截)。
func parseNPY(r io.Reader) (*npyArray, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(raw) < 10 || !bytes.Equal(raw[:6], npyMagic) {
		return nil, fmt.Errorf("非 .npy 文件(魔数不匹配)")
	}
	major := raw[6]
	var hdrLen int
	var bodyOff int
	switch major {
	case 1:
		hdrLen = int(binary.LittleEndian.Uint16(raw[8:10]))
		bodyOff = 10 + hdrLen
	case 2:
		if len(raw) < 12 {
			return nil, fmt.Errorf("npy v2 头部截断")
		}
		hdrLen = int(binary.LittleEndian.Uint32(raw[8:12]))
		bodyOff = 12 + hdrLen
	default:
		return nil, fmt.Errorf("不支持的 npy 版本: %d", major)
	}
	if bodyOff > len(raw) {
		return nil, fmt.Errorf("npy 头部长度越界")
	}
	hdr := string(raw[bodyOff-hdrLen : bodyOff])

	// 解析 dict(逐项正则匹配,容忍键序与空格差异)
	var descr string
	fortran := false
	var shape []int
	for _, m := range npyHeaderRe.FindAllStringSubmatch(hdr, -1) {
		if m[1] != "" {
			descr = m[1]
		}
		if m[2] != "" {
			fortran = m[2] == "True"
		}
		if m[3] != "" && strings.TrimSpace(m[3]) != "" {
			for _, tok := range strings.Split(m[3], ",") {
				tok = strings.TrimSpace(tok)
				if tok == "" {
					continue
				}
				n, err := strconv.Atoi(strings.TrimSuffix(tok, "L"))
				if err != nil {
					return nil, fmt.Errorf("shape 解析失败: %q", tok)
				}
				shape = append(shape, n)
			}
		}
	}
	if descr == "" || shape == nil {
		return nil, fmt.Errorf("npy 头部缺少 descr/shape: %s", hdr)
	}
	if fortran {
		return nil, fmt.Errorf("fortran_order=true 不支持(期望 C 序权重;请在 numpy 侧用 np.ascontiguousarray 转换)")
	}
	var itemSize int
	switch descr {
	case "<f4", "f4":
		itemSize = 4
	case "<f8", "f8", "float64":
		itemSize = 8
	default:
		return nil, fmt.Errorf("不支持的 dtype: %s(仅 <f4/<f8)", descr)
	}
	n := 1
	for _, d := range shape {
		n *= d
	}
	body := raw[bodyOff:]
	if len(body) < n*itemSize {
		return nil, fmt.Errorf("npy 数据截断: 需要 %d 字节,实际 %d", n*itemSize, len(body))
	}
	out := &npyArray{Shape: shape, Data: make([]float64, n)}
	if itemSize == 4 {
		for i := 0; i < n; i++ {
			out.Data[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(body[i*4:])))
		}
	} else {
		for i := 0; i < n; i++ {
			out.Data[i] = math.Float64frombits(binary.LittleEndian.Uint64(body[i*8:]))
		}
	}
	return out, nil
}

// parseNPZ 解析 zip 形式的 .npz → 数组名(不含 .npy 后缀)→ 数组。
func parseNPZ(r io.Reader) (map[string]*npyArray, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("npz 不是合法 zip: %w", err)
	}
	out := make(map[string]*npyArray)
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".npy") || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		arr, err := parseNPY(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		out[strings.TrimSuffix(f.Name, ".npy")] = arr
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("npz 内没有 .npy 数组")
	}
	return out, nil
}
