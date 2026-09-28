//go:build dev

package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/agentdeck/agentdeck/internal/remote"
)

// In the `dev` build only, AGENTDECK_DEV_FAKE_TAILNET=localhost:<port> swaps
// the embedded tailnet node for a local HTTPS listener with a self-signed
// certificate and one fixed peer, so the phone app can be driven in a local
// browser. Release builds never contain this file (TS-06.R27).
func init() {
	addr := os.Getenv("AGENTDECK_DEV_FAKE_TAILNET")
	if addr == "" {
		return
	}
	devRemoteNodeFactory = func() (remote.Node, error) { return &devTailnetNode{addr: addr}, nil }
}

type devTailnetNode struct{ addr string }

func (n *devTailnetNode) Start() error { return nil }

func (n *devTailnetNode) Status(context.Context) (remote.NodeStatus, error) {
	return remote.NodeStatus{BackendState: remote.BackendRunning, CertDomains: []string{n.addr}}, nil
}

func (n *devTailnetNode) ListenTLS() (net.Listener, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(n.addr)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	return tls.Listen("tcp", n.addr, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}})
}

func (n *devTailnetNode) WhoIs(context.Context, string) (remote.Peer, error) {
	return remote.Peer{StableID: "dev-phone", Login: "dev@localhost"}, nil
}

func (n *devTailnetNode) Close() error { return nil }
