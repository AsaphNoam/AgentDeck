// Package contextref owns pull-based context links: canonicalization of typed
// immutable source locators, source validation, bounded source composition,
// direct-grant authorization, and the recipient's personal share projection
// (TS-01.R22, FS-15).
//
// It delegates durable rows to internal/state, normalized-event projection to
// internal/transcript, pipeline-attempt reads to the state pipeline authority,
// and agent-facing transport to internal/messaging. It stores no copied source
// payload and calls no runtime prompt, activation, mail, lifecycle, SSE, or
// local HTTP path: creating, granting, listing, hiding, and revoking a
// reference start no model turn (FS-15.R10).
package contextref

// Bounds shared by the MCP tools, this service, its renderers, and their tests
// (TS-04.R28). They live here so no surface can quietly widen a limit.
const (
	// MaxLabelRunes bounds a grant's label.
	MaxLabelRunes = 200
	// MaxDescriptionRunes bounds a grant's description.
	MaxDescriptionRunes = 1000

	// DefaultListLimit and the accepted list range for list_context_links.
	DefaultListLimit = 20
	MinListLimit     = 1
	MaxListLimit     = 50

	// MaxPageBytes caps one read_context_link content page.
	MaxPageBytes = 32 * 1024
)

// OversizedRecordMarker replaces a physical transcript record the tolerant
// reader skipped for exceeding its 8 MiB safety limit. Context composition
// never returns a clean page that silently implies the record was rendered
// (TS-01.R22, TS-04.R28, INV §7).
const OversizedRecordMarker = "[AgentDeck omitted an oversized transcript record]"
