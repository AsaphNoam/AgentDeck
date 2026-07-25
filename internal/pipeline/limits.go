package pipeline

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
	MaxSummaryRunes     = 2000
	MaxDetailsRunes     = 16000
	MaxChecksRunes      = 16000
	MaxProposalBytes    = 256 * 1024
	MaxListPage         = 100
	MaxVisits           = 32
)
