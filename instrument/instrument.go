package instrument

import (
	"context"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/getlantern/errors"
	"github.com/getlantern/geo"
	"github.com/getlantern/multipath"
	"github.com/getlantern/proxy/v2/filters"
)

const (
	otelReportingInterval = 60 * time.Minute
)

var (
	originRootRegex = regexp.MustCompile(`([^\.]+\.[^\.]+$)`)
)

// Instrument is the common interface about what can be instrumented.
type Instrument interface {
	WrapFilter(prefix string, f filters.Filter) filters.Filter
	WrapConnErrorHandler(prefix string, f func(conn net.Conn, err error)) func(conn net.Conn, err error)
	Blacklist(b bool)
	Mimic(m bool)
	MultipathStats([]string) []multipath.StatsTracker
	Throttle(m bool, reason string)
	XBQHeaderSent()
	VersionCheck(redirect bool, method, reason string)
	ProxiedBytes(sent, recv int, platform, version, app, dataCapCohort string, clientIP net.IP, deviceID, originHost string)
	ReportToOTELPeriodically(interval time.Duration, tp *sdktrace.TracerProvider, includeDeviceID bool)
	ReportToOTEL(tp *sdktrace.TracerProvider, includeDeviceID bool)
	quicSentPacket()
	quicLostPacket()
	TLSMalformedHello()
	TLSNoSupportSessionTickets()
	TLSMissingSessionTicket()
	TLSInvalidSessionTicket()
}

// NoInstrument is an implementation of Instrument which does nothing
type NoInstrument struct {
}

func (i NoInstrument) WrapFilter(prefix string, f filters.Filter) filters.Filter { return f }
func (i NoInstrument) WrapConnErrorHandler(prefix string, f func(conn net.Conn, err error)) func(conn net.Conn, err error) {
	return f
}
func (i NoInstrument) Blacklist(b bool) {}
func (i NoInstrument) Mimic(m bool)     {}
func (i NoInstrument) MultipathStats(protocols []string) (trackers []multipath.StatsTracker) {
	for _, _ = range protocols {
		trackers = append(trackers, multipath.NullTracker{})
	}
	return
}
func (i NoInstrument) Throttle(m bool, reason string) {}

func (i NoInstrument) XBQHeaderSent()                                    {}
func (i NoInstrument) SuspectedProbing(fromIP net.IP, reason string)     {}
func (i NoInstrument) VersionCheck(redirect bool, method, reason string) {}
func (i NoInstrument) ProxiedBytes(sent, recv int, platform, version, app, dataCapCohort string, clientIP net.IP, deviceID, originHost string) {
}
func (i NoInstrument) quicSentPacket() {}
func (i NoInstrument) quicLostPacket() {}

func (i NoInstrument) TLSMalformedHello()          {}
func (i NoInstrument) TLSNoSupportSessionTickets() {}
func (i NoInstrument) TLSMissingSessionTicket()    {}
func (i NoInstrument) TLSInvalidSessionTicket()    {}

func (i NoInstrument) ReportToOTELPeriodically(interval time.Duration, tp *sdktrace.TracerProvider, includeDeviceID bool) {
}
func (i NoInstrument) ReportToOTEL(tp *sdktrace.TracerProvider, includeDeviceID bool) {}

// CommonLabels defines a set of common labels apply to all metrics instrumented.
type CommonLabels struct {
	Protocol              string
	BuildRevision         string
	BuildType             string
	BuildGoVersion        string
	SupportTLSResumption  bool
	RequireTLSResumption  bool
	MissingTicketReaction string
}

// PromLabels turns the common labels to Prometheus form.
func (c *CommonLabels) PromLabels() prometheus.Labels {
	return map[string]string{
		"protocol":                c.Protocol,
		"build_type":              c.BuildType,
		"build_version":           c.BuildGoVersion,
		"build_revision":          c.BuildRevision,
		"support_tls_resumption":  strconv.FormatBool(c.SupportTLSResumption),
		"require_tls_resumption":  strconv.FormatBool(c.RequireTLSResumption),
		"missing_ticket_reaction": c.MissingTicketReaction,
	}
}

type instrumentedFilter struct {
	requests prometheus.Counter
	errors   prometheus.Counter
	duration prometheus.Observer
	filters.Filter
}

