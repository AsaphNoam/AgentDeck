# AgentDeck — Repository map

Start with the authority, then the live state:

| Document | Role |
|---|---|
| [`docs/specs/README.md`](docs/specs/README.md) | **Product source of truth.** Index and lifecycle for feature specs (FS) and technical specs (TS/INV). |
| [`docs/features/HANDOFF.md`](docs/features/HANDOFF.md) | **Live work state.** An already-active package’s checkpoint, governing IDs, decisions, gates, findings, next step. |
| [`docs/product-backlog.md`](docs/product-backlog.md) | **Idea intake.** Inbox, discovery, candidates, and known gaps; never self-select from it. |
| [`docs/implementation-queue/`](docs/implementation-queue/README.md) | **Ready delivery work.** Specified features waiting to start, one package per feature. |
| [`docs/features/AGENT-WORKFLOW.md`](docs/features/AGENT-WORKFLOW.md) | **Process source of truth.** Implementation/review/fix/usability roles and GREEN checkpoints. |
| [`docs/features/INVARIANTS.md`](docs/features/INVARIANTS.md) | Normative technical appendix for recurring bug classes (`INV §n`). |
| [`architecture-flow.md`](architecture-flow.md) | Descriptive architecture orientation; TS wins on conflict. |
| [`docs/architecture-decisions.md`](docs/architecture-decisions.md) | Non-normative rationale behind selected TS decisions. |
| [`docs/archive/README.md`](docs/archive/README.md) | Superseded phase plans, master PRD, review evidence, and snapshots. Never build from it. |

## Feature ownership

| ID | Area | Primary code |
|---|---|---|
| FS-00 | Product concepts and boundaries | repository-wide |
| FS-01 | Agent lifecycle | `internal/server/{launch,resume,switch,sessions}.go`, `internal/runtime` |
| FS-02 | Dashboard, layout, groups, notifications | `ui/src/components/grid`, `internal/state`, SSE |
| FS-03 | Chat, streaming, permissions | `internal/runtime/chat.go`, `ui/src/components/chat` |
| FS-04 | Configuration and onboarding | `internal/config`, Settings/onboarding UI |
| FS-05 | Archive, search, tracking | `internal/archive`, `internal/index` |
| FS-06 | Messaging, nudger, budgets | `internal/messaging`, `internal/state/messages.go` |
| FS-07 | Terminal interface and drivers | `internal/runtime/terminal`, terminal UI |
| FS-08 | Native configuration federation | `internal/configsource`, ConfigSourcePanel |
| FS-09 | Backend/model catalog and capabilities | `internal/backend`, backend Settings/onboarding |

Technical ownership is split across TS-01 architecture, TS-02 persistence, TS-03 HTTP/SSE/WS,
TS-04 external protocols, TS-05 security, TS-06 build/test/delivery, and TS-07 federation.

## Runtime in one breath

Browser UI ⇄ loopback REST/SSE/WebSocket ⇄ one Go server ⇄ ACP stdio or terminal PTY ⇄ agent CLI.
The server also hosts the loopback messaging MCP endpoint, is the sole SQLite writer, composes
launch/resume/switch through shared boundaries, and embeds the built UI. Config remains local JSON;
native Claude/Codex federation is one-way and read-only.

## Working rule

Plans and handoffs must name governing FS/TS R/A IDs. If a behavior or architecture contract has no
owner, create a spec delta before implementation. Historical phase numbers are useful git/archive
context only and never determine current scope.
