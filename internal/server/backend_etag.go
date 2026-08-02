package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/agentdeck/agentdeck/internal/config"
)

// backendCatalogETag is a strong validator for the complete backend document.
// Encoding/json orders map keys, so equivalent catalog values produce the same
// validator without adding mutable revision state to backends.json.
func backendCatalogETag(backends config.BackendsConfig) string {
	body, err := json.Marshal(backends)
	if err != nil {
		// BackendsConfig has no unsupported JSON values. Keep a fixed fallback in
		// case a future field changes that invariant rather than leaking an error.
		return `"unavailable"`
	}
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}
