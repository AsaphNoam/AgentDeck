package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/pipeline"
	"github.com/AsaphNoam/Chuck/internal/remote"
	"github.com/AsaphNoam/Chuck/internal/state"
)

const testDomain = "chuck.tail1.ts.net"

// Every loopback route is either allowlisted or explicitly denied on the
// tailnet listener, and the allowlist names no route that does not exist
// (TS-13.R5, INV §10).
func TestRemoteRouteInventoryIsClassified(t *testing.T) {
	s := testServer(t, true)
	seen := map[string]bool{}
	for _, e := range s.routeTable() {
		seen[e.pattern] = true
		_, allowed := remoteAllowed[e.pattern]
		if allowed == remoteDenied[e.pattern] {
			t.Errorf("route %q must be exactly one of allowlisted or denied (allowed=%v)", e.pattern, allowed)
		}
	}
	for p := range remoteAllowed {
		if !seen[p] {
			t.Errorf("allowlisted route %q is not in the loopback inventory", p)
		}
	}
	for p := range remoteDenied {
		if !seen[p] {
			t.Errorf("denied route %q is not in the loopback inventory", p)
		}
	}
}

func testWhoIs(peers map[string]string) func(context.Context, string) (remote.Peer, error) {
	return func(_ context.Context, addr string) (remote.Peer, error) {
		if id, ok := peers[addr]; ok {
			return remote.Peer{StableID: id, Login: "me@example.com"}, nil
		}
		return remote.Peer{}, errNoPeer
	}
}

// pairTestDevice stores a paired device bound to nodeID and returns its token.
func pairTestDevice(t *testing.T, s *Server, id, nodeID string) string {
	t.Helper()
	token := "token-" + id
	if err := s.stateStore.InsertRemoteDevice(state.RemoteDevice{ID: id, Name: id, TokenHash: hashRemoteToken(token), NodeStableID: nodeID}); err != nil {
		t.Fatal(err)
	}
	return token
}

func phoneRequest(method, path, body, token string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.Host = testDomain
	r.RemoteAddr = "100.64.0.2:5000"
	if method != http.MethodGet {
		r.Header.Set("Origin", "https://"+testDomain)
	}
	if token != "" {
		r.AddCookie(&http.Cookie{Name: remoteDeviceCookie, Value: token})
	}
	return r
}

func TestRemoteGuardAndDeviceAuth(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "node-phone"}))
	token := pairTestDevice(t, s, "d1", "node-phone")
	other := pairTestDevice(t, s, "d2", "node-other")

	cases := []struct {
		name   string
		mutate func(*http.Request)
		token  string
		status int
		code   string
	}{
		{"wrong host", func(r *http.Request) { r.Host = "127.0.0.1:4317" }, token, 403, codeRemoteForbidden},
		{"foreign origin", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, token, 403, codeRemoteForbidden},
		{"unknown peer", func(r *http.Request) { r.RemoteAddr = "100.64.0.9:1" }, token, 403, codeRemoteForbidden},
		{"no cookie", nil, "", 401, codeRemoteUnpaired},
		{"unknown cookie", nil, "forged", 401, codeRemoteUnpaired},
		{"cookie from another node", nil, other, 401, codeRemoteDeviceMismatch},
		{"paired", nil, token, 200, ""},
		{"paired with :443 host", func(r *http.Request) { r.Host = testDomain + ":443" }, token, 200, ""},
	}
	for _, tc := range cases {
		r := phoneRequest(http.MethodGet, "/api/health", "", tc.token)
		if tc.mutate != nil {
			tc.mutate(r)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != tc.status || errorCode(t, rec) != tc.code {
			t.Errorf("%s: %d %s, want %d %s", tc.name, rec.Code, rec.Body, tc.status, tc.code)
		}
	}

	// A mutation without Origin is refused before authentication.
	r := phoneRequest(http.MethodPost, "/api/sessions/x/cancel", "", token)
	r.Header.Del("Origin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Fatalf("originless POST = %d", rec.Code)
	}

	// The first authenticated response slides the credential; the next does not.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/health", "", token))
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("cookie re-issued twice within a day: %q", got)
	}
}

