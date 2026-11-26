package app

import (
	"context"
	"crypto/rand"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"jindi-drama/internal/provider"
)

const catalogMediaPath = "/catalog-media/"
const maxCatalogLeases = 64

type catalogMediaLease struct {
	ctx     context.Context
	cancel  context.CancelFunc
	sources []provider.MediaSource
}

type catalogMediaRelay struct {
	mu     sync.Mutex
	leases map[string]catalogMediaLease
	host   string
	server *http.Server
	client *http.Client
	closed bool
}

func startCatalogMediaRelay(client *http.Client) (*catalogMediaRelay, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	r := &catalogMediaRelay{host: listener.Addr().String(), client: client, leases: make(map[string]catalogMediaLease)}
	r.server = &http.Server{Handler: r, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = r.server.Serve(listener) }()
	return r, nil
}

func (r *catalogMediaRelay) issue(ticket provider.PlaybackTicket, expiry time.Time) (provider.PlaybackTicket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for token, lease := range r.leases {
		if lease.ctx.Err() != nil {
			lease.cancel()
			delete(r.leases, token)
		}
	}
	if r.closed || len(r.leases) >= maxCatalogLeases || !expiry.After(time.Now()) {
		return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
	}
	token := rand.Text()
	for _, exists := r.leases[token]; exists; _, exists = r.leases[token] {
		token = rand.Text()
	}
	ctx, cancel := context.WithDeadline(context.Background(), expiry)
	r.leases[token] = catalogMediaLease{ctx: ctx, cancel: cancel, sources: ticket.Sources}
	context.AfterFunc(ctx, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if lease, ok := r.leases[token]; ok && lease.ctx == ctx {
			delete(r.leases, token)
		}
	})

	ticket.LeaseID = token
	ticket.Sources = append([]provider.MediaSource(nil), ticket.Sources...)
	for i := range ticket.Sources {
		ticket.Sources[i].Src = "http://" + r.host + catalogMediaPath + token + "/" + strconv.Itoa(i)
	}
	return ticket, nil
}

func (r *catalogMediaRelay) release(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lease, ok := r.leases[token]; ok {
		delete(r.leases, token)
		lease.cancel()
	}
}

func (r *catalogMediaRelay) close() {
	r.mu.Lock()
	r.closed = true
	for token, lease := range r.leases {
		lease.cancel()
		delete(r.leases, token)
	}
	r.mu.Unlock()
	_ = r.server.Close()
	r.client.CloseIdleConnections()
}

