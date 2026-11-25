// Package update 检查 GitHub Releases，并提供已验证版本的浏览器链接。
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	DefaultPreviewAPI = "https://api.github.com/repos/wuekevin/JindiDrama/releases"
	DefaultAPI        = DefaultPreviewAPI + "/latest"
	Stable            = "stable"
	Preview           = "preview"
	previewPageSize   = 20
	maxPreviewPages   = 10
	releaseURLPrefix  = "https://github.com/wuekevin/JindiDrama/releases/tag/"
	maxResponse       = 1 << 20
	maxNotes          = 4000
)

type Result struct {
	Current      string `json:"current"`
	Latest       string `json:"latest"`
	HasUpdate    bool   `json:"hasUpdate"`
	URL          string `json:"url"`
	Notes        string `json:"notes"`
	NoRelease    bool   `json:"noRelease"`
	Preview      bool   `json:"preview"`
	Channel      string `json:"channel"`
	DownloadName string `json:"downloadName,omitempty"`
	Download     *Asset `json:"-"`
}

type release struct {
	Tag    string         `json:"tag_name"`
	URL    string         `json:"html_url"`
	Body   string         `json:"body"`
	Pre    *bool          `json:"prerelease"`
	Draft  *bool          `json:"draft"`
	Assets []releaseAsset `json:"assets"`
}

// Check queries the stable channel. It exposes a download page only for a newer release.
func Check(ctx context.Context, client *http.Client, api, current string) (Result, error) {
	res := Result{Current: current, Channel: Stable}
	installed, err := normalizedVersion(current)
	if err != nil {
		return res, err
	}
	var r release
	_, found, err := readReleaseJSON(ctx, client, api, &r)
	if err != nil {
		return res, err
	}
	if !found {
		res.NoRelease = true
		return res, nil
	}
	if r.Pre == nil || r.Draft == nil {
		return res, errors.New("检查更新失败：缺少发布状态")
	}
	if *r.Pre || *r.Draft {
		res.NoRelease = true
		return res, nil
	}
	latest, err := normalizedVersion(r.Tag)
	if err != nil {
		return res, err
	}
	if semver.Prerelease(latest) != "" {
		res.NoRelease = true
		return res, nil
	}
	return releaseResult(res, r, installed, latest)
}

// CheckPreview checks all bounded pages for the greatest published SemVer.
func CheckPreview(ctx context.Context, client *http.Client, api, current string) (Result, error) {
	res := Result{Current: current, Channel: Preview}
	installed, err := normalizedVersion(current)
	if err != nil {
		return res, err
	}
	endpoint, err := url.Parse(api)
	if err != nil {
		return res, err
	}
	query := endpoint.Query()
	query.Set("per_page", strconv.Itoa(previewPageSize))
	var winner release
	best := ""
	invalidLink := false
	for page := 1; page <= maxPreviewPages; page++ {
		query.Set("page", strconv.Itoa(page))
		endpoint.RawQuery = query.Encode()
		var entries []release
		headers, found, err := readReleaseJSON(ctx, client, endpoint.String(), &entries)
		if err != nil {
			return res, err
		}
		if !found && page == 1 {
			res.NoRelease = true
			return res, nil
		}
		if !found || entries == nil {
			return res, errors.New("检查预发布更新失败：发布列表无效")
		}
		for _, entry := range entries {
			if entry.Pre == nil || entry.Draft == nil {
				return res, errors.New("检查预发布更新失败：缺少发布状态")
			}
			version, err := normalizedVersion(entry.Tag)
			if *entry.Draft || err != nil || *entry.Pre != (semver.Prerelease(version) != "") {
				continue
			}
			if tag, allowed := releasePageTag(entry.URL); !allowed || tag != entry.Tag {
				invalidLink = true
				continue
			}
			if semver.Compare(version, best) > 0 {
				best, winner = version, entry
			}
		}
		// Use the presence of next, keeping requests on the configured endpoint.
		if !strings.Contains(strings.Join(headers.Values("Link"), ","), `rel="next"`) {
			if best == "" {
				if invalidLink {
					return res, errors.New("检查预发布更新失败：版本下载页面无效")
				}
				res.NoRelease = true
				return res, nil
			}
			return releaseResult(res, winner, installed, best)
		}
	}
	return res, errors.New("检查预发布更新失败：发布列表超出分页限制")
}

func releaseResult(res Result, r release, installed, latest string) (Result, error) {
	tag, allowed := releasePageTag(r.URL)
	if !allowed || tag != r.Tag {
		return res, errors.New("检查更新失败：版本下载页面无效")
	}
	res.Latest = r.Tag
	res.Preview = semver.Prerelease(latest) != ""
	if semver.Compare(latest, installed) > 0 {
		res.HasUpdate = true
		res.URL = releaseURLPrefix + url.PathEscape(tag)
		notes := []rune(r.Body)
		res.Notes = string(notes[:min(len(notes), maxNotes)])
		res.Download = selectAsset(r.Assets, runtime.GOOS, runtime.GOARCH)
		if res.Download != nil {
			res.DownloadName = res.Download.Name
		}
	}
	return res, nil
}

func readReleaseJSON(ctx context.Context, client *http.Client, api string, target any) (http.Header, bool, error) {
	if client == nil {
		client = &http.Client{
			Timeout:       8 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "JindiDrama-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, errors.New("检查更新失败：网络不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("检查更新失败：服务返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(body) > maxResponse {
		return nil, false, errors.New("检查更新失败：响应读取失败或超出大小限制")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return nil, false, errors.New("检查更新失败：响应无法解析")
	}
	return resp.Header, true, nil
}

// Compare compares full SemVer versions with or without the conventional v prefix.
func Compare(a, b string) (int, error) {
	left, err := normalizedVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := normalizedVersion(b)
	if err != nil {
		return 0, err
	}
	return semver.Compare(left, right), nil
}

// ValidPackageVersion checks the version injected into desktop release builds.
func ValidPackageVersion(version string) bool {
	_, err := normalizedVersion(version)
	return strings.HasPrefix(version, "v") && err == nil
}

func normalizedVersion(version string) (string, error) {
	full := "v" + strings.TrimPrefix(version, "v")
	core := full
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		core = core[:i]
	}
	if strings.Count(core, ".") != 2 || !semver.IsValid(full) {
		return "", fmt.Errorf("无法识别的版本号：%.80q", version)
	}
	return full, nil
}

func releasePageTag(link string) (string, bool) {
	escaped, ok := strings.CutPrefix(link, releaseURLPrefix)
	if !ok {
		return "", false
	}
	tag, err := url.PathUnescape(escaped)
	if err != nil {
		return "", false
	}
	_, err = normalizedVersion(tag)
	return tag, err == nil
}
