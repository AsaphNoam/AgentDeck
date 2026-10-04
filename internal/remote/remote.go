// Package remote owns Chuck's optional remote-control channel: the embedded
// tailnet node and the connection state the desktop shows (TS-13.R1–R3). It is
// never a runtime — a failure here surfaces only as an Unavailable state and
// never stops the loopback server, agents, tasks, or pipelines.
package remote

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"
)

// Connection states (TS-13.R3, FS-20.R2).
const (
	StateOff         = "off"
	StateNeedsLogin  = "needs_login"
	StateStarting    = "starting"
	StateOn          = "on"
	StateUnavailable = "unavailable"
)

// Unavailable reasons (TS-13.R3).
const (
	ReasonMagicDNSDisabled = "magicdns_disabled"
	ReasonHTTPSDisabled    = "https_disabled"
	ReasonNodeError        = "node_error"
	ReasonListenerError    = "listener_error"
)

// Node listener prerequisite errors. The production node adapter maps the
// tailnet library's refusals onto these so the state mapping stays testable.
var (
	ErrMagicDNSDisabled = errors.New("remote: MagicDNS is disabled")
	ErrHTTPSDisabled    = errors.New("remote: HTTPS certificates are disabled")
)

// Backend states reported by a Node, mirroring ipn.State names.
const (
	BackendNeedsLogin = "NeedsLogin"
	BackendRunning    = "Running"
)

// NodeStatus is the subset of the node's status the state mapping needs.
type NodeStatus struct {
	BackendState string
	AuthURL      string
	CertDomains  []string
}

// Peer identifies the tailnet device behind a connection (TS-13.R7).
type Peer struct {
	StableID string
	Login    string
}

// Node is the embedded tailnet node. Production wraps tsnet.Server; tests use
// a fake so no suite contacts a real tailnet (TS-06.R27).
type Node interface {
	Start() error
	Status(ctx context.Context) (NodeStatus, error)
	ListenTLS() (net.Listener, error)
	WhoIs(ctx context.Context, remoteAddr string) (Peer, error)
	Close() error
}

// NodeFactory builds a fresh node for one enable generation.
type NodeFactory func() (Node, error)

// Listener is what the manager hands the server once the tailnet listener is
// up: the listener, the certificate domain the guard pins, and the node's peer
// lookup.
type Listener struct {
	Listener net.Listener
	Domain   string
	WhoIs    func(ctx context.Context, remoteAddr string) (Peer, error)
}

// ServeFunc serves a ready listener and returns a stop function that closes
// every open connection on it (FS-20.R4). It must not block.
type ServeFunc func(Listener) (stop func())

// Status is the published connection state (TS-13.R3). AuthURL is for the
// desktop only and is never sent to a phone.
type Status struct {
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
	AuthURL string `json:"auth_url,omitempty"`
	Address string `json:"address,omitempty"`
}

// pollInterval is how often a running generation re-reads node status.
const pollInterval = time.Second

// Manager runs at most one node generation at a time. Enable and Disable are
// generation-scoped: a late result from a superseded generation closes its own
// node instead of publishing state (INV §4, §5).
type Manager struct {
	newNode  NodeFactory
	serve    ServeFunc
	publish  func(Status)
	log      *slog.Logger
	interval time.Duration

	mu      sync.Mutex
	gen     uint64
	status  Status
	cancel  context.CancelFunc
	done    chan struct{}
	enabled bool
}

// NewManager builds a manager in the Off state. publish receives every state
// change; serve receives each ready listener.
func NewManager(newNode NodeFactory, serve ServeFunc, publish func(Status), log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{newNode: newNode, serve: serve, publish: publish, log: log, interval: pollInterval, status: Status{State: StateOff}}
}

// SetPollInterval shortens status polling for tests.
func (m *Manager) SetPollInterval(d time.Duration) { m.interval = d }

