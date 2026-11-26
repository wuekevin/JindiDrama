package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jindi-drama/internal/provider"
)

type catalogRoundTripper func(*http.Request) (*http.Response, error)

func (f catalogRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func testCatalogRelay(t *testing.T, upstream catalogRoundTripper) *catalogMediaRelay {
	t.Helper()
	client := newCatalogMediaClient()
	client.Transport = upstream
	relay, err := startCatalogMediaRelay(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(relay.close)
	return relay
}

func issueTestStream(t *testing.T, relay *catalogMediaRelay, path string) provider.PlaybackTicket {
	t.Helper()
	expires := time.Now().Add(time.Minute)
	ticket, err := relay.issue(provider.PlaybackTicket{Kind: "authorized", EpisodeID: "qa", ExpiresAt: expires.Format(time.RFC3339), Sources: []provider.MediaSource{
		{Src: "https://public.example.test/" + path + "?signature=only-in-go", Type: "video/mp4"},
	}}, expires)
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}

func TestCatalogRelayStreamsRangeWithoutForwardingCredentials(t *testing.T) {
	var calls atomic.Int32
	relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		if req.URL.RawQuery != "signature=only-in-go" || req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" ||
			req.Header.Get("Referer") != "" || req.Header.Get("Origin") != "" {
			t.Errorf("unexpected upstream request: %s %v", req.URL.RawQuery, req.Header)
		}
		headers := http.Header{"Content-Type": {"video/mp4"}, "Set-Cookie": {"origin=private"}, "X-Internal": {"hidden"}}
		status, payload := http.StatusOK, "0123456789"
		if req.Header.Get("Range") == "bytes=2-5" {
			status, payload = http.StatusPartialContent, "2345"
			headers.Set("Content-Range", "bytes 2-5/10")
		}
		length := int64(len(payload))
		if req.Method == http.MethodHead {
			payload = ""
		}
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(payload)), ContentLength: length}, nil
	})
	ticket := issueTestStream(t, relay, "movie")
	get := func(method, byteRange string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, ticket.Sources[0].Src, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Range", byteRange)
		req.Header.Set("Cookie", "renderer=private")
		req.Header.Set("Authorization", "Bearer renderer")
		req.Header.Set("Origin", "https://renderer.example")
		req.Header.Set("Referer", "https://renderer.example/private")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = res.Body.Close() })
		return res
	}

	partial := get(http.MethodGet, "bytes=2-5")
	bytes, err := io.ReadAll(partial.Body)
	if err != nil || string(bytes) != "2345" || partial.StatusCode != http.StatusPartialContent ||
		partial.Header.Get("Content-Range") != "bytes 2-5/10" || partial.Header.Get("Set-Cookie") != "" ||
		partial.Header.Get("X-Internal") != "" || partial.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected response: status=%d body=%s headers=%v err=%v", partial.StatusCode, bytes, partial.Header, err)
	}
	head := get(http.MethodHead, "")
	if head.StatusCode != http.StatusOK || head.ContentLength != 10 {
		t.Fatalf("unexpected HEAD: %+v", head)
	}
	if res := get(http.MethodGet, "bytes=0-2,5-7"); res.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("multiple ranges: %d", res.StatusCode)
	}
	relay.release(ticket.LeaseID)
	if res := get(http.MethodGet, ""); res.StatusCode != http.StatusNotFound || calls.Load() != 2 {
		t.Fatalf("revoked token: %d; upstream calls=%d", res.StatusCode, calls.Load())
	}
}

func TestCatalogRelayRefusesRedirectsAndInvalidMediaResponses(t *testing.T) {
	cases := []struct {
		path               string
		status             int
		mime, contentRange string
		length             int64
	}{
		{"redirect", http.StatusFound, "video/mp4", "", 4},
		{"html", http.StatusOK, "text/html", "", 4},
		{"misaligned", http.StatusPartialContent, "video/mp4", "bytes 3-5/10", 3},
		{"unconstrained", http.StatusPartialContent, "video/mp4", "bytes 2-5/*", 4},
		{"unknown-length", http.StatusPartialContent, "video/mp4", "bytes 2-5/10", -1},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			var calls atomic.Int32
			relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
				calls.Add(1)
				h := http.Header{"Content-Type": {tc.mime}, "Location": {"http://127.0.0.1/secret"}}
				if tc.contentRange != "" {
					h.Set("Content-Range", tc.contentRange)
				}
				return &http.Response{StatusCode: tc.status, Header: h, Body: io.NopCloser(strings.NewReader("abcd")), ContentLength: tc.length}, nil
			})
			ticket := issueTestStream(t, relay, tc.path)
			req, err := http.NewRequest(http.MethodGet, ticket.Sources[0].Src, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Range", "bytes=2-5")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusBadGateway || res.Header.Get("Location") != "" || calls.Load() != 1 {
				t.Fatalf("rejected response leaked: status=%d header=%v upstream calls=%d", res.StatusCode, res.Header, calls.Load())
			}
		})
	}
}

