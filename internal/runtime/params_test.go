package runtime

import (
	"encoding/json"
	"testing"
)

// Regression (review fix): the native-resume path (session/load) must forward
// additionalDirectories, or a multi-dir agent silently loses access to its extra
// project directories after resume/switch. session/new already forwarded them.
func TestSessionLoadParamsForwardsAddDirs(t *testing.T) {
	spec := LaunchSpec{
		Cwd:     "/work",
		AddDirs: []string{"/extra/one", "/extra/two"},
	}
	params := sessionLoadParams(spec, "sess-123")

	got, ok := params["additionalDirectories"]
	if !ok {
		t.Fatalf("session/load params missing additionalDirectories: %#v", params)
	}
	dirs, ok := got.([]string)
	if !ok {
		t.Fatalf("additionalDirectories = %T, want []string", got)
	}
	if len(dirs) != 2 || dirs[0] != "/extra/one" || dirs[1] != "/extra/two" {
		t.Fatalf("additionalDirectories = %v, want the spec's AddDirs", dirs)
	}
}

// Regression (review fix): the native-resume path (session/load) must carry the
// model + systemPrompt, or a same-backend model swap via switch-runtime that uses
// native resume silently keeps the OLD model (the new one never reaches the CLI).
func TestSessionLoadParamsCarriesModelAndSystemPrompt(t *testing.T) {
	spec := LaunchSpec{
		Cwd:          "/work",
		ModelID:      "opus-4-7",
		SystemPrompt: "be helpful",
	}
	params := sessionLoadParams(spec, "sess-123")

	if got := params["model"]; got != "opus-4-7" {
		t.Fatalf("session/load model = %v, want opus-4-7", got)
	}
	if got := params["systemPrompt"]; got != "be helpful" {
		t.Fatalf("session/load systemPrompt = %v, want %q", got, "be helpful")
	}
}

// FS-09.A9: the official Claude adapter receives Chuck's composed launch
// values through its documented ACP session metadata shape.
func TestClaudeSessionNewParamsUseMetaOptions(t *testing.T) {
	spec := LaunchSpec{
		Cwd:          "/work",
		AddDirs:      []string{"/extra/one", "/extra/two"},
		SystemPrompt: "be helpful",
		BackendType:  "claude-acp",
		ModelID:      "sonnet",
	}

	params := sessionNewParams(spec)

	if _, ok := params["model"]; ok {
		t.Fatalf("claude session/new should not send top-level model: %#v", params)
	}
	if _, ok := params["systemPrompt"]; ok {
		t.Fatalf("claude session/new should not send top-level systemPrompt: %#v", params)
	}
	if _, ok := params["additionalDirectories"]; ok {
		t.Fatalf("claude session/new should not send top-level additionalDirectories: %#v", params)
	}
	meta, ok := params["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("_meta = %T, want map[string]any", params["_meta"])
	}
	claudeCode, ok := meta["claudeCode"].(map[string]any)
	if !ok {
		t.Fatalf("_meta.claudeCode = %T, want map[string]any", meta["claudeCode"])
	}
	options, ok := claudeCode["options"].(map[string]any)
	if !ok {
		t.Fatalf("_meta.claudeCode.options = %T, want map[string]any", claudeCode["options"])
	}
	if got := options["model"]; got != "sonnet" {
		t.Fatalf("_meta.claudeCode.options.model = %v, want sonnet", got)
	}
	dirs, ok := options["additionalDirectories"].([]string)
	if !ok {
		t.Fatalf("_meta.claudeCode.options.additionalDirectories = %T, want []string", options["additionalDirectories"])
	}
	if len(dirs) != 2 || dirs[0] != "/extra/one" || dirs[1] != "/extra/two" {
		t.Fatalf("claude additionalDirectories = %v, want the spec's AddDirs", dirs)
	}
}