// Status returns the current connection state.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Enable starts a node generation unless one is already running.
func (m *Manager) Enable() {
	m.mu.Lock()
	if m.enabled {
		m.mu.Unlock()
		return
	}
	m.enabled = true
	m.gen++
	gen := m.gen
	ctx, cancel := context.WithCancel(context.Background())
	done, prev := make(chan struct{}), m.done
	m.cancel, m.done = cancel, done
	m.setLocked(gen, Status{State: StateStarting})
	m.mu.Unlock()
	go func() {
		defer close(done)
		// Two nodes must never share the state directory, so a new generation
		// starts only after the previous one has closed its node.
		if prev != nil {
			<-prev
		}
		m.run(ctx, gen)
	}()
}

// Disable ends the running generation, closes its node and every open remote
// connection, and waits for the generation to finish (FS-20.R4).
func (m *Manager) Disable() {
	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return
	}
	m.enabled = false
	m.gen++
	cancel, done := m.cancel, m.done
	m.cancel = nil
	m.setLocked(m.gen, Status{State: StateOff})
	m.mu.Unlock()
	cancel()
	<-done
}

// set publishes a status only when gen is still current.
func (m *Manager) set(gen uint64, st Status) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setLocked(gen, st)
}

func (m *Manager) setLocked(gen uint64, st Status) bool {
	if gen != m.gen {
		return false
	}
	if st == m.status {
		return true
	}
	m.status = st
	if m.publish != nil {
		m.publish(st)
	}
	return true
}

// run owns one generation's node from creation to close.
func (m *Manager) run(ctx context.Context, gen uint64) {
	if ctx.Err() != nil {
		return
	}
	node, err := m.newNode()
	if err != nil {
		m.log.Warn("remote: create node", "err", err)
		m.set(gen, Status{State: StateUnavailable, Reason: ReasonNodeError})
		return
	}
	var stop func()
	defer func() {
		if stop != nil {
			stop()
		}
		if err := node.Close(); err != nil {
			m.log.Debug("remote: close node", "err", err)
		}
	}()
	if err := node.Start(); err != nil {
		m.log.Warn("remote: start node", "err", err)
		m.set(gen, Status{State: StateUnavailable, Reason: ReasonNodeError})
		<-ctx.Done()
		return
	}
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		if !m.set(gen, m.poll(ctx, node, &stop)) {
			return // superseded
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// poll maps one node status read to a connection state, bringing the listener
// up the first time the node runs. Failures are re-polled, so a prerequisite
// repaired in the Tailscale admin console recovers without toggling.
func (m *Manager) poll(ctx context.Context, node Node, stop *func()) Status {
	ns, err := node.Status(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return Status{State: StateStarting}
		}
		m.log.Warn("remote: node status", "err", err)
		return Status{State: StateUnavailable, Reason: ReasonNodeError}
	}
	switch ns.BackendState {
	case BackendNeedsLogin:
		return Status{State: StateNeedsLogin, AuthURL: ns.AuthURL}
	case BackendRunning:
	default:
		return Status{State: StateStarting}
	}
	if len(ns.CertDomains) == 0 {
		return Status{State: StateUnavailable, Reason: ReasonHTTPSDisabled}
	}
	domain := ns.CertDomains[0]
	if *stop == nil {
		ln, err := node.ListenTLS()
		switch {
		case errors.Is(err, ErrMagicDNSDisabled):
			return Status{State: StateUnavailable, Reason: ReasonMagicDNSDisabled}
		case errors.Is(err, ErrHTTPSDisabled):
			return Status{State: StateUnavailable, Reason: ReasonHTTPSDisabled}
		case err != nil:
			m.log.Warn("remote: listen", "err", err)
			return Status{State: StateUnavailable, Reason: ReasonListenerError}
		}
		if m.serve != nil {
			*stop = m.serve(Listener{Listener: ln, Domain: domain, WhoIs: node.WhoIs})
		} else {
			*stop = func() { ln.Close() }
		}
	}
	return Status{State: StateOn, Address: "https://" + domain}
}
