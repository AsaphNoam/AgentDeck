package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agentdeck/agentdeck/internal/backend/providerexec"
	"github.com/agentdeck/agentdeck/internal/config"
	rt "github.com/agentdeck/agentdeck/internal/runtime"
)

func writeProviderExe(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func setBackendProvider(t *testing.T, srv *Server, id, mode string, env map[string]string) {
	t.Helper()
	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatal(err)
	}
	be := backends.Backends[id]
	be.ProviderMode = mode
	be.Env = env
	backends.Backends[id] = be
	if err := srv.configStore.WriteBackends(backends); err != nil {
		t.Fatal(err)
	}
}

// FS-09.A37/A45, TS-04.R71/R75: every lifecycle composer — launch, terminal,
// pipeline launch, resume, wake/continuation, switch — pins the adapter (and
// the Claude terminal argv) to the one shared selection, and Bundle mode wins
// over every executable override without deleting it.
func TestProviderSelectionReachesEveryLifecycleComposer(t *testing.T) {
	if providerexec.BundledRelPath("claude", runtime.GOOS, runtime.GOARCH) == "" {
		t.Skip("no bundled layout for this platform")
	}
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	agent, _ := srv.stateStore.ReadAgent(id)
	snap, _ := srv.stateStore.ReadSession(id)
	terminalAgent, terminalSnap := agent, snap
	terminalAgent.Interface, terminalSnap.Interface = "terminal", "terminal"

	root := filepath.Join(t.TempDir(), "runtime")
	bundled := writeProviderExe(t, providerexec.BundledPath(root, "claude"))
	t.Setenv(providerexec.RuntimeRootEnv, root)
	backendExe := writeProviderExe(t, filepath.Join(t.TempDir(), "claude"))

	composers := []struct {
		name    string
		compose func(t *testing.T) (rt.LaunchSpec, *rt.APIError)
	}{
		{"launch", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			spec, _, ae := srv.composeLaunch(t.Context(), launchRequest{Role: "impl", Project: "tmpproj"})
			return spec, ae
		}},
		{"terminal launch", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			spec, _, ae := srv.composeLaunch(t.Context(), launchRequest{Role: "impl", Project: "tmpproj", Interface: "terminal"})
			return spec, ae
		}},
		{"pipeline launch", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			spec, _, ae := srv.composeLaunchWithOptions(t.Context(), launchRequest{Role: "impl", Project: "tmpproj", Interface: "chat"}, launchOptions{Generation: "gen_stage"})
			return spec, ae
		}},
		{"resume", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			be, model := currentBackend(t, srv, agent.Backend, agent.Model)
			return srv.composeResumeSpec(agent, snap, be, model)
		}},
		{"terminal resume", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			be, model := currentBackend(t, srv, agent.Backend, agent.Model)
			return srv.composeResumeSpec(terminalAgent, terminalSnap, be, model)
		}},
		{"wake/continuation", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			be, model := currentBackend(t, srv, agent.Backend, agent.Model)
			return srv.composeResumeSpecWithGeneration(agent, snap, be, model, "gen_continue")
		}},
		{"switch", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			return srv.composeSwitchSpec(agent, "")
		}},
		{"terminal switch", func(t *testing.T) (rt.LaunchSpec, *rt.APIError) {
			return srv.composeSwitchSpec(terminalAgent, "")
		}},
	}
	for _, mode := range []struct{ mode, want string }{{"", backendExe}, {"bundled", bundled}} {
		setBackendProvider(t, srv, agent.Backend, mode.mode, map[string]string{"CLAUDE_CODE_EXECUTABLE": backendExe})
		for _, c := range composers {
			t.Run(c.name+"/"+mode.mode, func(t *testing.T) {
				spec, ae := c.compose(t)
				spec = mustCompose(t, spec, ae)
				if got := envValue(spec.StartEnv(), "CLAUDE_CODE_EXECUTABLE"); got != mode.want {
					t.Errorf("adapter executable = %q, want %q", got, mode.want)
				}
				if spec.ProviderExecutable != mode.want {
					t.Errorf("terminal executable = %q, want %q", spec.ProviderExecutable, mode.want)
				}
			})
		}
	}
	// Bundle mode left the stored override untouched (FS-09.R76).
	be, _ := currentBackend(t, srv, agent.Backend, agent.Model)
	if be.Env["CLAUDE_CODE_EXECUTABLE"] != backendExe {
		t.Fatalf("stored override changed: %v", be.Env)
	}
}

func currentBackend(t *testing.T, srv *Server, backendID, modelID string) (config.Backend, config.Model) {
	t.Helper()
	backends, err := srv.configStore.ReadBackends()
	if err != nil {
		t.Fatal(err)
	}
	be := backends.Backends[backendID]
	return be, be.Models[modelID]
}

// FS-09.A38/A47: a missing provider fails launch before any registration, a
// switch before stopping the working runtime, and a resume without replacing
// the conversation.
func TestMissingProviderFailsBeforeSideEffects(t *testing.T) {
	srv, ts := switchTestServer(t)
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	agent, _ := srv.stateStore.ReadAgent(id)
	before, _ := srv.stateStore.ReadSession(id)
	sessionBefore := runningSessionID(t, srv, id)

	setBackendProvider(t, srv, agent.Backend, "", map[string]string{"CLAUDE_CODE_EXECUTABLE": "/missing/claude"})

	if _, _, ae := srv.composeLaunch(t.Context(), launchRequest{Role: "impl", Project: "tmpproj"}); ae == nil || ae.Code != rt.CodeProviderExecutableMissing || ae.HTTPStatus() != 422 {
		t.Fatalf("launch error = %+v", ae)
	}
	if ae := srv.validateSwitchTarget(agent); ae == nil || ae.Code != rt.CodeProviderExecutableMissing {
		t.Fatalf("switch pre-stop validation = %+v", ae)
	}
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/switch-runtime", map[string]string{"model": agent.Model, "interface": "terminal"})
	if resp.StatusCode != 422 || apiErrorCode(t, body) != rt.CodeProviderExecutableMissing {
		t.Fatalf("switch = %d %s", resp.StatusCode, body)
	}
	if got := runningSessionID(t, srv, id); got != sessionBefore {
		t.Fatalf("working runtime was replaced: %q -> %q", sessionBefore, got)
	}

	setBackendProvider(t, srv, agent.Backend, "bundled", nil)
	t.Setenv(providerexec.RuntimeRootEnv, "")
	be, model := currentBackend(t, srv, agent.Backend, agent.Model)
	if _, ae := srv.composeResumeSpec(agent, before, be, model); ae == nil || ae.Code != rt.CodeBundledProviderMissing || ae.Details["source"] != "bundled" {
		t.Fatalf("resume error = %+v", ae)
	}
	after, _ := srv.stateStore.ReadSession(id)
	if after.LastSessionID != before.LastSessionID {
		t.Fatalf("native identity changed: %q -> %q", before.LastSessionID, after.LastSessionID)
	}
}
