// geo_china.go —— 中国省级行政区边界多边形（GeoJSON）+ 射线法定位。
//
// 数据来源：阿里云 DataV 公开省界数据
// （https://geo.datav.aliyun.com/areas_v3/bound/100000_full.json），
// 经预处理（剔除九段线装饰要素、坐标四舍五入到 6 位小数、
// 剥离多余属性）后 gzip 压缩为 geodata/china_provinces.json.gz，
// 通过 go:embed 编译进二进制，运行时零外部文件依赖。
//
// 判定方法：射线法（偶奇规则）点内判定。对 MultiPolygon 的全部环
// （外环 + 洞环）做射线交叉计数，偶数在外、奇数在内，天然支持
// 带洞多边形与飞地（如海南省的南海诸岛、浙江的众多岛屿）。
// 匹配前先做包围盒预筛加速；全部未命中 → 国外。
package data

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"math"
	"sync"
)

//go:embed geodata/china_provinces.json.gz
var chinaGeoGz []byte

type provincePolygon struct {
	Name  string          `json:"name"`
	Rings [][][]float64   `json:"rings"` // [ring][point][lon,lat]
	minLon, maxLon        float64        // 包围盒（load 时计算，用于预筛）
	minLat, maxLat        float64
}

var (
	polyOnce   sync.Once
	chinaPolys []provincePolygon
	polyErr    error
)

// loadChinaPolygons 惰性解压并解析一次嵌入的地理数据（线程安全）。
func loadChinaPolygons() ([]provincePolygon, error) {
	polyOnce.Do(func() {
		r, err := gzip.NewReader(bytes.NewReader(chinaGeoGz))
		if err != nil {
			polyErr = err
			return
		}
		raw, err := io.ReadAll(r)
		if err != nil {
			polyErr = err
			return
		}
		if err := json.Unmarshal(raw, &chinaPolys); err != nil {
			polyErr = err
			return
		}
		for i := range chinaPolys {
			p := &chinaPolys[i]
			p.minLon, p.minLat = math.Inf(1), math.Inf(1)
			p.maxLon, p.maxLat = math.Inf(-1), math.Inf(-1)
			for _, ring := range p.Rings {
				for _, pt := range ring {
					if pt[0] < p.minLon {
						p.minLon = pt[0]
					}
					if pt[0] > p.maxLon {
						p.maxLon = pt[0]
					}
					if pt[1] < p.minLat {
						p.minLat = pt[1]
					}
					if pt[1] > p.maxLat {
						p.maxLat = pt[1]
					}
				}
			}
		}
	})
	return chinaPolys, polyErr
}

// pointInRings 射线法点内判定（偶奇规则）。
// 环为 [[lon,lat],...]，闭合与否均可；少于 3 点的退化环跳过。
// 所有环共享一个计数器：洞环会把外环已置的「在内」翻转回「在外」。
func pointInRings(lat, lon float64, rings [][][]float64) bool {
	inside := false
	for _, ring := range rings {
		if len(ring) < 3 {
			continue
		}
		j := len(ring) - 1
		for i := 0; i < len(ring); i++ {
			yi, xi := ring[i][1], ring[i][0]
			yj, xj := ring[j][1], ring[j][0]
			// 射线穿过边（跳过水平边与端点重合边）
			if (yi > lat) != (yj > lat) {
				xint := xi + (lat-yi)*(xj-xi)/(yj-yi)
				if lon < xint {
					inside = !inside
				}
			}
			j = i
		}
	}
	return inside
}

// lookupChinaRegion 按经纬度查找省级行政区（射线法点内判定）；
// 不在任何省份多边形内（即不在国内）返回空串。
func lookupChinaRegion(lat, lon float64) string {
	polys, err := loadChinaPolygons()
	if err != nil {
		return ""
	}
	for _, p := range polys {
		if lon < p.minLon || lon > p.maxLon || lat < p.minLat || lat > p.maxLat {
			continue
		}
		if pointInRings(lat, lon, p.Rings) {
			return p.Name
		}
	}
	return ""
}
