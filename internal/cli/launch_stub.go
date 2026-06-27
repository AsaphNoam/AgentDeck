package cli

import "strings"

// isLaunchArg reports whether arg uses the reserved `<role>@<project>` launch
// syntax (contains "@" and is not a flag). The real launch lives in launch.go.
func isLaunchArg(arg string) bool {
	if strings.HasPrefix(arg, "-") {
		return false
	}
	return strings.Contains(arg, "@")
}
