package server

import (
	"fmt"
	"net"
	"strconv"
)

// BindHost is the only host the dashboard ever binds to. The server refuses to
// listen on anything that is not a loopback address (defense in depth behind the
// hard-coded value).
const BindHost = "127.0.0.1"

// LocalAddr builds the loopback listen address for port, validating the range.
func LocalAddr(port int) (string, error) {
	if port <= 0 || port > 65535 {
		return "", fmt.Errorf("invalid port %d", port)
	}
	return net.JoinHostPort(BindHost, strconv.Itoa(port)), nil
}

// assertLoopback fails closed if addr is not a loopback TCP address. This is the
// runtime guard backing the "never 0.0.0.0" requirement and is directly testable.
func assertLoopback(addr net.Addr) error {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("listener address is not TCP: %v", addr)
	}
	if !tcp.IP.IsLoopback() {
		return fmt.Errorf("refusing non-loopback bind address %s", tcp.IP)
	}
	return nil
}
