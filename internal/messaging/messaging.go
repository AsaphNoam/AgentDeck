// Package messaging hosts the in-process MCP messaging server (techspec §3).
//
// Phase 5.1 (this file) is the go-sdk handshake spike: it constructs one
// mcp.Server, registers a trivial `ping` tool, and exposes it over the go-sdk
// streamable HTTP transport on the dashboard's existing localhost listener at
// /mcp (techspec §2.2 (A)). The token→agent_id session registry that binds a
// caller's identity to its registered MCP session is stubbed here and filled in
// by RegisterMessagingMCP in 5.3. The three real tools (list_agents,
// send_message, check_messages) land in 5.2.
package messaging

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/version"
)

// TokenHeader is the HTTP header carrying a per-agent session token on the
// streamable HTTP transport (techspec §3.6). The dashboard maps token→agent_id
// at registration so identity is bound to the session, never to a tool argument.
const TokenHeader = "X-AgentDeck-Token"

// Server is the dashboard's in-process MCP messaging server. It owns one
// mcp.Server shared by all agents and the token→agent_id session registry.
type Server struct {
	store *state.Store
	log   *slog.Logger

	mcp     *mcp.Server
	handler http.Handler

	mu       sync.RWMutex
	sessions map[string]string // session token -> agent_id
}

// New constructs the messaging server, registers its tools, and builds the
// streamable HTTP handler. store is the shared state.db handle the real tool
// handlers (5.2) operate on; the spike's ping tool does not touch it.
func New(store *state.Store, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		store:    store,
		log:      log,
		sessions: map[string]string{},
	}

	s.mcp = mcp.NewServer(&mcp.Implementation{
		Name:    "agentdeck-messaging",
		Version: version.String(),
	}, nil)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_agents",
		Description: "List other live agents you can message (by address role@project, name, or agent_id).",
	}, s.handleListAgents)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "send_message",
		Description: "Send a message to another live agent. `to` is role@project, an agent name, or an agent_id.",
	}, s.handleSendMessage)
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "check_messages",
		Description: "Read your pending messages; flags them read (or deletes) as requested.",
	}, s.handleCheckMessages)

	// getServer resolves the per-request server. Reading the token header here
	// proves the per-agent session binding arrives over the transport (§3.1);
	// 5.2 uses it to scope tool identity. One shared server for now.
	s.handler = mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		if tok := r.Header.Get(TokenHeader); tok != "" {
			if agentID, ok := s.Lookup(tok); ok {
				s.log.Debug("mcp session resolved", "agent", agentID)
			}
		}
		return s.mcp
	}, nil)

	return s
}

// Handler returns the streamable HTTP handler to mount at /mcp.
func (s *Server) Handler() http.Handler { return s.handler }

// Register records a token→agent_id mapping (called by RegisterMessagingMCP at
// launch, 5.3). Revoke removes it on Stop.
func (s *Server) Register(token, agentID string) {
	s.mu.Lock()
	s.sessions[token] = agentID
	s.mu.Unlock()
}

// Revoke removes a token→agent_id mapping on agent teardown.
func (s *Server) Revoke(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// Lookup resolves the agent_id bound to a session token.
func (s *Server) Lookup(token string) (string, bool) {
	s.mu.RLock()
	agentID, ok := s.sessions[token]
	s.mu.RUnlock()
	return agentID, ok
}