func TestRemoteCookieIsHostScopedAndSliding(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/health", "", token))
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].Name != remoteDeviceCookie || c[0].Value != token || !c[0].Secure || !c[0].HttpOnly ||
		c[0].SameSite != http.SameSiteStrictMode || c[0].Path != "/" || c[0].MaxAge != 400*24*3600 || c[0].Domain != "" {
		t.Fatalf("cookie = %+v", c)
	}
	devices, _ := s.stateStore.ListRemoteDevices()
	if time.Since(devices[0].LastSeenAt) > time.Minute {
		t.Fatalf("last seen not recorded: %v", devices[0].LastSeenAt)
	}
}

// A phone may read only a path from its own tracked-files index, and a symlink
// substituted at that path is refused without following it (TS-13.R22).
func TestRemoteTrackedFileRead(t *testing.T) {
	s := testServer(t, true)
	root := seedReadableWorkspace(t, s, "a_remote")
	if _, err := s.stateStore.DB().Exec(`INSERT INTO tracked_files(agent_id, path, abs_path, first_seq, last_seq, first_ts, last_ts) VALUES (?, ?, ?, 1, 1, ?, ?)`, "a_remote", "main.go", root+"/main.go", time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/sessions/a_remote/file?path=main.go", "", token))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "package main") {
		t.Fatalf("tracked file = %d %s", rec.Code, rec.Body)
	}
	for _, path := range []string{root + "/main.go", "notes.md", "../main.go"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/sessions/a_remote/file?path="+url.QueryEscape(path), "", token))
		if rec.Code != 404 || errorCode(t, rec) != codeRemoteFileNotTracked || strings.Contains(rec.Body.String(), "package main") {
			t.Fatalf("path %q = %d %s", path, rec.Code, rec.Body)
		}
	}
	if err := os.Remove(root + "/main.go"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"/notes.md", root+"/main.go"); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/sessions/a_remote/file?path=main.go", "", token))
	if rec.Code != 404 || errorCode(t, rec) != codeRemoteFileNotTracked || strings.Contains(rec.Body.String(), "# Notes") {
		t.Fatalf("symlink tracked file = %d %s", rec.Code, rec.Body)
	}
}

func TestRemoteAllowlistAndFieldFilter(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")

	for _, c := range []struct{ method, path string }{
		{"GET", "/api/backends"},
		{"GET", "/api/remote"},
		{"PUT", "/api/config"},
		{"POST", "/api/sessions/a/restore"},
		{"GET", "/api/sessions/a/file-search"},
		{"GET", "/api/tasks"},
		{"POST", "/api/tasks"},
		{"GET", "/api/sessions/a/terminal/ws"},
		{"POST", "/mcp"},
		{"POST", "/api/hook"},
		{"GET", "/api/archive"},
		{"POST", "/"},
		{"GET", "/api/unknown"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(c.method, c.path, "{}", token))
		if rec.Code != 404 || errorCode(t, rec) != codeRemoteRouteNotAvailable {
			t.Errorf("%s %s = %d %s", c.method, c.path, rec.Code, rec.Body)
		}
	}

	for _, c := range []struct{ path, body string }{
		{"/api/sessions", `{"role":"chucky","project":"my-app","interface":"terminal"}`},
		{"/api/sessions", `{"role":"chucky","project":"my-app","group":"g"}`},
		{"/api/sessions", `{"role":"chucky","project":"my-app","resume":true}`},
		{"/api/sessions/a/switch-runtime", `{"backend":"claude","interface":"terminal"}`},
		{"/api/sessions/a/session-config", `{"effort":"high","backend":"claude"}`},
		{"/api/sessions/a/rename", `{"name":"Atlas","role":"implementer"}`},
		{"/api/pipeline-runs", `{"request_id":"r","template_id":"p","project":"my-app","goal":"x","assignments":{"standing":{"backend":"codex"}}}`},
		{"/api/pipeline-runs", `{"request_id":"r","template_id":"p","project":"my-app","goal":"x","orchestrator":{"backend":"codex"}}`},
		{"/api/pipeline-runs", `{"request_id":"r","template_id":"p","project":"my-app","goal":"x","orchestrator":{"backend":"","surprise":"codex"}}`},
		{"/api/pipeline-runs", `{"request_id":"r","template_id":"p","project":"my-app","goal":"x","dedicated_assignments":{"stage":{"model":"gpt"}}}`},
		{"/api/sessions", `not json`},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(http.MethodPost, c.path, c.body, token))
		if rec.Code != 400 || errorCode(t, rec) != codeRemoteFieldNotAllowed {
			t.Errorf("POST %s %s = %d %s", c.path, c.body, rec.Code, rec.Body)
		}
	}

	// An allowed launch body reaches the shared handler (which then validates it).
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodPost, "/api/sessions", `{"role":"chucky","project":"nope","name":"Phone","backend":"claude","model":"sonnet","effort":"high","fast":false}`, token))
	if code := errorCode(t, rec); code == codeRemoteFieldNotAllowed {
		t.Fatalf("allowed body blocked: %d %s", rec.Code, rec.Body)
	}
	// Allowlisted reads use the loopback handlers.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/projects", "", token))
	if rec.Code != 200 {
		t.Fatalf("GET /api/projects = %d", rec.Code)
	}
}

