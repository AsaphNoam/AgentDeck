package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
)

func validTemplate() Template {
	return Template{
		Version: 1,
		Title:   "Implement and verify",
		Inputs:  []ValueDecl{{Name: "spec", Description: "Specification", Required: true}},
		Stages: []Stage{
			{
				ID: "work", Title: "Work", Role: "implementer", Instruction: "Implement the change.",
				Inputs:  []StageInput{{Name: "specification", Value: "spec", Required: true}},
				Outputs: []StageOutput{{Name: "implementation", Value: "implementation", Description: "What changed"}},
				Transitions: OutcomeTransitions{
					Success: Transition{Stage: "review", Approval: "automatic"},
					Failure: Transition{Final: "failure", Approval: "required"},
				},
			},
			{
				ID: "review", Title: "Review", Role: "reviewer", Instruction: "Review the change.",
				Inputs:  []StageInput{{Name: "implementation", Value: "implementation", Required: true}},
				Outputs: []StageOutput{},
				Transitions: OutcomeTransitions{
					Success: Transition{Final: "success", Approval: "automatic"},
					Failure: Transition{Stage: "work", Approval: "automatic"},
				},
				MaxVisits: 2,
			},
		},
	}
}

func newTemplateStore(t *testing.T) (*TemplateStore, *config.Store) {
	t.Helper()
	home := t.TempDir()
	store := config.NewWithHome(home)
	if err := store.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRole("implementer", config.Role{Title: "Implementer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRole("reviewer", config.Role{Title: "Reviewer"}); err != nil {
		t.Fatal(err)
	}
	return NewTemplateStore(store), store
}

// FS-14.A9: bindings and bounded routing are rejected before a run can start.
func TestTemplateValidationCoversRoutesBindingsAndCycles(t *testing.T) {
	template := validTemplate()
	diagnostics := ValidateTemplate("quality", template, map[string]bool{"implementer": true, "reviewer": true})
	if len(diagnostics) == 0 || diagnostics[0].Code != "unbounded_cycle" {
		t.Fatalf("diagnostics = %+v, want unbounded work cycle", diagnostics)
	}
	template.Stages[0].MaxVisits = 2
	if diagnostics := ValidateTemplate("quality", template, map[string]bool{"implementer": true, "reviewer": true}); len(diagnostics) != 0 {
		t.Fatalf("valid template diagnostics = %+v", diagnostics)
	}
	template.Stages[1].Inputs[0].Value = "missing"
	if diagnostics := ValidateTemplate("quality", template, map[string]bool{"implementer": true, "reviewer": true}); !hasDiagnostic(diagnostics, "unresolved_value") {
		t.Fatalf("diagnostics = %+v, want unresolved_value", diagnostics)
	}
}

// FS-14.A1: reusable model-neutral templates round-trip independently of runs.
func TestTemplateStoreRoundTripAndInvalidHandEdit(t *testing.T) {
	service, configStore := newTemplateStore(t)
	template := validTemplate()
	template.Stages[0].MaxVisits = 2
	record, err := service.Create("quality", template)
	if err != nil || !record.Valid {
		t.Fatalf("Create = %+v err %v", record, err)
	}
	got, err := service.Read("quality")
	if err != nil || !got.Valid || got.Template.Title != template.Title {
		t.Fatalf("Read = %+v err %v", got, err)
	}
	if got.Template.Inputs == nil || got.Template.Stages[0].Outputs == nil {
		t.Fatalf("collections are nil: %+v", got.Template)
	}

	badPath := filepath.Join(configStore.Home(), "pipelines", "broken.json")
	if err := os.WriteFile(badPath, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "broken" || list[0].Valid || !hasDiagnostic(list[0].Diagnostics, "unsupported_version") {
		t.Fatalf("list = %+v", list)
	}
}

func TestTemplateCreateRefusesInvalidAndExisting(t *testing.T) {
	service, _ := newTemplateStore(t)
	template := validTemplate()
	record, err := service.Create("quality", template)
	if err != nil || record.Valid || !hasDiagnostic(record.Diagnostics, "unbounded_cycle") {
		t.Fatalf("invalid Create = %+v err %v", record, err)
	}
	template.Stages[0].MaxVisits = 2
	if record, err = service.Create("quality", template); err != nil || !record.Valid {
		t.Fatalf("valid Create = %+v err %v", record, err)
	}
	if _, err := service.Create("quality", template); err != ErrTemplateExists {
		t.Fatalf("duplicate Create err = %v, want ErrTemplateExists", err)
	}
}

func hasDiagnostic(items []Diagnostic, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}
