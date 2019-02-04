// Package blacklist provides a mechanism for blacklisting IP addresses that
// connect but never make it past our security filtering, either because they're
// not sending HTTP requests or sending invalid HTTP requests.
package blacklist

import (
	"fmt"
	"sync"
	"time"

	"github.com/getlantern/golog"
	"github.com/getlantern/http-proxy-lantern/instrument"
	"github.com/getlantern/ops"
	"github.com/getlantern/pcapper"
)

var (
	log = golog.LoggerFor("blacklist")

	blacklistingEnabled = false // we've temporarily turned off blacklisting for safety
)

// Options is a set of options to initialize a blacklist.
type Options struct {
	// The maximum amount of time we'll wait between the start of a connection
	// and seeing a successful HTTP request before we mark the connection as
	// failed.
	MaxIdleTime time.Duration
	// Consecutive connection attempts within this interval will be treated as a
	// single attempt.
	MaxConnectInterval time.Duration
	// The number of consecutive failures allowed before an IP is blacklisted
	AllowedFailures int
	// How long an IP is allowed to remain on the blacklist.  In practice, an
	// IP may end up on the blacklist up to 1.1 * blacklistExpiration.
	Expiration time.Duration
}

// Blacklist is a blacklist of IPs.
type Blacklist struct {
	maxIdleTime         time.Duration
	maxConnectInterval  time.Duration
	allowedFailures     int
	blacklistExpiration time.Duration
	connections         chan string
	successes           chan string
	firstConnectionTime map[string]time.Time
	lastConnectionTime  map[string]time.Time
	failureCounts       map[string]int
	blacklist           map[string]time.Time
	mutex               sync.RWMutex
	instrument          func(bool)
}

// New creates a new Blacklist with given options.
func New(opts Options) *Blacklist {
	bl := &Blacklist{
		maxIdleTime:         opts.MaxIdleTime,
		maxConnectInterval:  opts.MaxConnectInterval,
		allowedFailures:     opts.AllowedFailures,
		blacklistExpiration: opts.Expiration,
		connections:         make(chan string, 10000),
		successes:           make(chan string, 10000),
		firstConnectionTime: make(map[string]time.Time),
		lastConnectionTime:  make(map[string]time.Time),
		failureCounts:       make(map[string]int),
		blacklist:           make(map[string]time.Time),
		instrument:          instrument.Blacklist(),
	}
	go bl.track()
	return bl
}

// Succeed records a success for the given addr, which resets the failure count
// for that IP and removes it from the blacklist.
func (bl *Blacklist) Succeed(ip string) {
	select {
	case bl.successes <- ip:
		// ip submitted as success
	default:
		_ = log.Errorf("Unable to record success from %v", ip)
	}
}

// OnConnect records an attempt to connect from the given IP. If the IP is
// blacklisted, this returns false.
func (bl *Blacklist) OnConnect(ip string) bool {
	if !blacklistingEnabled {
		bl.instrument(false)
		return true
	}
	bl.mutex.RLock()
	defer bl.mutex.RUnlock()
	_, blacklisted := bl.blacklist[ip]
	if blacklisted {
		bl.instrument(true)
		return false
		log.Debugf("%v is blacklisted", ip)
	}
	select {
	case bl.connections <- ip:
		// ip submitted as connected
	default:
		_ = log.Errorf("Unable to record connection from %v", ip)
	}
	bl.instrument(false)
	return true
}

func (bl *Blacklist) track() {
	idleTimer := time.NewTimer(bl.maxIdleTime)
	blacklistTimer := time.NewTimer(bl.blacklistExpiration / 10)
	for {
		select {
		case ip := <-bl.connections:
			bl.onConnection(ip)
		case ip := <-bl.successes:
			bl.onSuccess(ip)
		case <-idleTimer.C:
			bl.checkForIdlers()
			idleTimer.Reset(bl.maxIdleTime)
		case <-blacklistTimer.C:
			bl.checkExpiration()
			blacklistTimer.Reset(bl.blacklistExpiration / 10)
		}
	}
}

func (bl *Blacklist) onConnection(ip string) {
	now := time.Now()
	t, exists := bl.lastConnectionTime[ip]
	bl.lastConnectionTime[ip] = now
	if now.Sub(t) > bl.maxConnectInterval {
		bl.failureCounts[ip] = 0
		return
	}

	_, exists = bl.firstConnectionTime[ip]
	if !exists {
		bl.firstConnectionTime[ip] = now
	}
}

func (bl *Blacklist) onSuccess(ip string) {
	bl.failureCounts[ip] = 0
	delete(bl.lastConnectionTime, ip)
	delete(bl.firstConnectionTime, ip)
	bl.mutex.Lock()
	delete(bl.blacklist, ip)
	bl.mutex.Unlock()
}

func (bl *Blacklist) checkForIdlers() {
	log.Trace("Checking for idlers")
	now := time.Now()
	var blacklistAdditions []string
	for ip, t := range bl.firstConnectionTime {
		if now.Sub(t) > bl.maxIdleTime {
			msg := fmt.Sprintf("%v connected but failed to successfully send an HTTP request within %v", ip, bl.maxIdleTime)
			log.Debug(msg)
			delete(bl.firstConnectionTime, ip)
			ops.Begin("connect_without_request").Set("client_ip", ip).End()
			pcapper.Dump(ip, fmt.Sprintf("Blacklist Check: %v", msg))

			count := bl.failureCounts[ip] + 1
			bl.failureCounts[ip] = count
			if count >= bl.allowedFailures {
				ops.Begin("blacklist").Set("client_ip", ip).End()
				_ = log.Errorf("Blacklisting %v", ip)
				blacklistAdditions = append(blacklistAdditions, ip)
			}
		}
	}
	if len(blacklistAdditions) > 0 {
		bl.mutex.Lock()
		for _, ip := range blacklistAdditions {
			bl.blacklist[ip] = now
		}
		bl.mutex.Unlock()
	}
}

func (bl *Blacklist) checkExpiration() {
	now := time.Now()
	bl.mutex.Lock()
	for ip, blacklistedAt := range bl.blacklist {
		if now.Sub(blacklistedAt) > bl.blacklistExpiration {
			log.Debugf("Removing %v from blacklist", ip)
			delete(bl.blacklist, ip)
			delete(bl.failureCounts, ip)
			delete(bl.firstConnectionTime, ip)
		}
	}
	bl.mutex.Unlock()
}
