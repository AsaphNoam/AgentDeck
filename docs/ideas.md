# AgentDeck ideas and improvements

This is a place to keep future thoughts without accidentally treating them as promises or approved
work. The specifications describe the product today; this file does not.

## New ideas

Put a half-formed thought here. It needs only a short title and, if useful, a sentence about what
prompted it.

Example:

```md
- **Pinned agents.** Let people keep frequently used agents at the top of the dashboard.
```

- **Fix card drag-and-drop usability.** Dashboard card reorder (FS-02.R12) technically works but
  reads as broken: the drag listener is bound only to the tiny 28×28px `::` handle
  (`AgentCard.tsx`), not the card itself, so dragging the card body does nothing — and because the
  card's `onClick` navigates to the agent, a failed drag attempt looks like an accidental page
  change. Dropping a card onto another group's section also doesn't move it between groups (`order`
  is one flat array independent of `group`), so it silently snaps back. Needs whole-card drag (with
  an activation distance so a plain click still opens the agent) and either real cross-group drop
  support or a clear affordance that drag only reorders within a group.
- **No Content-Security-Policy.** Neither the Go server nor `ui/index.html` sets a CSP header or
  meta tag. CSP is the third layer every diagram/Markdown-rendering hardening guide recommends
  alongside sanitization, and it would also bound the existing Markdown, diff, and xterm surfaces.
  Kept separate from `docs/ready-changes/mermaid-diagrams-in-chat.md` on 2026-08-27 because it is a
  server-wide change that could affect the dev proxy, xterm, and inline styles. Noted while
  researching safe diagram rendering.

- **Approval notifications link to the conversation.** When a pop-up notification fires because an
  agent needs approval, make it a link that opens that agent's conversation, so the user can jump
  straight to the pending permission instead of hunting for the right agent.

## Ideas being defined

These are worth shaping into a possible change, but are not ready to build. Defining an idea updates
the relevant feature and technical specifications; it does not change product code.

- **Choose an external base for AgentDeck-owned worktrees.** Support a deterministic per-project or
  per-repository checkout layout outside `$AGENTDECK_HOME` for operators whose repositories live
  under a separate workspace base. Define ownership records, base-directory changes, recovery,
  migration, and safe deletion before changing the canonical worktree path.
- **Continue a blocked pipeline stage from its agent conversation.** The run page already names the
  active stage, shows its attention reason, and accepts continuation input. Decide whether input
  sent directly to the stage agent should also become authoritative continuation input despite the
  current out-of-band chat contract, and how the run records and deduplicates that answer.
- **Live permission policy and provider-native autonomy modes.** Define a chat-page permission-mode
  control and the requested defaults: an AgentDeck-owned “approve for me” policy for Codex and the
  provider's automatic mode for Claude. Specify scope, persistence, provider mapping, whether a
  running session can change policy safely, and how this relates to frozen role/global
  `skip_permissions` and per-request Approve/Deny.
- **Finish hiding raw ids in UI labels.** Most selectors now use readable names, but remaining
  runtime, project, role, and template surfaces still need one cross-product rule: show the stable
  id only for duplicate names or an explicit detail surface, while keeping values and keys
  identity-safe. Example: the pipeline template select always renders `title (id)`
  (`ui/src/features/pipelines/RunStartForm.tsx`), unlike the duplicate-gated task/run labels
  (FS-16).
