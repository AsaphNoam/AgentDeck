package runtime

import "testing"

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
	if got := meta["systemPrompt"]; got != "be helpful" {
		t.Fatalf("_meta.systemPrompt = %v, want %q", got, "be helpful")
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

// Regression (review fix, federation §2.4): an empty ModelID means "inherit native
// resolution" — the model flag must be OMITTED so a bound source's native model
// takes effect instead of AgentDeck forcing a default over ACP. Both backend
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

func TestMCPServerParamUsesNamedPairs(t *testing.T) {
	httpParam := mcpServerParam(MCPServerSpec{
		Name:    "agentdeck-messaging",
		Type:    "http",
		URL:     "http://127.0.0.1:4318/mcp",
		Headers: map[string]string{"X-AgentDeck-Token": "tok-123"},
	})
	headers, ok := httpParam["headers"].([]map[string]string)
	if !ok || len(headers) != 1 {
		t.Fatalf("http headers = %#v, want one named pair", httpParam["headers"])
	}
	if headers[0]["name"] != "X-AgentDeck-Token" || headers[0]["value"] != "tok-123" {
		t.Fatalf("http headers = %#v, want token named pair", headers)
	}

	stdioParam := mcpServerParam(MCPServerSpec{
		Name:    "stdio-server",
		Command: "agentdeck",
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
