package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/server"
	"github.com/agentdeck/agentdeck/internal/state"
)

// stopTimeout bounds how long `stop` waits for graceful SIGTERM exit before
// escalating to SIGKILL.
const stopTimeout = 5 * time.Second

// newDashboardCmd builds the `dashboard` parent command and its subcommands.
func newDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Manage the AgentDeck dashboard server",
	}
	cmd.AddCommand(newDashboardStartCmd(), newDashboardStopCmd(), newDashboardOpenCmd())
	return cmd
}

// newLogger builds the slog JSON logger to stderr, honoring AGENTDECK_LOG_LEVEL.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch os.Getenv("AGENTDECK_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// resolveConfig opens the store, ensures the layout, seeds defaults, and returns
// the effective config (falling back to defaults on corrupt config.json).
func resolveConfig(log *slog.Logger) (*config.Store, config.Config, error) {
	cfgStore, err := config.New()
	if err != nil {
		return nil, config.Config{}, err
	}
	if err := cfgStore.EnsureLayout(); err != nil {
		return nil, config.Config{}, err
	}
	if err := cfgStore.SeedIfAbsent(); err != nil {
		return nil, config.Config{}, err
	}
	// (Re)install the hook scripts so they always match this binary (techspec §4.1).
	if err := hooks.Install(cfgStore.Home()); err != nil {
		log.Warn("install hooks", "err", err)
	}
	cfg, err := cfgStore.ReadConfig()
	if err != nil {
		// Corrupt/missing config → default (do not rewrite the corrupt file).
		log.Warn("config unreadable; using default", "err", err)
		cfg = config.DefaultConfig()
	}
	return cfgStore, cfg, nil
}

func newDashboardStartCmd() *cobra.Command {
	var port int
	var detach bool
	var daemon bool // hidden: set on the re-exec'd child

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the dashboard server",
		RunE: func(_ *cobra.Command, _ []string) error {
			log := newLogger()
			cfgStore, cfg, err := resolveConfig(log)
			if err != nil {
				return err
			}
			if port > 0 {
				cfg.Port = port // per-run override; not persisted
			}

			// Detached parent: re-exec self as a daemon child and exit.
			if detach && !daemon {
				return startDetached(cfgStore.Home(), cfg.Port)
			}

			// Refuse to start if a live instance already holds the pidfile.
			// Skipped for the re-exec'd daemon child: its parent intentionally
			// launched it and pre-wrote the pidfile with this child's own PID,
			// so the liveness check would otherwise match the child itself and
			// make it exit immediately ("already running" against itself).
			if !daemon {
				if info, ok, _ := readPidfile(cfgStore.Home()); ok && processAlive(info.PID) {
					fmt.Printf("already running pid=%d http://127.0.0.1:%d\n", info.PID, info.Port)
					return nil
				}
			}

			stateStore, err := state.Open(cfgStore.Home())
			if err != nil {
				return err
			}
			defer stateStore.Close()

			// Reconcile stale running rows left by a crashed prior run (§8.5).
			if err := runtime.ReconcileStale(stateStore); err != nil {
				log.Warn("reconcile stale sessions", "err", err)
			}
			registry := runtime.NewRegistry(stateStore)

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := writePidfile(cfgStore.Home(), pidInfo{PID: os.Getpid(), Port: cfg.Port}); err != nil {
				return err
			}
			defer removePidfile(cfgStore.Home())

			srv := server.New(cfgStore, stateStore, registry, cfg, log)
			return srv.Start(ctx)
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "override config port for this run")
	cmd.Flags().BoolVar(&detach, "detach", false, "run the server in the background")
	cmd.Flags().BoolVar(&daemon, "__daemon", false, "internal: marks the detached child process")
	_ = cmd.Flags().MarkHidden("__daemon")
	return cmd
}

