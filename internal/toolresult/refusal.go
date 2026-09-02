package toolresult

// RetryClass is the stable FS-17 retry vocabulary returned with every refused
// AgentDeck action.
type RetryClass string

const (
	RetryNever       RetryClass = "never"
	RetryAfterChange RetryClass = "after_change"
	RetryNextTurn    RetryClass = "next_turn"
	RetryTransient   RetryClass = "transient"
)

var retryClasses = map[string]RetryClass{
	"validation": RetryNever, "invalid_body": RetryNever, "invalid_subject": RetryNever,
	"invalid_outcome": RetryNever, "invalid_state": RetryNever, "invalid_cursor": RetryNever,
	"dependency_cycle": RetryNever, "target_ineligible": RetryNever, "already_reported": RetryNever,
	"not_assigned": RetryNever, "not_creator": RetryNever, "retry_requires_rearm": RetryNever,
	"task_not_found": RetryNever, "context_not_found": RetryNever, "context_source_unavailable": RetryNever,
	"proposal_forbidden": RetryNever, "session_unknown": RetryNever, "assignment_unknown": RetryNever,
	"stale_assignment":    RetryNever,
	"ambiguous_recipient": RetryAfterChange, "recipient_not_found": RetryAfterChange,
	"source_unavailable": RetryAfterChange, "validation_failed": RetryAfterChange,
	"message_budget_exceeded": RetryNextTurn,
	"internal":                RetryTransient, "store_unavailable": RetryTransient,
	"context_unavailable": RetryTransient, "pipeline_unavailable": RetryTransient,
}

func Classify(code string) RetryClass {
	if class, ok := retryClasses[code]; ok {
		return class
	}
	return RetryTransient
}

func Classes() map[string]RetryClass {
	copy := make(map[string]RetryClass, len(retryClasses))
	for code, class := range retryClasses {
		copy[code] = class
	}
	return copy
}

// StageReportGuidance derives the agent-facing consequence from the same class
// returned in the structured refusal. A refused call is not an accepted result.
func StageReportGuidance(code string) string {
	switch Classify(code) {
	case RetryAfterChange:
		return "AgentDeck did not accept this result and recorded nothing. This attempt still owes a result from you; correct the reported fields using the diagnostics, then call report_pipeline_stage_result again."
	case RetryTransient:
		return "AgentDeck did not accept this result and recorded nothing. This attempt still owes a result from you; retry report_pipeline_stage_result when the temporary problem is resolved."
	default:
		return ""
	}
}
