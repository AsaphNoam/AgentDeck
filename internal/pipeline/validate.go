package pipeline

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/config"
)

// ValidateTemplate is the canonical pure validator used by config reads, CRUD,
// run start, and proposal handling. roles is the current configured role-id set.
func ValidateTemplate(id string, template Template, roles map[string]bool) []Diagnostic {
	t := NormalizeTemplate(template)
	diagnostics := []Diagnostic{}
	add := func(field, code, message string) {
		if len(diagnostics) >= MaxDeclarations {
			return
		}
		diagnostics = append(diagnostics, Diagnostic{Field: field, Code: code, Message: message})
	}

	if !config.ValidSlug(id) {
		add("id", "invalid_slug", "pipeline id must be a lowercase filename-safe slug")
	}
	if t.Version != 1 {
		add("version", "unsupported_version", fmt.Sprintf("version must be 1, got %d", t.Version))
		return diagnostics
	}
	validateText(&diagnostics, "title", t.Title, true, MaxTitleRunes)
	if len(t.Inputs) > MaxDeclarations {
		add("inputs", "too_many", fmt.Sprintf("at most %d run inputs are allowed", MaxDeclarations))
	}
	if len(t.Stages) == 0 {
		add("stages", "required", "at least one stage is required")
	} else if len(t.Stages) > MaxStages {
		add("stages", "too_many", fmt.Sprintf("at most %d stages are allowed", MaxStages))
	}

	values := map[string]bool{}
	runInputNames := map[string]bool{}
	for i, input := range t.Inputs {
		base := fmt.Sprintf("inputs.%d", i)
		validateName(&diagnostics, base+".name", input.Name)
		validateText(&diagnostics, base+".description", input.Description, true, MaxDescriptionRunes)
		if runInputNames[input.Name] {
			add(base+".name", "duplicate", "run input names must be unique")
		}
		runInputNames[input.Name] = true
		values[input.Name] = true
	}

	stageByID := map[string]int{}
	for i, stage := range t.Stages {
		base := fmt.Sprintf("stages.%d", i)
		validateName(&diagnostics, base+".id", stage.ID)
		if _, exists := stageByID[stage.ID]; exists {
			add(base+".id", "duplicate", "stage ids must be unique")
		} else {
			stageByID[stage.ID] = i
		}
		validateText(&diagnostics, base+".title", stage.Title, true, MaxTitleRunes)
		if !config.ValidSlug(stage.Role) || !roles[stage.Role] {
			add(base+".role", "unknown_role", "stage role must name an existing configured role")
		}
		validateText(&diagnostics, base+".instruction", stage.Instruction, true, MaxInstructionRunes)
		if len(stage.Inputs) > MaxDeclarations {
			add(base+".inputs", "too_many", fmt.Sprintf("at most %d inputs are allowed", MaxDeclarations))
		}
		if len(stage.Outputs) > MaxDeclarations {
			add(base+".outputs", "too_many", fmt.Sprintf("at most %d outputs are allowed", MaxDeclarations))
		}
		if stage.MaxVisits < 0 || stage.MaxVisits > MaxVisits {
			add(base+".max_visits", "out_of_range", fmt.Sprintf("max_visits must be between 0 and %d", MaxVisits))
		}
		localNames := map[string]bool{}
		for j, input := range stage.Inputs {
			field := fmt.Sprintf("%s.inputs.%d", base, j)
			validateName(&diagnostics, field+".name", input.Name)
			validateName(&diagnostics, field+".value", input.Value)
			if localNames[input.Name] {
				add(field+".name", "duplicate", "stage-local declaration names must be unique")
			}
			localNames[input.Name] = true
		}
		for j, output := range stage.Outputs {
			field := fmt.Sprintf("%s.outputs.%d", base, j)
			validateName(&diagnostics, field+".name", output.Name)
			validateName(&diagnostics, field+".value", output.Value)
			validateText(&diagnostics, field+".description", output.Description, true, MaxDescriptionRunes)
			if localNames[output.Name] {
				add(field+".name", "duplicate", "stage-local declaration names must be unique")
			}
			localNames[output.Name] = true
			values[output.Value] = true
		}
	}

	graph := map[string][]string{}
	for i, stage := range t.Stages {
		base := fmt.Sprintf("stages.%d", i)
		for j, input := range stage.Inputs {
			if input.Value != "" && !values[input.Value] {
				add(fmt.Sprintf("%s.inputs.%d.value", base, j), "unresolved_value", "input binding must name a declared run input or stage output")
			}
		}
		for outcome, transition := range map[string]Transition{"success": stage.Transitions.Success, "failure": stage.Transitions.Failure} {
			field := base + ".transitions." + outcome
			if (transition.Stage == "") == (transition.Final == "") {
				add(field, "invalid_destination", "transition must contain exactly one stage or final destination")
			}
			if transition.Stage != "" {
				if _, ok := stageByID[transition.Stage]; !ok {
					add(field+".stage", "unknown_stage", "transition destination does not exist")
				} else {
					graph[stage.ID] = append(graph[stage.ID], transition.Stage)
				}
			}
			if transition.Final != "" && transition.Final != "success" && transition.Final != "failure" {
				add(field+".final", "invalid_outcome", "final outcome must be success or failure")
			}
			if transition.Approval != "automatic" && transition.Approval != "required" {
				add(field+".approval", "invalid_approval", "approval must be automatic or required")
			}
		}
	}

	if len(t.Stages) > 0 {
		reachable := map[string]bool{}
		var visit func(string)
		visit = func(id string) {
			if reachable[id] {
				return
			}
			reachable[id] = true
			for _, next := range graph[id] {
				visit(next)
			}
		}
		visit(t.Stages[0].ID)
		for i, stage := range t.Stages {
			if !reachable[stage.ID] {
				add(fmt.Sprintf("stages.%d.id", i), "unreachable", "stage is not reachable from the first stage")
			}
			if participatesInCycle(stage.ID, graph) && stage.MaxVisits <= 0 {
				add(fmt.Sprintf("stages.%d.max_visits", i), "unbounded_cycle", "every stage in a cycle requires a positive max_visits")
			}
		}
	}
	return diagnostics
}

func validateName(diagnostics *[]Diagnostic, field, value string) {
	if !config.ValidSlug(value) {
		*diagnostics = appendBounded(*diagnostics, Diagnostic{Field: field, Code: "invalid_name", Message: "must be a lowercase slug up to 63 characters"})
	}
}

func validateText(diagnostics *[]Diagnostic, field, value string, required bool, limit int) {
	if required && strings.TrimSpace(value) == "" {
		*diagnostics = appendBounded(*diagnostics, Diagnostic{Field: field, Code: "required", Message: "value is required"})
		return
	}
	if utf8.RuneCountInString(value) > limit {
		*diagnostics = appendBounded(*diagnostics, Diagnostic{Field: field, Code: "too_long", Message: fmt.Sprintf("must be at most %d characters", limit)})
	}
}

func appendBounded(items []Diagnostic, item Diagnostic) []Diagnostic {
	if len(items) >= MaxDeclarations {
		return items
	}
	return append(items, item)
}

func participatesInCycle(start string, graph map[string][]string) bool {
	seen := map[string]bool{}
	var reaches func(string) bool
	reaches = func(current string) bool {
		for _, next := range graph[current] {
			if next == start {
				return true
			}
			if !seen[next] {
				seen[next] = true
				if reaches(next) {
					return true
				}
			}
		}
		return false
	}
	return reaches(start)
}