func TestCatalogRelayRevokesAnInFlightUpstreamRequest(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
		close(started)
		<-req.Context().Done()
		close(canceled)
		return nil, req.Context().Err()
	})
	ticket := issueTestStream(t, relay, "never-responds")
	res := make(chan error, 1)
	go func() {
		response, err := (&http.Client{Timeout: 3 * time.Second}).Get(ticket.Sources[0].Src)
		if err == nil {
			defer response.Body.Close()
			if response.StatusCode != http.StatusBadGateway {
				err = fmt.Errorf("revoked upstream returned status %d", response.StatusCode)
			}
		} else if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			// Revocation can close the response before any headers arrive.
			err = nil
		}
		res <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request did not start")
	}
	relay.release(ticket.LeaseID)
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("release did not cancel upstream")
	}
	if err := <-res; err != nil {
		t.Fatal(err)
	}
}

func TestCatalogRelayForgetsExpiredTicketWithoutRendererRelease(t *testing.T) {
	relay := testCatalogRelay(t, func(*http.Request) (*http.Response, error) {
		t.Error("expired lease reached upstream")
		return nil, errCatalogAddress
	})
	deadline := time.Now().Add(60 * time.Millisecond)
	ticket, err := relay.issue(provider.PlaybackTicket{Sources: []provider.MediaSource{
		{Src: "https://media.example.test/one.mp4?signature=retained-in-go", Type: "video/mp4"},
	}}, deadline)
	if err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); {
		relay.mu.Lock()
		_, exists := relay.leases[ticket.LeaseID]
		relay.mu.Unlock()
		if !exists {
			res, err := http.Get(ticket.Sources[0].Src)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusNotFound {
				t.Fatalf("expired local source returned %d", res.StatusCode)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expired lease was not removed")
}

type blockedMediaBody struct {
	remaining int
	pumping   chan struct{}
	stopped   chan struct{}
}

func (b *blockedMediaBody) Read(out []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(out), b.remaining)
	b.remaining -= n
	clear(out[:n])
	if b.pumping != nil {
		close(b.pumping)
		b.pumping = nil
	}
	return n, nil
}

func (b *blockedMediaBody) Close() error {
	close(b.stopped)
	return nil
}

func TestCatalogRelayReleaseUnblocksAClientThatStoppedReading(t *testing.T) {
	pumping, stopped := make(chan struct{}), make(chan struct{})
	relay := testCatalogRelay(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK,
			Header: http.Header{"Content-Type": {"video/mp4"}},
			Body:   &blockedMediaBody{remaining: 64 << 20, pumping: pumping, stopped: stopped}, ContentLength: 64 << 20}, nil
	})
	ticket := issueTestStream(t, relay, "large")
	u, err := url.Parse(ticket.Sources[0].Src)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", u.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(4096)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\n\r\n", u.RequestURI(), u.Host); err != nil {
		t.Fatal(err)
	}
	if _, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodGet}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pumping:
	case <-time.After(2 * time.Second):
		t.Fatal("relay did not start writing the media body")
	}
	relay.release(ticket.LeaseID)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("the unread downstream stream did not close on release")
	}
}

func TestCatalogMediaRequiresPublicDialAddresses(t *testing.T) {
	for _, ip := range []string{"0.1.2.3", "127.0.0.1", "10.1.0.1", "169.254.169.254", "100.64.1.1", "192.0.2.1", "::1", "64:ff9b::a00:1", "2002:c0a8:1::", "3fff::1", "::ffff:192.168.0.1", "fc00::1"} {
		if publicCatalogIP(netip.MustParseAddr(ip)) {
			t.Errorf("%s passed public destination checks", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if !publicCatalogIP(netip.MustParseAddr(ip)) {
			t.Errorf("%s was rejected despite being public", ip)
		}
	}
}

func TestCatalogMediaDialsOnlyTheResolvedApprovedLiteral(t *testing.T) {
	var connected string
	resolve := func(_ context.Context, network, host string) ([]netip.Addr, error) {
		if network != "ip" || host != "media.example.test" {
			t.Fatalf("unexpected lookup: %s %s", network, host)
		}
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		connected = address
		a, b := net.Pipe()
		t.Cleanup(func() { _ = b.Close() })
		return a, nil
	}
	c, err := dialCheckedMediaAddress(t.Context(), "tcp", "media.example.test:443", resolve, dial)
	if err != nil || connected != "1.1.1.1:443" {
		t.Fatalf("unexpected dial: address=%s, err=%v", connected, err)
	}
	_ = c.Close()

	mixed := func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}, nil
	}
	connected = ""
	if _, err := dialCheckedMediaAddress(t.Context(), "tcp", "media.example.test:443", mixed, dial); !errors.Is(err, errCatalogAddress) || connected != "" {
		t.Fatalf("mixed public/private DNS was not rejected before dialing: address=%s, err=%v", connected, err)
	}
}