func (r *catalogMediaRelay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	suffix, pathOK := strings.CutPrefix(req.URL.Path, catalogMediaPath)
	token, indexText, slash := strings.Cut(suffix, "/")
	index, indexErr := strconv.Atoi(indexText)
	if req.Host != r.host || req.URL.RawQuery != "" || !pathOK || !slash || indexErr != nil || index < 0 {
		http.NotFound(w, req)
		return
	}
	r.mu.Lock()
	lease, found := r.leases[token]
	r.mu.Unlock()
	if !found || lease.ctx.Err() != nil || index >= len(lease.sources) {
		http.NotFound(w, req)
		return
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rangeHeader := req.Header.Get("Range")
	if len(req.Header.Values("Range")) > 1 || !validCatalogRange(rangeHeader) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	source := lease.sources[index]
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	controller := http.NewResponseController(w)
	deadline, _ := lease.ctx.Deadline()
	if err := controller.SetWriteDeadline(deadline); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	writesStopped := make(chan struct{})
	stop := context.AfterFunc(lease.ctx, func() {
		cancel()
		_ = controller.SetWriteDeadline(time.Now())
		close(writesStopped)
	})
	defer func() {
		if !stop() {
			<-writesStopped
		}
		// Keep the deadline through net/http's final response flush.
	}()
	up, err := http.NewRequestWithContext(ctx, req.Method, source.Src, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if rangeHeader != "" {
		up.Header.Set("Range", rangeHeader)
	}
	res, err := r.client.Do(up)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	mediaType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	encoding := res.Header.Get("Content-Encoding")
	if err != nil || mediaType != source.Type || (encoding != "" && encoding != "identity") ||
		res.ContentLength == 0 || !validCatalogResponseRange(res, rangeHeader) || lease.ctx.Err() != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", mediaType)
	if res.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(res.ContentLength, 10))
	}
	if res.Header.Get("Accept-Ranges") == "bytes" || res.StatusCode == http.StatusPartialContent {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	if res.StatusCode == http.StatusPartialContent {
		w.Header().Set("Content-Range", res.Header.Get("Content-Range"))
	}
	w.WriteHeader(res.StatusCode)
	if req.Method == http.MethodGet {
		// Keep memory bounded and make the first bytes available to the decoder.
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		var body io.Reader = res.Body
		if res.ContentLength >= 0 {
			body = io.LimitReader(body, res.ContentLength)
		}
		_, _ = io.Copy(w, body)
	}
}

func parseCatalogOffset(text string) (int64, bool) {
	if text == "" || strings.IndexFunc(text, func(c rune) bool { return c < '0' || c > '9' }) >= 0 {
		return 0, false
	}
	number, err := strconv.ParseInt(text, 10, 64)
	return number, err == nil
}

func validCatalogRange(header string) bool {
	if header == "" {
		return true
	}
	value, prefix := strings.CutPrefix(header, "bytes=")
	first, last, cut := strings.Cut(value, "-")
	if !prefix || !cut || (first == "" && last == "") {
		return false
	}
	start, validStart := parseCatalogOffset(first)
	end, validEnd := parseCatalogOffset(last)
	switch {
	case first == "":
		return validEnd && end > 0
	case last == "":
		return validStart
	default:
		return validStart && validEnd && start <= end
	}
}

func validCatalogResponseRange(res *http.Response, requestRange string) bool {
	if res.StatusCode == http.StatusOK {
		return res.Header.Get("Content-Range") == ""
	}
	if res.StatusCode != http.StatusPartialContent || requestRange == "" {
		return false
	}
	contentRange, prefix := strings.CutPrefix(res.Header.Get("Content-Range"), "bytes ")
	bounds, totalText, slash := strings.Cut(contentRange, "/")
	first, last, dash := strings.Cut(bounds, "-")
	start, validStart := parseCatalogOffset(first)
	end, validEnd := parseCatalogOffset(last)
	total, validTotal := parseCatalogOffset(totalText)
	if !prefix || !slash || !dash || !validStart || !validEnd || !validTotal || start > end || end >= total ||
		res.ContentLength != end-start+1 {
		return false
	}

	first, last, _ = strings.Cut(strings.TrimPrefix(requestRange, "bytes="), "-")
	requestedStart, _ := parseCatalogOffset(first)
	requestedEnd, _ := parseCatalogOffset(last)
	if first == "" {
		return start == max(0, total-requestedEnd) && end == total-1
	}
	if last == "" {
		return start == requestedStart && end == total-1
	}
	return start == requestedStart && end == min(total-1, requestedEnd)
}

func (a *App) issueCatalogMedia(ticket provider.PlaybackTicket, expiry time.Time) (provider.PlaybackTicket, error) {
	a.catalogMediaMu.Lock()
	defer a.catalogMediaMu.Unlock()
	if a.catalogMediaClosed {
		return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
	}
	if a.catalogMedia == nil {
		relay, err := startCatalogMediaRelay(newCatalogMediaClient())
		if err != nil {
			return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
		}
		a.catalogMedia = relay
	}
	return a.catalogMedia.issue(ticket, expiry)
}

func (a *App) ReleaseCatalogPlayback(leaseID string) {
	a.catalogMediaMu.Lock()
	defer a.catalogMediaMu.Unlock()
	if a.catalogMedia != nil {
		a.catalogMedia.release(leaseID)
	}
}

func (a *App) closeCatalogMedia() {
	a.catalogMediaMu.Lock()
	defer a.catalogMediaMu.Unlock()
	a.catalogMediaClosed = true
	if a.catalogMedia != nil {
		a.catalogMedia.close()
		a.catalogMedia = nil
	}
}
