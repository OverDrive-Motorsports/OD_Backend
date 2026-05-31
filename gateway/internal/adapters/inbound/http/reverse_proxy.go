/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## reverse_proxy.go - Implements upstream reverse proxying and path rewriting.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync/atomic"
)

type ReverseProxy struct {
	routePrefix string
	targets     []*url.URL
	counter     uint64
	proxy       *httputil.ReverseProxy
}

func NewReverseProxy(routePrefix string, targets []*url.URL) (*ReverseProxy, error) {
	rp := &ReverseProxy{routePrefix: normalizePrefix(routePrefix), targets: targets}
	rp.proxy = &httputil.ReverseProxy{
		Director:     rp.director,
		ErrorHandler: rp.handleProxyError,
	}

	return rp, nil
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}

func (p *ReverseProxy) director(req *http.Request) {
	target := p.nextTarget()
	originalHost := req.Host
	remoteIP := clientIP(req.RemoteAddr)

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host

	trimmedPath := strings.TrimPrefix(req.URL.Path, p.routePrefix)
	if trimmedPath == "" {
		trimmedPath = "/"
	}

	req.URL.Path = joinPaths(target.Path, trimmedPath)
	req.URL.RawPath = req.URL.EscapedPath()
	req.Header.Set("X-Forwarded-Host", originalHost)
	req.Header.Set("X-Forwarded-Proto", "http")
	if remoteIP != "" {
		existing := req.Header.Get("X-Forwarded-For")
		if existing == "" {
			req.Header.Set("X-Forwarded-For", remoteIP)
		} else {
			req.Header.Set("X-Forwarded-For", existing+", "+remoteIP)
		}
	}
}

func (p *ReverseProxy) nextTarget() *url.URL {
	index := atomic.AddUint64(&p.counter, 1)
	return p.targets[(index-1)%uint64(len(p.targets))]
}

func (p *ReverseProxy) handleProxyError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("proxy error for %s: %v", r.URL.Path, err)
	http.Error(w, "bad gateway", http.StatusBadGateway)
}

func normalizePrefix(prefix string) string {
	if prefix == "" {
		return "/"
	}

	if !strings.HasPrefix(prefix, "/") {
		return "/" + prefix
	}

	return strings.TrimSuffix(prefix, "/")
}

func joinPaths(basePath, extraPath string) string {
	if basePath == "" || basePath == "/" {
		if strings.HasPrefix(extraPath, "/") {
			return extraPath
		}
		return "/" + extraPath
	}

	joined := path.Join(basePath, extraPath)
	if !strings.HasPrefix(joined, "/") {
		return "/" + joined
	}

	return joined
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}