func (f *instrumentedFilter) Apply(cs *filters.ConnectionState, req *http.Request, next filters.Next) (*http.Response, *filters.ConnectionState, error) {
	start := time.Now()
	res, cs, err := f.Filter.Apply(cs, req, next)
	f.requests.Inc()
	if err != nil {
		f.errors.Inc()
	}
	f.duration.Observe(time.Since(start).Seconds())
	return res, cs, err
}

// PromInstrument is an implementation of Instrument which exports Prometheus
// metrics.
type PromInstrument struct {
	registry      *prometheus.Registry
	countryLookup geo.CountryLookup
	ispLookup     geo.ISPLookup
	statsMx       sync.Mutex

	filters                 map[string]*instrumentedFilter
	errorHandlers           map[string]func(conn net.Conn, err error)
	clientStats             map[clientDetails]*usage
	clientStatsWithDeviceID map[clientDetails]*usage
	originStats             map[originDetails]*usage

	activeClients1m, activeClients10m, activeClients1h *slidingWindowDistinctCount

	bytesSent, bytesRecv prometheus.Counter

	blacklisted, blacklistChecked, mimicked, mimicryChecked prometheus.Counter

	quicLostPackets, quicSentPackets prometheus.Counter

	tcpConsecRetransmissions, tcpSentDataPackets, xbqSent prometheus.Counter
	tcpRetransmissionRate                                 prometheus.Observer

	tlsMalformedHello, tlsNoSupportSessionTickets, tlsMissingSessionTicket, tlsInvalidSessionTicket prometheus.Counter

	throttlingChecked                     prometheus.Counter
	throttled, notThrottled, versionCheck *prometheus.CounterVec

	mpFramesSent, mpBytesSent, mpFramesReceived, mpBytesReceived, mpFramesRetransmitted, mpBytesRetransmitted *prometheus.CounterVec
}

func NewPrometheus(countryLookup geo.CountryLookup, ispLookup geo.ISPLookup, c CommonLabels) *PromInstrument {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	factory := promauto.With(reg)
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "proxy_info",
		ConstLabels: c.PromLabels(),
	}, func() float64 { return 1.0 })

	p := &PromInstrument{
		registry:                reg,
		countryLookup:           countryLookup,
		ispLookup:               ispLookup,
		filters:                 make(map[string]*instrumentedFilter),
		errorHandlers:           make(map[string]func(conn net.Conn, err error)),
		clientStats:             make(map[clientDetails]*usage),
		clientStatsWithDeviceID: make(map[clientDetails]*usage),
		originStats:             make(map[originDetails]*usage),

		activeClients1m: newSlidingWindowDistinctCount(prometheus.Opts{
			Name: "proxy_active_clients_1m",
		}, time.Minute, time.Second),
		activeClients10m: newSlidingWindowDistinctCount(prometheus.Opts{
			Name: "proxy_active_clients_10m",
		}, 10*time.Minute, 10*time.Second),
		activeClients1h: newSlidingWindowDistinctCount(prometheus.Opts{
			Name: "proxy_active_clients_1h",
		}, time.Hour, time.Minute),

		blacklistChecked: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_blacklist_checked_requests_total",
		}),
		blacklisted: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_blacklist_blacklisted_requests_total",
		}),
		bytesSent: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_downstream_sent_bytes_total",
			Help: "Bytes sent to the client connections. Pluggable transport overhead excluded",
		}),
		bytesRecv: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_downstream_received_bytes_total",
			Help: "Bytes received from the client connections. Pluggable transport overhead excluded",
		}),
		quicLostPackets: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_downstream_quic_lost_packets_total",
			Help: "Number of QUIC packets lost and effectively resent to the client connections.",
		}),
		quicSentPackets: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_downstream_quic_sent_packets_total",
			Help: "Number of QUIC packets sent to the client connections.",
		}),

		mimicryChecked: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_apache_mimicry_checked_total",
		}),
		mimicked: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_apache_mimicry_mimicked_total",
		}),
		mpFramesSent: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_multipath_sent_frames_total",
		}, []string{"path_protocol"}),
		mpBytesSent: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_multipath_sent_bytes_total",
		}, []string{"path_protocol"}),
		mpFramesReceived: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_multipath_received_frames_total",
		}, []string{"path_protocol"}),
		mpBytesReceived: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_multipath_received_bytes_total",
		}, []string{"path_protocol"}),
		mpFramesRetransmitted: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_multipath_retransmissions_total",
		}, []string{"path_protocol"}),
		mpBytesRetransmitted: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_multipath_retransmission_bytes_total",
		}, []string{"path_protocol"}),

		tcpConsecRetransmissions: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_downstream_tcp_consec_retransmissions_before_terminates_total",
			Help: "Number of TCP retransmissions happen before the connection gets terminated, as a measure of blocking in the form of continuously dropped packets.",
		}),
		tcpRetransmissionRate: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "proxy_tcp_retransmission_rate",
			Buckets: []float64{0.01, 0.1, 0.5},
		}),
		tcpSentDataPackets: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_downstream_tcp_sent_data_packets_total",
			Help: "Number of TCP data packets (packets with non-zero data length) sent to the client connections.",
		}),

		xbqSent: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_xbq_header_sent_total",
		}),

		throttlingChecked: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_device_throttling_checked_total",
		}),
		throttled: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_device_throttling_throttled_total",
		}, []string{"reason"}),
		notThrottled: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_device_throttling_not_throttled_total",
		}, []string{"reason"}),

		tlsMalformedHello: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_tls_malformed_hello_total",
		}),
		tlsNoSupportSessionTickets: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_tls_no_support_for_session_tickets_total",
		}),
		tlsMissingSessionTicket: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_tls_missing_session_ticket_total",
		}),
		tlsInvalidSessionTicket: factory.NewCounter(prometheus.CounterOpts{
			Name: "proxy_tls_invalid_session_ticket_total",
		}),

		versionCheck: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_version_check_total",
		}, []string{"method", "redirected", "reason"}),
	}

	reg.MustRegister(p.activeClients1m)
	reg.MustRegister(p.activeClients10m)
	reg.MustRegister(p.activeClients1h)
	return p
}

