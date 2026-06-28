package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/agentdeck/agentdeck/internal/backend/credcheck"
	"github.com/agentdeck/agentdeck/internal/bus"
	"github.com/agentdeck/agentdeck/internal/config"
	persistindex "github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// onboardingCacheEntry caches a cred-check result for the default backend/model
// to avoid re-probing on every /api/config poll. TTL is ~60s per §3.6.
type onboardingCacheEntry struct {
	result  credcheck.CredResult
	backend string
	model   string
	expires time.Time
}

const onboardingCacheTTL = 60 * time.Second

// shutdownTimeout bounds graceful shutdown after the context is cancelled.
const shutdownTimeout = 5 * time.Second

// Server owns the HTTP lifecycle and its dependencies.
type Server struct {
	configStore *config.Store
	stateStore  *state.Store
	stateMgr    *state.Manager
	eventBus    *bus.Bus
	registry    *runtime.Registry
	cfg         config.Config
	log         *slog.Logger

	hookMu     sync.Mutex
	hookTokens map[string]string // agent_id -> per-launch hook token (Phase 2 persists these)

	// credCheck is the credential probe function; defaults to credcheck.Check.
	// Tests inject a stub so real network/CLI calls are avoided.
	credCheck func(ctx context.Context, bk config.Backend, model config.Model, mergedEnv map[string]string) credcheck.CredResult

	// onboardingCacheMu guards onboardingCache.
	onboardingCacheMu sync.Mutex
	onboardingCache   *onboardingCacheEntry
}

// New constructs a Server. The config supplies the port; the stores back the data
// handlers; the registry drives agent runtimes; the logger is used by middleware.
func New(cfgStore *config.Store, stateStore *state.Store, registry *runtime.Registry, cfg config.Config, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	eventBus := bus.New()
	stateMgr := state.NewManager(stateStore, eventBus)
	if registry != nil {
		registry.SetPersistence(cfgStore.Home(), func(home, agentID string, meta *runtime.SessionMetaData) (runtime.TranscriptWriter, error) {
			return transcript.Open(home, agentID, meta)
		}, persistindex.New(stateStore.DB()))
		registry.SetEventSink(eventBus.PublishRuntimeEvent)
		registry.SetStateTouch(func(agentID string) {
			if _, err := stateMgr.Touch(agentID); err != nil {
				log.Debug("state touch failed", "agent", agentID, "err", err)
			}
		})
	}
	return &Server{
		configStore: cfgStore,
		stateStore:  stateStore,
		stateMgr:    stateMgr,
		eventBus:    eventBus,
		registry:    registry,
		cfg:         cfg,
		log:         log,
		hookTokens:  map[string]string{},
		credCheck:   credcheck.Check,
	}
}

// Start binds 127.0.0.1:{cfg.Port}, asserts the listener is loopback, serves
// until ctx is cancelled, then shuts down gracefully. It blocks until shutdown
// completes or a fatal serve error occurs.
func (s *Server) Start(ctx context.Context) error {
	if err := s.stateMgr.Start(); err != nil {
		return err
	}
	addr, err := LocalAddr(s.cfg.Port)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	// Defense in depth: refuse anything that is not loopback.
	if err := assertLoopback(ln.Addr()); err != nil {
		ln.Close()
		return err
	}

	srv := &http.Server{Handler: s.routes()}
	s.log.Info("dashboard listening", "addr", "http://"+ln.Addr().String())

	serveErr := make(chan error, 1)
	sweepCtx, stopSweep := context.WithCancel(ctx)
	defer stopSweep()
	s.startReconciliationSweep(sweepCtx)

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		s.log.Info("dashboard shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		// Stop every live agent so no orphaned CLI process groups survive (§8.5).
		if s.registry != nil {
			s.registry.Shutdown(shutCtx)
		}
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
