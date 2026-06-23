package cli

import (
	"fmt"
	"strings"
)

// isLaunchArg reports whether arg uses the reserved `<role>@<project>` launch
// syntax (contains "@" and is not a flag). Phase 1 fills this in with a real
// launch; Phase 0 only stubs it.
func isLaunchArg(arg string) bool {
	if strings.HasPrefix(arg, "-") {
		return false
	}
	return strings.Contains(arg, "@")
}

// runLaunchStub parses the role@project form and prints a not-implemented notice.
// Returns exit code 0 — this is a reserved no-op, not an error.
func runLaunchStub(arg string) int {
	role, project, ok := strings.Cut(arg, "@")
	if !ok || role == "" || project == "" {
		fmt.Printf("invalid launch syntax %q (expected <role>@<project>)\n", arg)
		return 0
	}
	fmt.Printf("launch %s@%s: not yet implemented (Phase 1)\n", role, project)
	return 0
}
