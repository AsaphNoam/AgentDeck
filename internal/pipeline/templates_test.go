package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AsaphNoam/Chuck/internal/config"
)

func validTemplate() Template {
	return Template{
		Version:          2,
		Title:            "Implement and verify",
		OrchestratorRole: "implementer",
		Inputs:           []ValueDecl{{Name: "spec", Description: "Specification", Required: true}},
		Stages: []Stage{
			{
				ID: "work", Title: "Work", Objective: "Implement the change.", Instruction: "Implement the change.",
				Inputs:  []StageInput{{Name: "specification", Value: "spec", Required: true}},
				Outputs: []StageOutput{{Name: "implementation", Value: "implementation", Description: "What changed"}},
			},
			{
				ID: "review", Title: "Review", Objective: "Review the change.", Instruction: "Review the change.",
				Inputs:  []StageInput{{Name: "implementation", Value: "implementation", Required: true}},
				Outputs: []StageOutput{},
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

// Version 2 only permits forward bindings and a single standing owner.
func TestTemplateValidationCoversBindingsAndVersionTwoShape(t *testing.T) {
	template := validTemplate()
	diagnostics := ValidateTemplate("quality", template, map[string]bool{"implementer": true, "reviewer": true})
	if len(diagnostics) != 0 {
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
	template.Version = 1
	record, err := service.Create("quality", template)
	if err != nil || record.Valid || !hasDiagnostic(record.Diagnostics, "unsupported_version") {
		t.Fatalf("invalid Create = %+v err %v", record, err)
	}
	template.Version = 2
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

func thinkTankTemplate() Template {
	template := validTemplate()
	template.Stages = append(template.Stages[:1], Stage{
		ID: "deliberate", Title: "Deliberate", Objective: "Weigh the approach.", Coordination: CoordinationThinkTank,
		ThinkTank: &ThinkTankStage{
			Participants: []ThinkTankParticipant{{ID: "a", Role: "reviewer", Limit: 2}, {ID: "b", Role: "reviewer", Limit: 3, MayLeave: true}},
			Openings:     true, JudgeRole: "implementer",
		},
		Inputs:  []StageInput{{Name: "implementation", Value: "implementation", Required: true}},
		Outputs: []StageOutput{{Name: "synthesis", Value: "synthesis", Description: "Judge synthesis"}},
	}, template.Stages[1])
	template.Stages[2].Inputs = []StageInput{{Name: "synthesis", Value: "synthesis", Required: true}}
	return template
}

// A think_tank stage repeats roles, needs a judge and exactly one output (TS-09.R51).
func TestTemplateValidationThinkTankStage(t *testing.T) {
	roles := map[string]bool{"implementer": true, "reviewer": true}
	if diagnostics := ValidateTemplate("quality", thinkTankTemplate(), roles); len(diagnostics) != 0 {
		t.Fatalf("valid think tank diagnostics = %+v", diagnostics)
	}
	cases := map[string]struct {
		mutate func(*Stage)
		field  string
	}{
		"missing config":   {func(s *Stage) { s.ThinkTank = nil }, "stages.1.think_tank"},
		"one participant":  {func(s *Stage) { s.ThinkTank.Participants = s.ThinkTank.Participants[:1] }, "stages.1.think_tank.participants"},
		"duplicate id":     {func(s *Stage) { s.ThinkTank.Participants[1].ID = "a" }, "stages.1.think_tank.participants.1.id"},
		"unknown role":     {func(s *Stage) { s.ThinkTank.Participants[0].Role = "ghost" }, "stages.1.think_tank.participants.0.role"},
		"limit zero":       {func(s *Stage) { s.ThinkTank.Participants[0].Limit = 0 }, "stages.1.think_tank.participants.0.limit"},
		"limit too high":   {func(s *Stage) { s.ThinkTank.Participants[0].Limit = 1001 }, "stages.1.think_tank.participants.0.limit"},
		"no judge":         {func(s *Stage) { s.ThinkTank.JudgeRole = "" }, "stages.1.think_tank.judge_role"},
		"no output":        {func(s *Stage) { s.Outputs = nil }, "stages.1.outputs"},
		"two outputs":      {func(s *Stage) { s.Outputs = append(s.Outputs, StageOutput{Name: "x", Value: "x", Description: "x"}) }, "stages.1.outputs"},
		"dedicated role":   {func(s *Stage) { s.DedicatedRole = "reviewer" }, "stages.1.dedicated_role"},
		"config elsewhere": {func(s *Stage) { s.Coordination = CoordinationStanding }, "stages.1.think_tank"},
	}
	for name, tc := range cases {
		template := thinkTankTemplate()
		tc.mutate(&template.Stages[1])
		diagnostics := ValidateTemplate("quality", template, roles)
		found := false
		for _, d := range diagnostics {
			found = found || d.Field == tc.field
		}
		if !found {
			t.Errorf("%s: diagnostics = %+v, want field %s", name, diagnostics, tc.field)
		}
	}
}
