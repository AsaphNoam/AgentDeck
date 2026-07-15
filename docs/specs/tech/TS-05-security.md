# TS-05 — Security & trust boundaries

**Status:** Current
**Code:** `internal/server`, `internal/config`, `internal/configsource`, `internal/runtime`, `internal/messaging`, `internal/backend`
**Absorbed:** [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md), the [phase archive manifest](../../archive/phases/README.md), and [`INVARIANTS.md`](../../features/INVARIANTS.md) §14

## 1. Scope

This spec defines AgentDeck's local threat model, network and browser boundary, producer identity,
filesystem protections, path validation, process environment policy, and secret/redaction rules.
AgentDeck is a personal local tool, not a multi-user or remotely exposed service.

## 2. Design & constraints

**R1 — Reachability is loopback-only.** The server binds `127.0.0.1`; runtime listener validation
fails closed on a non-loopback address. There is no supported `0.0.0.0` or remote mode.

**R2 — Browser requests are origin constrained.** A middleware around the entire mux rejects
non-local Host headers and non-local Origin headers before `/api`, `/mcp`, terminal WebSocket, or
static handling. Origin-less local CLI clients remain allowed; the configured Vite dev origin is
allowed in development.

**R3 — Loopback is not authentication.** The dashboard API has no user login/token. Any process
running as a caller able to connect locally can read and drive the API. This accepted product
boundary must be disclosed; adding API authentication is a future product/security delta.

**R4 — Producers have scoped unforgeable identity.** Hook and MCP writes require per-launch random
tokens bound server-side to an agent and launch generation. Tokens are not accepted from query
parameters, not logged, and are invalidated on stop/crash/switch.

**R5 — AgentDeck creates owner-only files.** `$AGENTDECK_HOME` and newly created subdirectories are
`0700`; newly created/rewritten configs, database, transcripts, registrations, tokens, and caches
are `0600`. Startup tightens the home directory and database, but does not recursively repair an
existing tree.

**R6 — Path policies are boundary-specific.** URL-decoded role/project ids pass strict slug
validation before path construction. Project `cwd` and `add_dirs` deliberately name user-selected
external roots and are expanded/existence-checked, not contained beneath AgentDeck home. Federation
roots/imports/cache targets use their approved-root and symlink policy from TS-07. Existing
valid-name role/project symlink files are a documented same-user hardening gap.

**R7 — Secrets are values, metadata is not.** APIs/UI may expose configured environment key names,
source paths, provenance, hashes, and health, but never environment values, credential contents,
hook/MCP tokens, or raw native config containing secrets. Logs and error bodies apply the same rule.

**R8 — Child processes inherit the server environment by design.** Launch starts from
`os.Environ()`, removes backend `StripEnvKeys`, then applies backend/model/launch overrides. This
allows arbitrary provider credentials and PATH/HOME/locale at the cost that unrelated host secrets
are visible to agent CLIs. Replacing it with allowlists is a security/product compatibility change.

**R9 — Permission policy fails closed.** Effective skip-permission state composes global, role,
and launch policy. Missing or invalid inputs do not silently enable bypass. Permission resolutions
are scoped to the requesting live agent/tool call and follow TS-04.R4.

**R10 — External content remains untrusted.** ACP text/tool payloads, native config, transcript
lines, CLI stderr, and MCP input are parsed as data and use boundary-specific validation/escaping.
They cannot choose sender identity, HTML execution, or log format unchecked. Existing record/tool
limits remain binding; TS-03 records the missing uniform HTTP-body bound.

**R13** — A project-resources leaf is an AgentDeck-owned owner-only directory selected
solely by a validated project id under AgentDeck home; it is never resolved from `cwd`, `add_dirs`,
title, or client input. AgentDeck rejects an existing resource parent or leaf symlink/non-directory
instead of following it. Its path may appear as non-secret launch/UI metadata, but its contents are
never read into API, SSE, transcript, analytics, or log data merely by this feature.

## 3. Interfaces & data shapes

Security-relevant interfaces are the single listener, Host/Origin middleware, hook/MCP token
registries, config/file-store mode helpers, federation approved-root sets, and backend env composer.
Their concrete payloads are owned by TS-03, TS-04, and TS-07.

## 4. Invariants

- **INV §11:** permission/config failure does not degrade into permissive behavior.
- **INV §12:** no spoofable identity crosses hook/MCP/process boundaries.
- **INV §13:** secrets never cross redaction or logging boundaries.
- **INV §14:** loopback binding plus browser Host/Origin validation is mandatory on every route.
- **R11 — Security-sensitive changes require adversarial tests.** Bind/origin/token/path/mode/redaction
  changes include negative regression coverage and cite the relevant requirement.
- **R12** — The macOS release installer has an explicit, limited delivery trust model.
  Before unpacking or activating a release it verifies the archive's SHA-256 against the matching
  GitHub Release manifest, stages it outside the selected runtime, and preserves the prior runtime on
  any verification or extraction failure. It never collects, copies, logs, or transmits provider
  credentials; guided sign-in delegates directly to the bundled provider path in an attached terminal.
  MVP artifacts are intentionally unsigned and unnotarized: checksums detect corruption but do not
  independently authenticate a compromised release account or manifest. Documentation must state that
  limit and the possible Gatekeeper approval; the installer must not bypass Gatekeeper, disable system
  protections, or request elevated privileges. The application-runtime root is separate from
  `AGENTDECK_HOME`, owner-writable only, and never used as authority for user config, sessions, or
  credentials (FS-10.R3–R9; TS-06.R13–R21).
- **INV §11:** an unusable project-resources path fails project creation or lifecycle composition;
  it never degrades into an agent launch without the promised accessible directory.

## 5. Deviations & open decisions

- Same-machine API callers are trusted as described by R3; real API authentication is not shipped.
- Full environment inheritance is accepted behavior under R8. The UI masks likely secret fields but
  plaintext values remain in owner-only config and child environments; AgentDeck is not a vault.
- Provider credential checks remain heuristic for some CLI versions/storage layouts. A passed check
  is readiness evidence, not a security guarantee or entitlement check.
- Existing descendant modes are not recursively repaired and valid-name role/project symlinks are
  not rejected. These are explicit backlog hardening items, not shipped guarantees.

## 6. Traceability

- Network/browser boundary: `internal/server/bind.go`, `security.go`, `routes.go`.
- Tokens/identity: `internal/state/manager.go`, `internal/messaging/messaging.go`,
  `internal/server/messaging_registration.go`.
- Modes/path validation: `internal/config/atomic.go`, `validate.go`, `internal/configsource`.
- Project-resource containment: `internal/config` project-resource helper and
  `internal/server` project/lifecycle composers.
- Env/redaction: `internal/server/launch.go`, `internal/runtime/chat.go`,
  `internal/configsource/security.go`.
- Regression anchors: `TestDNSRebindingHostRejected`, `TestCrossOriginRequestRejected`,
  `TestHomeTreeIsOwnerOnly`, `TestPathTraversalRejected`, federation redaction/symlink tests.
