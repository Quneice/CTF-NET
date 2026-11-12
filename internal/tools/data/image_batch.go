// data.extract.image-batch —— 图片批量分析。
//
// 上传多张图片（可整个文件夹），逐张提取格式/尺寸/EXIF 相机参数/GPS 定位，
// 输出结构化 JSON（stdout）供前端交互式筛选渲染器消费，
// 同时在产物目录写出 batch_report.json（美化版，可直接下载）。
// 筛选（国内/国外、省份、格式、尺寸、机型、有无 GPS）在前端客户端完成，
// 无需重复上传。
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&ImageBatchTool{BaseTool: core.BaseTool{
		NameAttr:        "image-batch",
		CategoryAttr:    core.Category("data.extract"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "批量上传图片/文件夹，自动提取元数据，交互式筛选（国内/国外、尺寸、格式等）",
		TimeoutAttr:     600,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "图片文件（可多选或整个文件夹，递归收集全部图片）"},
		},
		Runner: imageBatchRun,
	}})
}

// ImageBatchTool 图片批量分析工具。
type ImageBatchTool struct {
	core.BaseTool
}

// batchImageRecord 单张图片的分析记录（前端筛选渲染器消费的字段）。
type batchImageRecord struct {
	Name     string  `json:"name"`
	Format   string  `json:"format"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Size     int64   `json:"size"`
	MD5      string  `json:"md5"`
	Make     string  `json:"make"`
	Model    string  `json:"model"`
	ShotTime string  `json:"shot_time"`
	HasGPS   bool    `json:"has_gps"`
	GPSLat   float64 `json:"gps_lat,omitempty"`
	GPSLon   float64 `json:"gps_lon,omitempty"`
	// Location: "中国 · 省份" / "国外" / ""（无 GPS）
	Location string `json:"location"`
}

type batchSkippedFile struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type batchOutput struct {
	Summary      batchSummary        `json:"summary"`
	Images       []batchImageRecord  `json:"images"`
	SkippedFiles []batchSkippedFile  `json:"skipped_files"`
}

type batchSummary struct {
	Total     int            `json:"total"`
	Analyzed  int            `json:"analyzed"`
	Skipped   int            `json:"skipped"`
	Domestic  int            `json:"domestic"`
	Abroad    int            `json:"abroad"`
	NoGPS     int            `json:"no_gps"`
	TotalSize int64          `json:"total_size"`
	Formats   map[string]int `json:"formats"`
	Provinces map[string]int `json:"provinces"`
}

// kvValue 取报告中首个匹配 key 的值。
func kvValue(kvs []kv, key string) string {
	for _, item := range kvs {
		if item.k == key {
			return item.v
		}
	}
	return ""
}

// buildBatchRecord 从共享分析报告构造批量记录。
func buildBatchRecord(rep *imageReport) batchImageRecord {
	rec := batchImageRecord{
		Name:     rep.fileName,
		Format:   rep.format,
		Width:    rep.width,
		Height:   rep.height,
		Size:     rep.size,
		MD5:      rep.md5,
		Make:     kvValue(rep.main, "相机制造商"),
		Model:    kvValue(rep.main, "相机型号"),
		ShotTime: kvValue(rep.exif, "拍摄时间"),
	}
	if rec.ShotTime == "" {
		rec.ShotTime = kvValue(rep.main, "修改时间")
	}
	if rep.gps != nil {
		rec.HasGPS = true
		rec.GPSLat = rep.gps.latDec
		rec.GPSLon = rep.gps.lonDec
		if loc := lookupChinaRegion(rep.gps.latDec, rep.gps.lonDec); loc != "" {
			rec.Location = "中国 · " + loc
		} else {
			rec.Location = "国外"
		}
	}
	return rec
}

func imageBatchRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
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
	if len(names) > 5000 {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("文件数量过多（%d，上限 5000）", len(names))}, nil
	}
	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	const maxSize = 100 << 20

	out := batchOutput{
		Images:       make([]batchImageRecord, 0, len(names)),
		SkippedFiles: make([]batchSkippedFile, 0),
	}
	out.Summary.Formats = map[string]int{}
	out.Summary.Provinces = map[string]int{}
	out.Summary.Total = len(names)

	for _, n := range names {
		path := filepath.Join(workdir, filepath.FromSlash(n))
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			out.SkippedFiles = append(out.SkippedFiles, batchSkippedFile{n, "文件不存在或为目录"})
			continue
		}
		if st.Size() > maxSize {
			out.SkippedFiles = append(out.SkippedFiles, batchSkippedFile{n, "超过 100MB 上限"})
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			out.SkippedFiles = append(out.SkippedFiles, batchSkippedFile{n, "读取失败"})
			continue
		}
		rep := analyzeImageBytes(n, data)
		if rep.format == "未知" {
			out.SkippedFiles = append(out.SkippedFiles, batchSkippedFile{n, "无法识别的图片格式"})
			continue
		}
		rec := buildBatchRecord(rep)
		out.Images = append(out.Images, rec)
		out.Summary.Formats[rec.Format]++
		switch {
		case rec.Location == "":
			out.Summary.NoGPS++
		case strings.HasPrefix(rec.Location, "中国 · "):
			out.Summary.Domestic++
			if prov := strings.TrimPrefix(rec.Location, "中国 · "); prov != "" {
				out.Summary.Provinces[prov]++
			}
		default:
			out.Summary.Abroad++
		}
		out.Summary.TotalSize += rec.Size
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
	// 产物：可下载的美化版报告（collectArtifacts 自动收集顶层文件）
	if tc != nil && tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("创建产物目录失败: %v", err)}, nil
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "batch_report.json"), pretty, 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出报告失败: %v", err)}, nil
		}
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(compact)}, nil
}

// asStringSliceLocal 包内版参数数组归一（core 的同名函数未导出）。
func asStringSliceLocal(v any) ([]string, bool) {
	switch arr := v.(type) {
	case []string:
		return arr, true
	case []any:
		out := make([]string, 0, len(arr))
		for _, it := range arr {
			s, ok := it.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}
