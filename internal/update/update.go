// Package update 只负责「检查是否有新版本」：从 GitHub Releases 读取最新版本并比较。
// 不下载、不静默安装；用户通过浏览器打开 Release 页面自行下载（安装包签名前不做自动替换）。
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultAPI = "https://api.github.com/repos/wuekevin/JindiDrama/releases/latest"

type Result struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	HasUpdate bool   `json:"hasUpdate"`
	URL       string `json:"url"`
	Notes     string `json:"notes"`
	// NoRelease 为 true 表示仓库尚未发布任何 Release（不是错误）。
	NoRelease bool `json:"noRelease"`
}

type release struct {
	Tag  string `json:"tag_name"`
	URL  string `json:"html_url"`
	Body string `json:"body"`
	Pre  bool   `json:"prerelease"`
}

// Check 请求 api 并与 current 比较。
func Check(ctx context.Context, client *http.Client, api, current string) (Result, error) {
	res := Result{Current: current}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "JindiDrama-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return res, fmt.Errorf("检查更新失败：网络不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		res.NoRelease = true
		return res, nil
	}
	if resp.StatusCode != http.StatusOK {
		return res, fmt.Errorf("检查更新失败：服务返回 %d", resp.StatusCode)
	}
	var r release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return res, errors.New("检查更新失败：响应无法解析")
	}
	res.Latest, res.URL, res.Notes = r.Tag, r.URL, r.Body
	if len(res.Notes) > 4000 {
		res.Notes = res.Notes[:4000]
	}
	cmp, err := Compare(r.Tag, current)
	if err != nil {
		return res, err
	}
	res.HasUpdate = cmp > 0
	return res, nil
}

// Compare 比较 a、b 的 x.y.z（允许前缀 v 与 -预发布后缀，预发布低于同号正式版）。
func Compare(a, b string) (int, error) {
	pa, prea, err := parse(a)
	if err != nil {
		return 0, err
	}
	pb, preb, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1, nil
			}
			return -1, nil
		}
	}
	switch {
	case prea == preb:
		return 0, nil
	case prea == "":
		return 1, nil
	case preb == "":
		return -1, nil
	case prea > preb:
		return 1, nil
	}
	return -1, nil
}

func parse(v string) ([3]int, string, error) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	pre := ""
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, pre = v[:i], v[i+1:]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, "", fmt.Errorf("无法识别的版本号：%q", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, "", fmt.Errorf("无法识别的版本号：%q", v)
		}
		out[i] = n
	}
	return out, pre, nil
}

// SafeReleaseURL 仅允许打开本项目的 GitHub 页面，避免被外部响应诱导打开任意链接。
func SafeReleaseURL(u string) bool {
	return strings.HasPrefix(u, "https://github.com/wuekevin/JindiDrama/")
}
