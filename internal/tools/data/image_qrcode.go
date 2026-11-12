// data.extract.image-qrcode —— 图片二维码识别。
//
// 上传单张/多张图片（可整个文件夹），逐张解码像素并识别其中的二维码，
// 记录每个码的内容与格式，输出结构化 JSON（stdout）供前端渲染器消费，
// 同时在产物目录写出 qrcode_report.json（美化版，可直接下载）。
// 识别能力：QR Code / Data Matrix / Aztec（gozxing，CTF 常见二维条码一并覆盖）；
// 大图自动降采样加速；单图最多识别 8 个码。
package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"ctfnet/internal/core"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/aztec"
	"github.com/makiuchi-d/gozxing/datamatrix"
	"github.com/makiuchi-d/gozxing/qrcode"
	xgbmp "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	xgtiff "golang.org/x/image/tiff"
	xgwebp "golang.org/x/image/webp"
)

func init() {
	core.DefaultRegistry.Register(&ImageQrcodeTool{BaseTool: core.BaseTool{
		NameAttr:        "image-qrcode",
		CategoryAttr:    core.Category("data.extract"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "识别图片中的二维码并记录内容（QR/DataMatrix/Aztec/PDF417，支持批量与文件夹）",
		TimeoutAttr:     600,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "图片文件（可多选或整个文件夹，递归收集全部图片）"},
		},
		Runner: imageQrcodeRun,
	}})
}

// ImageQrcodeTool 图片二维码识别工具。
type ImageQrcodeTool struct {
	core.BaseTool
}

// qrCodeOut 单个识别出的二维码。
type qrCodeOut struct {
	Text   string `json:"text"`   // 解码内容（UTF-8 优先）
	Format string `json:"format"` // "QR Code" / "Data Matrix" / "Aztec" / "PDF417"
}

// qrcodeFileRecord 单张图片的识别记录。
type qrcodeFileRecord struct {
	Name   string      `json:"name"`
	Format string      `json:"format"` // 图片格式
	Codes  []qrCodeOut `json:"codes"`
}

type qrcodeSkippedFile struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type qrcodeOutput struct {
	Summary      qrcodeSummary       `json:"summary"`
	Images       []qrcodeFileRecord  `json:"images"`
	SkippedFiles []qrcodeSkippedFile `json:"skipped_files"`
}

type qrcodeSummary struct {
	Total      int            `json:"total"`
	Analyzed   int            `json:"analyzed"`
	Skipped    int            `json:"skipped"`
	WithQR     int            `json:"with_qr"`
	NoQR       int            `json:"no_qr"`
	TotalCodes int            `json:"total_codes"`
	Formats    map[string]int `json:"formats"`
}

// ---------------------------------------------------------------------------
// 像素解码 + 二维码识别
// ---------------------------------------------------------------------------

// decodeImagePixels 按格式把图片字节解码为像素图（仅支持二维码工具需要的常见格式）。
func decodeImagePixels(data []byte) (image.Image, string, error) {
	format, _, _, _ := sniffImageFormat(data)
	r := bytes.NewReader(data)
	switch format {
	case "JPEG":
		img, err := jpeg.Decode(r)
		return img, format, err
	case "PNG":
		img, err := png.Decode(r)
		return img, format, err
	case "GIF":
		img, err := gif.Decode(r)
		return img, format, err
	case "BMP":
		img, err := xgbmp.Decode(r)
		return img, format, err
	case "WebP":
		// x/image/webp 仅支持有损 VP8；无损 VP8L 会解码失败（走 skipped）
		img, err := xgwebp.Decode(r)
		return img, format, err
	case "TIFF":
		img, err := xgtiff.Decode(r)
		return img, format, err
	}
	return nil, format, fmt.Errorf("无法识别的图片格式")
}

// toRGBA 统一转为 RGBA，便于生成 gozxing 需要的 RGB 亮度源。
func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// downscaleRGBA 超大图降采样，限制二维码解码的像素规模（QR 不需要全分辨率）。
const maxDecodePixels = 3_500_000

func downscaleRGBA(img *image.RGBA) *image.RGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	pixels := w * h
	if pixels <= maxDecodePixels || pixels <= 0 {
		return img
	}
	f := float64(maxDecodePixels) / float64(pixels)
	// f = 目标像素/当前像素；边长缩放取平方根
	s := math.Sqrt(f)
	nw := int(float64(w) * s)
	nh := int(float64(h) * s)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

// rgbaLuminance 把 RGBA 转为 gozxing RGB 亮度源。
func rgbaLuminance(img *image.RGBA) gozxing.LuminanceSource {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	pix := make([]int, w*h)
	for i := range pix {
		o := i * 4
		r, g, b := int(img.Pix[o]), int(img.Pix[o+1]), int(img.Pix[o+2])
		pix[i] = (r << 16) | (g << 8) | b
	}
	return gozxing.NewRGBLuminanceSource(w, h, pix)
}

// codeFormatName 把 gozxing 格式枚举转成展示名。
func codeFormatName(f gozxing.BarcodeFormat) string {
	switch f {
	case gozxing.BarcodeFormat_QR_CODE:
		return "QR Code"
	case gozxing.BarcodeFormat_DATA_MATRIX:
		return "Data Matrix"
	case gozxing.BarcodeFormat_AZTEC:
		return "Aztec"
	default:
		return f.String()
	}
}

