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
	"github.com/agentdeck/agentdeck/internal/configsource"
	"github.com/agentdeck/agentdeck/internal/hooks"
	persistindex "github.com/agentdeck/agentdeck/internal/index"
	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/pipeline"
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

	indexer           *persistindex.Indexer
	messaging         *messaging.Server
	pipelineMgr       *pipeline.Manager
	pipelineTemplates *pipeline.TemplateStore
	sourceMgr         *configsource.Manager
	nudgeCh           chan string

	hookMu      sync.Mutex
	hookTokens  map[string]string // agent_id -> per-launch hook token (Phase 2 persists these)
	mcpCleanups map[string]func()

	// switchMu guards switching: the set of agents with an in-flight
	// switch-runtime (techspec §5.4 per-agent switch lock). A concurrent switch
	// for the same agent → 409 switch_in_progress.
	switchMu  sync.Mutex
	switching map[string]bool

	// lifecycleMu guards lifecycleBusy: the set of agents with an in-flight
	// exclusive lifecycle transition (TS-01.R16). Explicit resume, both wake paths,
	// and Stop take it before any registration side effect, so a losing racer never
	// replaces or tears down the winner's agent-keyed artifacts. Stop belongs here
	// because the registry's in-progress nil sentinel is indistinguishable from
	// "no handle": without the claim, a Stop landing inside a wake tore down the
	// wake's hook token, MCP session, and hook-settings file while the resume ran
	// on to report success (INV §4).
	lifecycleMu   sync.Mutex
	lifecycleBusy map[string]bool

	archiveMu          sync.Mutex
	projectArchiving   map[string]bool
	projectStartLeases map[string]int
	agentArchiving     map[string]bool
	agentStartLeases   map[string]int
	projectTransitions map[string]int
	archiveChanged     chan struct{}

	// credCheck is the credential probe function; defaults to credcheck.Check.
	// Tests inject a stub so real network/CLI calls are avoided.
	credCheck func(ctx context.Context, bk config.Backend, model config.Model, mergedEnv map[string]string) credcheck.CredResult
	// writeProject is the final project-config publication seam. Archive tests
	// inject a failure here to verify exact SQLite compensation.
	writeProject func(string, config.Project) error
	// writeConfig is the global-config publication seam. Config endpoint tests
	// inject a failure here to verify an optimistic appearance choice is not
	// published durably when the atomic rewrite fails.
	writeConfig func(config.Config) error
	// setAgentsArchived is the durable archive-flag seam. Transition tests block
	// it to prove competing idempotent actions still join the exclusive claim.
	setAgentsArchived func([]string, bool) error
	// restoreAgentArchiveStates is the cross-store compensation seam. Tests
	// inject a second failure to verify the durable fallback is published and
	// both causes are reported.
	restoreAgentArchiveStates func(map[string]bool) error

	// primerSummarizer is the one-shot target-backend summary seam for backend
	// switches. The default implementation is gated until live CLI invocation is
	// confirmed; tests inject deterministic behavior.
	primerSummarizer primerSummarizer

	// onboardingCacheMu guards onboardingCache.
	onboardingCacheMu sync.Mutex
	onboardingCache   *onboardingCacheEntry

	// catalogMu serializes every read-modify-write of backends.json: the
	// whole-document PUT, the item-scoped POST create, and an enabled source
	// bind's target-scoped model merge (TS-07.R17/R18). Without it a create and
	// a full save can each write a catalog that erases the other's entry.
	catalogMu sync.Mutex
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
		registry.SetStateTouch(touch)
		// Construct + register the terminal runtime here (it lives in a subpackage
		// that imports runtime, so the Registry can't build it without an import
		// cycle — see Registry.SetTerminalRuntime). Wire its state-touch the same
		// way the chat runtime is wired.
		term = terminal.New(stateStore)
		term.SetStateTouch(touch)
		// Wire durable persistence the same way the chat runtime is wired (above), so
		// a terminal agent gets a sessions row + transcript on Start/Resume and is a
		// first-class archive/resume citizen (Finding 7).
		term.SetPersistence(cfgStore.Home(), func(home, agentID string, meta *runtime.SessionMetaData) (runtime.TranscriptWriter, error) {
			return transcript.Open(home, agentID, meta)
		}, ix)
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
	// When an agent reads/deletes its mail, recompute + publish its state so the
	// unread_messages badge clears immediately (mirrors the send-side touch).
	msg.SetMessagesReadSink(touch)
	// Route budget breaches through the bus so the toast names the agent
	// (agent_name/address) like every other notification type.
	msg.SetBudgetExceededSink(eventBus.PublishBudgetExceeded)
	sourceMgr := newConfigSourceManager(cfgStore, eventBus)
	s := &Server{
		configStore:               cfgStore,
		stateStore:                stateStore,
		stateMgr:                  stateMgr,
		eventBus:                  eventBus,
		registry:                  registry,
		terminal:                  term,
		indexer:                   ix,
		messaging:                 msg,
		sourceMgr:                 sourceMgr,
		nudgeCh:                   nudgeCh,
		cfg:                       cfg,
		log:                       log,
		hookTokens:                map[string]string{},
		mcpCleanups:               map[string]func(){},
		switching:                 map[string]bool{},
		lifecycleBusy:             map[string]bool{},
		projectArchiving:          map[string]bool{},
		projectStartLeases:        map[string]int{},
		agentArchiving:            map[string]bool{},
		agentStartLeases:          map[string]int{},
		projectTransitions:        map[string]int{},
		archiveChanged:            make(chan struct{}),
		credCheck:                 credcheck.Check,
		writeProject:              cfgStore.WriteProject,
		writeConfig:               cfgStore.WriteConfig,
		setAgentsArchived:         stateStore.SetAgentsArchived,
		restoreAgentArchiveStates: stateStore.RestoreAgentArchiveStates,
		primerSummarizer:          defaultPrimerSummarizer,
	}
	s.pipelineTemplates = pipeline.NewTemplateStore(cfgStore)
	s.pipelineMgr = pipeline.NewManager(stateStore, s.pipelineTemplates, s, s)
	msg.SetPipelineManager(s.pipelineMgr)
	// list_agents and send_message resolution both answer from the dashboard's
	// addressable set, which adds the stopped chat agents a message can wake
	// (FS-06.R22) to the running registry.
	msg.SetAddressableAgents(s.addressableAgents)
	if registry != nil {
		registry.SetEventSink(func(ev runtime.Event) {
			eventBus.PublishRuntimeEvent(ev)
			if ev.Type == runtime.EvTurnEnd && s.pipelineMgr != nil {
				generation := registry.Generation(ev.AgentID)
				go func() {
					if err := s.pipelineMgr.OnTurnEnd(ev.AgentID, generation); err != nil {
						s.log.Warn("pipeline turn boundary", "agent_id", ev.AgentID, "err", err)
					}
				}()
			}
		})
	}
	// Tear down per-agent registration artifacts on the runtime crash path too,
	// not only on solicited stop/switch — otherwise a crashed agent leaves a live
	// hook token + MCP session (a spoofable messaging identity) and leaked files.
	if registry != nil {
		registry.SetExitHook(func(agentID, generation, cause string) {
			s.teardownAgentRegistration(agentID)
			if s.pipelineMgr != nil {
				go func() {
					if err := s.pipelineMgr.OnExit(agentID, generation, cause); err != nil {
						s.log.Warn("pipeline agent exit", "agent_id", agentID, "err", err)
					}
				}()
			}
		})
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

	// Derive every request context from a cancelable base so shutdown can actively
	// end long-lived streaming handlers. http.Server.Shutdown waits for in-flight
	// requests to return but never cancels their contexts, and the /api/events SSE
	// handler blocks until its request context is Done — so without this, a single
	// open dashboard tab kept Shutdown blocked for the full timeout, then the CLI
	// fell back to an ungraceful kill. Cancelling baseCtx at shutdown unblocks the
	// SSE select (`<-ctx.Done()`) so the stream returns and Shutdown completes.
	baseCtx, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()
	srv := &http.Server{
		Handler:     s.routes(),
		BaseContext: func(net.Listener) context.Context { return baseCtx },
	}
	s.log.Info("dashboard listening", "addr", "http://"+ln.Addr().String())

	serveErr := make(chan error, 1)
	sweepCtx, stopSweep := context.WithCancel(ctx)
	defer stopSweep()
	s.startReconciliationSweep(sweepCtx)
	s.startMessagingLoops(sweepCtx)
	if s.sourceMgr != nil {
		// Hydrate persisted bindings so the watcher detects external edits (invariant §1).
		projects, _ := s.configStore.ListProjects()
		s.sourceMgr.HydrateBindings(ctx, projects)
		go s.sourceMgr.Watch(sweepCtx)
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()
	if s.pipelineMgr != nil {
		if err := s.pipelineMgr.Startup(sweepCtx); err != nil {
			cancelBase()
			_ = srv.Shutdown(context.Background())
			return fmt.Errorf("pipeline startup recovery: %w", err)
		}
	}

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
		// End open streaming handlers (SSE) so Shutdown doesn't block on them.
		cancelBase()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
