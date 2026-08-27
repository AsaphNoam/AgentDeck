// Command stress-fixture runs the production AgentDeck server and embedded UI
// against a deterministic, high-volume fake ACP workload. It never reads or
// writes the user's AgentDeck home and never invokes a real provider.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/server"
	"github.com/agentdeck/agentdeck/internal/state"
)

type options struct {
	port       int
	workers    int
	chunks     int
	chunkBytes int
	delayMS    int
	repo       string
}

type launchedSession struct {
	Agent struct {
		AgentID string `json:"agent_id"`
		Name    string `json:"name"`
	} `json:"agent"`
}

func main() {
	var opts options
	flag.IntVar(&opts.port, "port", 4399, "loopback port for the isolated dashboard")
	flag.IntVar(&opts.workers, "workers", 6, "number of worker agents (plus one orchestrator)")
	flag.IntVar(&opts.chunks, "chunks", 3000, "streamed assistant deltas per agent")
	flag.IntVar(&opts.chunkBytes, "chunk-bytes", 128, "bytes per assistant delta")
	flag.IntVar(&opts.delayMS, "delay-ms", 5, "delay between assistant deltas")
	flag.StringVar(&opts.repo, "repo", ".", "AgentDeck repository root")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "stress fixture:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.port < 1 || opts.port > 65535 || opts.workers < 1 || opts.workers > 20 ||
		opts.chunks < 1 || opts.chunks > 10000 || opts.chunkBytes < 1 || opts.chunkBytes > 4096 ||
		opts.delayMS < 0 || opts.delayMS > 1000 {
		return errors.New("invalid bounds: workers 1..20, chunks 1..10000, chunk-bytes 1..4096, delay-ms 0..1000")
	}
	repo, err := filepath.Abs(opts.repo)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(repo, "go.mod")); err != nil {
		return fmt.Errorf("repo %s: %w", repo, err)
	}

	home, err := os.MkdirTemp("", "agentdeck-stress-")
	if err != nil {
		return err
	}

	fakeACP := filepath.Join(home, "fakeacp")
	build := exec.Command("go", "build", "-o", fakeACP, "./internal/runtime/testdata/fakeacp")
	build.Dir = repo
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build fake ACP: %w\n%s", err, output)
	}

	store := config.NewWithHome(home)
	if err := store.EnsureLayout(); err != nil {
		return err
	}
	if err := store.SeedIfAbsent(); err != nil {
		return err
	}
	cfg := config.DefaultConfig()
	cfg.Port = opts.port
	cfg.DefaultProject = "stress"
	cfg.DefaultRole = "teammate"
	cfg.OnboardingComplete = true
	if err := store.WriteConfig(cfg); err != nil {
		return err
	}
	if err := store.WriteProject("stress", config.Project{
		Title: "Stress fixture", Color: [3]int{230, 126, 34}, Cwd: repo, AddDirs: []string{},
		ContextPrompt: "Deterministic AgentDeck stress fixture.",
	}); err != nil {
		return err
	}
	backends := config.DefaultBackends()
	claude := backends.Backends["claude"]
	claude.Default = true
	claude.DefaultModel = "haiku"
	claude.AutoSyncModels = false
	claude.Env = map[string]string{
		"FAKEACP_SCENARIO":           "stress_stream",
		"FAKEACP_STRESS_CHUNKS":      strconv.Itoa(opts.chunks),
		"FAKEACP_STRESS_CHUNK_BYTES": strconv.Itoa(opts.chunkBytes),
		"FAKEACP_STRESS_DELAY_MS":    strconv.Itoa(opts.delayMS),
	}
	backends.Backends["claude"] = claude
	for id, backend := range backends.Backends {
		if id != "claude" {
			backend.Default = false
			backends.Backends[id] = backend
		}
	}
	if err := store.WriteBackends(backends); err != nil {
		return err
	}

	stateStore, err := state.Open(home)
	if err != nil {
		return err
	}
	defer stateStore.Close()
	registry := runtime.NewRegistry(stateStore)
	registry.Chat().SetCommand(fakeACP)
	defer registry.Shutdown(context.Background())
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv := server.New(store, stateStore, registry, cfg, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Start(ctx) }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", opts.port)
	if err := waitForHealth(ctx, baseURL, serverDone); err != nil {
		cancel()
		return err
	}
	launched, err := launchWorkload(ctx, baseURL, opts.workers)
	if err != nil {
		cancel()
		return err
	}

	fmt.Printf("AgentDeck stress fixture ready\nURL: %s\nHome: %s\nAgents: %d (Claude Haiku; deterministic fake ACP)\nDeltas: %d x %d bytes per agent, %dms apart\n",
		baseURL, home, len(launched), opts.chunks, opts.chunkBytes, opts.delayMS)
	for _, session := range launched {
		fmt.Printf("- %s (%s)\n", session.Agent.Name, session.Agent.AgentID)
	}
	fmt.Println("Press Ctrl-C to stop.")

	select {
	case err := <-serverDone:
		if err != nil && ctx.Err() == nil {
			return err
		}
	case <-ctx.Done():
		if err := <-serverDone; err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func waitForHealth(ctx context.Context, baseURL string, serverDone <-chan error) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/health", nil)
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-serverDone:
			if err == nil {
				return errors.New("dashboard stopped before becoming healthy")
			}
			return fmt.Errorf("dashboard failed before becoming healthy: %w", err)
		case <-deadline.C:
			return errors.New("dashboard did not become healthy within 10s")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func launchWorkload(ctx context.Context, baseURL string, workers int) ([]launchedSession, error) {
	names := []string{"Stress Orchestrator"}
	for i := 1; i <= workers; i++ {
		names = append(names, fmt.Sprintf("Stress Worker %02d", i))
	}
	launched := make([]launchedSession, 0, len(names))
	for i, name := range names {
		role := "teammate"
		if i == 0 {
			role = "agentdecker"
		}
		var session launchedSession
		if err := postJSON(ctx, baseURL+"/api/sessions", map[string]string{
			"role": role, "project": "stress", "backend": "claude", "model": "haiku",
			"name": name, "group": "stress-fixture",
		}, &session); err != nil {
			return nil, fmt.Errorf("launch %s: %w", name, err)
		}
		launched = append(launched, session)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(launched))
	for _, session := range launched {
		session := session
		wg.Add(1)
		go func() {
			defer wg.Done()
			prompt := "Execute the synthetic stress task and stream the deterministic result."
			if session.Agent.Name == "Stress Orchestrator" {
				prompt = fmt.Sprintf("Orchestrate the %d synthetic workers and stream the deterministic status report.", workers)
			}
			if err := postJSON(ctx, baseURL+"/api/sessions/"+session.Agent.AgentID+"/prompt", map[string]string{"text": prompt}, nil); err != nil {
				errCh <- fmt.Errorf("prompt %s: %w", session.Agent.Name, err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}
	return launched, nil
}

func postJSON(ctx context.Context, url string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode, data)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
