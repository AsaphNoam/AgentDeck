package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agentdeck/agentdeck/internal/strutil"
)

// File-change reports (TS-04.R66, FS-05.R38). Each root prompt asks for at
// most one AIR v1 report under a generation/turn-derived request id, and only
// the matching report is accepted. It supplements Files tracking with in-scope
// paths and the adapter's completeness facts; it never carries file content, a
// patch, or line counts, and never replaces ordinary ACP diffs.

// EvFileReport records one accepted file-change report.
const EvFileReport = "file_report"

const (
	fileReportVersion = 1
	maxReportPaths    = 1024
	maxReportPathLen  = 4096
	maxUncertainty    = 500
)

// FileReportData is an accepted report: validated in-scope absolute paths plus
// the adapter's declared completeness, truncation and uncertainty.
type FileReportData struct {
	RequestID        string   `json:"request_id"`
	Status           string   `json:"status"` // reported | unavailable
	Paths            []string `json:"paths"`
	DeclaredComplete bool     `json:"declared_complete"`
	Truncated        bool     `json:"truncated"`
	Uncertainty      string   `json:"uncertainty,omitempty"`
	Reason           string   `json:"reason,omitempty"`
}

// fileReportMeta adds the report request to a root prompt's params when the
// session negotiated reports, remembering the one request id it will accept.
func (as *agentState) fileReportMeta(params map[string]any) {
	if !as.capabilities().FileChangeReports {
		return
	}
	sum := sha256.Sum256([]byte(as.generation))
	as.mu.Lock()
	id := "fcr-" + hex.EncodeToString(sum[:4]) + "-" + strconv.FormatInt(as.turnSeq, 10)
	as.fileReportRequest = id
	as.mu.Unlock()
	params["_meta"] = map[string]any{"jetbrains": map[string]any{"air": map[string]any{
		"agentFileChangeReportRequest": map[string]any{"version": fileReportVersion, "requestId": id},
	}}}
}

// onFileReport consumes a root `session_info_update` carrying a report,
// reporting whether params was one. A stale, duplicate or unmatched report is
// dropped; the matching one is validated and recorded once.
func (c *ChatRuntime) onFileReport(as *agentState, params json.RawMessage) bool {
	var su struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Meta          struct {
				JetBrains struct {
					Air struct {
						Report *struct {
							Version          int      `json:"version"`
							RequestID        string   `json:"requestId"`
							Status           string   `json:"status"`
							Paths            []string `json:"paths"`
							DeclaredComplete bool     `json:"declaredComplete"`
							Truncated        bool     `json:"truncated"`
							Uncertainty      string   `json:"uncertainty"`
							Reason           string   `json:"reason"`
						} `json:"agentFileChangeReport"`
					} `json:"air"`
				} `json:"jetbrains"`
			} `json:"_meta"`
		} `json:"update"`
	}
	if json.Unmarshal(params, &su) != nil || su.Update.SessionUpdate != "session_info_update" {
		return false
	}
	r := su.Update.Meta.JetBrains.Air.Report
	if r == nil {
		return false
	}
	valid := r.Version == fileReportVersion && (r.Status == "reported" || r.Status == "unavailable")
	as.mu.Lock()
	matched := r.RequestID != "" && r.RequestID == as.fileReportRequest
	// Only a validated report consumes the outstanding request. A malformed
	// frame that happens to carry the matching request id must not burn the
	// slot — otherwise a genuine report arriving right after it is dropped as
	// unmatched (TS-04.R66).
	if matched && valid {
		as.fileReportRequest = ""
	}
	roots := as.roots
	as.mu.Unlock()
	if !matched || !valid {
		return true
	}
	paths, dropped := scopedReportPaths(roots, r.Paths)
	c.emit(as, EvFileReport, FileReportData{
		RequestID: r.RequestID, Status: r.Status, Paths: paths,
		// Only the adapter can declare completeness; anything dropped here
		// makes the report truncated, never more complete.
		DeclaredComplete: r.DeclaredComplete && !dropped && !r.Truncated,
		Truncated:        r.Truncated || dropped,
		Uncertainty:      strutil.ClipRunes(r.Uncertainty, maxUncertainty),
		Reason:           strutil.ClipRunes(r.Reason, 40),
	})
	return true
}

// scopedReportPaths keeps only bounded paths inside the working directory or an
// additional directory; relative paths resolve against the working directory.
func scopedReportPaths(roots, raw []string) ([]string, bool) {
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	dropped := false
	for _, p := range raw {
		if len(out) >= maxReportPaths || p == "" || len(p) > maxReportPathLen || strings.ContainsRune(p, 0) || len(roots) == 0 {
			dropped = true
			continue
		}
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(roots[0], abs)
		}
		abs = filepath.Clean(abs)
		if !withinAny(roots, abs) {
			dropped = true
			continue
		}
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out, dropped
}

func withinAny(roots []string, abs string) bool {
	for _, root := range roots {
		if root == "" {
			continue
		}
		rel, err := filepath.Rel(filepath.Clean(root), abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			return true
		}
	}
	return false
}
