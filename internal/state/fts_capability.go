package state

import (
	"database/sql"
	"fmt"
	"strings"
)

type SessionsFTSMode string

const (
	SessionsFTSFullText    SessionsFTSMode = "full_text"
	SessionsFTSFallback    SessionsFTSMode = "fallback"
	SessionsFTSUnavailable SessionsFTSMode = "unavailable"
)

type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

// DetectSessionsFTS is the shared capability boundary for the derived Archive
// index. A plain table is writable but not searchable with FTS; a virtual table
// whose module cannot load is neither, while authoritative session state remains usable.
func DetectSessionsFTS(q queryRower) (SessionsFTSMode, error) {
	var createSQL string
	if err := q.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'sessions_fts'`).Scan(&createSQL); err != nil {
		return "", fmt.Errorf("state: inspect sessions_fts capability: %w", err)
	}
	if !strings.Contains(strings.ToUpper(createSQL), "VIRTUAL") {
		return SessionsFTSFallback, nil
	}
	var count int
	probeErr := q.QueryRow(`SELECT COUNT(*) FROM sessions_fts WHERE 0`).Scan(&count)
	return classifySessionsFTS(createSQL, probeErr)
}

func classifySessionsFTS(createSQL string, probeErr error) (SessionsFTSMode, error) {
	if !strings.Contains(strings.ToUpper(createSQL), "VIRTUAL") {
		return SessionsFTSFallback, nil
	}
	if probeErr == nil {
		return SessionsFTSFullText, nil
	}
	if strings.Contains(strings.ToLower(probeErr.Error()), "no such module") {
		return SessionsFTSUnavailable, nil
	}
	return "", fmt.Errorf("state: probe sessions_fts capability: %w", probeErr)
}
