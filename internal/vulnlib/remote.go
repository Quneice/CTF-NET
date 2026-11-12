// 远程图片拉取（漏洞浏览器用）：http(s) 图片经全局代理（core.AppConfig.Proxy.URL）
// 转发为 base64 data URL，未配置代理时直连。
// 全局代理支持 http/https/socks5/socks5h（stdlib http.Transport 原生支持）。
package vulnlib

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ctfnet/internal/core"
)

// errRemoteNotImage 远程资源不是图片（Content-Type 非 image/*）。
var errRemoteNotImage = errors.New("远程资源不是图片")

// maxRemoteBytes 远程图片上限（与本地资源一致）。
const maxRemoteBytes = maxAssetBytes

// FetchRemoteImage 拉取远程图片并返回 base64 数据。
// 限制：仅 http/https、20s 超时、≤10MB、Content-Type 须为 image/*（缺失时按扩展名猜）。
func (s *Store) FetchRemoteImage(rawurl string) (*AssetData, error) {
	u, err := url.Parse(strings.TrimSpace(rawurl))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("仅支持 http/https 远程图片地址")
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // CTF 工具：容忍自签证书
	}
	if pu := strings.TrimSpace(core.GetAppConfig().Proxy.URL); pu != "" {
		proxyURL, perr := url.Parse(pu)
		if perr != nil || proxyURL.Host == "" {
			return nil, fmt.Errorf("全局代理地址不合法：%s", pu)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// 浏览器 UA：部分图床按 UA 反爬/防盗链
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("远程图片响应 %d", resp.StatusCode)
	}

	mime := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if mime == "" || mime == "application/octet-stream" {
		mime = mimeByExt(u.Path) // 响应头缺失时按 URL 扩展名
	}
	if !strings.HasPrefix(mime, "image/") {
		return nil, errRemoteNotImage
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRemoteBytes {
		return nil, errors.New("远程图片过大（>10MB）")
	}
	return &AssetData{
		Mime:    mime,
		DataB64: base64.StdEncoding.EncodeToString(data),
		Size:    int64(len(data)),
		AbsPath: rawurl,
	}, nil
}
