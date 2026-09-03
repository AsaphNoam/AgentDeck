// Package messaging hosts AgentDeck's in-process, token-bound MCP tools for
// coordination, tasks, pipelines, and context links.
package messaging

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agentdeck/agentdeck/internal/contextref"
	"github.com/agentdeck/agentdeck/internal/pipeline"
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

	onBudgetExceeded  func(agentID, turnID string, used int)
	onMessageInserted func(fromAgentID, toAgentID string)
	onMessagesRead    func(agentID string)
	onTaskResult      func(taskID string)
	budgetProvider    func() int
	budgetByAgent     map[string]turnBudgetLimit
	tasks             TaskControl
	addressable       func() ([]state.LiveAgent, error)
	projectAvailable  func(project string) (bool, error)
	pipelines         *pipeline.Manager
	context           *contextref.Service

	mcp     *mcp.Server
	handler http.Handler
	tools   []string

	mu       sync.RWMutex
	sessions map[string]SessionIdentity // session token -> server-derived identity
}

type turnBudgetLimit struct {
	turnID string
	limit  int
}

func (s *Server) SetBudgetProvider(fn func() int) {
	s.mu.Lock()
	s.budgetProvider = fn
	s.mu.Unlock()
}

func (s *Server) budgetLimit(agentID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := MessageBudgetPerTurn
	if s.budgetProvider != nil && s.budgetProvider() > 0 {
		limit = s.budgetProvider()
	}
	status, err := s.store.CurrentTurnBudget(agentID, limit)
	if err != nil {
		return limit
	}
	if frozen, ok := s.budgetByAgent[agentID]; ok && frozen.turnID == status.TurnID {
		return frozen.limit
	}
	s.budgetByAgent[agentID] = turnBudgetLimit{turnID: status.TurnID, limit: limit}
	return limit
}

func addTool[In, Out any](s *Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	s.tools = append(s.tools, tool.Name)
	mcp.AddTool(s.mcp, tool, handler)
}

// ToolNames returns the registered AgentDeck action names. Registration and
// permission identity composition share this exact source (INV §2).
func (s *Server) ToolNames() []string { return append([]string{}, s.tools...) }

type SessionIdentity struct {
	AgentID    string
	Generation string
}

// SetBudgetExceededSink wires the Phase 5 budget breach notification path.
func (s *Server) SetBudgetExceededSink(fn func(agentID, turnID string, used int)) {
	s.mu.Lock()
	s.onBudgetExceeded = fn
	s.mu.Unlock()
}

// SetMessageInsertedSink wires send_message inserts to the nudger's event-driven
// wake check. The ticker remains the fallback if the signal is dropped.
func (s *Server) SetMessageInsertedSink(fn func(fromAgentID, toAgentID string)) {
	s.mu.Lock()
	s.onMessageInserted = fn
	s.mu.Unlock()
}

func (s *Server) messageInserted(fromAgentID, toAgentID string) {
	s.mu.RLock()
	fn := s.onMessageInserted
	s.mu.RUnlock()
	if fn != nil {
		fn(fromAgentID, toAgentID)
	}
}

// SetAddressableAgents supplies the one addressable set list_agents projects and
// send_message resolves against (techspec §3.3): running agents plus stopped chat
// agents a message may wake. The dashboard owns it because the wake gates include
// the project-archive state, which lives in configuration rather than state.db.
// Without it this server addresses running agents only.
func (s *Server) SetAddressableAgents(fn func() ([]state.LiveAgent, error)) {
	s.mu.Lock()
	s.addressable = fn
	s.mu.Unlock()
}

// SetProjectAvailable supplies the configuration-owned project archive gate
// used when explaining why a stopped pipeline agent is not addressable.
func (s *Server) SetProjectAvailable(fn func(project string) (bool, error)) {
	s.mu.Lock()
	s.projectAvailable = fn
	s.mu.Unlock()
}

// addressableAgents answers from the dashboard-supplied directory, falling back
// to the running registry when no dashboard wired one.
func (s *Server) addressableAgents() ([]state.LiveAgent, error) {
	s.mu.RLock()
	fn := s.addressable
	s.mu.RUnlock()
	if fn != nil {
		return fn()
	}
	return s.store.LiveAgents()
}

// SetMessagesReadSink wires check_messages read/delete to a recipient state
// refresh so the unread_messages badge clears the moment mail is read. Without
// it the send path bumps the badge but nothing ever recomputes it back down.
func (s *Server) SetMessagesReadSink(fn func(agentID string)) {
	s.mu.Lock()
	s.onMessagesRead = fn
	s.mu.Unlock()
}

// SetTaskResultSink wires an accepted task result back to the control plane that
// evaluates the arms waiting on it. It is a notification, never the authority:
// the registration is already committed when it fires (TS-10.R2).
func (s *Server) SetTaskResultSink(fn func(taskID string)) {
	s.mu.Lock()
	s.onTaskResult = fn
	s.mu.Unlock()
}

func (s *Server) taskResultRecorded(taskID string) {
	s.mu.RLock()
	fn := s.onTaskResult
	s.mu.RUnlock()
	if fn != nil {
		fn(taskID)
	}
}

func (s *Server) messagesRead(agentID string) {
	s.mu.RLock()
	fn := s.onMessagesRead
	s.mu.RUnlock()
	if fn != nil {
		fn(agentID)
	}
}

