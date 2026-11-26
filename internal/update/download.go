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

var (
	errAssetDownloadTimeout = errors.New("下载更新超时，请重试")
	errWriteUpdateStage     = errors.New("无法写入更新文件，请检查保存位置和可用空间")
)

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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return errAssetDownloadTimeout
		}
		return errors.New("无法连接到 GitHub 下载更新")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return assetResponseError(resp)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != a.size {
		return errors.New("更新文件大小校验失败")
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(assetStageWriter{target}, hash), io.LimitReader(resp.Body, a.size+1))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	switch {
	case errors.Is(err, errWriteUpdateStage):
		return errWriteUpdateStage
	case errors.Is(err, context.DeadlineExceeded):
		return errAssetDownloadTimeout
	case err != nil || n != a.size:
		return errors.New("更新文件传输中断或大小不符，请重试")
	}
	if [sha256.Size]byte(hash.Sum(nil)) != a.digest {
		return errors.New("更新文件 SHA-256 校验失败")
	}
	return ctx.Err()
}

func assetResponseError(resp *http.Response) error {
	status := resp.StatusCode
	switch {
	case status == http.StatusTooManyRequests || status == http.StatusForbidden &&
		(resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != ""):
		return errors.New("GitHub 下载请求过于频繁，请稍后重试或前往发布页下载")
	case status == http.StatusForbidden || status == http.StatusUnauthorized:
		return errors.New("GitHub 拒绝下载该安装包，请前往发布页确认访问权限")
	case status == http.StatusNotFound || status == http.StatusGone:
		return errors.New("GitHub 暂未提供该安装包，请重新检查更新")
	case status >= 500 && status < 600:
		return errors.New("GitHub 更新下载服务暂时不可用，请稍后重试")
	default:
		return errors.New("无法从 GitHub 获取安装包，请前往发布页下载")
	}
}

type assetStageWriter struct{ io.Writer }

func (w assetStageWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err != nil || n != len(p) {
		return n, errWriteUpdateStage
	}
	return n, nil
}
