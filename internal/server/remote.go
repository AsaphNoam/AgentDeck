package server

import (
	"encoding/json"
	"errors"
	"net/http"
	goruntime "runtime"
	"time"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/remote"
	"github.com/AsaphNoam/Chuck/internal/runtime"
)

// remoteHostname is the tailnet device name; the product name governs it
// (FS-20.R1, FS-00.R16).
const remoteHostname = "chuck"

// devRemoteNodeFactory is set only by the dev-tagged remote_dev.go, to drive
// the phone app locally without a tailnet.
var devRemoteNodeFactory func() (remote.Node, error)

// remoteView is the GET /api/remote and remote_update shape (TS-13 §3).
type remoteView struct {
	remote.Status
	KeepAwake          bool                `json:"keep_awake"`
	KeepAwakeAvailable bool                `json:"keep_awake_available"`
	PendingPairing     *pendingPairingView `json:"pending_pairing,omitempty"`
	Devices            []remoteDeviceView  `json:"devices"`
}

// newRemoteManager wires the remote subsystem to the server. The node factory
// and serve hook go through Server fields so tests can inject fakes.
func (s *Server) newRemoteManager() *remote.Manager {
	return remote.NewManager(
		func() (remote.Node, error) { return s.newRemoteNode() },
		s.serveRemote,
		func(st remote.Status) { s.eventBus.Publish("remote_update", nil, s.remoteViewFor(st)) },
		s.log,
	)
}

func (s *Server) remoteViewFor(st remote.Status) remoteView {
	keepAwake := false
	if cfg, err := s.configStore.ReadConfig(); err == nil {
		keepAwake = cfg.KeepAwake
	}
	return remoteView{
		Status: st, KeepAwake: keepAwake, KeepAwakeAvailable: goruntime.GOOS == "darwin",
		PendingPairing: s.remotePairing.pendingView(time.Now()), Devices: s.remoteDevicesView(),
	}
}

// handleGetRemote implements loopback-only GET /api/remote.
func (s *Server) handleGetRemote(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.remoteViewFor(s.remote.Status()))
}

type remotePutBody struct {
	Enabled   *bool `json:"enabled"`
	KeepAwake *bool `json:"keep_awake"`
}

// handlePutRemote implements loopback-only PUT /api/remote. The preference is
// written before the node is started or stopped, and both happen under the
// config lock so concurrent toggles cannot leave the file and the node
// disagreeing (INV §5, §15).
func (s *Server) handlePutRemote(w http.ResponseWriter, r *http.Request) {
	var body remotePutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "malformed JSON"))
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := s.configStore.ReadConfig()
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) && !errors.Is(err, config.ErrCorrupt) {
			s.log.Error("remote: read config", "err", err)
			writeAPIError(w, apiError(runtime.CodeInternal, "internal error"))
			return
		}
		cfg = config.DefaultConfig()
	}
	if body.Enabled != nil {
		cfg.RemoteEnabled = *body.Enabled
	}
	if body.KeepAwake != nil {
		cfg.KeepAwake = *body.KeepAwake
	}
	if err := s.writeConfig(cfg); err != nil {
		s.log.Error("remote: write config", "err", err)
		writeAPIError(w, apiError(runtime.CodeInternal, "internal error"))
		return
	}
	if body.Enabled != nil {
		if cfg.RemoteEnabled {
			s.remote.Enable()
		} else {
			s.remote.Disable()
			s.remotePairing.reset()
		}
	}
	if body.KeepAwake != nil {
		s.nudgeKeepAwake()
	}
	// Preference changes move no node state, so the manager may not publish.
	view := s.remoteViewFor(s.remote.Status())
	s.eventBus.Publish("remote_update", nil, view)
	writeJSON(w, http.StatusOK, view)
}
