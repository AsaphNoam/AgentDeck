package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

func taskChatServer(t *testing.T, env map[string]string) (*Server, *httptest.Server, string) {
	t.Helper()
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "task_flow")
	for k, v := range env {
		t.Setenv(k, v)
	}
	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "serve"}); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt status = %d: %s", resp.StatusCode, body)
	}
	waitStatus(t, srv, id, "idle")
	return srv, ts, id
}

// TS-03.R44: 202 only after the runtime accepts; stale → 404; no negotiated
// control → 422 background_task_control_unavailable; refusal → retryable 409.
func TestBackgroundTaskStopRoute(t *testing.T) {
	srv, ts, id := taskChatServer(t, map[string]string{"FAKEACP_CAPS": "1"})
	update, err := srv.stateMgr.Touch(id)
	if err != nil || !update.RuntimeCapabilities.BackgroundTaskStop {
		t.Fatalf("runtime_capabilities = %+v err %v", update.RuntimeCapabilities, err)
	}
	stop := func(body map[string]string) (int, string) {
		resp, raw := post(t, ts.URL+"/api/sessions/"+id+"/background-task-stop", body)
		return resp.StatusCode, string(raw)
	}
	if code, body := stop(map[string]string{}); code != http.StatusUnprocessableEntity {
		t.Fatalf("missing task_id = %d %s", code, body)
	}
	if code, body := stop(map[string]string{"task_id": "task_1"}); code != http.StatusAccepted {
		t.Fatalf("stop = %d %s", code, body)
	}
	if code, body := stop(map[string]string{"task_id": "task_1"}); code != http.StatusNotFound {
		t.Fatalf("stale stop = %d %s", code, body)
	}
}

func TestBackgroundTaskStopRouteWithoutControl(t *testing.T) {
	_, ts, id := taskChatServer(t, nil)
	resp, body := post(t, ts.URL+"/api/sessions/"+id+"/background-task-stop", map[string]string{"task_id": "task_1"})
	if resp.StatusCode != http.StatusUnprocessableEntity || apiErrorCode(t, body) != "background_task_control_unavailable" {
		t.Fatalf("stop without control = %d %s", resp.StatusCode, body)
	}
}

func TestBackgroundTaskStopRouteRefusalIsRetryable(t *testing.T) {
	_, ts, id := taskChatServer(t, map[string]string{"FAKEACP_CAPS": "1", "FAKEACP_TASK_STOP": "refuse"})
	for i := 0; i < 2; i++ {
		resp, body := post(t, ts.URL+"/api/sessions/"+id+"/background-task-stop", map[string]string{"task_id": "task_1"})
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("refused stop %d = %d %s", i, resp.StatusCode, body)
		}
	}
}