// startDetached re-execs the current binary as a background daemon, redirecting
// stdio to {home}/dashboard.log, records the child PID in the pidfile, and prints
// a confirmation. The parent then returns (exits 0).
func startDetached(home string, port int) error {
	// Refuse before spawning if a live instance already holds the pidfile. The
	// foreground path checks this after the detach dispatch, so without this the
	// detach parent would overwrite the live server's pidfile with a doomed
	// child's PID; the child then dies on "address already in use" and its
	// `defer removePidfile` deletes the pidfile entirely, leaving stop/open/
	// reindex's sole-writer gate reporting "not running" while the original
	// server still runs (techspec §5.3/§7).
	if info, ok, _ := readPidfile(home); ok && processAlive(info.PID) {
		fmt.Printf("already running pid=%d http://127.0.0.1:%d\n", info.PID, info.Port)
		return nil
	}

	logPath := filepath.Join(home, "dashboard.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	args := []string{"dashboard", "start", "--__daemon", "--port", strconv.Itoa(port)}
	child := exec.Command(os.Args[0], args...)
	child.Stdout = logFile
	child.Stderr = logFile
	child.Stdin = nil
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach from controlling terminal

	if err := child.Start(); err != nil {
		return err
	}
	// Capture the PID before Release(): on Unix, Process.Release() sets Pid to
	// -1, so reading it afterwards would record/print -1.
	pid := child.Process.Pid
	if err := writePidfile(home, pidInfo{PID: pid, Port: port}); err != nil {
		return err
	}
	// Release the child so it keeps running after the parent exits.
	_ = child.Process.Release()

	// Verify the child is still alive before reporting success: if the port was
	// already taken (by a non-agentdeck process, say) the child exits almost
	// immediately and its `defer removePidfile` clears the pidfile, so a "started"
	// message would be a lie. Give it a brief grace window, then confirm.
	time.Sleep(startConfirmGrace)
	if !processAlive(pid) {
		return fmt.Errorf("dashboard failed to start; see %s", logPath)
	}
	fmt.Printf("started pid=%d http://127.0.0.1:%d\n", pid, port)
	return nil
}

// startConfirmGrace is how long the detach parent waits for the child to bind
// the port (or fail) before confirming it actually started.
const startConfirmGrace = 300 * time.Millisecond

func newDashboardStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the dashboard server",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfgStore, err := config.New()
			if err != nil {
				return err
			}
			info, ok, err := readPidfile(cfgStore.Home())
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("not running")
				return nil
			}
			if !processAlive(info.PID) {
				_ = removePidfile(cfgStore.Home())
				fmt.Println("not running (removed stale pidfile)")
				return nil
			}

			// Graceful SIGTERM, then poll, then SIGKILL fallback.
			_ = syscall.Kill(info.PID, syscall.SIGTERM)
			deadline := time.Now().Add(stopTimeout)
			for time.Now().Before(deadline) {
				if !processAlive(info.PID) {
					_ = removePidfile(cfgStore.Home())
					fmt.Printf("stopped pid=%d\n", info.PID)
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
			_ = syscall.Kill(info.PID, syscall.SIGKILL)
			_ = removePidfile(cfgStore.Home())
			fmt.Printf("killed pid=%d (did not exit gracefully)\n", info.PID)
			return nil
		},
	}
}

func newDashboardOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open the dashboard UI in the default browser",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfgStore, err := config.New()
			if err != nil {
				return err
			}
			port := config.DefaultConfig().Port
			if info, ok, _ := readPidfile(cfgStore.Home()); ok && info.Port > 0 {
				port = info.Port
			} else if cfg, err := cfgStore.ReadConfig(); err == nil && cfg.Port > 0 {
				port = cfg.Port
			}
			url := fmt.Sprintf("http://127.0.0.1:%d/", port)
			if err := browser.OpenURL(url); err != nil {
				return fmt.Errorf("open %s: %w", url, err)
			}
			fmt.Printf("opening %s\n", url)
			return nil
		},
	}
}
