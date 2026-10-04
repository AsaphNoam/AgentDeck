package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/remote"
)

// pairingServer is a server with remote control On and a tailnet handler whose
// peers are keyed by remote address.
func pairingServer(t *testing.T) (*Server, http.Handler, http.Handler) {
	t.Helper()
	s, _ := remoteTestServer(t)
	loop := s.routes()
	putRemote(t, loop, `{"enabled":true}`)
	deadline := time.Now().Add(2 * time.Second)
	for s.remote.Status().State != remote.StateOn {
		if time.Now().After(deadline) {
			t.Fatalf("remote never on: %+v", s.remote.Status())
		}
		time.Sleep(2 * time.Millisecond)
	}
	phone := s.remoteRoutes(testDomain, testWhoIs(map[string]string{
		"100.64.0.2:5000": "node-phone",
		"100.64.0.3:5000": "node-other",
	}))
	return s, loop, phone
}

func loopPost(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newLocalRequest(http.MethodPost, path, bytes.NewBufferString(body)))
	return rec
}

func issueCode(t *testing.T, loop http.Handler) (code, qr string) {
	t.Helper()
	rec := loopPost(t, loop, "/api/remote/pairings", "")
	var out struct {
		Code  string `json:"code"`
		QRURL string `json:"qr_url"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("issue code = %d %s", rec.Code, rec.Body)
	}
	return out.Code, out.QRURL
}

func claim(h http.Handler, addr, code string) *httptest.ResponseRecorder {
	r := phoneRequest(http.MethodPost, "/api/remote/pair", `{"code":"`+code+`","name":"Pixel 9"}`, "")
	r.RemoteAddr = addr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func wait(h http.Handler, addr, pendingID string) *httptest.ResponseRecorder {
	r := phoneRequest(http.MethodGet, "/api/remote/pair/"+pendingID, "", "")
	r.RemoteAddr = addr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func pendingID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		PendingID string `json:"pending_id"`
	}
	if rec.Code != http.StatusAccepted || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.PendingID == "" {
		t.Fatalf("claim = %d %s", rec.Code, rec.Body)
	}
	return out.PendingID
}

func TestRemotePairingNeedsRemoteOn(t *testing.T) {
	s := testServer(t, true)
	if rec := loopPost(t, s.routes(), "/api/remote/pairings", ""); rec.Code != http.StatusConflict {
		t.Fatalf("pairing while off = %d", rec.Code)
	}
}

func TestRemotePairingAllowFlow(t *testing.T) {
	s, loop, phone := pairingServer(t)
	code, qr := issueCode(t, loop)
	if len(code) != pairCodeLength || strings.ContainsAny(code, "01ILO") || qr != "https://"+testDomain+"/pair#"+code {
		t.Fatalf("code=%q qr=%q", code, qr)
	}
	if rec := claim(phone, "100.64.0.2:5000", "WRONG123"); rec.Code != 400 || errorCode(t, rec) != codeRemotePairingInvalid {
		t.Fatalf("wrong code = %d %s", rec.Code, rec.Body)
	}
	pid := pendingID(t, claim(phone, "100.64.0.2:5000", strings.ToLower(code)))
	if rec := claim(phone, "100.64.0.2:5000", code); rec.Code != 400 {
		t.Fatalf("reused code = %d", rec.Code)
	}

	var view remoteView
	_ = json.Unmarshal(doGET(t, loop, "/api/remote").Body.Bytes(), &view)
	if view.PendingPairing == nil || view.PendingPairing.Name != "Pixel 9" {
		t.Fatalf("pending = %+v", view.PendingPairing)
	}
	// Another tailnet device cannot collect this phone's wait.
	if rec := wait(phone, "100.64.0.3:5000", pid); rec.Code != 400 {
		t.Fatalf("foreign wait = %d", rec.Code)
	}
	// Nothing is authorized before the desktop allows it.
	if rec := loopPost(t, loop, "/api/remote/pairings/"+view.PendingPairing.ID+"/allow", ""); rec.Code != 200 {
		t.Fatalf("allow = %d %s", rec.Code, rec.Body)
	}
	if rec := loopPost(t, loop, "/api/remote/pairings/"+view.PendingPairing.ID+"/decline", ""); rec.Code != http.StatusConflict {
		t.Fatalf("second decision = %d", rec.Code)
	}
	rec := wait(phone, "100.64.0.2:5000", pid)
	cookies := rec.Result().Cookies()
	if rec.Code != 200 || len(cookies) != 1 || cookies[0].Name != remoteDeviceCookie {
		t.Fatalf("wait = %d %s %v", rec.Code, rec.Body, cookies)
	}
	token := cookies[0].Value
	if rec := wait(phone, "100.64.0.2:5000", pid); rec.Code != 400 {
		t.Fatalf("second collection = %d", rec.Code)
	}

	h := httptest.NewRecorder()
	phone.ServeHTTP(h, phoneRequest(http.MethodGet, "/api/health", "", token))
	if h.Code != 200 {
		t.Fatalf("paired health = %d", h.Code)
	}

	// The desktop list never exposes secrets.
	list := doGET(t, loop, "/api/remote/devices").Body.String()
	if strings.Contains(list, token) || strings.Contains(list, hashRemoteToken(token)) || strings.Contains(list, "node-phone") || !strings.Contains(list, "Pixel 9") {
		t.Fatalf("device list = %s", list)
	}

	// Self-service rename, then unpair ends access.
	r := phoneRequest(http.MethodPatch, "/api/remote/self", `{"name":"My phone"}`, token)
	h = httptest.NewRecorder()
	phone.ServeHTTP(h, r)
	if h.Code != 204 || !strings.Contains(doGET(t, loop, "/api/remote/devices").Body.String(), "My phone") {
		t.Fatalf("self rename = %d", h.Code)
	}
	h = httptest.NewRecorder()
	phone.ServeHTTP(h, phoneRequest(http.MethodDelete, "/api/remote/self", "", token))
	if h.Code != 204 {
		t.Fatalf("unpair = %d", h.Code)
	}
	h = httptest.NewRecorder()
	phone.ServeHTTP(h, phoneRequest(http.MethodGet, "/api/health", "", token))
	if h.Code != 401 || errorCode(t, h) != codeRemoteUnpaired {
		t.Fatalf("after unpair = %d", h.Code)
	}
	_ = s
}

func TestRemotePairingDeclineExpiryAndLimits(t *testing.T) {
	s, loop, phone := pairingServer(t)

	code, _ := issueCode(t, loop)
	pid := pendingID(t, claim(phone, "100.64.0.2:5000", code))
	var view remoteView
	_ = json.Unmarshal(doGET(t, loop, "/api/remote").Body.Bytes(), &view)
	if rec := loopPost(t, loop, "/api/remote/pairings/"+view.PendingPairing.ID+"/decline", ""); rec.Code != 200 {
		t.Fatalf("decline = %d", rec.Code)
	}
	if rec := wait(phone, "100.64.0.2:5000", pid); rec.Code != 400 || errorCode(t, rec) != codeRemotePairingInvalid {
		t.Fatalf("declined wait = %d %s", rec.Code, rec.Body)
	}
	if devices, _ := s.stateStore.ListRemoteDevices(); len(devices) != 0 {
		t.Fatalf("decline paired %d devices", len(devices))
	}

	// Expired code.
	code, _ = issueCode(t, loop)
	s.remotePairing.mu.Lock()
	s.remotePairing.code.expires = time.Now().Add(-time.Second)
	s.remotePairing.mu.Unlock()
	if rec := claim(phone, "100.64.0.3:5000", code); rec.Code != 400 {
		t.Fatalf("expired code = %d", rec.Code)
	}

	// Five failures burn the code even when the right one follows.
	code, _ = issueCode(t, loop)
	for i := 0; i < pairCodeMaxFailures; i++ {
		claim(phone, "100.64.0.3:5000", "AAAAAAAA")
	}
	if rec := claim(phone, "100.64.0.3:5000", code); rec.Code != 400 {
		t.Fatalf("code survived %d failures: %d", pairCodeMaxFailures, rec.Code)
	}

	// The failing node is then rate-limited regardless of the code.
	for i := 0; i < pairPeerMaxFailures; i++ {
		claim(phone, "100.64.0.3:5000", "AAAAAAAA")
	}
	code, _ = issueCode(t, loop)
	if rec := claim(phone, "100.64.0.3:5000", code); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit = %d", rec.Code)
	}
	// Another node is unaffected.
	pendingID(t, claim(phone, "100.64.0.2:5000", code))

	// Turning remote off drops outstanding codes and requests.
	putRemote(t, loop, `{"enabled":false}`)
	view = remoteView{}
	_ = json.Unmarshal(doGET(t, loop, "/api/remote").Body.Bytes(), &view)
	if view.PendingPairing != nil {
		t.Fatal("pending pairing survived disable")
	}
}

// Failures from many distinct nodes stay bounded: expired windows are pruned
// and a full table evicts its oldest window (TS-13.R8, INV §16).
func TestRemotePairingFailureTrackingIsBounded(t *testing.T) {
	p := &remotePairing{failures: map[string]*peerFailures{}}
	start := time.Now()
	for i := 0; i < pairPeerMaxTracked+10; i++ {
		p.recordFailureLocked(fmt.Sprintf("node-%d", i), start.Add(time.Duration(i)*time.Millisecond))
	}
	if len(p.failures) != pairPeerMaxTracked {
		t.Fatalf("tracked %d peers, cap %d", len(p.failures), pairPeerMaxTracked)
	}
	if p.failures["node-0"] != nil || p.failures[fmt.Sprintf("node-%d", pairPeerMaxTracked+9)] == nil {
		t.Fatal("eviction did not drop the oldest window")
	}

	p.pruneFailuresLocked(start.Add(pairPeerWindow + time.Hour))
	if len(p.failures) != 0 {
		t.Fatalf("expired windows survived pruning: %d", len(p.failures))
	}
}

// Concurrent claims of one code pair at most one phone (INV §5).
func TestRemotePairingClaimIsAtomic(t *testing.T) {
	_, loop, phone := pairingServer(t)
	code, _ := issueCode(t, loop)
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := claim(phone, "100.64.0.2:5000", code); rec.Code == http.StatusAccepted {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatalf("accepted = %d, want 1", accepted)
	}
}

func TestRemoteRevokeFromDesktop(t *testing.T) {
	s, loop, phone := pairingServer(t)
	token := pairTestDevice(t, s, "d1", "node-phone")
	rec := httptest.NewRecorder()
	loop.ServeHTTP(rec, newLocalRequest(http.MethodPatch, "/api/remote/devices/d1", bytes.NewBufferString(`{"name":"  "}`)))
	if rec.Code != 400 {
		t.Fatalf("blank rename = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	loop.ServeHTTP(rec, newLocalRequest(http.MethodDelete, "/api/remote/devices/d1", nil))
	if rec.Code != 204 {
		t.Fatalf("revoke = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	loop.ServeHTTP(rec, newLocalRequest(http.MethodDelete, "/api/remote/devices/d1", nil))
	if rec.Code != 404 {
		t.Fatalf("second revoke = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	phone.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/health", "", token))
	if rec.Code != 401 {
		t.Fatalf("revoked device = %d", rec.Code)
	}
}

func TestRemoteLogsNeverCarryPairingSecrets(t *testing.T) {
	s, loop, _ := pairingServer(t)
	var buf bytes.Buffer
	var mu sync.Mutex
	s.log = slog.New(slog.NewTextHandler(&lockedWriter{w: &buf, mu: &mu}, nil))
	phone := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "node-phone"}))
	code, _ := issueCode(t, loop)
	pid := pendingID(t, claim(phone, "100.64.0.2:5000", code))
	go func() {
		time.Sleep(20 * time.Millisecond)
		var view remoteView
		_ = json.Unmarshal(doGET(t, loop, "/api/remote").Body.Bytes(), &view)
		loopPost(t, loop, "/api/remote/pairings/"+view.PendingPairing.ID+"/allow", "")
	}()
	rec := wait(phone, "100.64.0.2:5000", pid)
	token := rec.Result().Cookies()[0].Value
	mu.Lock()
	defer mu.Unlock()
	logs := buf.String()
	for _, secret := range []string{code, pid, token, hashRemoteToken(token)} {
		if strings.Contains(logs, secret) {
			t.Fatalf("log carries a pairing secret: %s", logs)
		}
	}
}

type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func TestQRSVGIsSelfContained(t *testing.T) {
	svg := qrSVG("https://" + testDomain + "/pair#ABCD2345")
	if !strings.HasPrefix(svg, "<svg ") || !strings.HasSuffix(svg, "</svg>") || !strings.Contains(svg, `d="M`) || strings.Contains(svg, "ABCD2345") {
		t.Fatalf("qr svg = %.120s…", svg)
	}
}