// Run runs the PromInstrument exporter on the given address. The
// path is /metrics.
func (p *PromInstrument) Run(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{}))
	server := http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return server.ListenAndServe()
}

// WrapFilter wraps a filter to instrument the requests/errors/duration
// (so-called RED) of processed requests.
func (p *PromInstrument) WrapFilter(prefix string, f filters.Filter) filters.Filter {
	wrapped := p.filters[prefix]
	if wrapped == nil {
		wrapped = &instrumentedFilter{
			promauto.With(p.registry).NewCounter(prometheus.CounterOpts{
				Name: prefix + "_requests_total",
			}),
			promauto.With(p.registry).NewCounter(prometheus.CounterOpts{
				Name: prefix + "_request_errors_total",
			}),
			promauto.With(p.registry).NewHistogram(prometheus.HistogramOpts{
				Name:    prefix + "_request_duration_seconds",
				Buckets: []float64{0.001, 0.01, 0.1, 1},
			}),
			f}
		p.filters[prefix] = wrapped
	}
	return wrapped
}

// WrapConnErrorHandler wraps an error handler to instrument the error count.
func (p *PromInstrument) WrapConnErrorHandler(prefix string, f func(conn net.Conn, err error)) func(conn net.Conn, err error) {
	h := p.errorHandlers[prefix]
	if h == nil {
		errors := promauto.With(p.registry).NewCounter(prometheus.CounterOpts{
			Name: prefix + "_errors_total",
		})
		consec_errors := promauto.With(p.registry).NewCounter(prometheus.CounterOpts{
			Name: prefix + "_consec_per_client_ip_errors_total",
		})
		if f == nil {
			f = func(conn net.Conn, err error) {}
		}
		var mu sync.Mutex
		var lastRemoteIP string
		h = func(conn net.Conn, err error) {
			errors.Inc()
			addr := conn.RemoteAddr()
			if addr == nil {
				return
			}
			host, _, err := net.SplitHostPort(addr.String())
			if err != nil {
				return
			}
			mu.Lock()
			if lastRemoteIP != host {
				lastRemoteIP = host
				mu.Unlock()
				consec_errors.Inc()
			} else {
				mu.Unlock()
			}
			f(conn, err)
		}
		p.errorHandlers[prefix] = h
	}
	return h
}

// Blacklist instruments the blacklist checking.
func (p *PromInstrument) Blacklist(b bool) {
	p.blacklistChecked.Inc()
	if b {
		p.blacklisted.Inc()
	}
}

// Mimic instruments the Apache mimicry.
func (p *PromInstrument) Mimic(m bool) {
	p.mimicryChecked.Inc()
	if m {
		p.mimicked.Inc()
	}
}

