package remote

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"regexp"

	"tailscale.com/client/local"
	"tailscale.com/envknob"
	"tailscale.com/tsnet"
)

// tsnetNode is the production Node: an embedded, non-ephemeral tsnet.Server
// whose state lives in AgentDeck's home (TS-13.R2). Verified against
// tailscale.com v1.102.5 (TS-06.R27).
type tsnetNode struct {
	srv *tsnet.Server
	lc  *local.Client
}

// NewTSNetNode builds the embedded node. dir is created owner-only.
func NewTSNetNode(dir, hostname string, log *slog.Logger) (Node, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("remote: node state dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("remote: node state dir: %w", err)
	}
	// AgentDeck operates no cloud service and uploads nothing; this also keeps
	// the embedded node from sending its logs to Tailscale's log service.
	envknob.SetNoLogsNoSupport()
	return &tsnetNode{srv: &tsnet.Server{
		Dir:      dir,
		Hostname: hostname,
		Logf: func(format string, args ...any) {
			log.Debug("tailscale", "msg", RedactTailscaleLog(fmt.Sprintf(format, args...)))
		},
		UserLogf: func(format string, args ...any) {
			log.Info("tailscale", "msg", RedactTailscaleLog(fmt.Sprintf(format, args...)))
		},
	}}, nil
}

func (n *tsnetNode) Start() error {
	if err := n.srv.Start(); err != nil {
		return err
	}
	lc, err := n.srv.LocalClient()
	if err != nil {
		return err
	}
	n.lc = lc
	return nil
}

func (n *tsnetNode) Status(ctx context.Context) (NodeStatus, error) {
	st, err := n.lc.StatusWithoutPeers(ctx)
	if err != nil {
		return NodeStatus{}, err
	}
	return NodeStatus{BackendState: st.BackendState, AuthURL: st.AuthURL, CertDomains: st.CertDomains}, nil
}

// ListenTLS checks tsnet's own prerequisites first so the refusals map to
// stable reasons instead of error strings.
func (n *tsnetNode) ListenTLS() (net.Listener, error) {
	st, err := n.lc.StatusWithoutPeers(context.Background())
	if err != nil {
		return nil, err
	}
	if st.CurrentTailnet != nil && !st.CurrentTailnet.MagicDNSEnabled {
		return nil, ErrMagicDNSDisabled
	}
	if len(st.CertDomains) == 0 {
		return nil, ErrHTTPSDisabled
	}
	return n.srv.ListenTLS("tcp", ":443")
}

func (n *tsnetNode) WhoIs(ctx context.Context, remoteAddr string) (Peer, error) {
	who, err := n.lc.WhoIs(ctx, remoteAddr)
	if err != nil {
		return Peer{}, err
	}
	if who.Node == nil {
		return Peer{}, fmt.Errorf("remote: whois %s: no node", remoteAddr)
	}
	p := Peer{StableID: string(who.Node.StableID)}
	if who.UserProfile != nil {
		p.Login = who.UserProfile.LoginName
	}
	return p, nil
}

func (n *tsnetNode) Close() error { return n.srv.Close() }

var tailscaleSecrets = regexp.MustCompile(`(https://login\.tailscale\.com/a/)\S+|(tskey-)\S+|((?:priv|nlpriv|discopriv|node|machine)key:)[0-9a-f]+`)

// RedactTailscaleLog strips sign-in URLs, auth keys, and key material from a
// tailnet log line (TS-13.R12).
func RedactTailscaleLog(line string) string {
	return tailscaleSecrets.ReplaceAllString(line, "$1$2$3[redacted]")
}
