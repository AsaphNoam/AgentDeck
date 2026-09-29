package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/agentdeck/agentdeck/internal/remote"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"rsc.io/qr"
)

// Pairing is a desktop-issued, single-use code claimed atomically; codes and
// pending requests live only in memory (TS-13.R8, TS-02.R37).
const (
	pairCodeLength      = 8
	pairCodeAlphabet    = "23456789ABCDEFGHJKMNPQRSTUVWXYZ" // no 0/O, 1/I/L
	pairCodeTTL         = 5 * time.Minute
	pairCodeMaxFailures = 5
	pairPeerMaxFailures = 10
	pairPeerWindow      = 5 * time.Minute
	pairPeerMaxTracked  = 256 // failing tailnet nodes remembered at once (INV §16)
	pairWaitTimeout     = 20 * time.Second
	remoteNameMax       = 64

	codeRemotePairingInvalid = "remote_pairing_invalid"
	codeRemoteRateLimited    = "remote_rate_limited"
	codeRemoteNotOn          = "remote_not_on"
)

type pairCode struct {
	id       string
	code     string
	expires  time.Time
	failures int
}

// pendingPairing is one claimed code awaiting the desktop's answer.
type pendingPairing struct {
	id        string // desktop-facing
	waitToken string // the phone's bearer for its wait
	name      string
	peer      remote.Peer
	requested time.Time
	expires   time.Time
	state     string // waiting | allowed | declined
	token     string // device credential, held only until the phone collects it
	done      chan struct{}
}

type peerFailures struct {
	count int
	since time.Time
}

type remotePairing struct {
	mu       sync.Mutex
	code     *pairCode
	pending  *pendingPairing
	failures map[string]*peerFailures
}

// pendingPairingView is what the desktop sees of a pending request.
type pendingPairingView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Login       string    `json:"login,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
}

// remoteDeviceView is a device as the desktop lists it — never token hashes,
// node ids, or push endpoints (TS-13.R9).
type remoteDeviceView struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	PairedAt      time.Time `json:"paired_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	Notifications string    `json:"notifications"` // on | off | expired
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomPairCode() string {
	b := make([]byte, pairCodeLength)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = pairCodeAlphabet[int(b[i])%len(pairCodeAlphabet)]
	}
	return string(b)
}

// cleanRemoteName trims a phone name and rejects empty, overlong, or
// control-character names (INV §8).
func cleanRemoteName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > remoteNameMax {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return name, true
}

// pendingView reports the waiting request, dropping it once expired.
func (p *remotePairing) pendingView(now time.Time) *pendingPairingView {
	p.mu.Lock()
	defer p.mu.Unlock()
	pp := p.pending
	if pp == nil || pp.state != "waiting" {
		return nil
	}
	if now.After(pp.expires) {
		p.expireLocked()
		return nil
	}
	return &pendingPairingView{ID: pp.id, Name: pp.name, Login: pp.peer.Login, RequestedAt: pp.requested}
}

func (p *remotePairing) expireLocked() {
	if p.pending != nil {
		if p.pending.state == "waiting" {
			close(p.pending.done)
		}
		p.pending = nil
	}
}

// reset drops every code and pending request (remote control turned off).
func (p *remotePairing) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.code = nil
	p.expireLocked()
}

// remoteDevicesView lists paired phones for the desktop.
func (s *Server) remoteDevicesView() []remoteDeviceView {
	devices, err := s.stateStore.ListRemoteDevices()
	if err != nil {
		s.log.Warn("remote: list devices", "err", err)
		return []remoteDeviceView{}
	}
	out := make([]remoteDeviceView, 0, len(devices))
	for _, d := range devices {
		n := "off"
		switch {
		case d.PushEndpoint != "" && d.PushState == "expired":
			n = "expired"
		case d.PushEndpoint != "" && d.PushEnabled:
			n = "on"
		}
		out = append(out, remoteDeviceView{ID: d.ID, Name: d.Name, PairedAt: d.PairedAt, LastSeenAt: d.LastSeenAt, Notifications: n})
	}
	return out
}

func (s *Server) publishRemote() {
	s.eventBus.Publish("remote_update", nil, s.remoteViewFor(s.remote.Status()))
}

// handleCreatePairing implements loopback-only POST /api/remote/pairings. A new
// code replaces any outstanding one.
func (s *Server) handleCreatePairing(w http.ResponseWriter, _ *http.Request) {
	st := s.remote.Status()
	if st.State != remote.StateOn {
		writeRemoteError(w, http.StatusConflict, codeRemoteNotOn, "turn remote control on before pairing a phone")
		return
	}
	pc := &pairCode{id: randomToken(9), code: randomPairCode(), expires: time.Now().Add(pairCodeTTL)}
	s.remotePairing.mu.Lock()
	s.remotePairing.code = pc
	s.remotePairing.mu.Unlock()
	// The code rides in the fragment so it never reaches request logs.
	qrURL := st.Address + "/pair#" + pc.code
	writeJSON(w, http.StatusOK, map[string]any{
		"id": pc.id, "code": pc.code, "expires_at": pc.expires.UTC(),
		"qr_url": qrURL, "qr_svg": qrSVG(qrURL),
	})
}