- **Edit a sent message, or split a conversation at an earlier one.** From the 2026-08-10 play
  session: like Codex, editing the most recent message edits it in place, and editing an older one
  forks the conversation from that point; a separate request asked for an explicit split action.
  Both need the same missing capability. Designing edit on 2026-08-27 established that AgentDeck
  cannot give it the meaning Codex does, and the user chose to hold the idea rather than ship a
  weaker meaning under the same name. Codex owns its own conversation state; AgentDeck supervises a
  provider CLI session that has already ingested the message. Findings, so a later attempt does not
  re-derive them:
  - **The protocol has no rewind.** The ACP client is hand-rolled and pins protocol version 1.
    Whole-session Clone (FS-01.R36) now uses `session/fork` and `session/delete`, but the fork takes
    only a session id and branches at the latest completed turn. Nothing edits, rewinds, truncates,
    or forks from an earlier point; `session/load` is a whole-session resume keyed by id and takes
    no offset or sequence.
  - **The transcript is append-only.** `internal/transcript/writer.go` exposes only
    `Append`/`Sync`/`Close`, and `Open()` repairs a torn trailing record on the assumption that only
    the final line can ever be incomplete (TS-02.R8, INV §9).
  - **The search index is immutable per turn.** One FTS document per completed turn, never reread or
    rewritten (TS-02.R16), and one document blends several messages by seq range — so an edited
    message's original text stays searchable no matter what the UI shows. FS-05.R32 names the
    transcript and search index as things archival may not change.
  - **The only rewind-shaped seam is lossy.** The switch-runtime history primer
    (`internal/server/primer.go`) rebuilds a session from an 8000-token budget: the last six turns
    verbatim and a summary of everything older. Using it to fix a typo would silently discard the
    agent's context.
  Anything shippable is therefore one of: correct-and-resend as a new turn with the supersession
  stated honestly; a true session rebuild that accepts the primer's context loss; or a display-only
  supersede that makes the visible history stop matching what the agent received. Each needs a
  product decision about what "edit" should promise. An earlier split additionally needs a truthful
  provider-context boundary, parent/child lineage, visible history rules, and behavior for
  providers without point-in-time fork support.
- **Broader agent authority.** Agents can already create, list, read, retry and re-arm their own
  tasks (TS-10.R34). Two further powers remain undefined: stopping, resuming or launching another
  agent without going through a task, which is a new authority surface and needs a threat model;
  and creating several related tasks as one unit, which TS-10 §5 excludes (multiple arms already
  provide fan-in/join).
- **Detached configuration import.** Define verified copyable fields/assets and provider injection
  paths before implementing detached import. The API still returns 501; FS-08.R35 removed the
  unavailable UI controls.
- **Activity map.** Explore a repository/session activity view using server APIs only, with clear
  privacy, scale, and normal-user value boundaries.

## ACP Wait-list

These are capabilities AgentDeck implements above ACP, or has deliberately deferred, because the
pinned adapter contract is missing or unverified. An adapter release is a reason to recheck the
capability; it is not by itself authority to remove the fallback or ship the deferred feature.

- **Steering.** The pinned Claude 0.75.1 and Codex 1.12.0 adapters advertise
  `_session/steering`, and AgentDeck uses it. Codex 1.12.0 still starts a detached turn when a steer
  arrives idle and ignores AgentDeck's `promptRequired` metadata, so the packaged steering patch
  remains necessary until upstream advertises the same no-consumption contract.
- **Host-held queued Send.** AgentDeck holds a busy agent's next prompt because the pinned Claude
  adapter queues while the pinned Codex adapter supersedes and interrupts the active turn. Keep the
  host-side hold even after steering exists: Send must remain portable, withdrawable, and distinct
  from Steer.
- **Internal actions without MCP.** AgentDeck's fifteen coordination actions remain on its scoped,
  authenticated HTTP MCP server. `migrate-internal-actions-from-mcp.md` is paused until packaged
  Codex/ACP exposes a narrowly scoped direct transport reachable under the default sandbox. The
  pinned Codex ACP 1.12.0 still advertises ACP MCP transport unsupported and HTTP supported, so the
  gate stays closed.
- **Semantic agent wake.** Mail and task activation use a short, host-generated `session/prompt`
  because ACP exposes no portable notification that wakes an idle model. Replace this bridge only
  if an adapter advertises a semantic wake capability; steering is not that capability.
- **MCP resources and templates for context.** Context links stay exposed as bounded MCP tools. MCP
  `resources/list` and `resources/read`, and ACP `Resource`/`ResourceLink` delivery, remain deferred
  until their behavior passes real Claude and Codex provider checks; prompt-level resource-link
  support alone does not satisfy that gate.
