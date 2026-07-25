package pipeline

// Template is the version-1 model-neutral pipeline configuration stored under
// pipelines/{id}.json. Its immutable id comes from the filename and therefore
// is not duplicated in this document.
type Template struct {
	Version int         `json:"version"`
	Title   string      `json:"title"`
	Inputs  []ValueDecl `json:"inputs"`
	Stages  []Stage     `json:"stages"`
}

type ValueDecl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type Stage struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Role        string             `json:"role"`
	Instruction string             `json:"instruction"`
	Inputs      []StageInput       `json:"inputs"`
	Outputs     []StageOutput      `json:"outputs"`
	MaxVisits   int                `json:"max_visits,omitempty"`
	Transitions OutcomeTransitions `json:"transitions"`
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