// Throttle instruments the device based throttling.
func (p *PromInstrument) Throttle(m bool, reason string) {
	p.throttlingChecked.Inc()
	if m {
		p.throttled.With(prometheus.Labels{"reason": reason}).Inc()
	} else {
		p.notThrottled.With(prometheus.Labels{"reason": reason}).Inc()
	}
}

// XBQHeaderSent counts the number of times XBQ header is sent along with the
// response.
func (p *PromInstrument) XBQHeaderSent() {
	p.xbqSent.Inc()
}

func (p *PromInstrument) TLSMalformedHello() {
	p.tlsMalformedHello.Inc()
}

func (p *PromInstrument) TLSNoSupportSessionTickets() {
	p.tlsNoSupportSessionTickets.Inc()
}

func (p *PromInstrument) TLSMissingSessionTicket() {
	p.tlsMissingSessionTicket.Inc()
}

func (p *PromInstrument) TLSInvalidSessionTicket() {
	p.tlsInvalidSessionTicket.Inc()
}

// VersionCheck records the number of times the Lantern version header is
// checked and if redirecting to the upgrade page is required.
func (p *PromInstrument) VersionCheck(redirect bool, method, reason string) {
	labels := prometheus.Labels{"method": method, "redirected": strconv.FormatBool(redirect), "reason": reason}
	p.versionCheck.With(labels).Inc()
}

// ProxiedBytes records the volume of application data clients sent and
// received via the proxy.
func (p *PromInstrument) ProxiedBytes(sent, recv int, platform, version, app, dataCapCohort string, clientIP net.IP, deviceID, originHost string) {
	p.bytesSent.Add(float64(sent))
	p.bytesRecv.Add(float64(recv))

	// Track the cardinality of clients.
	p.activeClients1m.Add(deviceID)
	p.activeClients10m.Add(deviceID)
	p.activeClients1h.Add(deviceID)

	country := p.countryLookup.CountryCode(clientIP)
	isp := p.ispLookup.ISP(clientIP)
	asn := p.ispLookup.ASN(clientIP)
	by_isp := prometheus.Labels{"country": country, "isp": "omitted"}
	// We care about ISPs within these countries only, to reduce cardinality of the metrics
	if country == "CN" || country == "IR" || country == "AE" || country == "TK" {
		by_isp["isp"] = isp
	}

	clientKey := clientDetails{
		platform: platform,
		version:  version,
		country:  country,
		isp:      isp,
		asn:      asn,
	}
	clientKeyWithDeviceID := clientDetails{
		deviceID: deviceID,
		platform: platform,
		version:  version,
		country:  country,
		isp:      isp,
	}
	p.statsMx.Lock()
	p.clientStats[clientKey] = p.clientStats[clientKey].add(sent, recv)
	p.clientStatsWithDeviceID[clientKeyWithDeviceID] = p.clientStatsWithDeviceID[clientKeyWithDeviceID].add(sent, recv)
	if originHost != "" {
		originRoot, err := p.originRoot(originHost)
		if err == nil {
			// only record if we could extract originRoot
			originKey := originDetails{
				origin:   originRoot,
				platform: platform,
				version:  version,
				country:  country,
			}
			p.originStats[originKey] = p.originStats[originKey].add(sent, recv)
		}
	}
	p.statsMx.Unlock()
}

// quicPackets is used by QuicTracer to update QUIC retransmissions mainly for block detection.
func (p *PromInstrument) quicSentPacket() {
	p.quicSentPackets.Inc()
}

func (p *PromInstrument) quicLostPacket() {
	p.quicLostPackets.Inc()
}

type stats struct {
	framesSent          prometheus.Counter
	bytesSent           prometheus.Counter
	framesRetransmitted prometheus.Counter
	bytesRetransmitted  prometheus.Counter
	framesReceived      prometheus.Counter
	bytesReceived       prometheus.Counter
}

func (s *stats) OnRecv(n uint64) {
	s.framesReceived.Inc()
	s.bytesReceived.Add(float64(n))
}
func (s *stats) OnSent(n uint64) {
	s.framesSent.Inc()
	s.bytesSent.Add(float64(n))
}
func (s *stats) OnRetransmit(n uint64) {
	s.framesRetransmitted.Inc()
	s.bytesRetransmitted.Add(float64(n))
}
func (s *stats) UpdateRTT(time.Duration) {
	// do nothing as the RTT from different clients can vary significantly
}