func (s *Server) budgetExceeded(agentID, turnID string, used int) {
	s.log.Warn("budget exceeded", "agent", agentID, "turn", turnID, "used", used)
	s.mu.RLock()
	fn := s.onBudgetExceeded
	s.mu.RUnlock()
	if fn != nil {
		fn(agentID, turnID, used)
	}
}

// New constructs the messaging server, registers its tools, and builds the
// streamable HTTP handler. store is the shared state.db handle the real tool
// handlers (5.2) operate on; the spike's ping tool does not touch it.
func New(store *state.Store, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		store:         store,
		log:           log,
		sessions:      map[string]SessionIdentity{},
		budgetByAgent: map[string]turnBudgetLimit{},
	}

	s.mcp = mcp.NewServer(&mcp.Implementation{
		Name:    "agentdeck-messaging",
		Version: version.String(),
	}, nil)
	addTool(s, &mcp.Tool{
		Name:        "list_agents",
		Description: "List other agents you can message (by address role@project, name, or agent_id). Includes both running agents and stopped agents a message wakes; each entry's `availability` is \"running\" or \"stopped_wakeable\".",
	}, s.handleListAgents)
	addTool(s, &mcp.Tool{
		Name:        "send_message",
		Description: "Send a message to another agent, running or stopped-wakeable; a stopped recipient is woken to receive it, which takes longer. `to` is role@project, an agent name, or an agent_id.",
	}, s.handleSendMessage)
	addTool(s, &mcp.Tool{
		Name:        "check_messages",
		Description: "Read your pending messages; flags them read (or deletes) as requested.",
	}, s.handleCheckMessages)
	addTool(s, &mcp.Tool{
		Name:        "report_pipeline_stage_result",
		Description: "Report the authoritative success, failure, or blocked result for your current pipeline stage attempt. An accepted result is final for that attempt.",
	}, s.handleReportPipelineStageResult)
	addTool(s, &mcp.Tool{
		Name:        "propose_pipeline_template",
		Description: "Validate and propose a model-neutral pipeline template for exact human approval; this never saves it.",
	}, s.handleProposePipelineTemplate)
	addTool(s, &mcp.Tool{
		Name:        "propose_pipeline_run",
		Description: "Validate and propose an exact saved-template run configuration for human approval; this never starts it.",
	}, s.handleProposePipelineRun)
	addTool(s, &mcp.Tool{
		Name:        "get_assigned_task",
		Description: "Read the task you are currently assigned: its instruction and the context references attached to it. Returns assigned=false when you have none.",
	}, s.handleGetAssignedTask)
	addTool(s, &mcp.Tool{
		Name:        "create_task",
		Description: "Create durable work that AgentDeck starts when its prerequisites are satisfied. Assign it to an existing agent by role@project, name, or agent_id, or omit the target to have a new agent launched.",
	}, s.handleCreateTask)
	addTool(s, &mcp.Tool{
		Name:        "cancel_task",
		Description: "Cancel a task you created. Its outcome becomes cancelled and anything waiting on it is resolved.",
	}, s.handleCancelTask)
	addTool(s, &mcp.Tool{
		Name:        "report_task_result",
		Description: "Record the authoritative success, failure, or blocked result for the task you are assigned. Your runtime is released after this turn ends, so you still receive this response.",
	}, s.handleReportTaskResult)
	addTool(s, &mcp.Tool{
		Name:        "share_context",
		Description: "Give another chat agent access to context you own — your current turn so far, your latest completed turn, or your accepted pipeline report — without copying it. Returns a reusable context_ref_id; the recipient reads it explicitly and is not woken.",
	}, s.handleShareContext)
	addTool(s, &mcp.Tool{
		Name:        "list_context_links",
		Description: "List context other agents shared directly with you, newest first. Returns labels and source metadata only, never the content.",
	}, s.handleListContextLinks)
	addTool(s, &mcp.Tool{
		Name:        "read_context_link",
		Description: "Read one bounded page of a context reference you are authorized for; pass the returned cursor to continue.",
	}, s.handleReadContextLink)
	addTool(s, &mcp.Tool{
		Name:        "set_context_link_visibility",
		Description: "Hide or unhide one of your shared-context entries. This is your own list state; it revokes nothing.",
	}, s.handleSetContextLinkVisibility)
	addTool(s, &mcp.Tool{
		Name:        "revoke_context_grant",
		Description: "Withdraw a context share you granted. The reference and any other grant are unaffected.",
	}, s.handleRevokeContextGrant)

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
	s.RegisterSession(token, agentID, "")
}

func (s *Server) RegisterSession(token, agentID, generation string) {
	s.mu.Lock()
	s.sessions[token] = SessionIdentity{AgentID: agentID, Generation: generation}
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
	identity, ok := s.LookupSession(token)
	return identity.AgentID, ok
}

func (s *Server) LookupSession(token string) (SessionIdentity, bool) {
	s.mu.RLock()
	identity, ok := s.sessions[token]
	s.mu.RUnlock()
	return identity, ok
}

func (s *Server) SetPipelineManager(manager *pipeline.Manager) {
	s.mu.Lock()
	s.pipelines = manager
	s.mu.Unlock()
}
