package vmess

import (
	"context"
	"net"

	vmess "github.com/getlantern/sing-vmess"
	N "github.com/getlantern/sing-vmess/network"
	"github.com/sagernet/sing/common/metadata"
)

type listener struct {
	net.Listener
	service *vmess.Service[int]
}

// NewVMessListener wraps a net.Listener with a VMess service
// VMess is used only for obfuscation, not for transport, thus the destination is not read from VMess header
// VMess will accept connections only from users that are identified by UUIDs passed here
// The UUIDs are generated on route setup and stored in the database, there could be several UUIDs per proxy
// The UUIDs will be distributed between the users. There is no 1-1 mapping between UUIDs and users and this is just
// intended as a source of extra entropy
func NewVMessListener(baseListener net.Listener, uuids []string) (net.Listener, error) {
	var userNum []int
	var userAlt []int
	for i := range uuids {
		userNum = append(userNum, i)
		userAlt = append(userAlt, 0) // we don't use altId
	}

	l := &listener{baseListener, nil}
	l.service = vmess.NewService[int]()

	if err := l.service.UpdateUsers(userNum, uuids, userAlt); err != nil {
		return nil, err
	}
	return l, l.service.Start()
}

func (l *listener) Close() error {
	return l.service.Close()
}

// handler is a connection handler for VMess inbound connections.
// it's only purpose is to implement N.ConnectionHandler and store the connection
type handler struct {
	conn        net.Conn
	source      metadata.Socksaddr
	destination metadata.Socksaddr
}

func (h *handler) NewConnectionEx(_ context.Context, conn net.Conn, source metadata.Socksaddr, destination metadata.Socksaddr, _ N.CloseHandlerFunc) {
	h.conn = conn
	h.source = source
	h.destination = destination
}

func (h *handler) NewPacketConnectionEx(_ context.Context, _ N.PacketConn, _ metadata.Socksaddr, _ metadata.Socksaddr, _ N.CloseHandlerFunc) {
	// not used
}

func (l *listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	h := &handler{}
	err = l.service.NewConnection(context.Background(), conn, metadata.Socksaddr{}, nil, h)
	if err != nil {
		return nil, err
	}
	return h.conn, nil
}