// qrSVG renders text as a self-contained SVG QR code with a quiet zone, so the
// desktop needs no QR library. It returns "" if encoding fails.
func qrSVG(text string) string {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return ""
	}
	const quiet = 4
	size := code.Size + 2*quiet
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, size, size)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, size, size)
	for y := 0; y < code.Size; y++ {
		for x := 0; x < code.Size; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

// pruneFailuresLocked drops every per-peer failure window that has expired.
func (p *remotePairing) pruneFailuresLocked(now time.Time) {
	for id, f := range p.failures {
		if now.Sub(f.since) > pairPeerWindow {
			delete(p.failures, id)
		}
	}
}

// recordFailureLocked counts a failed claim for peer. Bounded: at most
// pairPeerMaxTracked nodes, evicting the oldest window when full (INV §16).
func (p *remotePairing) recordFailureLocked(peer string, now time.Time) {
	f := p.failures[peer]
	if f == nil {
		if len(p.failures) >= pairPeerMaxTracked {
			oldest := ""
			for id, o := range p.failures {
				if oldest == "" || o.since.Before(p.failures[oldest].since) {
					oldest = id
				}
			}
			delete(p.failures, oldest)
		}
		f = &peerFailures{since: now}
		p.failures[peer] = f
	}
	f.count++
}

type pairClaimBody struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// handlePairClaim implements tailnet-only, unauthenticated POST
// /api/remote/pair. A match claims the code atomically (INV §5).
func (s *Server) handlePairClaim(w http.ResponseWriter, r *http.Request) {
	rr := remoteFrom(r.Context())
	var body pairClaimBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeRemoteError(w, http.StatusBadRequest, codeRemotePairingInvalid, "start again from AgentDeck on your Mac")
		return
	}
	name, ok := cleanRemoteName(body.Name)
	if !ok {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "enter a name for this phone (up to 64 characters)"))
		return
	}
	code := strings.ToUpper(strings.TrimSpace(body.Code))
	now := time.Now()
	p := s.remotePairing
	p.mu.Lock()
	p.pruneFailuresLocked(now)
	if f := p.failures[rr.peer.StableID]; f != nil && f.count >= pairPeerMaxFailures {
		p.mu.Unlock()
		writeRemoteError(w, http.StatusTooManyRequests, codeRemoteRateLimited, "too many attempts; wait a few minutes")
		return
	}
	pc := p.code
	if pc == nil || now.After(pc.expires) || subtle.ConstantTimeCompare([]byte(code), []byte(pc.code)) != 1 {
		if pc != nil {
			pc.failures++
			if pc.failures >= pairCodeMaxFailures || now.After(pc.expires) {
				p.code = nil
			}
		}
		p.recordFailureLocked(rr.peer.StableID, now)
		p.mu.Unlock()
		writeRemoteError(w, http.StatusBadRequest, codeRemotePairingInvalid, "start again from AgentDeck on your Mac")
		return
	}
	p.code = nil // single use
	if p.pending != nil && p.pending.state == "waiting" {
		p.pending.state = "declined"
		close(p.pending.done)
	}
	pp := &pendingPairing{
		id: randomToken(9), waitToken: randomToken(24), name: name, peer: rr.peer,
		requested: now.UTC(), expires: now.Add(pairCodeTTL), state: "waiting", done: make(chan struct{}),
	}
	p.pending = pp
	p.mu.Unlock()
	s.publishRemote()
	writeJSON(w, http.StatusAccepted, map[string]any{"pending_id": pp.waitToken})
}

// handlePairDecision implements loopback-only POST
// /api/remote/pairings/{id}/allow|decline.
func (s *Server) handlePairDecision(allow bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		p := s.remotePairing
		p.mu.Lock()
		pp := p.pending
		if pp == nil || pp.id != id || pp.state != "waiting" || time.Now().After(pp.expires) {
			p.mu.Unlock()
			writeRemoteError(w, http.StatusConflict, codeRemotePairingInvalid, "this pairing request is no longer waiting")
			return
		}
		if !allow {
			pp.state = "declined"
			close(pp.done)
			p.mu.Unlock()
			s.publishRemote()
			writeJSON(w, http.StatusOK, map[string]any{"declined": true})
			return
		}
		token := randomToken(32)
		device := state.RemoteDevice{
			ID: "rd_" + randomToken(9), Name: pp.name, TokenHash: hashRemoteToken(token),
			NodeStableID: pp.peer.StableID, NodeLogin: pp.peer.Login,
		}
		// The device row commits before the phone can collect its cookie
		// (INV §15). Holding the lock keeps a concurrent decision out.
		if err := s.stateStore.InsertRemoteDevice(device); err != nil {
			p.mu.Unlock()
			s.log.Error("remote: insert device", "err", err)
			writeAPIError(w, apiError(runtime.CodeInternal, "internal error"))
			return
		}
		pp.state, pp.token = "allowed", token
		close(pp.done)
		p.mu.Unlock()
		s.publishRemote()
		writeJSON(w, http.StatusOK, map[string]any{"device_id": device.ID})
	}
}

