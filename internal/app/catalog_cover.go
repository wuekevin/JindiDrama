package app

import (
	"bytes"
	"container/list"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"jindi-drama/internal/provider"

	_ "golang.org/x/image/webp"
)

const (
	catalogCoverPath        = "/catalog-cover/"
	catalogCoverTokenBytes  = 16
	catalogCoverLifetime    = 10 * time.Minute
	catalogCoverCapacity    = 2048
	catalogCoverTimeout     = 8 * time.Second
	catalogCoverConcurrent  = 4
	catalogCoverOutstanding = 32
	maxCatalogCoverBytes    = 2 << 20
	maxCachedCoverBytes     = 8 << 20
	maxCatalogCoverSide     = 4096
	maxCatalogCoverPixels   = 4_000_000
)

type catalogCoverTicket struct {
	token   string
	address string
	expires time.Time
	data    []byte
	freshTo time.Time
	fetchID uint64
}

type catalogCoverService struct {
	client      *http.Client
	active      chan struct{}
	outstanding chan struct{}

	mu          sync.Mutex
	closed      bool
	issued      list.List
	byToken     map[string]*list.Element
	byAddress   map[string]*list.Element
	cachedBytes int
}

func newCatalogCoverService(client *http.Client) *catalogCoverService {
	return &catalogCoverService{
		client:      client,
		active:      make(chan struct{}, catalogCoverConcurrent),
		outstanding: make(chan struct{}, catalogCoverOutstanding),
		byToken:     make(map[string]*list.Element),
		byAddress:   make(map[string]*list.Element),
	}
}

func (a *App) projectCatalogCovers(rows []provider.Drama) {
	if a.catalogCovers != nil {
		for i := range rows {
			a.projectCatalogCover(&rows[i])
		}
	}
}

func (a *App) projectCatalogCover(drama *provider.Drama) {
	cover := a.catalogCovers
	if cover == nil || drama.CoverURL == "" {
		return
	}
	address := drama.CoverURL
	drama.CoverURL = ""
	if len(address) > 4096 {
		return
	}
	if _, ok := catalogHTTPSURL(address); !ok {
		return
	}
	relay, err := a.catalogRelay()
	if err != nil {
		return
	}
	if token := cover.issue(address); token != "" {
		drama.Cover = "http://" + relay.host + catalogCoverPath + token
	}
}

func (s *catalogCoverService) issue(address string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ""
	}
	now := time.Now()
	s.expire(now)
	if element := s.byAddress[address]; element != nil {
		return element.Value.(catalogCoverTicket).token
	}

	var random [catalogCoverTokenBytes]byte
	rand.Read(random[:])
	token := hex.EncodeToString(random[:])
	if s.byToken[token] != nil {
		return ""
	}
	if s.issued.Len() == catalogCoverCapacity {
		s.remove(s.issued.Front())
	}
	element := s.issued.PushBack(catalogCoverTicket{token: token, address: address, expires: now.Add(catalogCoverLifetime)})
	s.byToken[token] = element
	s.byAddress[address] = element
	return token
}

func (s *catalogCoverService) lookup(token string) catalogCoverTicket {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.expire(now)
	if element := s.byToken[token]; element != nil {
		ticket := element.Value.(catalogCoverTicket)
		if len(ticket.data) != 0 && !now.Before(ticket.freshTo) {
			s.dropCover(element)
			ticket.data = nil
		}
		return ticket
	}
	return catalogCoverTicket{}
}

func (s *catalogCoverService) beginFetch(token string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(time.Now())
	element := s.byToken[token]
	if element == nil {
		return 0
	}
	ticket := element.Value.(catalogCoverTicket)
	ticket.fetchID++
	element.Value = ticket
	return ticket.fetchID
}

func (s *catalogCoverService) retain(token string, fetchID uint64, data []byte, freshTo time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.expire(now)
	element := s.byToken[token]
	if element == nil || len(data) == 0 {
		return
	}
	ticket := element.Value.(catalogCoverTicket)
	if fetchID != ticket.fetchID {
		return
	}
	s.dropCover(element)
	if !now.Before(freshTo) {
		return
	}
	if s.cachedBytes+cap(data) > maxCachedCoverBytes {
		for candidate := s.issued.Front(); candidate != nil; candidate = candidate.Next() {
			if cached := candidate.Value.(catalogCoverTicket); len(cached.data) != 0 && !now.Before(cached.freshTo) {
				s.dropCover(candidate)
			}
		}
	}
	if s.cachedBytes+cap(data) > maxCachedCoverBytes {
		return
	}
	s.cachedBytes += cap(data)
	ticket.data, ticket.freshTo = data, freshTo
	element.Value = ticket
}

func (s *catalogCoverService) dropCover(element *list.Element) {
	ticket := element.Value.(catalogCoverTicket)
	s.cachedBytes -= cap(ticket.data)
	ticket.data = nil
	element.Value = ticket
}

// Tickets expire in issuance order. Reusing a ticket keeps its original expiry.
func (s *catalogCoverService) expire(now time.Time) {
	for front := s.issued.Front(); front != nil && !now.Before(front.Value.(catalogCoverTicket).expires); front = s.issued.Front() {
		s.remove(front)
	}
}

func (s *catalogCoverService) remove(element *list.Element) {
	ticket := s.issued.Remove(element).(catalogCoverTicket)
	s.cachedBytes -= cap(ticket.data)
	delete(s.byToken, ticket.token)
	delete(s.byAddress, ticket.address)
}

