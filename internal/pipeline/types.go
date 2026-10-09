package pipeline

// Template is the version-1 model-neutral pipeline configuration stored under
// pipelines/{id}.json. Its immutable id comes from the filename and therefore
// is not duplicated in this document.
type Template struct {
	Version          int    `json:"version"`
	Title            string `json:"title"`
	OrchestratorRole string `json:"orchestrator_role"`
	// OrchestratorInstructions are shared standing guidance frozen into each
	// run-created orchestrator's system prompt (TS-09.R57–R59).
	OrchestratorInstructions string      `json:"orchestrator_instructions,omitempty"`
	Inputs                   []ValueDecl `json:"inputs"`
	Stages                   []Stage     `json:"stages"`
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
	ID            string `json:"id"`
	Title         string `json:"title"`
	Objective     string `json:"objective"`
	Coordination  string `json:"coordination,omitempty"`
	DedicatedRole string `json:"dedicated_role,omitempty"`
	// ThinkTank configures a think_tank coordination stage (TS-09.R51).
	ThinkTank            *ThinkTankStage `json:"think_tank,omitempty"`
	ApprovalAfterSuccess bool            `json:"approval_after_success,omitempty"`
	// Legacy fields remain decodable so hand-edited v1 files can receive a
	// useful diagnostic instead of silently losing data.
	Role        string             `json:"role,omitempty"`
	Instruction string             `json:"instruction,omitempty"`
	Inputs      []StageInput       `json:"inputs"`
	Outputs     []StageOutput      `json:"outputs"`
	MaxVisits   int                `json:"max_visits,omitempty"`
	Transitions OutcomeTransitions `json:"transitions,omitempty"`
}

// ThinkTankStage is a model-neutral room configuration: runtimes are supplied
// per run by think_tank_assignments, never stored in the template.
type ThinkTankStage struct {
	Participants []ThinkTankParticipant `json:"participants"`
	Openings     bool                   `json:"openings,omitempty"`
	JudgeRole    string                 `json:"judge_role"`
}

type ThinkTankParticipant struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Limit    int    `json:"limit"`
	MayLeave bool   `json:"may_leave,omitempty"`
}

const (
	CoordinationStanding  = "standing"
	CoordinationDedicated = "dedicated"
	CoordinationThinkTank = "think_tank"
)

// StageCoordination normalizes an absent coordination to standing.
func StageCoordination(st Stage) string {
	if st.Coordination == "" {
		return CoordinationStanding
	}
	return st.Coordination
}

// firstOrdinaryStage is the stage that first needs the standing owner.
func firstOrdinaryStage(t Template) (Stage, bool) {
	for _, stage := range t.Stages {
		if StageCoordination(stage) != CoordinationThinkTank {
			return stage, true
		}
	}
	return Stage{}, false
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
		if tt := t.Stages[i].ThinkTank; tt != nil {
			copied := *tt
			if copied.Participants == nil {
				copied.Participants = []ThinkTankParticipant{}
			}
			t.Stages[i].ThinkTank = &copied
		}
	}
	return t
}
