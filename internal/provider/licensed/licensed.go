// Package licensed 读取由应用运营方配置的本地媒体清单。
package licensed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"jindi-drama/internal/provider"
)

const MediaPrefix = "/_licensed/media/"
const maxManifest = 1 << 20

var dramaID = regexp.MustCompile(`^local-[a-z0-9][a-z0-9-]{0,47}$`)

type catalogEntry struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	License     string     `json:"license"`
	AllowExport bool       `json:"allowExport"`
	Tags        []string   `json:"tags"`
	Episodes    [][]string `json:"episodes"`
}

type mediaFile struct {
	name string
	mime string
}

type DemoProvider struct {
	mediaBase string
	root      *os.Root
	dramas    []provider.Drama
	index     map[string]int
	media     map[string]mediaFile
	sources   map[string][][]provider.MediaSource
}

func New(mediaDir string) (*DemoProvider, error) {
	p := &DemoProvider{}
	if mediaDir == "" {
		return p, nil
	}
	root, err := os.OpenRoot(mediaDir)
	if err != nil {
		return nil, errors.New("无法打开授权片库目录")
	}
	p.root = root
	if err := p.load(); err != nil {
		_ = p.Close()
		return nil, err
	}
	return p, nil
}

func (p *DemoProvider) Close() error {
	if p.root != nil {
		return p.root.Close()
	}
	return nil
}

func (p *DemoProvider) load() error {
	info, err := p.root.Stat("catalog.json")
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("授权片库缺少 catalog.json")
	}
	file, _, err := p.openRegularFile("catalog.json")
	if err != nil {
		return errors.New("无法读取授权片库清单")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxManifest+1))
	if err != nil || len(data) > maxManifest {
		return errors.New("授权片库清单读取失败或超过 1 MiB")
	}
	var entries []catalogEntry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&entries); err != nil || decoder.Decode(new(any)) != io.EOF || len(entries) == 0 {
		return errors.New("授权片库清单格式错误或没有条目")
	}
	p.index = make(map[string]int, len(entries))
	p.media = make(map[string]mediaFile)
	p.sources = make(map[string][][]provider.MediaSource, len(entries))
	for i, entry := range entries {
		if err := p.add(entry); err != nil {
			return fmt.Errorf("授权片库第 %d 条记录：%w", i+1, err)
		}
	}
	return nil
}

func (p *DemoProvider) add(entry catalogEntry) error {
	title, license := strings.TrimSpace(entry.Title), strings.TrimSpace(entry.License)
	if !dramaID.MatchString(entry.ID) || title == "" || license == "" || len(entry.Episodes) == 0 {
		return errors.New("需要有效的 local- 标识、标题、许可说明和分集")
	}
	if _, exists := p.index[entry.ID]; exists {
		return errors.New("剧集标识重复")
	}

	episodes := make([][]provider.MediaSource, len(entry.Episodes))
	for i, files := range entry.Episodes {
		if len(files) == 0 || len(files) > 2 {
			return fmt.Errorf("第 %d 集需要一至两个媒体文件", i+1)
		}
		for _, name := range files {
			src, err := p.addMedia(entry.ID, i+1, name)
			if err != nil {
				return fmt.Errorf("第 %d 集：%w", i+1, err)
			}
			episodes[i] = append(episodes[i], src)
		}
	}
	p.sources[entry.ID] = episodes
	p.index[entry.ID] = len(p.dramas)
	p.dramas = append(p.dramas, provider.Drama{
		ID: entry.ID, Title: title, Summary: entry.Summary, Tags: entry.Tags, License: license,
		AllowExport: entry.AllowExport, UpdatedTo: len(episodes), EpisodeCount: len(episodes),
	})
	return nil
}

func (p *DemoProvider) addMedia(id string, episode int, name string) (provider.MediaSource, error) {
	ext := strings.ToLower(path.Ext(name))
	mime := map[string]string{".mp4": "video/mp4", ".webm": "video/webm"}[ext]
	if !validRelativeMediaPath(name) || mime == "" {
		return provider.MediaSource{}, errors.New("媒体必须是片库内的 MP4 或 WebM 相对路径")
	}
	file, _, err := p.openMedia(name)
	if err != nil {
		return provider.MediaSource{}, err
	}
	_ = file.Close()
	url := MediaPrefix + id + "/" + strconv.Itoa(episode) + "/clip" + ext
	if _, exists := p.media[url]; exists {
		return provider.MediaSource{}, errors.New("同一分集的媒体格式重复")
	}
	p.media[url] = mediaFile{name: name, mime: mime}
	return provider.MediaSource{Src: url, Type: mime}, nil
}

func validRelativeMediaPath(name string) bool {
	return name != "." && !strings.ContainsAny(name, `\:`) && fs.ValidPath(name) && filepath.IsLocal(name)
}

func (*DemoProvider) Name() string { return "licensed-demo" }