// The phone sends empty runtime assignments (it may not choose them), so the
// Mac fills the standing owner and each dedicated coordinator with its
// configured default and the run starts (FS-20.R15/A4, FS-14.R80). The body is
// the one ui/src/remote/NewWorkScreen.tsx submits.
func TestRemotePipelineStartUsesMacDefaults(t *testing.T) {
	s := testServer(t, true)
	t.Cleanup(func() { s.registry.Shutdown(context.Background()) })
	s.registry.Chat().SetCommand(buildFakeACP(t))
	if err := s.configStore.WriteProject("phone-run", config.Project{Title: "Phone run", Cwd: t.TempDir(), AddDirs: []string{}}); err != nil {
		t.Fatal(err)
	}
	template := apiTemplate()
	template.Stages = append(template.Stages, pipeline.Stage{
		ID: "review", Title: "Review", Objective: "Review it.", Coordination: "dedicated", DedicatedRole: "implementer",
		Inputs: []pipeline.StageInput{}, Outputs: []pipeline.StageOutput{},
	})
	if _, err := s.pipelineTemplates.Create("two-stage", template); err != nil {
		t.Fatal(err)
	}
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		t.Fatal(err)
	}
	var want pipeline.RuntimeAssignment
	for id, backend := range backends.Backends {
		if backend.Default {
			want = pipeline.RuntimeAssignment{Backend: id, Model: backend.DefaultModel, Effort: backend.Models[backend.DefaultModel].DefaultEffort}
		}
	}
	if want.Backend == "" {
		t.Fatal("seeded backends have no default")
	}

	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodPost, "/api/pipeline-runs", `{"request_id":"phone-1","template_id":"two-stage","display_name":"","project":"phone-run","goal":"Ship it","inputs":{},"orchestrator":{"backend":"","model":"","effort":"","fast":false},"dedicated_assignments":{}}`, token))
	if rec.Code != http.StatusCreated {
		t.Fatalf("phone start = %d %s", rec.Code, rec.Body)
	}
	var started struct {
		Run pipeline.RunDetail `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"standing", "review"} {
		if got := started.Run.Assignments[key]; got != want {
			t.Errorf("assignment %s = %+v, want %+v", key, got, want)
		}
	}
}

// A phone start fills every Think Tank slot with the Mac's default and refuses a
// phone-chosen room runtime (TS-09.R51, TS-13.R5).
func TestRemotePipelineStartFillsThinkTankSlots(t *testing.T) {
	s := testServer(t, true)
	template := apiTemplate()
	template.Stages = append(template.Stages, pipeline.Stage{
		ID: "debate", Title: "Debate", Objective: "Weigh it.", Coordination: pipeline.CoordinationThinkTank,
		ThinkTank: &pipeline.ThinkTankStage{JudgeRole: "implementer", Participants: []pipeline.ThinkTankParticipant{
			{ID: "pro", Role: "implementer", Limit: 1}, {ID: "con", Role: "implementer", Limit: 1},
		}},
		Inputs: []pipeline.StageInput{}, Outputs: []pipeline.StageOutput{{Name: "synthesis", Value: "synthesis", Description: "Synthesis"}},
	})
	if record, err := s.pipelineTemplates.Create("room-stage", template); err != nil || !record.Valid {
		t.Fatalf("create = %+v err=%v", record, err)
	}
	var filled struct {
		Rooms map[string]pipeline.ThinkTankAssignment `json:"think_tank_assignments"`
	}
	if err := json.Unmarshal(s.withDefaultPipelineRuntimes([]byte(`{"template_id":"room-stage"}`)), &filled); err != nil {
		t.Fatal(err)
	}
	room := filled.Rooms["debate"]
	if room.Judge.Backend == "" || room.Participants["pro"] != room.Judge || room.Participants["con"] != room.Judge {
		t.Fatalf("filled room slots = %+v", room)
	}

	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodPost, "/api/pipeline-runs", `{"request_id":"phone-room","template_id":"room-stage","project":"p","goal":"g","think_tank_assignments":{"debate":{"participants":{"pro":{"backend":"codex","model":"m"}}}}}`, token))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != codeRemoteFieldNotAllowed {
		t.Fatalf("phone-chosen room runtime = %d %s", rec.Code, rec.Body)
	}
}

// Every tailnet mutation body is bounded before its handler runs, including
// actions with no field filter (TS-13.R5/R6, INV §16).
func TestRemoteMutationBodyIsBounded(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")

	big := `{"text":"` + strings.Repeat("x", remoteBodyLimit) + `"}`
	for _, path := range []string{"/api/sessions/nope/prompt", "/api/sessions/nope/steer", "/api/tasks"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(http.MethodPost, path, big, token))
		if rec.Code != http.StatusRequestEntityTooLarge || errorCode(t, rec) != codeRemoteBodyTooLarge {
			t.Errorf("oversized POST %s = %d %s", path, rec.Code, rec.Body)
		}
	}
	// A normal body still reaches the shared handler.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodPost, "/api/sessions/nope/steer", `{"text":"hi"}`, token))
	if code := errorCode(t, rec); code != "not_found" {
		t.Fatalf("normal body blocked: %d %s", rec.Code, rec.Body)
	}
}

func TestRemoteAdmissionCannotOutliveRevoke(t *testing.T) {
	devices := newRemoteDevices()
	validated := make(chan struct{})
	release := make(chan struct{})
	admitted := make(chan context.Context, 1)
	go func() {
		ctx, done, err := devices.admit(context.Background(), "d1", func() error {
			close(validated)
			<-release
			return nil
		})
		if err != nil {
			admitted <- nil
			return
		}
		defer done()
		admitted <- ctx
		<-ctx.Done()
	}()
	<-validated
	revoked := make(chan struct{})
	go func() {
		devices.end("d1")
		close(revoked)
	}()
	close(release)
	ctx := <-admitted
	if ctx == nil {
		t.Fatal("request was not admitted")
	}
	select {
	case <-revoked:
	case <-time.After(time.Second):
		t.Fatal("revoke did not cross the admission boundary")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("request admitted during revoke survived")
	}
}

// readEventTypes collects SSE event names until the stream ends or the
// deadline passes.
func readEventTypes(body *bufio.Reader, until time.Time, stop func(string) bool) ([]string, error) {
	var types []string
	for time.Now().Before(until) {
		line, err := body.ReadString('\n')
		if err != nil {
			return types, err
		}
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "event: "); ok {
			types = append(types, name)
			if stop != nil && stop(name) {
				return types, nil
			}
		}
	}
	return types, nil
}

// Over a real listener: the phone stream never carries remote_update, revoking
// the device closes its stream promptly, and turning remote off closes every
// open phone connection (TS-13.R3/R9, FS-20.R4).
func TestRemoteStreamFilteringRevokeAndDisable(t *testing.T) {
	s, _ := remoteTestServer(t)
	token := pairTestDevice(t, s, "d1", "node-phone")
	var stop func()
	lnReady := make(chan string, 1)
	s.remote = remote.NewManager(func() (remote.Node, error) {
		return &fakeTailnetNode{domain: testDomain}, nil
	}, func(l remote.Listener) func() {
		l.WhoIs = func(context.Context, string) (remote.Peer, error) { return remote.Peer{StableID: "node-phone"}, nil }
		stop = s.serveRemote(l)
		lnReady <- l.Listener.Addr().String()
		return stop
	}, func(st remote.Status) { s.eventBus.Publish("remote_update", nil, s.remoteViewFor(st)) }, s.log)
	s.remote.SetPollInterval(time.Millisecond)
	s.remote.Enable()
	t.Cleanup(s.remote.Disable)
	addr := <-lnReady

	open := func() (*http.Response, *bufio.Reader) {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/api/events", nil)
		req.Host = testDomain
		req.AddCookie(&http.Cookie{Name: remoteDeviceCookie, Value: token})
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("open stream: %v %v", resp, err)
		}
		return resp, bufio.NewReader(resp.Body)
	}

	resp, body := open()
	// The hydration marker is a state_update; the test snapshot is empty.
	if _, err := readEventTypes(body, time.Now().Add(2*time.Second), func(n string) bool { return n == "state_update" }); err != nil {
		t.Fatal(err)
	}
	s.eventBus.Publish("remote_update", nil, s.remoteViewFor(s.remote.Status()))
	s.eventBus.Publish("task_update", nil, map[string]any{})
	types, err := readEventTypes(body, time.Now().Add(2*time.Second), func(n string) bool { return n == "task_update" })
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range types {
		if typ == "remote_update" {
			t.Fatal("phone stream carried remote_update")
		}
	}

	start := time.Now()
	s.remoteDevices.end("d1")
	if _, err := readEventTypes(body, time.Now().Add(3*time.Second), nil); err == nil {
		t.Fatal("revoked device stream stayed open")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("revoke took %v", time.Since(start))
	}
	resp.Body.Close()

	resp, body = open()
	defer resp.Body.Close()
	putRemote(t, s.routes(), `{"enabled":false}`)
	if _, err := readEventTypes(body, time.Now().Add(3*time.Second), nil); err == nil {
		t.Fatal("disable left the phone stream open")
	}
	if _, err := http.Get("http://" + addr + "/api/health"); err == nil {
		t.Fatal("phone address still answers after disable")
	}
}

// Every task route is desktop-only (FS-20.R40, TS-13.R21).
func TestRemoteDeniesEveryTaskRoute(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")
	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/tasks"}, {http.MethodPost, "/api/tasks"}, {http.MethodGet, "/api/tasks/t"},
		{http.MethodDelete, "/api/tasks/t"}, {http.MethodPost, "/api/tasks/t/cancel"},
		{http.MethodPost, "/api/tasks/t/result"}, {http.MethodPost, "/api/tasks/t/retry"}, {http.MethodPost, "/api/tasks/t/rearm"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(request.method, request.path, `{}`, token))
		if rec.Code != http.StatusNotFound || errorCode(t, rec) != codeRemoteRouteNotAvailable {
			t.Errorf("%s %s = %d %s", request.method, request.path, rec.Code, rec.Body)
		}
	}
}

// A phone replaces an interrupted orchestrator with a changed runtime; the
// shared validator rejects a stale or unsupported choice without touching the
// run (FS-20.R31, TS-13.R15).
func TestRemoteReplaceValidatesChosenRuntime(t *testing.T) {
	srv, _ := wakeTestServer(t)
	h := srv.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, srv, "d1", "n")
	now := time.Now().UTC()
	snapshot, err := json.Marshal(pipeline.Template{Version: 2, Title: "One", OrchestratorRole: "impl", Stages: []pipeline.Stage{
		{ID: "work", Title: "Work", Objective: "Work", Coordination: "standing", Inputs: []pipeline.StageInput{}, Outputs: []pipeline.StageOutput{}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	taskID, err := srv.stateStore.NewTaskID()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := srv.stateStore.CreatePipelineRun(state.CreatePipelineRunParams{Run: state.PipelineRunRecord{
		RunID: "pr_replace", TemplateID: "one", TemplateSnapshot: snapshot, DisplayName: "One", Project: "tmpproj",
		Goal: "goal", State: "queued", Revision: 1, PendingAction: "dispatch_stage_task",
		CurrentStageID: "work", CreatedAt: now, UpdatedAt: now,
	}, RequestID: "pr_replace", RequestHash: "hash", InitialStageTask: &state.CreatePipelineStageTaskParams{
		RunID: "pr_replace", ExpectedRevision: 1, StageIndex: 0, AttemptNumber: 1, StageID: "work",
		Task: state.Task{TaskID: taskID, Project: "tmpproj", DisplayName: "Work", Instruction: "work",
			TargetKind: state.TargetLaunch, Role: "impl", CreatedByKind: "pipeline"},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stateStore.DB().Exec(`UPDATE tasks SET state = ? WHERE task_id = ?`, state.TaskInterrupted, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stateStore.DB().Exec(`UPDATE pipeline_runs SET state = 'paused', pending_action = '' WHERE run_id = 'pr_replace'`); err != nil {
		t.Fatal(err)
	}
	run, err := srv.stateStore.ReadPipelineRun("pr_replace")
	if err != nil {
		t.Fatal(err)
	}
	replace := func(orchestrator string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		body := fmt.Sprintf(`{"revision":%d,"orchestrator":%s}`, run.Revision, orchestrator)
		h.ServeHTTP(rec, phoneRequest(http.MethodPost, "/api/pipeline-runs/pr_replace/replace", body, token))
		return rec
	}

	for _, stale := range []string{
		`{"backend":"claude","model":"retired-model"}`,
		`{"backend":"claude","model":"sonnet","effort":"ultra"}`,
		`{"backend":"claude","model":"sonnet","fast":true}`,
	} {
		if rec := replace(stale); rec.Code < 400 {
			t.Fatalf("replace with %s = %d %s", stale, rec.Code, rec.Body)
		}
		if after, err := srv.stateStore.ReadPipelineRun("pr_replace"); err != nil || after.Revision != run.Revision || after.State != "paused" {
			t.Fatalf("refused replacement changed the run: %+v, %v", after, err)
		}
	}
	if rec := replace(`{"backend":"claude","model":"sonnet","effort":"high"}`); rec.Code != http.StatusOK {
		t.Fatalf("replace with a changed runtime = %d %s", rec.Code, rec.Body)
	}
	stages, err := srv.stateStore.ListPipelineStageTasks("pr_replace")
	if err != nil || len(stages) != 2 {
		t.Fatalf("stages = %+v, %v", stages, err)
	}
	replacement, err := srv.stateStore.ReadTask(stages[1].TaskID)
	if err != nil || replacement.Backend != "claude" || replacement.Model != "sonnet" || replacement.Effort != "high" {
		t.Fatalf("replacement task = %+v, %v", replacement, err)
	}
}

// The phone's runtime catalog carries ids, names, efforts, and fast support
// only; backend type, env, and credentials never reach it (TS-13.R15).
func TestRemoteRuntimeOptionsAreSecretFree(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")
	backends := config.DefaultBackends()
	claude := backends.Backends["claude"]
	claude.Env = map[string]string{"ANTHROPIC_API_KEY": "sk-backend-secret"}
	sonnet := claude.Models["sonnet"]
	sonnet.Env = map[string]string{"ANTHROPIC_BASE_URL": "https://model-secret.example"}
	claude.Models["sonnet"] = sonnet
	backends.Backends["claude"] = claude
	if err := s.configStore.WriteBackends(backends); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/remote/runtime-options", "", token))
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime options = %d %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, leak := range []string{"sk-backend-secret", "model-secret", "env", "claude-acp", `"type"`} {
		if strings.Contains(body, leak) {
			t.Fatalf("runtime options leak %q: %s", leak, body)
		}
	}
	var got struct {
		Backends []remoteRuntimeBackend `json:"backends"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var found *remoteRuntimeModel
	var foundBackend *remoteRuntimeBackend
	for _, b := range got.Backends {
		if b.ID == "claude" {
			foundBackend = &b
		}
		for i, m := range b.Models {
			if b.ID == "claude" && m.ID == "sonnet" {
				found = &b.Models[i]
			}
		}
	}
	if found == nil || foundBackend == nil || len(found.Efforts) == 0 || found.DefaultEffort != sonnet.DefaultEffort ||
		foundBackend.Default != claude.Default || foundBackend.DefaultModel != claude.DefaultModel {
		t.Fatalf("claude/sonnet missing or incomplete: %+v", got.Backends)
	}
}