func (prom *PromInstrument) MultipathStats(protocols []string) (trackers []multipath.StatsTracker) {
	for _, p := range protocols {
		path_protocol := prometheus.Labels{"path_protocol": p}
		trackers = append(trackers, &stats{
			framesSent:          prom.mpFramesSent.With(path_protocol),
			bytesSent:           prom.mpBytesSent.With(path_protocol),
			framesReceived:      prom.mpFramesReceived.With(path_protocol),
			bytesReceived:       prom.mpBytesReceived.With(path_protocol),
			framesRetransmitted: prom.mpFramesRetransmitted.With(path_protocol),
			bytesRetransmitted:  prom.mpBytesRetransmitted.With(path_protocol),
		})
	}
	return
}

type clientDetails struct {
	deviceID string
	platform string
	version  string
	country  string
	isp      string
	asn      string
}

type originDetails struct {
	origin   string
	platform string
	version  string
	country  string
}

type usage struct {
	sent int
	recv int
}

func (u *usage) add(sent int, recv int) *usage {
	if u == nil {
		u = &usage{}
	}
	u.sent += sent
	u.recv += recv
	return u
}

func (p *PromInstrument) ReportToOTELPeriodically(interval time.Duration, tp *sdktrace.TracerProvider, includeDeviceID bool) {
	for {
		// We randomize the sleep time to avoid bursty submission to OpenTelemetry.
		// Even though each proxy sends relatively little data, proxies often run fairly
		// closely synchronized since they all update to a new binary and restart around the same
		// time. By randomizing each proxy's interval, we smooth out the pattern of submissions.
		sleepInterval := rand.Int63n(int64(interval * 2))
		time.Sleep(time.Duration(sleepInterval))
		p.ReportToOTEL(tp, includeDeviceID)
	}
}

func (p *PromInstrument) ReportToOTEL(tp *sdktrace.TracerProvider, includeDeviceID bool) {
	var clientStats map[clientDetails]*usage
	p.statsMx.Lock()
	if includeDeviceID {
		clientStats = p.clientStatsWithDeviceID
		p.clientStatsWithDeviceID = make(map[clientDetails]*usage)
	} else {
		clientStats = p.clientStats
		p.clientStats = make(map[clientDetails]*usage)
	}
	originStats := p.originStats
	p.originStats = make(map[originDetails]*usage)
	p.statsMx.Unlock()

	for key, value := range clientStats {
		_, span := tp.Tracer("").
			Start(
				context.Background(),
				"proxied_bytes",
				trace.WithAttributes(
					attribute.Int("bytes_sent", value.sent),
					attribute.Int("bytes_recv", value.recv),
					attribute.Int("bytes_total", value.sent+value.recv),
					attribute.String("device_id", key.deviceID),
					attribute.String("client_platform", key.platform),
					attribute.String("client_version", key.version),
					attribute.String("client_country", key.country),
					attribute.String("client_isp", key.isp),
					attribute.String("client_asn", key.asn)))
		span.End()
	}
	if !includeDeviceID {
		// In order to prevent associating origins with device IDs, only report
		// origin stats if we're not including device IDs.
		for key, value := range originStats {
			_, span := tp.Tracer("").
				Start(
					context.Background(),
					"origin_bytes",
					trace.WithAttributes(
						attribute.Int("origin_bytes_sent", value.sent),
						attribute.Int("origin_bytes_recv", value.recv),
						attribute.Int("origin_bytes_total", value.sent+value.recv),
						attribute.String("origin", key.origin),
						attribute.String("client_platform", key.platform),
						attribute.String("client_version", key.version),
						attribute.String("client_country", key.country)))
			span.End()
		}
	}
}

func (p *PromInstrument) originRoot(origin string) (string, error) {
	ip := net.ParseIP(origin)
	if ip != nil {
		// origin is an IP address, try to get domain name
		origins, err := net.LookupAddr(origin)
		if err != nil || net.ParseIP(origins[0]) != nil {
			// failed to reverse lookup, try to get ASN
			asn := p.ispLookup.ASN(ip)
			if asn != "" {
				return asn, nil
			}
			return "", errors.New("unable to lookup ip %v", ip)
		}
		return p.originRoot(stripTrailingDot(origins[0]))
	}
	matches := originRootRegex.FindStringSubmatch(origin)
	if matches == nil {
		// regex didn't match, return origin as is
		return origin, nil
	}
	return matches[1], nil
}

func stripTrailingDot(s string) string {
	return strings.TrimRight(s, ".")
}
