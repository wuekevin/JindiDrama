package licensed

import (
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"time"
)

// StartLoopback must run before the provider is published to the WebView.
// WebView2's Wails asset writer buffers the full HTTP response in memory.
func (p *DemoProvider) StartLoopback() (func() error, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("无法启动本地片库媒体服务")
	}
	host := listener.Addr().String()
	accessPath := "/" + rand.Text()
	assets := http.StripPrefix(accessPath, p.Middleware(http.NotFoundHandler()))
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != host {
				http.NotFound(w, r)
				return
			}
			assets.ServeHTTP(w, r)
		}),
	}
	p.mediaBase = "http://" + host + accessPath
	go func() { _ = server.Serve(listener) }()
	return server.Close, nil
}
