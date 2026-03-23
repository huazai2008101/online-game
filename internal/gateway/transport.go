package gateway

import (
	"net"
	"net/http"
	"time"
)

// NewTransport creates a new HTTP transport with custom settings
func NewTransport(keepAlive time.Duration, maxIdle int) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: keepAlive,
		}).DialContext,
		MaxIdleConns:          maxIdle,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}
