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
