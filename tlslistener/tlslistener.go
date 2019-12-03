// Package tlslistener provides a wrapper around tls.Listen that allows
// descending into the wrapped net.Conn
package tlslistener

import (
	"crypto/tls"
	"net"

	"github.com/getlantern/golog"
	"github.com/getlantern/tlsdefaults"

	utls "github.com/getlantern/utls"
)

// Wrap wraps the specified listener in our default TLS listener.
func Wrap(wrapped net.Listener, keyFile string, certFile string, sessionTicketKeyFile string,
	requireSessionTickets bool, missingTicketReaction HandshakeReaction) (net.Listener, error) {
	cfg, err := tlsdefaults.BuildListenerConfig(wrapped.Addr().String(), keyFile, certFile)
	if err != nil {
		return nil, err
	}
	// This is a bit of a hack to make pre-shared TLS sessions work with uTLS. Ideally we'll make this
	// work with TLS 1.3, see https://github.com/getlantern/lantern-internal/issues/3057.
	cfg.MaxVersion = tls.VersionTLS12

	log := golog.LoggerFor("lantern-proxy-tlslistener")

	utlsConfig := &utls.Config{}
	onKeys := func(keys [][32]byte) {
		utlsConfig.SetSessionTicketKeys(keys)
	}
	expectTickets := sessionTicketKeyFile != ""
	if expectTickets {
		log.Debugf("Will rotate session ticket key and store in %v", sessionTicketKeyFile)
		maintainSessionTicketKey(cfg, sessionTicketKeyFile, onKeys)
	}

	listener := &tlslistener{wrapped, cfg, log, expectTickets, requireSessionTickets, utlsConfig, missingTicketReaction}
	return listener, nil
}

type tlslistener struct {
	wrapped               net.Listener
	cfg                   *tls.Config
	log                   golog.Logger
	expectTickets         bool
	requireTickets        bool
	utlsCfg               *utls.Config
	missingTicketReaction HandshakeReaction
}

func (l *tlslistener) Accept() (net.Conn, error) {
	conn, err := l.wrapped.Accept()
	if err != nil {
		return nil, err
	}
	if !l.expectTickets || !l.requireTickets {
		return &tlsconn{tls.Server(conn, l.cfg), conn}, nil
	}
	helloConn, cfg := newClientHelloRecordingConn(conn, l.cfg, l.utlsCfg, l.missingTicketReaction)
	return &tlsconn{tls.Server(helloConn, cfg), conn}, nil
}

func (l *tlslistener) Addr() net.Addr {
	return l.wrapped.Addr()
}

func (l *tlslistener) Close() error {
	return l.wrapped.Close()
}

type tlsconn struct {
	net.Conn
	wrapped net.Conn
}

func (conn *tlsconn) Wrapped() net.Conn {
	return conn.wrapped
}
