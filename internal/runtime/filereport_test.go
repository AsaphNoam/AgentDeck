package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reportTurn(t *testing.T, env ...string) (LaunchSpec, []Event, string) {
	t.Helper()
	c, spec := newChatTest(t, "stream_text")
	dump := filepath.Join(t.TempDir(), "prompt.json")
	spec.Env = append(spec.Env, append(env, "FAKEACP_FILE_REPORT=1", "FAKEACP_PROMPT_DUMP="+dump)...)
	ctx := context.Background()
	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(ctx, h.AgentID) })
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(unsub)
	if err := c.SendPrompt(ctx, h.AgentID, "edit"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	evs := drainTurn(t, ch)
	raw, _ := os.ReadFile(dump)
	return spec, evs, string(raw)
}

// Each root prompt requests one report; only the matching report is recorded,
// with out-of-scope paths removed and the report marked truncated rather than
// complete (TS-04.R66, FS-03.A42).
func TestFileReportAcceptsOnlyTheMatchingInScopeReport(t *testing.T) {
	spec, evs, prompt := reportTurn(t, "FAKEACP_CAPS=1")
	var req struct {
		Meta struct {
			JetBrains struct {
				Air struct {
					Request struct {
						Version   int    `json:"version"`
						RequestID string `json:"requestId"`
					} `json:"agentFileChangeReportRequest"`
				} `json:"air"`
			} `json:"jetbrains"`
		} `json:"_meta"`
	}
	_ = json.Unmarshal([]byte(prompt), &req)
	id := req.Meta.JetBrains.Air.Request.RequestID
	if req.Meta.JetBrains.Air.Request.Version != 1 || !strings.HasPrefix(id, "fcr-") {
		t.Fatalf("prompt report request = %s", prompt)
	}
	var reports []FileReportData
	for _, ev := range evs {
		if ev.Type == EvFileReport {
			var d FileReportData
			_ = json.Unmarshal(ev.Data, &d)
			reports = append(reports, d)
		}
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %+v, want exactly the matching one", reports)
	}
	r := reports[0]
	if r.RequestID != id || r.Status != "reported" || r.DeclaredComplete || !r.Truncated || r.Uncertainty == "" {
		t.Fatalf("report = %+v", r)
	}
	if len(r.Paths) != 1 || r.Paths[0] != filepath.Join(spec.Cwd, "main.go") {
		t.Fatalf("paths = %q", r.Paths)
	}
}

func TestFileReportNeedsNegotiation(t *testing.T) {
	_, evs, prompt := reportTurn(t)
	if strings.Contains(prompt, "agentFileChangeReportRequest") {
		t.Fatalf("unnegotiated prompt asked for a report: %s", prompt)
	}
	for _, ev := range evs {
		if ev.Type == EvFileReport {
			t.Fatalf("unnegotiated session recorded a report: %s", ev.Data)
		}
	}
}
