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
