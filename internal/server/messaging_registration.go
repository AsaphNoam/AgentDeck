package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

const messagingMCPName = "agentdeck-messaging"

// registerMessagingMCP wires one agent to the dashboard-owned MCP server
// (techspec §3.6). The live CLI verdict is still gated, so both current chat
// backends receive the preferred HTTP transport entry.
func (s *Server) registerMessagingMCP(agent state.Agent) (runtime.MCPServerSpec, error) {
	token, err := mintMessagingToken()
	if err != nil {
		return runtime.MCPServerSpec{}, err
	}
	if s.messaging == nil {
		return runtime.MCPServerSpec{}, fmt.Errorf("messaging server is not configured")
	}
	s.messaging.Register(token, agent.AgentID)

	// Both current backends (claude-acp, codex-acp) take the in-process HTTP
	// streamable transport. A stdio fallback (an `agentdeck mcp` proxy subcommand)
	// would only be needed if a real CLI rejects HTTP — that's still gated on the
	// live two-CLI acceptance (see HANDOFF "Blocked on human"); it isn't wired
	// (no such subcommand exists), so we don't emit an unreachable/broken branch.
	spec := runtime.MCPServerSpec{
		Name: messagingMCPName,
		Type: "http",
		URL:  fmt.Sprintf("http://127.0.0.1:%d/mcp", s.cfg.Port),
		Headers: map[string]string{
			messaging.TokenHeader: token,
		},
	}

	path, err := s.writeMessagingMCPConfig(agent.AgentID, spec)
	if err != nil {
		s.messaging.Revoke(token)
		return runtime.MCPServerSpec{}, err
	}
	s.rememberMessagingCleanup(agent.AgentID, func() {
		s.messaging.Revoke(token)
		_ = os.Remove(path)
	})
	return spec, nil
}

func (s *Server) writeMessagingMCPConfig(agentID string, spec runtime.MCPServerSpec) (string, error) {
	dir := filepath.Join(s.configStore.Home(), "mcp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("write mcp config: mkdir: %w", err)
	}
	path := filepath.Join(dir, agentID+".mcp.json")
	entry := map[string]any{}
	if spec.Type == "http" {
		entry["type"] = "http"
		entry["url"] = spec.URL
		entry["headers"] = spec.Headers
	} else {
		entry["command"] = spec.Command
		entry["args"] = spec.Args
		if len(spec.Env) > 0 {
			entry["env"] = spec.Env
		}
	}
	body := map[string]any{"mcpServers": map[string]any{spec.Name: entry}}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return "", fmt.Errorf("write mcp config: marshal: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write mcp config: %w", err)
	}
	return path, nil
}

func mintMessagingToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("mint messaging token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *Server) rememberMessagingCleanup(agentID string, cleanup func()) {
	s.hookMu.Lock()
	s.mcpCleanups[agentID] = cleanup
	s.hookMu.Unlock()
}

func (s *Server) cleanupMessagingMCP(agentID string) {
	s.hookMu.Lock()
	cleanup := s.mcpCleanups[agentID]
	delete(s.mcpCleanups, agentID)
	s.hookMu.Unlock()
	if cleanup != nil {
		cleanup()
	}
}

func (s *Server) cleanupAllMessagingMCP() {
	s.hookMu.Lock()
	cleanups := s.mcpCleanups
	s.mcpCleanups = map[string]func(){}
	s.hookMu.Unlock()
	for _, cleanup := range cleanups {
		if cleanup != nil {
			cleanup()
		}
	}
}
