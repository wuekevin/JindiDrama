package app

import (
	"encoding/binary"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
)

type catalogHLSResources struct {
	mu    sync.Mutex
	index map[hlsResource]int
	items []hlsResource
}

func (h *catalogHLSResources) resource(path string) (hlsResource, bool) {
	indexText, playlist := strings.CutSuffix(path, ".m3u8")
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 || strconv.Itoa(index) != indexText {
		return hlsResource{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if index >= len(h.items) || h.items[index].playlist != playlist {
		return hlsResource{}, false
	}
	return h.items[index], true
}

// Commit the entire checked manifest at once. A rejected manifest contributes
// no resources, and concurrent re-reads keep using the same local addresses.
func (h *catalogHLSResources) publish(manifest hlsManifest, prefix string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	pending := make(map[hlsResource]int)
	var additions []hlsResource
	var output strings.Builder
	for _, line := range manifest.lines {
		output.WriteString(line.prefix)
		resource := line.resource
		if resource.url != "" {
			index, exists := h.index[resource]
			if !exists {
				index, exists = pending[resource]
			}
			if !exists {
				index = len(h.items) + len(additions)
				if index >= maxCatalogHLSResources {
					return "", false
				}
				pending[resource] = index
				additions = append(additions, resource)
			}
			output.WriteString(prefix)
			output.WriteString(strconv.Itoa(index))
			if resource.playlist {
				output.WriteString(".m3u8")
			}
			output.WriteString(line.suffix)
		}
		output.WriteByte('\n')
	}
	for key, index := range pending {
		h.index[key] = index
	}
	h.items = append(h.items, additions...)
	return output.String(), true
}

const catalogHLSOrigin = "http://wails.localhost"

func catalogHLSCORS(w http.ResponseWriter, req *http.Request) bool {
	w.Header().Add("Vary", "Origin")
	if len(req.Header.Values("Origin")) != 1 || req.Header.Get("Origin") != catalogHLSOrigin {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", catalogHLSOrigin)
	w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges")
	return true
}

func catalogHLSPreflight(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Vary", "Access-Control-Request-Method, Access-Control-Request-Headers")
	method := req.Header.Get("Access-Control-Request-Method")
	headers := req.Header.Get("Access-Control-Request-Headers")
	if (method != http.MethodGet && method != http.MethodHead) ||
		(headers != "" && !strings.EqualFold(strings.TrimSpace(headers), "Range")) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.WriteHeader(http.StatusNoContent)
}

func serveCatalogHLSManifest(w http.ResponseWriter, req *http.Request, upstream *url.URL, res *http.Response, mediaType string,
	lease catalogMediaLease, token string, allowMaster bool) {
	switch mediaType {
	case catalogHLSType, "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl":
	case "application/octet-stream", "text/plain":
		if !strings.EqualFold(path.Ext(upstream.Path), ".m3u8") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
	default:
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !validCatalogResponseRange(res, "") || res.ContentLength > maxCatalogHLSManifest {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxCatalogHLSManifest+1))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	manifest, err := parseCatalogHLSManifest(body, upstream, allowMaster)
	if err != nil || lease.ctx.Err() != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	rewritten, ok := lease.hls.publish(manifest, catalogMediaPath+token+"/hls/")
	if !ok || lease.ctx.Err() != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", catalogHLSType)
	w.Header().Set("Content-Length", strconv.Itoa(len(rewritten)))
	w.WriteHeader(http.StatusOK)
	if req.Method == http.MethodGet {
		_, _ = io.WriteString(w, rewritten)
	}
}

func canonicalCatalogHLSPayloadType(mediaType string) string {
	switch mediaType {
	case "video/mp2t":
		return mediaType
	case "video/mp4", "audio/mp4":
		return mediaType
	case "application/mp4", "video/iso.segment":
		return "video/mp4"
	case "audio/iso.segment":
		return "audio/mp4"
	default:
		return ""
	}
}

// HLS byte ranges may start inside a transport packet or a box, so
// format signatures can only be validated at byte zero.
func validCatalogHLSPayload(prefix []byte, mediaType string, res *http.Response) bool {
	if len(prefix) == 0 {
		return false
	}
	partial := res.StatusCode == http.StatusPartialContent
	if partial {
		contentRange := strings.TrimPrefix(res.Header.Get("Content-Range"), "bytes ")
		first, _, _ := strings.Cut(contentRange, "-")
		start, _ := parseCatalogOffset(first)
		if start != 0 {
			return true
		}
	}
	if mediaType == "video/mp2t" {
		const packetSize = 188
		if !partial && len(prefix) < packetSize {
			return false
		}
		for offset := 0; offset < len(prefix); offset += packetSize {
			if prefix[offset] != 0x47 {
				return false
			}
		}
		return true
	}
	if len(prefix) < 8 {
		return partial
	}
	switch string(prefix[4:8]) {
	case "ftyp", "styp", "moov", "moof", "mdat", "sidx", "free", "skip", "emsg", "prft":
	default:
		return false
	}
	size := uint64(binary.BigEndian.Uint32(prefix[:4]))
	minSize := uint64(8)
	switch size {
	case 0:
		return true // This box extends to the end of its resource.
	case 1:
		if len(prefix) < 16 {
			return partial
		}
		size = binary.BigEndian.Uint64(prefix[8:16])
		minSize = 16
	}
	return size >= minSize && (partial || res.ContentLength < 0 || size <= uint64(res.ContentLength))
}