func (s *catalogCoverService) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.issued.Init()
	s.cachedBytes = 0
	clear(s.byToken)
	clear(s.byAddress)
	s.client.CloseIdleConnections()
}

func (r *catalogMediaRelay) serveCover(w http.ResponseWriter, req *http.Request, token string) {
	if r.covers == nil || req.Host != r.host || req.Method != http.MethodGet ||
		req.URL.Scheme != "" || req.URL.Host != "" || req.URL.User != nil || req.URL.RawPath != "" ||
		req.URL.RawQuery != "" || req.URL.ForceQuery || req.URL.Fragment != "" || req.URL.RawFragment != "" ||
		req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" || !validCatalogCoverToken(token) {
		http.NotFound(w, req)
		return
	}
	r.covers.serve(w, req, token)
}

func validCatalogCoverToken(token string) bool {
	return len(token) == hex.EncodedLen(catalogCoverTokenBytes) &&
		strings.IndexFunc(token, func(r rune) bool {
			return (r < '0' || r > '9') && (r < 'a' || r > 'f')
		}) < 0
}

func (s *catalogCoverService) serve(w http.ResponseWriter, r *http.Request, token string) {
	if s.lookup(token).address == "" {
		http.NotFound(w, r)
		return
	}
	select {
	case s.outstanding <- struct{}{}:
		defer func() { <-s.outstanding }()
	default:
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), catalogCoverTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	_ = http.NewResponseController(w).SetWriteDeadline(deadline)
	select {
	case s.active <- struct{}{}:
		defer func() { <-s.active }()
	case <-ctx.Done():
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// A ticket may expire or be removed while waiting for a fetch slot.
	ticket := s.lookup(token)
	if ticket.address == "" {
		http.NotFound(w, r)
		return
	}
	if ctx.Err() != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	data := ticket.data
	cacheControl := r.Header.Values("Cache-Control")
	pragma := r.Header.Values("Pragma")
	bypass := len(cacheControl) != 0 || len(pragma) != 0
	if len(data) == 0 || bypass {
		fetchID := s.beginFetch(token)
		if fetchID == 0 {
			http.NotFound(w, r)
			return
		}
		var freshTo time.Time
		data, freshTo = s.fetch(ctx, ticket.address)
		canRefresh := len(cacheControl) == 1 && len(pragma) == 0 &&
			(strings.EqualFold(strings.TrimSpace(cacheControl[0]), "no-cache") || strings.TrimSpace(cacheControl[0]) == "max-age=0")
		if bypass && !canRefresh {
			freshTo = time.Time{}
		}
		if ctx.Err() == nil {
			s.retain(token, fetchID, data, freshTo)
		}
	}
	if len(data) == 0 || ctx.Err() != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = w.Write(data)
}

func (s *catalogCoverService) fetch(ctx context.Context, address string) ([]byte, time.Time) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, time.Time{}
	}
	req.Header.Set("Accept", "image/jpeg, image/png, image/webp")
	requestedAt := time.Now()
	res, err := s.client.Do(req)
	if err != nil {
		return nil, time.Time{}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.ContentLength > maxCatalogCoverBytes {
		return nil, time.Time{}
	}
	contentType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil {
		return nil, time.Time{}
	}
	var expectedFormat string
	switch contentType {
	case "image/jpeg":
		expectedFormat = "jpeg"
	case "image/png":
		expectedFormat = "png"
	case "image/webp":
		expectedFormat = "webp"
	default:
		return nil, time.Time{}
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, maxCatalogCoverBytes+1))
	if err != nil || len(data) > maxCatalogCoverBytes || http.DetectContentType(data) != contentType {
		return nil, time.Time{}
	}
	// RIFF lengths count every byte after the first two header fields.
	if expectedFormat == "webp" && (len(data) < 12 || binary.LittleEndian.Uint32(data[4:8]) != uint32(len(data)-8)) {
		return nil, time.Time{}
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != expectedFormat || !validCatalogCoverSize(config.Width, config.Height) || ctx.Err() != nil {
		return nil, time.Time{}
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != expectedFormat || decoded.Bounds().Size() != image.Pt(config.Width, config.Height) || ctx.Err() != nil {
		return nil, time.Time{}
	}
	var output catalogCoverJPEG
	if err := jpeg.Encode(&output, decoded, &jpeg.Options{Quality: 82}); err != nil {
		return nil, time.Time{}
	}
	jpegData := output.buffer.Bytes()
	if req.URL.RawQuery == "" && !req.URL.ForceQuery && ctx.Err() == nil {
		return jpegData, catalogCoverFreshUntil(res.Header, requestedAt)
	}
	return jpegData, time.Time{}
}

func validCatalogCoverSize(width, height int) bool {
	return width > 0 && height > 0 && width <= maxCatalogCoverSide && height <= maxCatalogCoverSide &&
		width*height <= maxCatalogCoverPixels
}

type catalogCoverJPEG struct {
	buffer bytes.Buffer
}

func (w *catalogCoverJPEG) Write(data []byte) (int, error) {
	if len(data) > maxCatalogCoverBytes-w.buffer.Len() {
		return 0, errors.New("catalog cover JPEG exceeds size limit")
	}
	return w.buffer.Write(data)
}
