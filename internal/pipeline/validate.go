package pipeline

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/config"
)

// ValidateTemplate is the canonical pure validator for version-2 templates.
func ValidateTemplate(id string, t Template, roles map[string]bool) []Diagnostic {
	t = NormalizeTemplate(t)
	d := []Diagnostic{}
	add := func(f, c, m string) {
		if len(d) < MaxDeclarations {
			d = append(d, Diagnostic{Field: f, Code: c, Message: m})
		}
	}
	if !config.ValidSlug(id) {
		add("id", "invalid_slug", "pipeline id must be a lowercase filename-safe slug")
	}
	if t.Version != 2 {
		add("version", "unsupported_version", fmt.Sprintf("version must be 2, got %d", t.Version))
		return d
	}
	validateText(&d, "title", t.Title, true, MaxTitleRunes)
	if t.Executor != "" {
		add("executor", "legacy_field", "executor is not supported in version 2")
	}
	if !config.ValidSlug(t.OrchestratorRole) || !roles[t.OrchestratorRole] {
		add("orchestrator_role", "unknown_role", "orchestrator_role must name an existing configured role")
	}
	if len(t.Inputs) > MaxDeclarations {
		add("inputs", "too_many", fmt.Sprintf("at most %d run inputs are allowed", MaxDeclarations))
	}
	if len(t.Stages) == 0 {
		add("stages", "required", "at least one stage is required")
	} else if len(t.Stages) > 32 {
		add("stages", "too_many", "at most 32 stages are allowed")
	}
	values := map[string]bool{}
	names := map[string]bool{}
	for i, in := range t.Inputs {
		f := fmt.Sprintf("inputs.%d", i)
		validateName(&d, f+".name", in.Name)
		validateText(&d, f+".description", in.Description, true, MaxDescriptionRunes)
		if names[in.Name] {
			add(f+".name", "duplicate", "run input names must be unique")
		}
		names[in.Name] = true
		values[in.Name] = true
	}
	ids := map[string]bool{}
	produced := map[string]int{}
	for i, st := range t.Stages {
		b := fmt.Sprintf("stages.%d", i)
		validateName(&d, b+".id", st.ID)
		if ids[st.ID] {
			add(b+".id", "duplicate", "stage ids must be unique")
		}
		ids[st.ID] = true
		validateText(&d, b+".title", st.Title, true, MaxTitleRunes)
		validateText(&d, b+".objective", st.Objective, true, MaxInstructionRunes)
		if st.Instruction != "" && st.Instruction != st.Objective {
			add(b+".instruction", "legacy_field", "instruction is replaced by objective")
		}
		if st.Role != "" {
			add(b+".role", "legacy_field", "stage role is replaced by orchestrator_role")
		}
		c := st.Coordination
		if c == "" {
			c = "standing"
		}
		if c != "standing" && c != "dedicated" {
			add(b+".coordination", "invalid_coordination", "coordination must be standing or dedicated")
		}
		if c == "dedicated" {
			if !config.ValidSlug(st.DedicatedRole) || !roles[st.DedicatedRole] {
				add(b+".dedicated_role", "unknown_role", "dedicated_role must name an existing configured role")
			}
		} else if st.DedicatedRole != "" {
			add(b+".dedicated_role", "unexpected", "dedicated_role requires dedicated coordination")
		}
		if st.MaxVisits != 0 {
			add(b+".max_visits", "legacy_field", "max_visits and cyclic routing are not supported in version 2")
		}
		if st.Transitions != (OutcomeTransitions{}) {
			add(b+".transitions", "legacy_field", "transitions are not supported in version 2")
		}
		if len(st.Inputs) > MaxDeclarations {
			add(b+".inputs", "too_many", fmt.Sprintf("at most %d inputs are allowed", MaxDeclarations))
		}
		if len(st.Outputs) > MaxDeclarations {
			add(b+".outputs", "too_many", fmt.Sprintf("at most %d outputs are allowed", MaxDeclarations))
		}
		local := map[string]bool{}
		for j, in := range st.Inputs {
			f := fmt.Sprintf("%s.inputs.%d", b, j)
			validateName(&d, f+".name", in.Name)
			validateName(&d, f+".value", in.Value)
			if local[in.Name] {
				add(f+".name", "duplicate", "stage-local declaration names must be unique")
			}
			local[in.Name] = true
			if !values[in.Value] {
				add(f+".value", "unresolved_value", "input binding must name a run input or an output of an earlier stage")
			} else if p, ok := produced[in.Value]; ok && p >= i {
				add(f+".value", "future_binding", "input binding must name an output of an earlier stage")
			}
		}
		for j, out := range st.Outputs {
			f := fmt.Sprintf("%s.outputs.%d", b, j)
			validateName(&d, f+".name", out.Name)
			validateName(&d, f+".value", out.Value)
			validateText(&d, f+".description", out.Description, true, MaxDescriptionRunes)
			if local[out.Name] {
				add(f+".name", "duplicate", "stage-local declaration names must be unique")
			}
			local[out.Name] = true
			if _, ok := produced[out.Value]; ok {
				add(f+".value", "duplicate", "stage output value keys must have one producer")
			} else {
				produced[out.Value] = i
				values[out.Value] = true
			}
		}
	}
	return d
}
func validateName(d *[]Diagnostic, f, v string) {
	if !config.ValidSlug(v) {
		*d = appendBounded(*d, Diagnostic{Field: f, Code: "invalid_name", Message: "must be a lowercase slug up to 63 characters"})
	}
}
func validateText(d *[]Diagnostic, f, v string, req bool, limit int) {
	if req && strings.TrimSpace(v) == "" {
		*d = appendBounded(*d, Diagnostic{Field: f, Code: "required", Message: "value is required"})
		return
	}
	if utf8.RuneCountInString(v) > limit {
		*d = appendBounded(*d, Diagnostic{Field: f, Code: "too_long", Message: fmt.Sprintf("must be at most %d characters", limit)})
	}
}
func appendBounded(x []Diagnostic, v Diagnostic) []Diagnostic {
	if len(x) >= MaxDeclarations {
		return x
	}
	return append(x, v)
}
