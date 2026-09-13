package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/backend"
)

// The pinned ACP session-request schemas are the oracle for what AgentDeck may
// put on a session/new or session/load wire. Both lists are transcribed from the
// protocol schema the pinned adapters decode with (TS-04.R47), NOT read back out
// of sessionNewParams/sessionLoadParams: an oracle copied from the builder it
// checks cannot fail, which is how BR-1 shipped a `model` member that every layer
// agreed on and no provider ever read (INV §17, INV §12).
var (
	acpNewSessionSchema  = []string{"cwd", "additionalDirectories", "mcpServers", "_meta"}
	acpLoadSessionSchema = []string{"sessionId", "cwd", "additionalDirectories", "mcpServers", "_meta"}
)

// acpOutOfSchemaMembers records, per backend type, the top-level members
// AgentDeck still sends outside that schema. A pinned decoder strips them, so
// every entry is an *unverified* delivery path rather than a working one, and
// FS-09.A6 keeps the claim gated until the CLI is installed and its real
// mechanism is checked at the effective provider. TS-04.R47 deliberately leaves
// the two gated adapters unchanged rather than trading a possibly-working path
// for an assumption, so this table is the machine-readable form of that debt —
// not permission to add more. claude-acp and codex-acp must stay empty: both
// have a traced delivery mechanism (`_meta` and post-session configuration).
var acpOutOfSchemaMembers = map[string][]string{
	"claude-acp":    {},
	"codex-acp":     {},
	"opencode-acp":  {"model", "systemPrompt"},
	"openhands-acp": {"model", "systemPrompt"},
}

func TestSessionParamsMatchPinnedACPSchema(t *testing.T) {
	types := backend.Types()
	if len(types) == 0 {
		t.Fatal("backend.Types() is empty; the oracle would check nothing")
	}
	for _, backendType := range types {
		expected, ok := acpOutOfSchemaMembers[backendType]
		if !ok {
			t.Errorf("backend %q has no declared out-of-schema member set; a new adapter must "+
				"state which of its members survive the pinned decoder", backendType)
			continue
		}
		spec := LaunchSpec{
			Cwd:          "/work",
			AddDirs:      []string{"/extra"},
			SystemPrompt: "be helpful",
			ModelID:      "some-model",
			BackendType:  backendType,
		}
		for method, params := range map[string]map[string]any{
			"session/new":  sessionNewParams(spec),
			"session/load": sessionLoadParams(spec, "sess-1"),
		} {
			schema := acpNewSessionSchema
			if method == "session/load" {
				schema = acpLoadSessionSchema
			}
			got := membersOutsideSchema(params, schema)
			if strings.Join(got, ",") != strings.Join(expected, ",") {
				t.Errorf("%s %s out-of-schema members = %v, want %v; the pinned decoder drops "+
					"these, so an added member delivers nothing", backendType, method, got, expected)
			}
		}
	}
}

// The fake peer must drop what the pinned peer drops. While it echoed the raw
// parameters, an integration test could assert delivery of a member no provider
// receives — the second half of the BR-1 oracle error (INV §17). This drives the
// real fake over the wire rather than re-deriving its decode in the test.
func TestFakeACPDropsOutOfSchemaSessionMembers(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()

	// opencode-acp still sends top-level model and systemPrompt (FS-09.A6 gated).
	spec.BackendType = "opencode-acp"
	spec.ModelID = "some-model"
	spec.SystemPrompt = "be helpful"
	dump := filepath.Join(t.TempDir(), "new_params.json")
	spec.Env = append(spec.Env, "FAKEACP_NEW_DUMP="+dump)

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("read session/new dump (session/new not invoked?): %v", err)
	}
	var received map[string]json.RawMessage
	if err := json.Unmarshal(raw, &received); err != nil {
		t.Fatalf("unmarshal session/new params: %v\n%s", err, raw)
	}
	if got := membersOutsideSchema(rawMembers(received), acpNewSessionSchema); len(got) != 0 {
		t.Fatalf("fake peer accepted out-of-schema members %v; the pinned decoder strips them, "+
			"so no test may prove delivery through them", got)
	}
	if _, ok := received["cwd"]; !ok {
		t.Fatalf("schema member cwd was dropped: %s", raw)
	}
}

func membersOutsideSchema(params map[string]any, schema []string) []string {
	declared := make(map[string]bool, len(schema))
	for _, member := range schema {
		declared[member] = true
	}
	outside := []string{}
	for member := range params {
		if !declared[member] {
			outside = append(outside, member)
		}
	}
	sort.Strings(outside)
	return outside
}

func rawMembers(decoded map[string]json.RawMessage) map[string]any {
	members := make(map[string]any, len(decoded))
	for name, value := range decoded {
		members[name] = value
	}
	return members
}
