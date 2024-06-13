package shadowsocks

import (
	"context"
	"fmt"
	"net"

	"github.com/Jigsaw-Code/outline-sdk/transport"
)

type LocalDialer struct {
	connections chan net.Conn
}

func (d *LocalDialer) DialStream(ctx context.Context, addr string) (transport.StreamConn, error) {
	cliConn, ok := ctx.Value(ClientConnCtxKey{}).(transport.StreamConn)
	if !ok {
		return nil, fmt.Errorf("expected stream connection in context but received type %T", ctx.Value(ClientConnCtxKey{}))
	}

	c1, c2 := net.Pipe()
	a := &tcpConnAdapter{c1}
	b := &lfwd{
		Conn:           c2,
		remoteAddr:     cliConn.RemoteAddr(),
		clientTCPConn:  cliConn,
		upstreamTarget: addr,
	}
	d.connections <- b

	return a, nil
}
