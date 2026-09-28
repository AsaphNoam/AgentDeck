package remote

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeNode is a scripted tailnet node. Tests change its status between polls.
type fakeNode struct {
	mu        sync.Mutex
	status    NodeStatus
	listenErr error
	startErr  error
	closed    bool
	listens   int
}

func (n *fakeNode) Start() error { return n.startErr }
func (n *fakeNode) Status(context.Context) (NodeStatus, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.status, nil
}
func (n *fakeNode) ListenTLS() (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.listenErr != nil {
		return nil, n.listenErr
	}
	n.listens++
	return net.Listen("tcp", "127.0.0.1:0")
}
func (n *fakeNode) WhoIs(context.Context, string) (Peer, error) { return Peer{}, nil }
func (n *fakeNode) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.closed = true
	return nil
}
func (n *fakeNode) set(f func(*fakeNode)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	f(n)
}

type recorder struct {
	mu     sync.Mutex
	states []Status
}

func (r *recorder) publish(s Status) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = append(r.states, s)
}

func waitState(t *testing.T, m *Manager, want Status) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.Status() == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("status = %+v, want %+v", m.Status(), want)
}

func newTestManager(node *fakeNode, serve ServeFunc) (*Manager, *recorder) {
	rec := &recorder{}
	m := NewManager(func() (Node, error) { return node, nil }, serve, rec.publish, nil)
	m.SetPollInterval(time.Millisecond)
	return m, rec
}

func TestManagerMapsNodeStates(t *testing.T) {
	node := &fakeNode{status: NodeStatus{BackendState: "Starting"}}
	var served, stopped int
	var mu sync.Mutex
	m, _ := newTestManager(node, func(l Listener) func() {
		mu.Lock()
		served++
		mu.Unlock()
		if l.Domain != "agentdeck.tail.ts.net" {
			t.Errorf("domain = %q", l.Domain)
		}
		return func() {
			mu.Lock()
			stopped++
			mu.Unlock()
			l.Listener.Close()
		}
	})
	if got := m.Status(); got.State != StateOff {
		t.Fatalf("initial state = %+v", got)
	}
	m.Enable()
	waitState(t, m, Status{State: StateStarting})

	node.set(func(n *fakeNode) {
		n.status = NodeStatus{BackendState: BackendNeedsLogin, AuthURL: "https://login.example/a"}
	})
	waitState(t, m, Status{State: StateNeedsLogin, AuthURL: "https://login.example/a"})

	node.set(func(n *fakeNode) { n.status = NodeStatus{BackendState: BackendRunning} })
	waitState(t, m, Status{State: StateUnavailable, Reason: ReasonHTTPSDisabled})

	node.set(func(n *fakeNode) {
		n.status = NodeStatus{BackendState: BackendRunning, CertDomains: []string{"agentdeck.tail.ts.net"}}
		n.listenErr = ErrMagicDNSDisabled
	})
	waitState(t, m, Status{State: StateUnavailable, Reason: ReasonMagicDNSDisabled})

	// A repaired prerequisite recovers without toggling (re-polled).
	node.set(func(n *fakeNode) { n.listenErr = nil })
	waitState(t, m, Status{State: StateOn, Address: "https://agentdeck.tail.ts.net"})

	// Sign-in loss returns to Needs sign-in (FS-20.R26) without re-listening.
	node.set(func(n *fakeNode) { n.status = NodeStatus{BackendState: BackendNeedsLogin} })
	waitState(t, m, Status{State: StateNeedsLogin})
	node.set(func(n *fakeNode) {
		n.status = NodeStatus{BackendState: BackendRunning, CertDomains: []string{"agentdeck.tail.ts.net"}}
	})
	waitState(t, m, Status{State: StateOn, Address: "https://agentdeck.tail.ts.net"})

	m.Disable()
	if got := m.Status(); got.State != StateOff {
		t.Fatalf("after disable = %+v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if served != 1 || stopped != 1 {
		t.Fatalf("served=%d stopped=%d, want 1/1", served, stopped)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if !node.closed || node.listens != 1 {
		t.Fatalf("closed=%v listens=%d", node.closed, node.listens)
	}
}

func TestManagerListenerAndStartFailures(t *testing.T) {
	node := &fakeNode{
		status:    NodeStatus{BackendState: BackendRunning, CertDomains: []string{"d.ts.net"}},
		listenErr: errors.New("boom"),
	}
	m, _ := newTestManager(node, nil)
	m.Enable()
	waitState(t, m, Status{State: StateUnavailable, Reason: ReasonListenerError})
	m.Disable()

	failing := &fakeNode{startErr: errors.New("no state dir")}
	m2, _ := newTestManager(failing, nil)
	m2.Enable()
	waitState(t, m2, Status{State: StateUnavailable, Reason: ReasonNodeError})
	m2.Disable()
	if !failing.closed {
		t.Fatal("failed start must still close the node")
	}

	m3 := NewManager(func() (Node, error) { return nil, errors.New("no factory") }, nil, nil, nil)
	m3.Enable()
	waitState(t, m3, Status{State: StateUnavailable, Reason: ReasonNodeError})
	m3.Disable()
}

// Rapid toggles never run two nodes at once and a superseded generation never
// publishes after a newer one (INV §4, §5).
func TestManagerGenerationsNeverOverlap(t *testing.T) {
	var mu sync.Mutex
	live, maxLive := 0, 0
	factory := func() (Node, error) {
		mu.Lock()
		live++
		if live > maxLive {
			maxLive = live
		}
		mu.Unlock()
		return &countingNode{fakeNode: fakeNode{status: NodeStatus{BackendState: BackendRunning, CertDomains: []string{"d.ts.net"}}}, onClose: func() {
			mu.Lock()
			live--
			mu.Unlock()
		}}, nil
	}
	m := NewManager(factory, nil, nil, nil)
	m.SetPollInterval(time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				m.Enable()
				m.Disable()
			}
		}()
	}
	wg.Wait()
	m.Enable()
	waitState(t, m, Status{State: StateOn, Address: "https://d.ts.net"})
	m.Disable()
	if got := m.Status(); got.State != StateOff {
		t.Fatalf("final = %+v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if maxLive > 1 || live != 0 {
		t.Fatalf("maxLive=%d live=%d", maxLive, live)
	}
}

type countingNode struct {
	fakeNode
	onClose func()
}

func (n *countingNode) Close() error {
	n.onClose()
	return nil
}

func TestRedactTailscaleLog(t *testing.T) {
	in := "To authenticate, visit: https://login.tailscale.com/a/1b2c3d4e5f key tskey-auth-kAbC-123 nodekey:0123abcd privkey:deadbeef ok"
	got := RedactTailscaleLog(in)
	for _, secret := range []string{"1b2c3d4e5f", "kAbC-123", "0123abcd", "deadbeef"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted line still carries %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "https://login.tailscale.com/a/[redacted]") || !strings.HasSuffix(got, " ok") {
		t.Fatalf("redaction mangled the line: %s", got)
	}
}
