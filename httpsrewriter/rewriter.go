// Package httpsrewriter rewrite the scheme and dest port of requests to HTTPS
// for the configured domains. It can also wrap a dialer to dial TLS. See
// https://github.com/getlantern/config-server/issues/4 for its usage.

package httpsrewriter

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/getlantern/golog"
	"github.com/getlantern/http-proxy-lantern/domains"
	"github.com/getlantern/proxy/filters"
)

var log = golog.LoggerFor("httpsRewritter")

type Dial func(ctx context.Context, network, address string) (net.Conn, error)

type Rewriter struct{}

func (f *Rewriter) Dialer(d Dial) Dial {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := d(ctx, network, address)
		if err != nil {
			return conn, err
		}
		if cfg := domains.ConfigForAddress(address); cfg.RewriteToHTTPS {
			conn = tls.Client(conn, &tls.Config{ServerName: cfg.Host})
			log.Debugf("Added TLS to connection to %s", address)
		}
		return conn, err
	}
}

func (f *Rewriter) Apply(ctx filters.Context, req *http.Request, next filters.Next) (*http.Response, filters.Context, error) {
	f.RewriteIfNecessary(req)
	return next(ctx, req)
}

func (f *Rewriter) RewriteIfNecessary(req *http.Request) {
	if req.Method == "CONNECT" {
		return
	}
	if cfg := domains.ConfigForRequest(req); cfg.RewriteToHTTPS {
		f.rewrite(cfg.Host, req)
	}
}

func (f *Rewriter) rewrite(host string, req *http.Request) {
	// for some reason we forgot, the scheme has to be cleared, rather than set to "https"
	req.URL.Scheme = ""
	req.Host = host + ":443"
	log.Debugf("Rewrote request to HTTPS: %v  with host %v", req.URL, req.Host)
}