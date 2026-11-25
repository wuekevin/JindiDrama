package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxAssetSize = 512 << 20

type releaseAsset struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

type Asset struct {
	Name   string
	id     int64
	size   int64
	digest [sha256.Size]byte
}

func selectAsset(entries []releaseAsset, system, arch string) *Asset {
	var name string
	switch {
	case system == "windows" && arch == "amd64":
		name = "JindiDrama-amd64-installer.exe"
	case system == "darwin" && (arch == "arm64" || arch == "amd64"):
		name = "JindiDrama-macos-universal.zip"
	default:
		return nil
	}
	var selected *Asset
	for _, entry := range entries {
		if entry.Name != name {
			continue
		}
		digest, found := strings.CutPrefix(entry.Digest, "sha256:")
		decoded, err := hex.DecodeString(digest)
		if selected != nil || entry.ID <= 0 || entry.State != "uploaded" ||
			entry.Size <= 0 || entry.Size > maxAssetSize || !found || err != nil || len(decoded) != sha256.Size {
			return nil
		}
		selected = &Asset{Name: name, id: entry.ID, size: entry.Size, digest: [sha256.Size]byte(decoded)}
	}
	return selected
}

func allowedAssetRedirect(target *url.URL) bool {
	return target.Scheme == "https" && target.User == nil && target.Fragment == "" &&
		(target.Host == "release-assets.githubusercontent.com" || target.Host == "objects.githubusercontent.com")
}

func assetClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 3 || !allowedAssetRedirect(req.URL) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// Download writes to a caller-owned staging file. Publish it only on success.
func (a *Asset) Download(ctx context.Context, target io.Writer) error {
	endpoint := DefaultPreviewAPI + "/assets/" + strconv.FormatInt(a.id, 10)
	return a.download(ctx, target, assetClient(), endpoint)
}

func (a *Asset) download(ctx context.Context, target io.Writer, client *http.Client, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "JindiDrama-update-download")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("无法连接到 GitHub 下载更新")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("GitHub 暂未提供该安装包，请重新检查更新")
	}
	if resp.ContentLength >= 0 && resp.ContentLength != a.size {
		return errors.New("更新文件大小校验失败")
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(target, hash), io.LimitReader(resp.Body, a.size+1))
	if err != nil || n != a.size {
		return errors.New("更新文件传输中断或大小不符，请重试")
	}
	if [sha256.Size]byte(hash.Sum(nil)) != a.digest {
		return errors.New("更新文件 SHA-256 校验失败")
	}
	return ctx.Err()
}
