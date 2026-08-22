package contextref

import (
	"encoding/json"
	"sort"
)

// decodeOutputs reads the attempt's declared outputs defensively: a malformed
// or absent object yields no outputs rather than failing the whole read (INV §7).
func decodeOutputs(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

// sortedKeys keeps rendering deterministic across reads and pages.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
