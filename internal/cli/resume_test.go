package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// TestListInactiveSessionsNameFilter guards the advisory finding that bare-form
// auto-resume must respect --name: with a name given, only the session actually
// named that matches; without a name, all role@project inactive sessions match.
func TestListInactiveSessionsNameFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"results":[
			{"agent_id":"a_1","name":"Atlas","role":"impl","project":"app","active":false},
			{"agent_id":"a_2","name":"Borg","role":"impl","project":"app","active":false},
			{"agent_id":"a_3","name":"Atlas","role":"impl","project":"other","active":false},
			{"agent_id":"a_4","name":"Live","role":"impl","project":"app","active":true}
		]}`)
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(u.Port())

	// No name → all inactive impl@app sessions.
	all, err := listInactiveSessions(port, "impl", "app", "")
	if err != nil {
		t.Fatalf("listInactiveSessions: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("no-name match = %d sessions, want 2 (a_1,a_2)", len(all))
	}

	// With --name Atlas → only the impl@app session named Atlas (a_1), not Borg,
	// and not the impl@other Atlas or the active one.
	named, err := listInactiveSessions(port, "impl", "app", "Atlas")
	if err != nil {
		t.Fatalf("listInactiveSessions (named): %v", err)
	}
	if len(named) != 1 || named[0].AgentID != "a_1" {
		t.Fatalf("named match = %+v, want exactly a_1", named)
	}
}