// handlePairWait implements tailnet-only GET /api/remote/pair/{pending_id}: it
// waits briefly for the desktop's answer and, once allowed, sets the device
// cookie exactly once.
func (s *Server) handlePairWait(w http.ResponseWriter, r *http.Request) {
	rr := remoteFrom(r.Context())
	waitToken := r.PathValue("pending_id")
	p := s.remotePairing
	p.mu.Lock()
	pp := p.pending
	if pp == nil || subtle.ConstantTimeCompare([]byte(pp.waitToken), []byte(waitToken)) != 1 || pp.peer.StableID != rr.peer.StableID {
		p.mu.Unlock()
		writeRemoteError(w, http.StatusBadRequest, codeRemotePairingInvalid, "start again from AgentDeck on your Mac")
		return
	}
	done := pp.done
	p.mu.Unlock()

	timer := time.NewTimer(pairWaitTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	case <-r.Context().Done():
		return
	}

	p.mu.Lock()
	if p.pending != pp {
		p.mu.Unlock()
		writeRemoteError(w, http.StatusBadRequest, codeRemotePairingInvalid, "start again from AgentDeck on your Mac")
		return
	}
	switch {
	case pp.state == "allowed":
		token := pp.token
		p.pending = nil // collected once
		p.mu.Unlock()
		setRemoteDeviceCookie(w, token)
		writeJSON(w, http.StatusOK, map[string]any{"status": "allowed"})
	case pp.state == "waiting" && time.Now().Before(pp.expires):
		p.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"status": "waiting"})
	default:
		p.expireLocked()
		p.mu.Unlock()
		writeRemoteError(w, http.StatusBadRequest, codeRemotePairingInvalid, "start again from AgentDeck on your Mac")
	}
}

// handleListRemoteDevices implements loopback-only GET /api/remote/devices.
func (s *Server) handleListRemoteDevices(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"devices": s.remoteDevicesView()})
}

type remoteRenameBody struct {
	Name string `json:"name"`
}

func (s *Server) renameRemoteDevice(w http.ResponseWriter, r *http.Request, id string) bool {
	var body remoteRenameBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "malformed JSON"))
		return false
	}
	name, ok := cleanRemoteName(body.Name)
	if !ok {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "enter a name for this phone (up to 64 characters)"))
		return false
	}
	if err := s.stateStore.RenameRemoteDevice(id, name); err != nil {
		s.writeRemoteDeviceError(w, err)
		return false
	}
	s.publishRemote()
	return true
}

// removeRemoteDevice deletes the row first, then ends the device's open
// requests and streams (TS-13.R9, INV §4).
func (s *Server) removeRemoteDevice(w http.ResponseWriter, id string) bool {
	if err := s.stateStore.DeleteRemoteDevice(id); err != nil {
		s.writeRemoteDeviceError(w, err)
		return false
	}
	s.remoteDevices.end(id)
	s.publishRemote()
	return true
}

func (s *Server) writeRemoteDeviceError(w http.ResponseWriter, err error) {
	if errors.Is(err, state.ErrNotFound) {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such paired phone"))
		return
	}
	s.log.Error("remote: device update", "err", err)
	writeAPIError(w, apiError(runtime.CodeInternal, "internal error"))
}

// handleRenameRemoteDevice implements loopback-only PATCH /api/remote/devices/{id}.
func (s *Server) handleRenameRemoteDevice(w http.ResponseWriter, r *http.Request) {
	if s.renameRemoteDevice(w, r, r.PathValue("id")) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleRevokeRemoteDevice implements loopback-only DELETE /api/remote/devices/{id}.
func (s *Server) handleRevokeRemoteDevice(w http.ResponseWriter, r *http.Request) {
	if s.removeRemoteDevice(w, r.PathValue("id")) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleRenameSelf implements tailnet-only PATCH /api/remote/self.
func (s *Server) handleRenameSelf(w http.ResponseWriter, r *http.Request) {
	if s.renameRemoteDevice(w, r, remoteFrom(r.Context()).device.ID) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleUnpairSelf implements tailnet-only DELETE /api/remote/self.
func (s *Server) handleUnpairSelf(w http.ResponseWriter, r *http.Request) {
	id := remoteFrom(r.Context()).device.ID
	if err := s.stateStore.DeleteRemoteDevice(id); err != nil {
		s.writeRemoteDeviceError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: remoteDeviceCookie, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
	// End the device's other streams after this response is written.
	s.remoteDevices.end(id)
	s.publishRemote()
}
