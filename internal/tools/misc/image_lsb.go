// misc.image_lsb —— PNG/BMP 图像 LSB 隐写提取(zsteg 类套路)。
// R/G/B/A 通道 × bit0-2 位平面 × MSB/LSB 打包位序,auto 扫描;
// 与 WAV 工具共享 internal/stego 引擎(魔数/文本/flag 识别、EOI/IEND 截断、预览)。
// 支持 PNG(8/16bit、灰度、调色板回退)与 BMP;JPEG 有损压缩破坏 LSB,明确引导。
package misc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/bmp"

	"ctfnet/internal/core"
	"ctfnet/internal/stego"
)

func init() {
	core.DefaultRegistry.Register(&ImageLSBTool{BaseTool: core.BaseTool{
		NameAttr:        "image_lsb",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "PNG/BMP 图像 LSB 隐写提取:像素通道位平面扫描,自动识别隐藏类型(图片/压缩包/文本/flag)",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "上传的 PNG/BMP 图片 file_id"},
			{Name: "plane", Type: "str", Required: false, Default: "auto",
				Description: "位平面:auto | 0 | 1 | 2(auto 扫描 0-2)"},
			{Name: "order", Type: "str", Required: false, Default: "auto",
				Description: "打包位序:auto | msb | lsb"},
			{Name: "channel", Type: "str", Required: false, Default: "auto",
				Description: "通道:auto | rgb(逐像素 R,G,B 交错) | r | g | b | a"},
		},
		Runner: imageLSBRun,
	}})
}

// ImageLSBTool 图像 LSB 隐写工具。
type ImageLSBTool struct {
	core.BaseTool
}

// imgChannels 从解码图提取各通道原始存储值(位深 8 或 16;直接读 Pix 避免预乘失真)。
type imgChannels struct {
	mode     string
	depth    int
	r, g, b  []uint64
	a        []uint64
	hasAlpha bool
	isGray   bool
}

func extractChannels(img image.Image) *imgChannels {
	c := &imgChannels{}
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	n := w * h
	switch t := img.(type) {
	case *image.NRGBA:
		c.mode, c.depth, c.hasAlpha = "NRGBA 8bit", 8, true
		c.r, c.g, c.b, c.a = make([]uint64, n), make([]uint64, n), make([]uint64, n), make([]uint64, n)
		for i := 0; i < n; i++ {
			c.r[i], c.g[i], c.b[i], c.a[i] = uint64(t.Pix[i*4]), uint64(t.Pix[i*4+1]), uint64(t.Pix[i*4+2]), uint64(t.Pix[i*4+3])
		}
	case *image.RGBA:
		c.mode, c.depth, c.hasAlpha = "RGBA 8bit", 8, true
		c.r, c.g, c.b, c.a = make([]uint64, n), make([]uint64, n), make([]uint64, n), make([]uint64, n)
		for i := 0; i < n; i++ {
			c.r[i], c.g[i], c.b[i], c.a[i] = uint64(t.Pix[i*4]), uint64(t.Pix[i*4+1]), uint64(t.Pix[i*4+2]), uint64(t.Pix[i*4+3])
		}
	case *image.Gray:
		c.mode, c.depth, c.isGray = "Gray 8bit", 8, true
		c.r = make([]uint64, n)
		for i := 0; i < n; i++ {
			c.r[i] = uint64(t.Pix[i])
		}
	case *image.Gray16:
		c.mode, c.depth, c.isGray = "Gray16", 16, true
		c.r = make([]uint64, n)
		for i := 0; i < n; i++ {
			c.r[i] = uint64(t.Pix[i*2]) | uint64(t.Pix[i*2+1])<<8
		}
	case *image.NRGBA64:
		c.mode, c.depth, c.hasAlpha = "NRGBA64 16bit", 16, true
		c.r, c.g, c.b, c.a = make([]uint64, n), make([]uint64, n), make([]uint64, n), make([]uint64, n)
		for i := 0; i < n; i++ {
			c.r[i] = uint64(t.Pix[i*8]) | uint64(t.Pix[i*8+1])<<8
			c.g[i] = uint64(t.Pix[i*8+2]) | uint64(t.Pix[i*8+3])<<8
			c.b[i] = uint64(t.Pix[i*8+4]) | uint64(t.Pix[i*8+5])<<8
			c.a[i] = uint64(t.Pix[i*8+6]) | uint64(t.Pix[i*8+7])<<8
		}
	default: // RGBA64 / Paletted / 其他:走 At().RGBA()(预乘,仅作近似;调色板图 LSB 本就存于索引/调色板)
		c.mode, c.depth = "generic 8bit(RGBA 近似)", 8
		c.r, c.g, c.b, c.a = make([]uint64, n), make([]uint64, n), make([]uint64, n), make([]uint64, n)
		i := 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				c.r[i], c.g[i], c.b[i], c.a[i] = uint64(r>>8), uint64(g>>8), uint64(b>>8), uint64(a>>8)
				i++
			}
		}
		if _, ok := img.(*image.RGBA64); ok {
			c.mode, c.depth = "RGBA64 16bit", 16
			i = 0
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					r, g, b, a := img.At(x, y).RGBA()
					c.r[i], c.g[i], c.b[i], c.a[i] = uint64(r), uint64(g), uint64(b), uint64(a)
					i++
				}
			}
		} else {
			c.hasAlpha = true
		}
	}
	return c
}

