package messaging

import "time"

// Locked Phase 5 constants (techspec §13). Budget enforcement at the turn
// boundary and the nudger/janitor loops that use the timing constants land in
// 5.3; the values are fixed here as the single reference.
const (
	// MessageBudgetPerTurn caps combined inbound+outbound messaging per turn.
	MessageBudgetPerTurn = 15

	// NudgeInterval is the nudger ticker period.
	NudgeInterval = 2 * time.Second
	// NudgeCooldown is the minimum gap between re-nudging the same agent.
	NudgeCooldown = 3 * time.Second
	// NudgeInFlightTimeout clears a stuck in-flight nudge flag.
	NudgeInFlightTimeout = 60 * time.Second

	// JanitorInterval is the retention-sweep period.
	JanitorInterval = 60 * time.Second
	// MailReadTTL deletes read messages older than this.
	MailReadTTL = 24 * time.Hour
	// MailHardTTL deletes any message older than this (hard cap).
	MailHardTTL = 168 * time.Hour
)

// check_messages limit bounds (techspec §3.5).
const (
	defaultCheckLimit = 15
	maxCheckLimit     = 50
)

// send_message body/subject bounds (techspec §3.4).
const (
	maxBodyLen    = 8000
	maxSubjectLen = 200
)
