package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/remote"
)

// fakeTailnetNode is a running tailnet node with HTTPS on, backed by a local
// TCP listener; tests never contact a real tailnet (TS-06.R27).
type fakeTailnetNode struct {
	mu     sync.Mutex
	domain string
	peers  map[string]remote.Peer // remote addr -> peer
	closed bool
}

func (n *fakeTailnetNode) Start() error { return nil }
func (n *fakeTailnetNode) Status(context.Context) (remote.NodeStatus, error) {
	return remote.NodeStatus{BackendState: remote.BackendRunning, CertDomains: []string{n.domain}}, nil
}
func (n *fakeTailnetNode) ListenTLS() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
func (n *fakeTailnetNode) WhoIs(_ context.Context, addr string) (remote.Peer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if p, ok := n.peers[addr]; ok {
		return p, nil
	}
	return remote.Peer{}, errNoPeer
}
func (n *fakeTailnetNode) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.closed = true
	return nil
}

var errNoPeer = &net.AddrError{Err: "no such peer"}

func remoteTestServer(t *testing.T) (*Server, *fakeTailnetNode) {
	t.Helper()
	s := testServer(t, true)
	node := &fakeTailnetNode{domain: "chuck.tail1.ts.net", peers: map[string]remote.Peer{}}
	s.newRemoteNode = func() (remote.Node, error) { return node, nil }
	s.remote = s.newRemoteManager()
	s.remote.SetPollInterval(time.Millisecond)
	t.Cleanup(s.remote.Disable)
	return s, node
}

func putRemote(t *testing.T, h http.Handler, body string) remoteView {
	t.Helper()
	req := newLocalRequest(http.MethodPut, "/api/remote", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/remote %s = %d %s", body, rec.Code, rec.Body)
	}
	var v remoteView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRemoteToggleLifecycle(t *testing.T) {
	s, node := remoteTestServer(t)
	h := s.routes()
	events, unsub := s.eventBus.Subscribe()
	defer unsub()

	var v remoteView
	if err := json.Unmarshal(doGET(t, h, "/api/remote").Body.Bytes(), &v); err != nil || v.State != remote.StateOff || v.KeepAwake {
		t.Fatalf("initial GET = %+v, %v", v, err)
	}

	putRemote(t, h, `{"enabled":true}`)
	deadline := time.Now().Add(2 * time.Second)
	for s.remote.Status().State != remote.StateOn {
		if time.Now().After(deadline) {
			t.Fatalf("never reached on: %+v", s.remote.Status())
		}
		time.Sleep(2 * time.Millisecond)
	}
	if err := json.Unmarshal(doGET(t, h, "/api/remote").Body.Bytes(), &v); err != nil || v.Address != "https://chuck.tail1.ts.net" {
		t.Fatalf("on GET = %+v, %v", v, err)
	}
	cfg, _ := s.configStore.ReadConfig()
	if !cfg.RemoteEnabled {
		t.Fatal("remote_enabled not persisted")
	}

	// keep_awake is independent of the node and persists alongside it.
	v = putRemote(t, h, `{"keep_awake":true}`)
	if !v.KeepAwake || v.State != remote.StateOn {
		t.Fatalf("keep-awake PUT = %+v", v)
	}

	v = putRemote(t, h, `{"enabled":false}`)
	if v.State != remote.StateOff || !v.KeepAwake {
		t.Fatalf("disable PUT = %+v", v)
	}
	node.mu.Lock()
	closed := node.closed
	node.mu.Unlock()
	if !closed {
		t.Fatal("disable must close the node")
	}
	cfg, _ = s.configStore.ReadConfig()
	if cfg.RemoteEnabled || !cfg.KeepAwake {
		t.Fatalf("config after disable = %+v", cfg)
	}

	// Every state change reached the loopback stream as remote_update.
	seen := map[string]bool{}
	for len(events) > 0 {
		ev := <-events
		if ev.Type == "remote_update" {
			seen[ev.Data.(remoteView).State] = true
		}
	}
	for _, st := range []string{remote.StateStarting, remote.StateOn, remote.StateOff} {
		if !seen[st] {
			t.Fatalf("remote_update states = %v, missing %s", seen, st)
		}
	}
}

// PUT /api/config and PUT /api/remote share one read-modify-write lock, so a
// concurrent appearance change never erases remote_enabled or vice versa.
func TestRemoteAndConfigWritesDoNotEraseEachOther(t *testing.T) {
	s, _ := remoteTestServer(t)
	h := s.routes()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := newLocalRequest(http.MethodPut, "/api/config", bytes.NewBufferString(`{"task_concurrency":3}`))
			h.ServeHTTP(httptest.NewRecorder(), req)
		}()
		go func() {
			defer wg.Done()
			req := newLocalRequest(http.MethodPut, "/api/remote", bytes.NewBufferString(`{"keep_awake":true}`))
			h.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}
	wg.Wait()
	cfg, err := s.configStore.ReadConfig()
	if err != nil || !cfg.KeepAwake || cfg.TaskConcurrency != 3 {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
}

func TestRemoteUnsupportedBuildIsUnavailable(t *testing.T) {
	s := testServer(t, true)
	h := s.routes()
	putRemote(t, h, `{"enabled":true}`)
	t.Cleanup(s.remote.Disable)
	deadline := time.Now().Add(2 * time.Second)
	for s.remote.Status().State != remote.StateUnavailable {
		if time.Now().After(deadline) {
			t.Fatalf("status = %+v", s.remote.Status())
		}
		time.Sleep(2 * time.Millisecond)
	}
	if st := s.remote.Status(); st.Reason != remote.ReasonNodeError {
		t.Fatalf("reason = %q", st.Reason)
	}
	// The loopback API keeps serving (TS-13.R1).
	if rec := doGET(t, h, "/api/health"); rec.Code != http.StatusOK {
		t.Fatalf("health = %d", rec.Code)
	}
}
