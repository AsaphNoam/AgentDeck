package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// ValidSlug reports whether s is a valid role/project id. The id must begin
// with a lowercase letter or digit and contain only lowercase letters, digits,
// and hyphens, up to 63 characters total. This prevents path traversal (no
// slashes, dots, whitespace, uppercase) and keeps filenames safe.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func ValidSlug(s string) bool { return slugRE.MatchString(s) }

// projectIDTimestamp is the length of the "-YYYYMMDDThhmmssz" suffix that
// GenerateProjectID appends (hyphen + 8 date + "t" + 6 time + "z" = 17).
const projectIDTimestamp = len("-20060102t150405z")

// GenerateProjectID derives a unique, filesystem-safe project id from a title
// (FS-04.R31). The result is a lowercase slug of the title plus a local-time
// timestamp suffix, e.g. title "AgentDeck Demo" at 2026-07-14 20:28:25 local ->
// "agentdeck-demo-20260714t202825z". The whole id always satisfies ValidSlug.
// Callers that supply their own id bypass this; it is only used when POST
// /api/projects omits the id.
func GenerateProjectID(title string, now time.Time) string {
	base := slugify(title)
	if base == "" {
		base = "project"
	}
	// Leave room for the timestamp suffix within the 63-char slug budget.
	if maxBase := 63 - projectIDTimestamp; len(base) > maxBase {
		base = strings.Trim(base[:maxBase], "-")
		if base == "" {
			base = "project"
		}
	}
	ts := now.Format("20060102") + "t" + now.Format("150405") + "z"
	return base + "-" + ts
}

// slugify lowercases s and collapses every run of characters outside [a-z0-9]
// into a single hyphen, trimming leading/trailing hyphens. ASCII-only: other
// runes are treated as separators (a title of only such runes yields "").
func slugify(s string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
		} else {
			pendingHyphen = true
		}
	}
	return b.String()
}

// FieldError is one entry in the Phase 3 §5.6 validation error shape.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidationErrors is the §5.6 validation error envelope.
type ValidationErrors struct {
	Errors []FieldError `json:"errors"`
}

func (e ValidationErrors) Error() string {
	return fmt.Sprintf("validation_failed: %d error(s)", len(e.Errors))
}

// ValidateRole validates the fields of a Role. Returns nil or a ValidationErrors.
func ValidateRole(slug string, r Role, checkSlug bool) *ValidationErrors {
	var errs []FieldError
	if checkSlug {
		if !ValidSlug(slug) {
			errs = append(errs, FieldError{
				Field:   "role",
				Code:    "invalid_slug",
				Message: `must match ^[a-z0-9][a-z0-9-]{0,62}$`,
			})
		}
	}
	if r.Title == "" {
		errs = append(errs, FieldError{Field: "title", Code: "required", Message: "title is required"})
	} else if len(r.Title) > 120 {
		errs = append(errs, FieldError{Field: "title", Code: "too_long", Message: "title must be ≤ 120 characters"})
	}
	// system_prompt may be empty string but the field must be present in input;
	// since Role is a struct decode, a missing field yields "". No extra check needed.
	if len(errs) == 0 {
		return nil
	}
	return &ValidationErrors{Errors: errs}
}

// ValidateProject validates Project fields. Returns nil or a ValidationErrors.
// Warnings (non-blocking) are returned separately via FieldError with appropriate codes.
func ValidateProject(slug string, p Project, checkSlug bool) (errs *ValidationErrors, warnings []FieldError) {
	var errList []FieldError
	if checkSlug {
		if !ValidSlug(slug) {
			errList = append(errList, FieldError{
				Field:   "project",
				Code:    "invalid_slug",
				Message: `must match ^[a-z0-9][a-z0-9-]{0,62}$`,
			})
		}
	}
	if p.Title == "" {
		errList = append(errList, FieldError{Field: "title", Code: "required", Message: "title is required"})
	} else if len(p.Title) > 120 {
		errList = append(errList, FieldError{Field: "title", Code: "too_long", Message: "title must be ≤ 120 characters"})
	}
	if p.Cwd == "" {
		errList = append(errList, FieldError{Field: "cwd", Code: "required", Message: "cwd is required"})
	}
	for i, c := range p.Color {
		if c < 0 || c > 255 {
			errList = append(errList, FieldError{
				Field:   "color",
				Code:    "out_of_range",
				Message: fmt.Sprintf("color[%d] must be 0–255", i),
			})
			break
		}
	}
	if len(errList) > 0 {
		return &ValidationErrors{Errors: errList}, nil
	}
	// Non-blocking cwd existence check (warning, not an error).
	if p.Cwd != "" {
		expanded, err := ExpandTilde(p.Cwd)
		if err != nil || !isDirExist(expanded) {
			warnings = append(warnings, FieldError{
				Field:   "cwd",
				Code:    "cwd_not_found",
				Message: fmt.Sprintf("directory %s does not exist yet", p.Cwd),
			})
		}
	}
	return nil, warnings
}

