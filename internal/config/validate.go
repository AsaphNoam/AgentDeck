package config

import (
	"fmt"
	"os"
	"regexp"
)

// ValidSlug reports whether s is a valid role/project id. The id must begin
// with a lowercase letter or digit and contain only lowercase letters, digits,
// and hyphens, up to 63 characters total. This prevents path traversal (no
// slashes, dots, whitespace, uppercase) and keeps filenames safe.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func ValidSlug(s string) bool { return slugRE.MatchString(s) }

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
