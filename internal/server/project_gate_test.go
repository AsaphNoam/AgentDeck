package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// corruptProject overwrites the durable project definition with unparseable JSON
// so ReadProject returns config.ErrCorrupt (an unexpected, non-absent read
// failure) on every subsequent lifecycle read.
func corruptProject(t *testing.T, srv *Server, id string) {
	t.Helper()
	path := filepath.Join(srv.configStore.Home(), "projects", id+".json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatalf("corrupt project %s: %v", id, err)
	}
	if _, err := srv.configStore.ReadProject(id); !errors.Is(err, config.ErrCorrupt) {
		t.Fatalf("ReadProject(%s) after corruption = %v, want ErrCorrupt", id, err)
	}
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return env.Error.Code
}

// FS-05.R34 / TS-03.R20 / INV §7 (also §5/§8): an unexpected project-read error
// must fail closed on Resume, Switch runtime, agent Restore, and pipeline start,
// rather than being treated like an active project. A corrupt definition may
// itself record archived: true, so lifecycle work must not proceed while the
// archive state is unknown.
func TestUnreadableProjectFailsClosedOnLifecyclePaths(t *testing.T) {
	t.Run("resume", func(t *testing.T) {
		srv := testServer(t, true)
		if err := srv.stateStore.WriteAgent(state.Agent{AgentID: "a_resume", Name: "Resume", Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
		corruptProject(t, srv, "my-app")
		rec := httptest.NewRecorder()
		srv.routes().ServeHTTP(rec, newLocalRequest(http.MethodPost, "/api/sessions/a_resume/resume", nil))
		if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != runtime.CodeInternal {
			t.Fatalf("resume with corrupt project = %d %q, want 500 internal", rec.Code, rec.Body.String())
		}
	})

	t.Run("switch", func(t *testing.T) {
		srv := testServer(t, true)
		if err := srv.stateStore.WriteAgent(state.Agent{AgentID: "a_switch", Name: "Switch", Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
		corruptProject(t, srv, "my-app")
		rec := httptest.NewRecorder()
		srv.routes().ServeHTTP(rec, newLocalRequest(http.MethodPost, "/api/sessions/a_switch/switch-runtime", nil))
		if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != runtime.CodeInternal {
			t.Fatalf("switch with corrupt project = %d %q, want 500 internal", rec.Code, rec.Body.String())
		}
	})

	t.Run("restore", func(t *testing.T) {
		srv := testServer(t, true)
		if err := srv.stateStore.WriteAgent(state.Agent{AgentID: "a_restore", Name: "Restore", Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC(), Archived: true}); err != nil {
			t.Fatal(err)
		}
		corruptProject(t, srv, "my-app")
		rec := httptest.NewRecorder()
		srv.routes().ServeHTTP(rec, newLocalRequest(http.MethodPost, "/api/sessions/a_restore/restore", nil))
		if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != runtime.CodeInternal {
			t.Fatalf("restore with corrupt project = %d %q, want 500 internal", rec.Code, rec.Body.String())
		}
		got, err := srv.stateStore.ReadAgent("a_restore")
		if err != nil || !got.Archived {
			t.Fatalf("agent after failed restore = %+v err=%v, want still archived", got, err)
		}
	})

	t.Run("pipeline start", func(t *testing.T) {
		srv := testServer(t, true)
		corruptProject(t, srv, "my-app")
		release, err := srv.AcquirePipelineStart(context.Background(), "my-app")
		var gate *pipeline.ProjectGateError
		if !errors.As(err, &gate) || gate.Code != runtime.CodeInternal {
			t.Fatalf("pipeline start with corrupt project = %v, want internal gate error", err)
		}
		if release != nil {
			t.Fatal("pipeline start returned a release func on failure")
		}
		// The start lease must be released so a later start can proceed.
		if ae := srv.acquireProjectStart("my-app"); ae != nil {
			t.Fatalf("project start lease still held after failed acquire: %v", ae)
		}
		srv.releaseProjectStart("my-app")
	})
}
