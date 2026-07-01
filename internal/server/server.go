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
	"github.com/agentdeck/agentdeck/internal/hooks"
	persistindex "github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/runtime/terminal"
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
	terminal    *terminal.Runtime
	cfg         config.Config
	log         *slog.Logger

	indexer   *persistindex.Indexer
	messaging *messaging.Server
	nudgeCh   chan string

	hookMu      sync.Mutex
	hookTokens  map[string]string // agent_id -> per-launch hook token (Phase 2 persists these)
	mcpCleanups map[string]func()

	// switchMu guards switching: the set of agents with an in-flight
	// switch-runtime (techspec §5.4 per-agent switch lock). A concurrent switch
	// for the same agent → 409 switch_in_progress.
	switchMu  sync.Mutex
	switching map[string]bool

	// credCheck is the credential probe function; defaults to credcheck.Check.
	// Tests inject a stub so real network/CLI calls are avoided.
	credCheck func(ctx context.Context, bk config.Backend, model config.Model, mergedEnv map[string]string) credcheck.CredResult

	// primerSummarizer is the one-shot target-backend summary seam for backend
	// switches. The default implementation is gated until live CLI invocation is
	// confirmed; tests inject deterministic behavior.
	primerSummarizer primerSummarizer

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
	ix := persistindex.New(stateStore.DB())
	msg := messaging.New(stateStore, log)
	nudgeCh := make(chan string, 32)
	touch := func(agentID string) {
		if _, err := stateMgr.Touch(agentID); err != nil {
			log.Debug("state touch failed", "agent", agentID, "err", err)
		}
	}
	var term *terminal.Runtime
	if registry != nil {
		registry.SetPersistence(cfgStore.Home(), func(home, agentID string, meta *runtime.SessionMetaData) (runtime.TranscriptWriter, error) {
			return transcript.Open(home, agentID, meta)
		}, ix)
		registry.SetEventSink(eventBus.PublishRuntimeEvent)
		registry.SetStateTouch(touch)
		// Construct + register the terminal runtime here (it lives in a subpackage
		// that imports runtime, so the Registry can't build it without an import
		// cycle — see Registry.SetTerminalRuntime). Wire its state-touch the same
		// way the chat runtime is wired.
		term = terminal.New(stateStore)
		term.SetStateTouch(touch)
		registry.SetTerminalRuntime(term)
	}
	msg.SetMessageInsertedSink(func(fromAgentID, toAgentID string) {
		select {
		case nudgeCh <- toAgentID:
		default:
		}
		// Touch publishes a state_update (via the manager's bus publisher) for
		// both ends, carrying the recipient's recomputed unread_messages badge.
		if _, err := stateMgr.Touch(toAgentID); err != nil {
			log.Debug("touch recipient failed", "agent", toAgentID, "err", err)
		}
		if update, err := stateMgr.Touch(fromAgentID); err == nil {
			// Re-publish the sender with the outbound-pulse timestamp, which
			// recompute does not persist.
			update.LastSentAt = time.Now().UTC().Format(time.RFC3339)
			eventBus.PublishStateUpdate(update)
		}
	})
	// Route budget breaches through the bus so the toast names the agent
	// (agent_name/address) like every other notification type.
	msg.SetBudgetExceededSink(eventBus.PublishBudgetExceeded)
	s := &Server{
		configStore:      cfgStore,
		stateStore:       stateStore,
		stateMgr:         stateMgr,
		eventBus:         eventBus,
		registry:         registry,
		terminal:         term,
		indexer:          ix,
		messaging:        msg,
		nudgeCh:          nudgeCh,
		cfg:              cfg,
		log:              log,
		hookTokens:       map[string]string{},
		mcpCleanups:      map[string]func(){},
		switching:        map[string]bool{},
		credCheck:        credcheck.Check,
		primerSummarizer: defaultPrimerSummarizer,
	}
	// Tear down per-agent registration artifacts on the runtime crash path too,
	// not only on solicited stop/switch — otherwise a crashed agent leaves a live
	// hook token + MCP session (a spoofable messaging identity) and leaked files.
	if registry != nil {
		registry.SetExitHook(s.teardownAgentRegistration)
	}
	return s
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
	s.startMessagingLoops(sweepCtx)

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
		s.cleanupAllMessagingMCP()
		if err := hooks.RemoveAllAgentSettings(s.configStore.Home()); err != nil {
			s.log.Warn("cleanup hook settings dir", "err", err)
		}
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