- **Declared MCP tool output schemas.** Structured tool results ship without declared
  `outputSchema`. Add schemas only after the pinned Claude and Codex adapters' handling of them is
  verified against real providers.
- **Codex session model and effort delivery.** AgentDeck applies model and effort through ordered
  post-session configuration because Codex ACP session creation has no model field. Keep this path
  until a replacement is both advertised and provider-verified.
- **Codex system-prompt delivery.** AgentDeck injects its prompt overlay through
  `CODEX_CONFIG.developer_instructions` because Codex ACP does not consume generic ACP
  `systemPrompt`. Keep and reverify the overlay on every adapter bump until the adapter exposes a
  proven portable replacement.
- **Codex executable authority.** The release wrapper defaults `CODEX_PATH` to AgentDeck's directly
  pinned private Codex executable and the assembled tree proves there is exactly one Codex at the
  pinned compatible version (currently adapter/CLI 1.12.0/0.154.0). Keep that rule on every bump;
  explicit `CODEX_PATH` overrides remain supported.

## Known things to improve

These describe incomplete or deliberately limited shipped behavior. Their owning specification is
the authority; move an item to ready changes only after its exact requirements and acceptance checks
are clear.

- **Same-machine trust boundary.** The loopback API relies on same-machine trust, and agent
  processes inherit the full environment except for backend strip keys (TS-05 §5). Revisit loopback
  authentication only with an explicit threat model and UI/CLI handshake design whose benefit
  outweighs its setup and compatibility cost; remote-control device authentication (FS-20) is a
  separate concern. Revisit an environment allowlist only if it preserves provider compatibility.
- **Chat and tracking refresh fidelity.** Make replayed streaming deltas match live deltas; show
  chat initial-load errors instead of a silent panel (FS-03 §6); refresh visible files/commands
  without stale-request overwrite; and let hook-only activity update recency.
- **Cross-turn transcript search.** Turn-document indexing intentionally chooses a small design over
  the more complete segmented model: all query terms and quoted phrases must occur within one turn,
  annotation flush, metadata document, or migrated legacy document. It does not combine a term from
  one turn with a term from another, does not match a phrase across turn boundaries, and one
  pathologically large turn can still use substantial temporary memory. Revisit with size-bounded
  chunks, boundary overlap, and per-term session aggregation only if real sessions show oversized
  individual turns or users repeatedly fail to find conversations because their query spans turns;
  those additions otherwise impose more schema, ranking, pagination, and phrase-boundary machinery
  than the observed long-session rewrite problem warrants.
- **Coordination notice hygiene.** Republish unread counts after janitor expiry, notify only on the
  first budget breach, and remove the duplicate waiting-input/permission notices (FS-06 §6).
- **Terminal and backend loose ends.** Either add an optional terminal driver picker or stop
  advertising unreachable drivers; implement or retire the planned terminal tab cap (FS-07); make
  OpenCode/OpenHands executable overrides actually launch and give them missing/old-CLI guidance
  like Claude's (FS-09 §6). Codex terminal rejection is intentional (FS-07 §6), not a gap.
- **Federation UI and watches.** Expose custom roots/profiles, refresh the effective view after
  source events, register prompt watches after binding, and clear preview consent on project change.
- **HTTP API contract.** Decide how mixed legacy error envelopes converge, and define shared JSON
  request-body limits with a structured over-limit error before enforcing them (TS-03 §5).
- **Frontend state ownership.** Define Zustand/React Query ownership and mutation-error behavior
  before broad frontend refactors.
- **Process and filesystem hardening.** Corroborate process identity, scope crash cleanup by
  generation, serialize concurrent events, define/test detached-start pidfile races, and bound
  aggregate shutdown grace across many agents (`StopAll` stops them sequentially). Decide whether
  startup repairs existing descendant modes and whether valid-name role/project files may be
  symlinks (TS-05 §5); add adversarial tests for the chosen rules.
