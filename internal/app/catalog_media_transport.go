package app

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"
)

var errCatalogAddress = errors.New("媒体地址未通过验证")

func newCatalogMediaClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           catalogIdleDial(dialCatalogMedia),
			ForceAttemptHTTP2:     true,
			DisableCompression:    true,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: 12 * time.Second,
			IdleConnTimeout:       30 * time.Second,
			MaxIdleConns:          16,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// HTTP/1.1 can block inside a single body Read after consuming part of a
// chunked response. Renew the deadline at each underlying socket read.
type catalogIdleConn struct {
	net.Conn
	timeout atomic.Int64
}

func (c *catalogIdleConn) Read(out []byte) (int, error) {
	if timeout := time.Duration(c.timeout.Load()); timeout > 0 {
		if err := c.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(out)
}

func catalogIdleDial(dial func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &catalogIdleConn{Conn: conn}, nil
	}
}

func enableCatalogWireIdle(conn net.Conn, timeout time.Duration) bool {
	secured, ok := conn.(*tls.Conn)
	if !ok || secured.ConnectionState().NegotiatedProtocol == "h2" {
		return false
	}
	wire, ok := secured.NetConn().(*catalogIdleConn)
	if !ok {
		return false
	}
	// A pooled connection can already have a read pending before this request.
	if err := wire.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return false
	}
	wire.timeout.Store(int64(timeout))
	return true
}

var reservedCatalogNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}
var globalCatalogIPv6 = netip.MustParsePrefix("2000::/3")

func publicCatalogIP(address netip.Addr) bool {
	ip := address.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || (ip.Is6() && !globalCatalogIPv6.Contains(ip)) {
		return false
	}
	for _, reserved := range reservedCatalogNetworks {
		if reserved.Contains(ip) {
			return false
		}
	}
	return true
}

func dialCatalogMedia(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{}
	return dialCheckedMediaAddress(ctx, network, address, net.DefaultResolver.LookupNetIP, dialer.DialContext)
}

func dialCheckedMediaAddress(ctx context.Context, network, address string,
	lookup func(context.Context, string, string) ([]netip.Addr, error),
	connect func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" || !catalogMediaHostname(host) {
		return nil, errCatalogAddress
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ips, err := lookup(ctx, "ip", strings.TrimSuffix(host, "."))
	if err != nil || len(ips) == 0 {
		return nil, errCatalogAddress
	}
	for _, ip := range ips {
		if !publicCatalogIP(ip) {
			return nil, errCatalogAddress
		}
	}
	for _, ip := range ips {
		// Use the checked literal. TLS still uses the URL hostname through
		// net/http, without another DNS lookup before connecting.
		conn, err := connect(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, errCatalogAddress
}
