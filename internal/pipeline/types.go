package pipeline

// Template is the version-1 model-neutral pipeline configuration stored under
// pipelines/{id}.json. Its immutable id comes from the filename and therefore
// is not duplicated in this document.
type Template struct {
	Version          int         `json:"version"`
	Title            string      `json:"title"`
	OrchestratorRole string      `json:"orchestrator_role"`
	Inputs           []ValueDecl `json:"inputs"`
	Stages           []Stage     `json:"stages"`
	// Executor is retained solely to diagnose old documents. It is not valid
	// in version 2 templates.
	Executor string `json:"executor,omitempty"`
}

type ValueDecl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type Stage struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Objective            string `json:"objective"`
	Coordination         string `json:"coordination,omitempty"`
	DedicatedRole        string `json:"dedicated_role,omitempty"`
	ApprovalAfterSuccess bool   `json:"approval_after_success,omitempty"`
	// Legacy fields remain decodable so hand-edited v1 files can receive a
	// useful diagnostic instead of silently losing data.
	Role        string             `json:"role,omitempty"`
	Instruction string             `json:"instruction,omitempty"`
	Inputs      []StageInput       `json:"inputs"`
	Outputs     []StageOutput      `json:"outputs"`
	MaxVisits   int                `json:"max_visits,omitempty"`
	Transitions OutcomeTransitions `json:"transitions,omitempty"`
}

type StageInput struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Required bool   `json:"required"`
}

type StageOutput struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type OutcomeTransitions struct {
	Success Transition `json:"success"`
	Failure Transition `json:"failure"`
}

type Transition struct {
	Stage    string `json:"stage,omitempty"`
	Final    string `json:"final,omitempty"`
	Approval string `json:"approval"`
}

// Diagnostic is a bounded field-addressed template validation error.
type Diagnostic struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type TemplateRecord struct {
	ID          string       `json:"id"`
	Template    Template     `json:"template"`
	Valid       bool         `json:"valid"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// NormalizeTemplate gives every collection a non-nil JSON shape without
// changing template semantics.
func NormalizeTemplate(t Template) Template {
	if t.Inputs == nil {
		t.Inputs = []ValueDecl{}
	}
	if t.Stages == nil {
		t.Stages = []Stage{}
	}
	for i := range t.Stages {
		if t.Stages[i].Inputs == nil {
			t.Stages[i].Inputs = []StageInput{}
		}
		if t.Stages[i].Outputs == nil {
			t.Stages[i].Outputs = []StageOutput{}
		}
	}
	return t
}
