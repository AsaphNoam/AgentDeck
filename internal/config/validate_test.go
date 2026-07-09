package config

import "testing"

func TestValidSlug(t *testing.T) {
	valid := []string{"a", "ab", "a1", "abc-def", "0abc", "a-b-c", "a" + "a" + "a", "implementer", "my-app"}
	for _, s := range valid {
		if !ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = false, want true", s)
		}
	}
	invalid := []string{
		"",                             // empty
		"-abc",                         // leading hyphen
		"ABC",                          // uppercase
		"a b",                          // space
		"a/b",                          // slash
		"../etc",                       // path traversal
		"a.b",                          // dot
		"a_b",                          // underscore
		"A",                            // uppercase single
		"a" + string(make([]byte, 63)), // too long (64 total)
	}
	for _, s := range invalid {
		if ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = true, want false", s)
		}
	}
}

func TestValidateRole(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		r := Role{Title: "Reviewer", SystemPrompt: "Review code.", SkipPermissions: boolPtr(false)}
		if ve := ValidateRole("reviewer", r, true); ve != nil {
			t.Errorf("unexpected error: %v", ve.Errors)
		}
	})
	t.Run("valid null skip_permissions", func(t *testing.T) {
		r := Role{Title: "Reviewer", SystemPrompt: ""}
		if ve := ValidateRole("reviewer", r, true); ve != nil {
			t.Errorf("unexpected error: %v", ve.Errors)
		}
	})
	t.Run("invalid slug", func(t *testing.T) {
		r := Role{Title: "X"}
		ve := ValidateRole("BAD SLUG", r, true)
		if ve == nil {
			t.Fatal("expected validation error")
		}
		if !hasCode(ve, "invalid_slug") {
			t.Errorf("expected invalid_slug, got %v", ve.Errors)
		}
	})
	t.Run("missing title", func(t *testing.T) {
		r := Role{Title: ""}
		ve := ValidateRole("good-slug", r, true)
		if ve == nil {
			t.Fatal("expected validation error")
		}
		if !hasCode(ve, "required") {
			t.Errorf("expected required, got %v", ve.Errors)
		}
	})
	t.Run("title too long", func(t *testing.T) {
		r := Role{Title: string(make([]byte, 121))}
		ve := ValidateRole("good-slug", r, true)
		if ve == nil {
			t.Fatal("expected validation error")
		}
		if !hasCode(ve, "too_long") {
			t.Errorf("expected too_long, got %v", ve.Errors)
		}
	})
	t.Run("skip slug check on PUT", func(t *testing.T) {
		r := Role{Title: "X"}
		// Even a "bad" path id (already validated at route level) should not fail
		// ValidateRole when checkSlug=false.
		if ve := ValidateRole("some-id", r, false); ve != nil {
			t.Errorf("unexpected error: %v", ve.Errors)
		}
	})
}

func TestValidateProject(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		p := Project{Title: "My App", Color: [3]int{100, 180, 255}, Cwd: "/tmp", AddDirs: []string{}}
		ve, warns := ValidateProject("my-app", p, true)
		if ve != nil {
			t.Errorf("unexpected error: %v", ve.Errors)
		}
		// /tmp exists, no warning expected.
		if len(warns) != 0 {
			t.Errorf("unexpected warnings: %v", warns)
		}
	})
	t.Run("missing title", func(t *testing.T) {
		p := Project{Cwd: "/tmp"}
		ve, _ := ValidateProject("my-app", p, true)
		if ve == nil {
			t.Fatal("expected error")
		}
		if !hasCode(ve, "required") {
			t.Errorf("expected required, got %v", ve.Errors)
		}
	})
	t.Run("missing cwd", func(t *testing.T) {
		p := Project{Title: "X"}
		ve, _ := ValidateProject("my-app", p, true)
		if ve == nil {
			t.Fatal("expected error")
		}
		if !hasCode(ve, "required") {
			t.Errorf("expected required, got %v", ve.Errors)
		}
	})
	t.Run("bad color", func(t *testing.T) {
		p := Project{Title: "X", Cwd: "/tmp", Color: [3]int{0, 300, 0}}
		ve, _ := ValidateProject("my-app", p, true)
		if ve == nil {
			t.Fatal("expected error")
		}
		if !hasCode(ve, "out_of_range") {
			t.Errorf("expected out_of_range, got %v", ve.Errors)
		}
	})
	t.Run("invalid slug", func(t *testing.T) {
		p := Project{Title: "X", Cwd: "/tmp"}
		ve, _ := ValidateProject("BAD SLUG", p, true)
		if ve == nil {
			t.Fatal("expected error")
		}
		if !hasCode(ve, "invalid_slug") {
			t.Errorf("expected invalid_slug, got %v", ve.Errors)
		}
	})
	t.Run("cwd_not_found warning", func(t *testing.T) {
		p := Project{Title: "X", Cwd: "/nonexistent-path-99999/xyz"}
		ve, warns := ValidateProject("my-app", p, true)
		if ve != nil {
			t.Fatalf("unexpected error: %v", ve.Errors)
		}
		if len(warns) == 0 {
			t.Fatal("expected cwd_not_found warning")
		}
		found := false
		for _, w := range warns {
			if w.Code == "cwd_not_found" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected cwd_not_found, got %v", warns)
		}
	})
}

func hasCode(ve *ValidationErrors, code string) bool {
	for _, e := range ve.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}
