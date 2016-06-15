package devicefilter

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gorilla/context"
	redislib "gopkg.in/redis.v3"

	"github.com/getlantern/golog"

	"github.com/getlantern/http-proxy/filters"
	"github.com/getlantern/http-proxy/listeners"

	"github.com/getlantern/http-proxy-lantern/blacklist"
	"github.com/getlantern/http-proxy-lantern/common"
	throttle "github.com/getlantern/http-proxy-lantern/listeners"
	"github.com/getlantern/http-proxy-lantern/redis"
	"github.com/getlantern/http-proxy-lantern/usage"
)

var (
	log = golog.LoggerFor("devicefilter")

	epoch = time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)
)

// deviceFilterPre does the device-based filtering
type deviceFilterPre struct {
	throttleAfterMiB uint64
	redisClient      *redislib.Client
}

// deviceFilterPost cleans up
type deviceFilterPost struct {
	bl *blacklist.Blacklist
}

func NewPre(rc *redislib.Client, throttleAfterMiB uint64) filters.Filter {
	if throttleAfterMiB > 0 {
		log.Debugf("Throttling clients after %v MiB", throttleAfterMiB)
	}

	return &deviceFilterPre{
		redisClient:      rc,
		throttleAfterMiB: throttleAfterMiB,
	}
}

func (f *deviceFilterPre) Apply(w http.ResponseWriter, req *http.Request, next filters.Next) error {
	if log.IsTraceEnabled() {
		reqStr, _ := httputil.DumpRequest(req, true)
		log.Tracef("DeviceFilter Middleware received request:\n%s", reqStr)
	}

	lanternDeviceID := req.Header.Get(common.DeviceIdHeader)

	if lanternDeviceID == "" {
		log.Debugf("No %s header found from %s for request to %v", common.DeviceIdHeader, req.RemoteAddr, req.Host)
	} else {
		// Attached the uid to connection to report stats to redis correctly
		// "conn" in context is previously attached in server.go
		wc := context.Get(req, "conn").(listeners.WrapConn)
		// Sets the ID to the provided key. This message is captured only
		// by the measured wrapper
		wc.ControlMessage("measured", lanternDeviceID)

		if f.throttleAfterMiB > 0 {
			// Throttling enabled
			u := usage.Get(lanternDeviceID)
			if u == nil {
				// Eagerly request device ID data to Redis and store it in usage
				go redis.ForceRetrieveDeviceUsage(f.redisClient, lanternDeviceID)
				return next()
			}
			uMiB := u.Bytes / 1024768
			// Encode usage information in a header. The header is expected to follow
			// this format:
			//
			// <used>/<allowed>/<asof>
			//
			// <used> is the string representation of a 64-bit unsigned integer
			// <allowed> is the string representation of a 64-bit unsigned integer
			// <asof> is the 64-bit signed integer representing seconds since a custom
			// epoch (00:00:00 01/01/2016 UTC).
			w.Header().Set("XBQ", fmt.Sprintf("%d/%d/%d", uMiB, f.throttleAfterMiB, int64(u.AsOf.Sub(epoch).Seconds())))
			if uMiB > f.throttleAfterMiB {
				// Start limiting the throughput on this connection
				wc.ControlMessage("throttle", throttle.On)
			}
		}
	}

	return next()
}

func NewPost(bl *blacklist.Blacklist) filters.Filter {
	return &deviceFilterPost{
		bl: bl,
	}
}

func (f *deviceFilterPost) Apply(w http.ResponseWriter, req *http.Request, next filters.Next) error {
	// For privacy, delete the DeviceId header before passing it along
	req.Header.Del(common.DeviceIdHeader)
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	f.bl.Succeed(ip)
	return next()
}