type imageLSBFileMeta struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Format string `json:"format"` // png | bmp
	Mode   string `json:"mode"`   // NRGBA 8bit / Gray 8bit …
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type imageLSBReport struct {
	File       imageLSBFileMeta    `json:"file"`
	Params     map[string]any      `json:"params"`
	Candidates []audioLSBCandidate `json:"candidates"`
	Hits       []audioLSBHit       `json:"hits"`
	Flags      []string            `json:"flags"`
	Artifacts  []string            `json:"artifacts"`
	Summary    string              `json:"summary"`
}

func imageLSBRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	report := func(p int, msg string) {
		if tc != nil && tc.ReportProgress != nil {
			tc.ReportProgress(p, msg)
		}
	}
	report(5, "读取图片")
	filePath := filepath.Join(tc.Workdir, paramStr(params, "file"))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	var img image.Image
	format := ""
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
		img, err = png.Decode(bytes.NewReader(data))
		format = "png"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		img, err = bmp.Decode(bytes.NewReader(data))
		format = "bmp"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return &core.ToolResult{Status: core.StatusError,
			Stderr: "JPEG 为有损压缩格式,LSB 信息在编码时已被破坏;请提供 PNG/BMP 无损图(JPEG 隐写另见 steghide/JPhide 等专用工具)"}, nil
	default:
		return &core.ToolResult{Status: core.StatusError, Stderr: "无法识别的图片格式(支持 PNG / BMP)"}, nil
	}
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("图片解码失败: %v", err)}, nil
	}

	ch := extractChannels(img)
	st, _ := os.Stat(filePath)
	name := filepath.Base(filePath)
	var fileSize int64
	if st != nil {
		fileSize = st.Size()
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()

	// 参数归一化
	getStr := func(k, def string) string {
		v := strings.ToLower(strings.TrimSpace(paramStr(params, k)))
		if v == "" {
			return def
		}
		return v
	}
	planeArg, orderArg, chanArg := getStr("plane", "auto"), getStr("order", "auto"), getStr("channel", "auto")
	planes := []int{0, 1, 2}
	if planeArg != "auto" {
		if len(planeArg) != 1 || planeArg[0] < '0' || planeArg[0] > '7' {
			return &core.ToolResult{Status: core.StatusError, Stderr: "plane 非法(0-7 或 auto)"}, nil
		}
		planes = []int{int(planeArg[0] - '0')}
	}
	orders := []string{"msb", "lsb"}
	if orderArg == "msb" || orderArg == "lsb" {
		orders = []string{orderArg}
	} else if orderArg != "auto" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "order 仅支持 auto/msb/lsb"}, nil
	}
	chanList := []string{"rgb", "r", "g", "b"}
	if ch.isGray {
		chanList = []string{"r"}
	}
	if ch.hasAlpha {
		chanList = append(chanList, "a")
	}
	if chanArg != "auto" {
		ok := false
		for _, c := range chanList {
			if c == chanArg {
				ok = true
				break
			}
		}
		if !ok {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("channel 非法(%s):可用 %v", chanArg, chanList)}, nil
		}
		chanList = []string{chanArg}
	}

	rep := imageLSBReport{
		File:       imageLSBFileMeta{name, fileSize, format, ch.mode, w, h},
		Params:     map[string]any{"plane": planeArg, "order": orderArg, "channel": chanArg},
		Candidates: []audioLSBCandidate{},
		Hits:       []audioLSBHit{},
		Flags:      []string{},
		Artifacts:  []string{},
	}

	// 采样源:单通道 count=w*h;rgb 交错 count=w*h*3
	sampleCount := func(channel string) int {
		if channel == "rgb" && !ch.isGray {
			return w * h * 3
		}
		return w * h
	}
	sampleFunc := func(channel string) func(int) uint64 {
		switch channel {
		case "rgb":
			return func(k int) uint64 {
				i, rem := k/3, k%3
				switch rem {
				case 0:
					return ch.r[i]
				case 1:
					return ch.g[i]
				default:
					return ch.b[i]
				}
			}
		case "g":
			if ch.isGray {
				return func(i int) uint64 { return ch.r[i] }
			}
			return func(i int) uint64 { return ch.g[i] }
		case "b":
			return func(i int) uint64 { return ch.b[i] }
		case "a":
			return func(i int) uint64 { return ch.a[i] }
		default: // r / gray
			return func(i int) uint64 { return ch.r[i] }
		}
	}

	total := len(planes) * len(orders) * len(chanList)
	done := 0
	for _, plane := range planes {
		for _, order := range orders {
			for _, channel := range chanList {
				done++
				report(10+85*done/total, fmt.Sprintf("通道 %s bit%d/%s(%d/%d)", channel, plane, order, done, total))
				bs, ones, cnt := stego.Pack(sampleCount(channel), sampleFunc(channel), plane, 1, order == "msb", nil)
				if len(bs) == 0 || cnt == 0 {
					continue
				}
				cand := audioLSBCandidate{Plane: plane, Order: order, Channel: channel, OnesRatio: float64(ones) / float64(cnt)}
				typ, ext, mime, off := stego.Detect(bs)
				win := bs
				if len(win) > stego.ScanWindow {
					win = win[:stego.ScanWindow]
				}
				rep.Flags = append(rep.Flags, stego.Flags(win)...)
				if typ == "" {
					if idx := stego.FlagRe.FindStringIndex(string(win)); idx != nil {
						typ, ext, mime, off = "TEXT", "txt", "text/plain", idx[0]
					} else {
						rep.Candidates = append(rep.Candidates, cand)
						continue
					}
				}
				seg := stego.Cut(bs, off, typ)
				if len(seg) == 0 {
					rep.Candidates = append(rep.Candidates, cand)
					continue
				}
				art := fmt.Sprintf("lsb_%s_bit%d_%s.%s", channel, plane, order, ext)
				if tc.ArtifactsDir != "" {
					if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
						return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
					}
					if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, art), seg, 0o644); err != nil {
						return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出产物失败: %v", err)}, nil
					}
				}
				hit := audioLSBHit{audioLSBCandidate: cand, Offset: off, Size: len(seg), Artifact: art}
				hit.Type, hit.Hit, hit.Format = stego.TypeOf(typ, ext), true, typ
				cand.Type, cand.Hit = stego.TypeOf(typ, ext), true
				rep.Candidates = append(rep.Candidates, cand)
				if strings.HasPrefix(mime, "image/") && len(seg) <= stego.MaxPreview {
					hit.Preview = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(seg)
				}
				if typ == "TEXT" {
					hit.Text = string(seg)
				}
				rep.Hits = append(rep.Hits, hit)
				rep.Artifacts = append(rep.Artifacts, art)
			}
		}
	}
	report(95, "汇总报告")

	var sb strings.Builder
	fmt.Fprintf(&sb, "图片 %s:%s %s %d×%d(%s)\n", name, format, ch.mode, w, h, ch.mode)
	if len(rep.Hits) == 0 {
		sb.WriteString("未发现已知类型 LSB 隐写(可尝试:更高位平面、zsteg 全参数、或转战频域/格式结构)\n")
	} else {
		for _, h := range rep.Hits {
			fmt.Fprintf(&sb, "[+] %s/bit%d/%s → %s %dB(偏移 %d)%s\n",
				h.Channel, h.Plane, h.Order, h.Type, h.Size, h.Offset, h.Artifact)
		}
	}
	for _, f := range rep.Flags {
		fmt.Fprintf(&sb, "[flag] %s\n", f)
	}
	rep.Summary = sb.String()

	out, err := json.Marshal(rep)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	report(100, "完成")
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: string(out),
		Data:   map[string]any{"hit_count": len(rep.Hits), "flags": rep.Flags},
	}, nil
}
