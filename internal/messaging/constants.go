package messaging

import "time"

// Messaging budget and timing constants; see TS-04 and FS-06 for the
// requirements that govern their use.
const (
	// MessageBudgetPerTurn caps combined inbound+outbound messaging per turn.
	MessageBudgetPerTurn = 15

	// ActivationSweepInterval is the mail activation executor's ticker period.
	ActivationSweepInterval = 2 * time.Second
	// ActivationBatch bounds one sweep both ways: how many pending rows it reads
	// and how many activations may be in flight at once. Each one can resume a
	// stopped recipient, which starts a CLI process, so an unbounded backlog of
	// agents with waiting mail would launch all of them together every tick. The
	// rows it leaves behind are durable and the next sweep takes them.
	ActivationBatch = 8

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