// isDirExist reports whether path exists and is a directory.
func isDirExist(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// ---- Backends validators (§3.3 invariants 1–6) ----

// knownBackendTypes are the accepted values for Backend.Type (the four-value
// union: two ACP CLIs from Phase 1/6 plus the Phase 7 OpenCode/OpenHands).
var knownBackendTypes = map[string]bool{
	"claude-acp":    true,
	"codex-acp":     true,
	"opencode-acp":  true,
	"openhands-acp": true,
}

// ValidateBackendsConfig validates the structural invariants for a BackendsConfig
// document. It does NOT run credential checks (those are handled by the handler).
// Returns nil on success, or a ValidationErrors listing every violation.
// Auto-promotions (§3.3 default invariants) are applied to the document in-place
// before validation so callers get the normalized copy back.
func ValidateBackendsConfig(b *BackendsConfig) *ValidationErrors {
	var errs []FieldError

	// Invariant 1: version must be 2.
	if b.Version != 2 {
		errs = append(errs, FieldError{
			Field:   "version",
			Code:    "unsupported_version",
			Message: fmt.Sprintf("version must be 2, got %d", b.Version),
		})
		return &ValidationErrors{Errors: errs} // hard stop; rest is meaningless
	}

	// Invariant 2: exactly one default backend (auto-promote if zero).
	defaultCount := 0
	for _, bk := range b.Backends {
		if bk.Default {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		errs = append(errs, FieldError{
			Field:   "backends",
			Code:    "multiple_default_backends",
			Message: "exactly one backend must have default=true",
		})
	} else if defaultCount == 0 && len(b.Backends) > 0 {
		// Auto-promote the first (sorted) backend.
		autoPromoteDefaultBackend(b)
	}

	// Per-backend invariants (3–6).
	for id, bk := range b.Backends {
		// Invariant 5: type must be known.
		if !knownBackendTypes[bk.Type] {
			errs = append(errs, FieldError{
				Field:   fmt.Sprintf("backends.%s.type", id),
				Code:    "unknown_backend_type",
				Message: fmt.Sprintf("unknown backend type %q; must be one of claude-acp, codex-acp, opencode-acp, openhands-acp", bk.Type),
			})
		}
		// Invariant 4: at least one model.
		if len(bk.Models) == 0 {
			errs = append(errs, FieldError{
				Field:   fmt.Sprintf("backends.%s.models", id),
				Code:    "backend_without_models",
				Message: fmt.Sprintf("backend %q must have at least one model", id),
			})
			continue
		}
		// Invariant 3: exactly one default model per backend (auto-promote if zero).
		if bk.DefaultModel == "" {
			// Auto-promote sorted-first model.
			autoPromoteDefaultModel(b, id)
			bk = b.Backends[id] // refresh after mutation
		} else if _, exists := bk.Models[bk.DefaultModel]; !exists {
			errs = append(errs, FieldError{
				Field:   fmt.Sprintf("backends.%s.default_model", id),
				Code:    "unknown_default_model",
				Message: fmt.Sprintf("default_model %q not found in models for backend %q", bk.DefaultModel, id),
			})
		}
		// Invariant 6: each model's model field must be non-empty.
		for mID, m := range bk.Models {
			if m.Model == "" {
				errs = append(errs, FieldError{
					Field:   fmt.Sprintf("backends.%s.models.%s.model", id, mID),
					Code:    "required",
					Message: fmt.Sprintf("model %q in backend %q must have a non-empty model field", mID, id),
				})
			}
		}
	}

	if len(errs) > 0 {
		return &ValidationErrors{Errors: errs}
	}
	return nil
}

// autoPromoteDefaultBackend sets Default=true on the backend with the
// lexicographically smallest key. b.Backends must be non-empty.
func autoPromoteDefaultBackend(b *BackendsConfig) {
	first := sortedKeys(b.Backends)[0]
	bk := b.Backends[first]
	bk.Default = true
	b.Backends[first] = bk
}

// autoPromoteDefaultModel sets DefaultModel on the backend entry to the
// lexicographically smallest model key. The backend must have models.
func autoPromoteDefaultModel(b *BackendsConfig, backendID string) {
	bk := b.Backends[backendID]
	first := sortedModelKeys(bk.Models)[0]
	bk.DefaultModel = first
	b.Backends[backendID] = bk
}

func sortedKeys(m map[string]Backend) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortedModelKeys(m map[string]Model) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// sortStrings sorts a string slice in-place (insertion sort; small N).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
