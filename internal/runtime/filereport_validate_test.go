package runtime

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/agentdeck/agentdeck/internal/state"
)

// TS-04.R66 review fix: onFileReport must validate a matching frame (version,
// status) before consuming the outstanding request. Consuming first let a
// malformed matching frame burn the one accepted slot, dropping the genuine
// report that followed it.

// fileReportFrame builds a session_info_update params payload in the shape
// onFileReport decodes (codex-acp 1.12's agentFileChangeReport meta).
func fileReportFrame(t *testing.T, requestID string, version int, status string, paths []string) json.RawMessage {
	t.Helper()
	body := map[string]any{
		"update": map[string]any{
			"sessionUpdate": "session_info_update",
			"_meta": map[string]any{
				"jetbrains": map[string]any{
					"air": map[string]any{
						"agentFileChangeReport": map[string]any{
							"version":   version,
							"requestId": requestID,
							"status":    status,
							"paths":     paths,
						},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal file report frame: %v", err)
	}
	return raw
}

// newFileReportAgentState builds a minimal ChatRuntime/agentState pair for
// calling onFileReport directly, without a live ACP process.
func newFileReportAgentState(t *testing.T, requestID, root string) (*ChatRuntime, *agentState) {
	t.Helper()
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	c := NewChatRuntime(st)
	as := &agentState{
		agentID:           "a_filereport",
		hub:               NewHub(),
		roots:             []string{root},
		fileReportRequest: requestID,
	}
	return c, as
}

func recordedFileReports(as *agentState) []FileReportData {
	var out []FileReportData
	for _, ev := range as.transcript {
		if ev.Type == EvFileReport {
			var d FileReportData
			_ = json.Unmarshal(ev.Data, &d)
			out = append(out, d)
		}
	}
	return out
}

// A malformed matching frame (bad version) must not consume the outstanding
// request before it is validated — otherwise the genuine report that follows
// it under the same request id is dropped as unmatched.
func TestFileReportMalformedFrameDoesNotConsumeMatchingRequest(t *testing.T) {
	root := t.TempDir()
	c, as := newFileReportAgentState(t, "fcr-x", root)

	malformed := fileReportFrame(t, "fcr-x", fileReportVersion+1, "reported", []string{"main.go"})
	if handled := c.onFileReport(as, malformed); !handled {
		t.Fatal("malformed matching frame should still report handled=true")
	}
	as.mu.Lock()
	stillPending := as.fileReportRequest
	as.mu.Unlock()
	if stillPending != "fcr-x" {
		t.Fatalf("malformed frame consumed the pending request; fileReportRequest = %q, want fcr-x", stillPending)
	}
	if got := recordedFileReports(as); len(got) != 0 {
		t.Fatalf("malformed frame recorded a report: %+v", got)
	}

	valid := fileReportFrame(t, "fcr-x", fileReportVersion, "reported", []string{"main.go"})
	if handled := c.onFileReport(as, valid); !handled {
		t.Fatal("valid matching frame should report handled=true")
	}
	got := recordedFileReports(as)
	if len(got) != 1 {
		t.Fatalf("reports after valid frame = %+v, want exactly one", got)
	}
	if got[0].RequestID != "fcr-x" || len(got[0].Paths) != 1 || got[0].Paths[0] != filepath.Join(root, "main.go") {
		t.Fatalf("recorded report = %+v", got[0])
	}
	as.mu.Lock()
	consumed := as.fileReportRequest
	as.mu.Unlock()
	if consumed != "" {
		t.Fatalf("fileReportRequest after valid report = %q, want consumed", consumed)
	}
}

// Once a valid report has consumed the outstanding request, a later frame
// carrying the same request id must not be recorded a second time.
func TestFileReportDuplicateValidReportNotRecordedTwice(t *testing.T) {
	root := t.TempDir()
	c, as := newFileReportAgentState(t, "fcr-y", root)

	first := fileReportFrame(t, "fcr-y", fileReportVersion, "reported", []string{"main.go"})
	if handled := c.onFileReport(as, first); !handled {
		t.Fatal("first valid frame should report handled=true")
	}
	if got := recordedFileReports(as); len(got) != 1 {
		t.Fatalf("reports after first frame = %+v, want exactly one", got)
	}

	dup := fileReportFrame(t, "fcr-y", fileReportVersion, "reported", []string{"other.go"})
	if handled := c.onFileReport(as, dup); !handled {
		t.Fatal("duplicate frame should still report handled=true")
	}
	got := recordedFileReports(as)
	if len(got) != 1 {
		t.Fatalf("reports after duplicate frame = %+v, want still exactly one", got)
	}
	if got[0].Paths[0] != filepath.Join(root, "main.go") {
		t.Fatalf("recorded report changed on duplicate: %+v", got[0])
	}
}
