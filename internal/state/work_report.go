package state

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The one definition of what an agent may report about work it was given, used
// by both the pipeline stage report and the task report (TS-10.R7). A second
// copy of any of it is a defect under INV §2.
const (
	MaxResultSummaryRunes = 2000
	MaxResultDetailsRunes = 16000
	MaxResultChecksRunes  = 16000
)

// ErrInvalidOutcome is returned for an outcome outside the agent-reportable
// vocabulary. `cancelled` is deliberately absent: it is host-written only
// (FS-16.R3).
var ErrInvalidOutcome = errors.New("state: outcome must be success, failure, or blocked")

// ErrInvalidReportFields is returned when the report's text does not fit its
// documented bounds.
var ErrInvalidReportFields = errors.New("state: summary is required and report fields must fit their limits")

// AgentReportableOutcome reports whether an agent may record this outcome.
func AgentReportableOutcome(outcome string) bool {
	return outcome == OutcomeSuccess || outcome == OutcomeFailure || outcome == OutcomeBlocked
}

// ValidateAgentReport checks one agent-reported result's vocabulary and bounds.
func ValidateAgentReport(outcome, summary, details, checks string) error {
	if !AgentReportableOutcome(outcome) {
		return fmt.Errorf("%w: got %q", ErrInvalidOutcome, outcome)
	}
	if strings.TrimSpace(summary) == "" ||
		utf8.RuneCountInString(summary) > MaxResultSummaryRunes ||
		utf8.RuneCountInString(details) > MaxResultDetailsRunes ||
		utf8.RuneCountInString(checks) > MaxResultChecksRunes {
		return ErrInvalidReportFields
	}
	return nil
}

// OwnsReportedWork is the staleness check: a caller may report only on the
// assignment its own live generation holds. A stopped-and-resumed agent is the
// same principal but a different generation, and the work it was given belonged
// to the earlier one.
func OwnsReportedWork(callerAgentID, callerGeneration, ownerAgentID, ownerGeneration string) bool {
	return callerAgentID != "" && callerAgentID == ownerAgentID &&
		callerGeneration == ownerGeneration
}
