package strutil

// FirstNonEmpty returns the first non-empty string from vals, or "" if all are empty.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ClipRunes truncates s to at most max runes (not bytes), so a multi-byte string
// is never cut mid-rune. A non-positive max returns the empty string.
func ClipRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