// FS-18.A13, TS-04.R69: Claude new and load both send the object shape the
// pinned claude-agent-acp 0.75.1 forwards as a native-preset append; a string
// there replaces Claude Code's coding instructions. The expected wire JSON is
// written out here rather than taken from the builder (INV §17).
func TestClaudeSessionParamsAppendToNativePreset(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prompt string
		want   string
	}{
		{"composed prompt", "be helpful", `{"append":"be helpful","preset":"claude_code","type":"preset"}`},
		{"empty prompt", "", `{"append":"","preset":"claude_code","type":"preset"}`},
	} {
		spec := LaunchSpec{Cwd: "/work", SystemPrompt: tc.prompt, BackendType: "claude-acp"}
		for name, params := range map[string]map[string]any{
			"session/new":  sessionNewParams(spec),
			"session/load": sessionLoadParams(spec, "sess-123"),
		} {
			raw, err := json.Marshal(params)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Meta struct {
					SystemPrompt json.RawMessage `json:"systemPrompt"`
				} `json:"_meta"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			if got := string(wire.Meta.SystemPrompt); got != tc.want {
				t.Fatalf("%s %s _meta.systemPrompt = %s, want %s", tc.name, name, got, tc.want)
			}
		}
	}
}

func TestCodexSessionParamsOmitUnsupportedSystemPrompt(t *testing.T) {
	spec := LaunchSpec{
		Cwd:          "/work",
		SystemPrompt: "be helpful",
		BackendType:  "codex-acp",
	}
	for name, params := range map[string]map[string]any{
		"session/new":  sessionNewParams(spec),
		"session/load": sessionLoadParams(spec, "sess-123"),
	} {
		if _, ok := params["systemPrompt"]; ok {
			t.Fatalf("%s sends unsupported Codex systemPrompt: %#v", name, params)
		}
	}
}

// Regression (review fix, federation §2.4): an empty ModelID means "inherit native
// resolution" — the model flag must be OMITTED so a bound source's native model
// takes effect instead of Chuck forcing a default over ACP. Both backend
// shapes (claude _meta options and the generic top-level) must drop the key.
func TestSessionParamsOmitModelWhenInherited(t *testing.T) {
	t.Run("claude session/new", func(t *testing.T) {
		params := sessionNewParams(LaunchSpec{Cwd: "/work", BackendType: "claude-acp"})
		options := params["_meta"].(map[string]any)["claudeCode"].(map[string]any)["options"].(map[string]any)
		if _, ok := options["model"]; ok {
			t.Fatalf("inherited claude session/new must omit model: %#v", options)
		}
	})
	t.Run("claude session/load", func(t *testing.T) {
		params := sessionLoadParams(LaunchSpec{Cwd: "/work", BackendType: "claude-acp"}, "sess-1")
		options := params["_meta"].(map[string]any)["claudeCode"].(map[string]any)["options"].(map[string]any)
		if _, ok := options["model"]; ok {
			t.Fatalf("inherited claude session/load must omit model: %#v", options)
		}
	})
	t.Run("generic session/new", func(t *testing.T) {
		params := sessionNewParams(LaunchSpec{Cwd: "/work", BackendType: "codex-acp"})
		if _, ok := params["model"]; ok {
			t.Fatalf("inherited generic session/new must omit model: %#v", params)
		}
	})
	t.Run("generic session/load", func(t *testing.T) {
		params := sessionLoadParams(LaunchSpec{Cwd: "/work", BackendType: "codex-acp"}, "sess-1")
		if _, ok := params["model"]; ok {
			t.Fatalf("inherited generic session/load must omit model: %#v", params)
		}
	})
}

// Codex ignores model fields on session creation/load; both values are applied
// through the advertised post-session configuration contract instead.
func TestCodexSessionParamsOmitModelAndEffort(t *testing.T) {
	spec := LaunchSpec{Cwd: "/work", BackendType: "codex-acp", ModelID: "gpt-5", Effort: "high"}
	for name, params := range map[string]map[string]any{
		"session/new":  sessionNewParams(spec),
		"session/load": sessionLoadParams(spec, "sess-123"),
	} {
		if got, ok := params["model"]; ok {
			t.Fatalf("%s model = %v, want omitted", name, got)
		}
	}
}

func TestMCPServerParamUsesNamedPairs(t *testing.T) {
	httpParam := mcpServerParam(MCPServerSpec{
		Name:    "chuck-messaging",
		Type:    "http",
		URL:     "http://127.0.0.1:4318/mcp",
		Headers: map[string]string{"X-Chuck-Token": "tok-123"},
	})
	headers, ok := httpParam["headers"].([]map[string]string)
	if !ok || len(headers) != 1 {
		t.Fatalf("http headers = %#v, want one named pair", httpParam["headers"])
	}
	if headers[0]["name"] != "X-Chuck-Token" || headers[0]["value"] != "tok-123" {
		t.Fatalf("http headers = %#v, want token named pair", headers)
	}

	stdioParam := mcpServerParam(MCPServerSpec{
		Name:    "stdio-server",
		Command: "chuck",
		Args:    []string{"mcp-stdio"},
		Env:     []string{"TOKEN=tok-123"},
	})
	env, ok := stdioParam["env"].([]map[string]string)
	if !ok || len(env) != 1 {
		t.Fatalf("stdio env = %#v, want one named pair", stdioParam["env"])
	}
	if env[0]["name"] != "TOKEN" || env[0]["value"] != "tok-123" {
		t.Fatalf("stdio env = %#v, want TOKEN named pair", env)
	}
}
