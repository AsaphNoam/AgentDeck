package pipeline

import "github.com/agentdeck/agentdeck/internal/state"

// Limits is the single authority for pipeline JSON, prompt, report, proposal,
// and list bounds (TS-09.R19). Values are deliberately conservative for a local
// control plane while leaving enough room for detailed work specifications.
const (
	MaxTemplates        = 64
	MaxTemplateBytes    = 256 * 1024
	MaxStages           = 32
	MaxDeclarations     = 64
	MaxTitleRunes       = 120
	MaxDescriptionRunes = 1000
	MaxInstructionRunes = 16000
	MaxGoalRunes        = 16000
	MaxValueRunes       = 64000
	MaxProposalRecords  = 100
	MaxListPage         = 100
	MaxDelegatedAgents  = 20
	MaxVisits           = 32
	MaxProposalBytes    = 256 * 1024
)

// The report bounds are the shared work-result limits, not a second set: one
// vocabulary and one set of field limits serve a stage report and a task report
// alike (TS-10.R7, FS-16.R3).
const (
	MaxSummaryRunes = state.MaxResultSummaryRunes
	MaxDetailsRunes = state.MaxResultDetailsRunes
	MaxChecksRunes  = state.MaxResultChecksRunes
)