// decodeCodeText 解码内容文本：原始字节优先按 UTF-8 解释，无效时回退库的默认字符集。
func decodeCodeText(res *gozxing.Result) string {
	raw := res.GetRawBytes()
	if len(raw) > 0 && utf8.Valid(raw) {
		return strings.TrimRight(string(raw), "\x00")
	}
	return strings.TrimRight(res.GetText(), "\x00")
}

// maskCodeRegion 用白色覆盖已识别二维码所在区域，便于循环识别下一枚。
func maskCodeRegion(img *image.RGBA, pts []gozxing.ResultPoint, margin int) bool {
	if len(pts) < 2 {
		return false
	}
	b := img.Bounds()
	x0, y0 := b.Max.X, b.Max.Y
	x1, y1 := b.Min.X, b.Min.Y
	for _, p := range pts {
		x, y := int(p.GetX()), int(p.GetY())
		if x < x0 {
			x0 = x
		}
		if x > x1 {
			x1 = x
		}
		if y < y0 {
			y0 = y
		}
		if y > y1 {
			y1 = y
		}
	}
	x0 -= margin
	y0 -= margin
	x1 += margin
	y1 += margin
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return false
	}
	draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	return true
}

// scanQRCodes 识别一张图片中的全部二维码（逐个识别并遮罩，直到找不出为止）。
const maxCodesPerImage = 8

func scanQRCodes(img image.Image) []qrCodeOut {
	rgba := downscaleRGBA(toRGBA(img))
	// gozxing v0.1.1 无 MultiFormatReader，按格式轮询独立 reader（QR 优先）
	readers := []gozxing.Reader{
		qrcode.NewQRCodeReader(),
		datamatrix.NewDataMatrixReader(),
		aztec.NewAztecReader(),
	}
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}

	var codes []qrCodeOut
	for i := 0; i < maxCodesPerImage; i++ {
		bmp, err := gozxing.NewBinaryBitmap(gozxing.NewHybridBinarizer(rgbaLuminance(rgba)))
		if err != nil {
			break
		}
		var res *gozxing.Result
		for _, rd := range readers {
			r, err := rd.Decode(bmp, hints)
			if err == nil {
				res = r
				break
			}
		}
		if res == nil {
			break // 所有格式都找不到更多码
		}
		codes = append(codes, qrCodeOut{
			Text:   decodeCodeText(res),
			Format: codeFormatName(res.GetBarcodeFormat()),
		})
		if !maskCodeRegion(rgba, res.GetResultPoints(), 8) {
			break // 无法定位区域，避免死循环重复识别同一枚
		}
	}
	return codes
}

// ---------------------------------------------------------------------------
// 工具主流程
// ---------------------------------------------------------------------------

func imageQrcodeRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	_ = ctx
	names, _ := asStringSliceLocal(params["file"])
	if len(names) == 0 {
		if s, ok := params["file"].(string); ok && s != "" {
			names = []string{s}
		}
	}
	if len(names) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少图片文件参数"}, nil
	}
	if len(names) > 500 {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("文件数量过多（%d，上限 500）", len(names))}, nil
	}
	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	const maxSize = 100 << 20

	out := qrcodeOutput{
		Images:       make([]qrcodeFileRecord, 0, len(names)),
		SkippedFiles: make([]qrcodeSkippedFile, 0),
	}
	out.Summary.Formats = map[string]int{}
	out.Summary.Total = len(names)

	for _, n := range names {
		path := filepath.Join(workdir, filepath.FromSlash(n))
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			out.SkippedFiles = append(out.SkippedFiles, qrcodeSkippedFile{n, "文件不存在或为目录"})
			continue
		}
		if st.Size() > maxSize {
			out.SkippedFiles = append(out.SkippedFiles, qrcodeSkippedFile{n, "超过 100MB 上限"})
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			out.SkippedFiles = append(out.SkippedFiles, qrcodeSkippedFile{n, "读取失败"})
			continue
		}
		img, format, err := decodeImagePixels(data)
		if err != nil {
			out.SkippedFiles = append(out.SkippedFiles, qrcodeSkippedFile{n, "图片解码失败: " + err.Error()})
			continue
		}
		codes := scanQRCodes(img)
		rec := qrcodeFileRecord{Name: n, Format: format, Codes: codes}
		if rec.Codes == nil {
			rec.Codes = []qrCodeOut{}
		}
		out.Images = append(out.Images, rec)
		out.Summary.Formats[format]++
		out.Summary.TotalCodes += len(codes)
		if len(codes) > 0 {
			out.Summary.WithQR++
		} else {
			out.Summary.NoQR++
		}
	}
	sort.Slice(out.Images, func(i, j int) bool { return out.Images[i].Name < out.Images[j].Name })
	out.Summary.Analyzed = len(out.Images)
	out.Summary.Skipped = len(out.SkippedFiles)

	pretty, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "序列化结果失败"}, nil
	}
	compact, err := json.Marshal(out)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "序列化结果失败"}, nil
	}
	// 产物：可下载的美化版报告
	if tc != nil && tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("创建产物目录失败: %v", err)}, nil
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "qrcode_report.json"), pretty, 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出报告失败: %v", err)}, nil
		}
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(compact)}, nil
}
