package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jindi-drama/internal/provider"
)

const catalogMediaPath = "/catalog-media/"
const maxCatalogLeases = 64
const catalogReadIdleTimeout = 12 * time.Second

type catalogMediaLease struct {
	ctx     context.Context
	cancel  context.CancelFunc
	sources []provider.MediaSource
	hls     *catalogHLSResources
}

type catalogMediaRelay struct {
	mu              sync.Mutex
	leases          map[string]catalogMediaLease
	host            string
	server          *http.Server
	cancel          context.CancelFunc
	client          *http.Client
	covers          *catalogCoverService
	readIdleTimeout time.Duration
	closed          bool
}

type catalogIdleBody struct {
	io.ReadCloser
	cancel  context.CancelFunc
	timeout time.Duration
}

func (b catalogIdleBody) Read(out []byte) (int, error) {
	// Cancel only while reading upstream. Downstream writes keep the lease deadline.
	timer := time.AfterFunc(b.timeout, b.cancel)
	defer timer.Stop()
	return b.ReadCloser.Read(out)
}

func startCatalogMediaRelay(client *http.Client, covers *catalogCoverService) (*catalogMediaRelay, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &catalogMediaRelay{
		host: listener.Addr().String(), cancel: cancel, client: client, covers: covers,
		readIdleTimeout: catalogReadIdleTimeout, leases: make(map[string]catalogMediaLease),
	}
	r.server = &http.Server{
		Handler: r, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
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
	lease := catalogMediaLease{ctx: ctx, cancel: cancel, sources: ticket.Sources}
	for _, source := range ticket.Sources {
		if source.Type == catalogHLSType {
			lease.hls = &catalogHLSResources{index: make(map[hlsResource]int)}
			break
		}
	}
	r.leases[token] = lease
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
		if ticket.Sources[i].Type == catalogHLSType {
			ticket.Sources[i].Src += ".m3u8"
		}
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
	r.cancel()
	_ = r.server.Close()
	r.client.CloseIdleConnections()
}

func (r *catalogMediaRelay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if token, isCover := strings.CutPrefix(req.URL.Path, catalogCoverPath); isCover {
		r.serveCover(w, req, token)
		return
	}
	suffix, pathOK := strings.CutPrefix(req.URL.Path, catalogMediaPath)
	token, address, slash := strings.Cut(suffix, "/")
	if req.Host != r.host || req.URL.RawQuery != "" || req.URL.ForceQuery || req.URL.RawPath != "" || !pathOK || !slash {
		http.NotFound(w, req)
		return
	}
	r.mu.Lock()
	lease, found := r.leases[token]
	r.mu.Unlock()
	if !found || lease.ctx.Err() != nil {
		http.NotFound(w, req)
		return
	}
	var upstreamURL, expectedType string
	var hlsRequest, playlist, allowMaster bool
	if child, ok := strings.CutPrefix(address, "hls/"); ok && lease.hls != nil {
		resource, exists := lease.hls.resource(child)
		if !exists {
			http.NotFound(w, req)
			return
		}
		upstreamURL, hlsRequest, playlist = resource.url, true, resource.playlist
	} else {
		indexText, extension := strings.CutSuffix(address, ".m3u8")
		index, err := strconv.Atoi(indexText)
		if err != nil || index < 0 || index >= len(lease.sources) {
			http.NotFound(w, req)
			return
		}
		source := lease.sources[index]
		hlsRequest = source.Type == catalogHLSType
		if extension != hlsRequest || (hlsRequest && strconv.Itoa(index) != indexText) {
			http.NotFound(w, req)
			return
		}
		upstreamURL, expectedType = source.Src, source.Type
		playlist, allowMaster = hlsRequest, hlsRequest
	}
	if hlsRequest {
		allowedOrigin := catalogHLSCORS(w, req)
		if req.Method == http.MethodOptions && allowedOrigin {
			catalogHLSPreflight(w, req)
			return
		}
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
	method := req.Method
	if playlist {
		// A manifest is served only after validating and rewriting its full body.
		method, rangeHeader = http.MethodGet, ""
	}
	var wireIdle atomic.Bool
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			wireIdle.Store(enableCatalogWireIdle(info.Conn, r.readIdleTimeout))
		},
	})
	up, err := http.NewRequestWithContext(ctx, method, upstreamURL, nil)
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
	if !wireIdle.Load() {
		// A multiplexed connection can stay active while this stream stalls.
		res.Body = catalogIdleBody{ReadCloser: res.Body, cancel: cancel, timeout: r.readIdleTimeout}
	}
	defer res.Body.Close()
	mediaType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	encoding := res.Header.Get("Content-Encoding")
	if err != nil || (encoding != "" && encoding != "identity") || res.ContentLength == 0 || lease.ctx.Err() != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if playlist {
		serveCatalogHLSManifest(w, req, up.URL, res, mediaType, lease, token, allowMaster)
		return
	}
	if hlsRequest {
		mediaType = canonicalCatalogHLSPayloadType(mediaType)
	}
	if mediaType == "" || (!hlsRequest && mediaType != expectedType) || !validCatalogResponseRange(res, rangeHeader) {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	var body io.Reader = res.Body
	if hlsRequest && req.Method == http.MethodGet {
		const prefixLimit = 512
		count := int64(prefixLimit)
		if res.ContentLength >= 0 {
			count = min(count, res.ContentLength)
		}
		buffered := bufio.NewReaderSize(body, prefixLimit)
		prefix, readErr := buffered.Peek(int(count))
		if (readErr != nil && (readErr != io.EOF || res.ContentLength >= 0)) ||
			!validCatalogHLSPayload(prefix, mediaType, res) || lease.ctx.Err() != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		body = buffered
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
		if res.ContentLength >= 0 {
			body = io.LimitReader(body, res.ContentLength)
		}
		copied, err := io.Copy(w, body)
		if err != nil || (res.ContentLength >= 0 && copied != res.ContentLength) {
			// A normal final chunk would make truncated media look complete.
			panic(http.ErrAbortHandler)
		}
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

func (a *App) catalogRelay() (*catalogMediaRelay, error) {
	a.catalogMediaMu.Lock()
	defer a.catalogMediaMu.Unlock()
	if a.catalogMediaClosed {
		return nil, errCatalogPlaybackUnavailable
	}
	if a.catalogMedia == nil {
		relay, err := startCatalogMediaRelay(newCatalogMediaClient(), a.catalogCovers)
		if err != nil {
			return nil, errCatalogPlaybackUnavailable
		}
		a.catalogMedia = relay
	}
	return a.catalogMedia, nil
}

func (a *App) issueCatalogMedia(ticket provider.PlaybackTicket, expiry time.Time) (provider.PlaybackTicket, error) {
	relay, err := a.catalogRelay()
	if err != nil {
		return provider.PlaybackTicket{}, err
	}
	return relay.issue(ticket, expiry)
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
