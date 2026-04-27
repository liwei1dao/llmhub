// Package httpclient provides a shared HTTP client for upstream provider
// calls, with sensible defaults (keep-alive pool, timeouts, HTTP/2).
package httpclient

import (
	"net"
	"net/http"
	"time"
)

// New returns an http.Client tuned for upstream provider traffic.
func New() *http.Client {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: tr,
		// No global timeout: streaming responses are long-lived.
		// Per-request timeouts are applied via context.
	}
}
