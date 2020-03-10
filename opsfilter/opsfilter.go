package opsfilter

import (
	"net"
	"net/http"
	"strings"

	"github.com/getlantern/golog"
	"github.com/getlantern/ops"
	"github.com/getlantern/proxy"
	"github.com/getlantern/proxy/filters"

	"github.com/getlantern/http-proxy-lantern/bbr"
	"github.com/getlantern/http-proxy-lantern/common"
	"github.com/getlantern/http-proxy/listeners"
)

var (
	log = golog.LoggerFor("logging")
)

type opsfilter struct {
	bm bbr.Middleware
}

// New constructs a new filter that adds ops context.
func New(bm bbr.Middleware) filters.Filter {
	return &opsfilter{bm}
}

func (f *opsfilter) Apply(ctx filters.Context, req *http.Request, next filters.Next) (*http.Response, filters.Context, error) {
	deviceID := req.Header.Get(common.DeviceIdHeader)
	originHost, originPort, _ := net.SplitHostPort(req.Host)
	if (originPort == "0" || originPort == "") && req.Method != http.MethodConnect {
		// Default port for HTTP
		originPort = "80"
	}
	if originHost == "" && !strings.Contains(req.Host, ":") {
		originHost = req.Host
	}
	platform := req.Header.Get(common.PlatformHeader)
	version := req.Header.Get(common.VersionHeader)

	op := ops.Begin("proxy").
		Set("device_id", deviceID).
		Set("origin", req.Host).
		Set("origin_host", originHost).
		Set("origin_port", originPort).
		Set("proxy_dial_timeout", req.Header.Get(proxy.DialTimeoutHeader)).
		Set("app_platform", platform).
		Set("app_version", version)
	log.Tracef("Starting op")
	defer op.End()

	measuredCtx := map[string]interface{}{
		"origin":      req.Host,
		"origin_host": originHost,
		"origin_port": originPort,
	}

	addMeasuredHeader := func(key, headerValue string) {
		if headerValue != "" {
			measuredCtx[key] = headerValue
		}
	}

	// On persistent HTTP connections, some or all of the below may be missing on requests after the first. By only setting
	// the values when they're available, the measured listener will preserve any values that were already included in the
	// first request on the connection.
	addMeasuredHeader("deviceid", deviceID)
	addMeasuredHeader("app_version", version)
	addMeasuredHeader("app_platform", platform)

	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		op.Set("client_ip", clientIP)
		measuredCtx["client_ip"] = clientIP
	}

	// Send the same context data to measured as well
	wc := ctx.DownstreamConn().(listeners.WrapConn)
	wc.ControlMessage("measured", measuredCtx)

	resp, nextCtx, nextErr := next(ctx, req)
	op.FailIf(nextErr)

	return resp, nextCtx, nextErr
}