func (p *DemoProvider) Capabilities(context.Context) provider.Capabilities {
	c := provider.Capabilities{}
	for _, k := range provider.AllCapabilities {
		c[k] = provider.Unavailable
	}
	if p.root != nil {
		c["catalog"], c["search"], c["playback"] = provider.Ready, provider.Ready, provider.Ready
	}
	return c
}

func (p *DemoProvider) Home(context.Context) ([]provider.Drama, error) {
	if p.root == nil {
		return nil, provider.ErrUnsupported
	}
	result := make([]provider.Drama, 0, len(p.dramas))
	for _, item := range p.dramas {
		result = append(result, copyDrama(item))
	}
	return result, nil
}

func (p *DemoProvider) HomeFeed(ctx context.Context) (provider.HomeFeed, error) {
	items, err := p.Home(ctx)
	if err != nil {
		return provider.HomeFeed{}, err
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return provider.HomeFeed{TodayNew: ids}, nil
}

func (p *DemoProvider) Search(ctx context.Context, query string) ([]provider.Drama, error) {
	items, err := p.Home(ctx)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]provider.Drama, 0)
	for _, item := range items {
		text := strings.Join([]string{item.Title, item.Summary, strings.Join(item.Tags, " ")}, "\n")
		if strings.Contains(strings.ToLower(text), query) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (p *DemoProvider) Drama(_ context.Context, id string) (provider.Drama, error) {
	if i, ok := p.index[id]; ok {
		return copyDrama(p.dramas[i]), nil
	}
	return provider.Drama{}, provider.ErrNotFound
}

func copyDrama(item provider.Drama) provider.Drama {
	item.Tags = append([]string{}, item.Tags...)
	return item
}

func (p *DemoProvider) Episodes(ctx context.Context, id string) ([]provider.Episode, error) {
	item, err := p.Drama(ctx, id)
	if err != nil {
		return nil, err
	}
	episodes := make([]provider.Episode, item.EpisodeCount)
	for i := range episodes {
		episodes[i] = provider.Episode{ID: episodeID(id, i+1), DramaID: id, Index: i + 1, Available: true}
	}
	return episodes, nil
}

func (p *DemoProvider) ResolvePlayback(_ context.Context, id string, episode int) (provider.PlaybackTicket, error) {
	episodes, ok := p.sources[id]
	if !ok {
		return provider.PlaybackTicket{}, provider.ErrNotFound
	}
	if episode < 1 || episode > len(episodes) {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}
	sources := append([]provider.MediaSource(nil), episodes[episode-1]...)
	if p.mediaBase != "" {
		for i := range sources {
			sources[i].Src = p.mediaBase + sources[i].Src
		}
	}
	return provider.PlaybackTicket{
		Kind: "licensed", EpisodeID: episodeID(id, episode),
		Sources: sources,
	}, nil
}

func episodeID(id string, episode int) string {
	return id + "-" + strconv.Itoa(episode)
}

// OpenExport opens the registered local source permitted for a single episode.
func (p *DemoProvider) OpenExport(id string, episode int) (*os.File, string, error) {
	i, ok := p.index[id]
	if !ok {
		return nil, "", provider.ErrNotFound
	}
	if !p.dramas[i].AllowExport {
		return nil, "", errors.New("该短剧的片库配置未开放导出")
	}
	episodes := p.sources[id]
	if episode < 1 || episode > len(episodes) {
		return nil, "", provider.ErrEpisodeUnavailable
	}
	source := episodes[episode-1][0]
	for _, candidate := range episodes[episode-1] {
		if candidate.Type == "video/mp4" {
			source = candidate
			break
		}
	}
	file, _, err := p.openMedia(p.media[source.Src].name)
	return file, path.Ext(source.Src), err
}

func (p *DemoProvider) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, MediaPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		media, ok := p.media[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		p.serveMedia(w, r, media)
	})
}

func (p *DemoProvider) openMedia(name string) (*os.File, os.FileInfo, error) {
	const unavailable = "找不到可读取的媒体文件"
	info, err := p.root.Stat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, nil, errors.New(unavailable)
	}
	file, info, err := p.openRegularFile(name)
	if err != nil {
		return nil, nil, errors.New(unavailable)
	}
	if info.Size() == 0 {
		_ = file.Close()
		return nil, nil, errors.New(unavailable)
	}
	return file, info, nil
}

// Validate the opened descriptor because the directory entry can change after Stat.
func (p *DemoProvider) openRegularFile(name string) (*os.File, os.FileInfo, error) {
	file, err := p.root.OpenFile(name, regularReadFlags, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, fs.ErrInvalid
	}
	return file, info, nil
}

func (p *DemoProvider) serveMedia(w http.ResponseWriter, r *http.Request, media mediaFile) {
	file, info, err := p.openMedia(media.name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", media.mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", info.ModTime(), file)
}
