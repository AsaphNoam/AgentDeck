package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TS-04.R63 / FS-03.R58 regression: a native child's permission_resolved must
// carry the same activity scope (ActivityID/ParentActivityID) as its
// permission_request, for every resolution path — auto-approve is covered by
// TestAgentDeckToolPermissionAutoApprovesExactIdentity at root scope; these
// cover a scoped child's approve, deny, timeout and cancel paths, which
// previously emitted permission_resolved at root scope regardless of where
// the request was scoped.

// waitChildPermissionRequest reads ch until a permission_request arrives,
// returning the raw event (for its scope) and its decoded payload.
func waitChildPermissionRequest(t *testing.T, ch <-chan Event) (Event, PermissionRequestData) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before permission_request")
			}
			if ev.Type != EvPermissionRequest {
				continue
			}
			var d PermissionRequestData
			_ = json.Unmarshal(ev.Data, &d)
			return ev, d
		case <-deadline:
			t.Fatal("no child permission request")
		}
	}
}

// assertSameScope fails unless got carries want's ActivityID/ParentActivityID,
// and that it comes strictly after want in sequence (request before resolved).
func assertSameScope(t *testing.T, request, resolved Event) {
	t.Helper()
	if resolved.ActivityID != request.ActivityID || resolved.ParentActivityID != request.ParentActivityID {
		t.Fatalf("permission_resolved scope = %q/%q, want request scope %q/%q",
			resolved.ActivityID, resolved.ParentActivityID, request.ActivityID, request.ParentActivityID)
	}
	if resolved.Seq <= request.Seq {
		t.Fatalf("permission_resolved seq %d did not follow permission_request seq %d", resolved.Seq, request.Seq)
	}
}

func TestChildPermissionResolvedScopeApprove(t *testing.T) {
	c, h, ch, _, _ := startSubagentAgent(t, "FAKEACP_CAPS=1", "FAKEACP_CHILD_PERMISSION=1")
	child := activityIDFor("th_child_1")

	reqEv, reqData := waitChildPermissionRequest(t, ch)
	if reqEv.ActivityID != child {
		t.Fatalf("request scope = %q, want %q", reqEv.ActivityID, child)
	}

	if err := c.Permission(context.Background(), h.AgentID, reqData.ToolCallID, "approve"); err != nil {
		t.Fatalf("Permission approve: %v", err)
	}

	resolved := waitForEvent(t, ch, EvPermissionResolved)
	assertSameScope(t, reqEv, resolved)
	var rd PermissionResolvedData
	_ = json.Unmarshal(resolved.Data, &rd)
	if rd.Decision != "approve" || rd.ToolCallID != reqData.ToolCallID {
		t.Fatalf("resolution = %+v, want approve for %q", rd, reqData.ToolCallID)
	}
}

func TestChildPermissionResolvedScopeDeny(t *testing.T) {
	c, h, ch, _, _ := startSubagentAgent(t, "FAKEACP_CAPS=1", "FAKEACP_CHILD_PERMISSION=1")
	child := activityIDFor("th_child_1")

	reqEv, reqData := waitChildPermissionRequest(t, ch)
	if reqEv.ActivityID != child {
		t.Fatalf("request scope = %q, want %q", reqEv.ActivityID, child)
	}

	if err := c.Permission(context.Background(), h.AgentID, reqData.ToolCallID, "deny"); err != nil {
		t.Fatalf("Permission deny: %v", err)
	}

	resolved := waitForEvent(t, ch, EvPermissionResolved)
	assertSameScope(t, reqEv, resolved)
	var rd PermissionResolvedData
	_ = json.Unmarshal(resolved.Data, &rd)
	if rd.Decision != "deny" {
		t.Fatalf("resolution = %+v, want deny", rd)
	}
}

func TestChildPermissionResolvedScopeTimeout(t *testing.T) {
	t.Setenv("PERMISSION_TIMEOUT", "150ms")
	_, _, ch, _, _ := startSubagentAgent(t, "FAKEACP_CAPS=1", "FAKEACP_CHILD_PERMISSION=1")
	child := activityIDFor("th_child_1")

	reqEv, _ := waitChildPermissionRequest(t, ch)
	if reqEv.ActivityID != child {
		t.Fatalf("request scope = %q, want %q", reqEv.ActivityID, child)
	}
	// Do NOT decide; the runtime must auto-deny after PERMISSION_TIMEOUT.
	resolved := waitForEvent(t, ch, EvPermissionResolved)
	assertSameScope(t, reqEv, resolved)
	var rd PermissionResolvedData
	_ = json.Unmarshal(resolved.Data, &rd)
	if rd.Decision != "timeout" {
		t.Fatalf("resolution = %+v, want timeout", rd)
	}
}

func TestChildPermissionResolvedScopeCancel(t *testing.T) {
	c, h, ch, _, _ := startSubagentAgent(t, "FAKEACP_CAPS=1", "FAKEACP_CHILD_PERMISSION=1")
	child := activityIDFor("th_child_1")

	reqEv, _ := waitChildPermissionRequest(t, ch)
	if reqEv.ActivityID != child {
		t.Fatalf("request scope = %q, want %q", reqEv.ActivityID, child)
	}

	if cancelled, err := c.Cancel(context.Background(), h.AgentID); err != nil {
		t.Fatalf("Cancel: %v", err)
	} else if !cancelled {
		t.Fatal("Cancel reported no-op, want cancelled=true (pending child permission was in flight)")
	}

	resolved := waitForEvent(t, ch, EvPermissionResolved)
	assertSameScope(t, reqEv, resolved)
	var rd PermissionResolvedData
	_ = json.Unmarshal(resolved.Data, &rd)
	if rd.Decision != "cancelled" {
		t.Fatalf("resolution = %+v, want cancelled", rd)
	}
}
