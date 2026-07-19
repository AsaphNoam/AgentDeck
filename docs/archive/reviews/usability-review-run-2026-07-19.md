# Usability Review Run — 2026-07-19

**Scope:** The full non-credentialed journey matrix from `USABILITY-REVIEW.md` (J1–J12), driven
against the real built app with fresh isolated state. The matrix pass began on a binary built from
the pre-fix tree (before `ed7e3f7`/`c64d7bf`); after discovering those fixes had landed on `main`
mid-run, the affected journeys (J3, J4) were rebuilt from `c64d7bf` and re-driven, so every claim
below is stated against `c64d7bf` unless marked pre-fix.

**Review surface:** production `sqlite_fts5` binary (`make dist` → `bin/agentdeck`) plus an untagged
fallback binary for J8; Playwright driving Chromium (browser ladder rung 1: screenshots +
console-error capture); the deterministic fake ACP peer registered through a PATH shim
(`claude-agent-acp` answers the credential probe — toggleable logged-in/logged-out — and execs
`fakeacp`); isolated `AGENTDECK_HOME` fixtures per journey (`fresh/`, `seeded/`, `lived-in/` with
three scripted archived sessions); localhost API/state checks; Phase A static sweeps S1–S5 run
inline. Product code was not changed.

Screenshots and driver scripts live in the ephemeral run directory
`/tmp/agentdeck-usability-2026-07-18/` (`run/J*/`, `driver/`). Every finding is reproducible from
its steps alone.

## Executive summary

1. **MINOR / Worth fixing (NEW) — cancelling a turn with a pending permission leaves the prompt
   actionable forever.** `Cancel` resolves the withheld request via
   `resolvePending(as, id, "cancelled", "")` (`internal/runtime/permission.go`), which never emits
   `permission_resolved`. The live UI and the durable transcript keep the prompt in its pending
   state with active Approve/Deny buttons — including after reload — and clicking one returns
   `409 permission already resolved`. Reproduced 3/3 at `c64d7bf` (J4.5).
2. **Verified fixed at `c64d7bf` (browser-level):** the permission-deny status race — deny returned
   to idle/Send in 3/3 UI runs (this was the outstanding "rerun the deny journey" step; on the
   pre-fix binary it wedged the agent busy in 2/3 runs with reload unable to clear it).
3. **Verified fixed at `c64d7bf` (browser-level):** durable-transcript delta fragmentation — after
   reload the streamed reply renders as one coalesced paragraph identical to the live view (on the
   pre-fix binary each `assistant_text` delta rendered as its own block).

## Journey results (all at `c64d7bf` where re-run; others on the same-tree UI, pre-fix binary)

| # | Journey | Result |
|---|---|---|
| J1 | Install & first paint | **PASS** (3/3) — styled shell (Instrument Sans, light canvas), zero console errors. Fresh home with satisfied onboarding probes goes straight to the designed empty Dashboard (gate behavior, not a bug). |
| J2 | Onboarding end-to-end | **PASS** (13/13) — missing-CLI and not-logged-in branches show actionable guidance; passing check advances; full wizard completes; on-disk config sane; seeded models merge-preserved; wizard non-dismissible and styled. Real logged-in CLI branch **SKIPPED(env)**: `claude-agent-acp` is not installed and live-provider runs are human-gated. |
| J3 | First launch + chat round-trip | **PASS** (10/10 after HEAD re-run) — includes durable user prompt + coalesced reply after reload. Busy-state transition observed via the J4 gated turn (`waiting_input`). |
| J4 | Permission prompt flow | **PASS except finding 1** — prompt appears with tool detail; approve runs the tool (sentinel) and resolves; deny does not run the tool and returns to idle (3/3 at HEAD); re-resolving an old permission → clean 409; cancel-during-pending leaves a stale actionable prompt (finding 1). |
| J5 | Grid & layout | **PASS** (7/7) — group collapse, density, drag reorder all persist across reload and server restart; stopping an agent in the saved order keeps the grid sane; empty grid has a designed empty state. |
| J6 | Terminal runtime | **SKIPPED** — terminal support is Claude-only (FS-07) and live-CLI journeys are gated on human authorization; the fake ACP peer cannot drive the PTY path. |
| J7 | Stop / resume / switch | **PASS** (7/7) — resume from archive preserves name, system-prompt SHA, add_dirs, and spawns a live process; chat works after resume; switch-runtime (model swap via context menu + browser prompts) keeps identity and history; stop via menu reflects on card and API. |
| J8 | Archive & search | **PASS** — FTS5 build: 3 sessions, transcript search returns the right session with snippet/tags, user-prompt text is searchable, no-match shows empty state. Untagged build: metadata `LIKE` fallback works (`Migrator`, `pm`); transcript-text queries return empty per FS-05.R6. Empty-archive variant renders its designed empty state. |
| J9 | Settings & config editing | **PASS** (8/8) — role/project/backend edits round-trip to disk; collections merge-preserved (codex + other models kept); invalid project cwd surfaces an error without saving; FS-11 `resource_dir` shown read-only; config-source panel styled; notifications toggle persists. |
| J10 | Multi-agent + messaging | **PASS** (7/7) — MCP handshake per-agent token; `list_agents` (self excluded by default), `send_message`; unread Mail badge appears on the recipient card; idle recipient is nudged awake (unprompted turn observed); `check_messages` delivers the body and the badge clears. |
| J11 | Failure & recovery | **PASS** (7/7) — server kill → `reconnecting` indicator; restart → reconnects with accurate cards; killed agent process and mid-turn adapter crash both surface as ERROR/stopped (no stuck busy); unknown-role launch → clean 422; stop of a non-running agent → clean 200 no-op. |
| J12 | Restart durability | **PASS** (4/4 after harness re-check) — agents, read/unread state, transcripts, archive contents, and the switched model all survive server restart; no false busy. |

## Static sweeps (Phase A)

- **S1** serialization: only `omitempty` collection risks are `layout.groups`, backend/model `env`,
  and validation `warnings`; all observed round-trips were sane (J5/J9). No new contract defect.
- **S2** CSS wiring: the repo's own `check-presentation-contract.mjs` passes (no
  referenced-but-undefined classes).
- **S3** external-CLI variance: credential probes (`claude-agent-acp --cli auth status`, retry
  without `--no-color`) and terminal drivers are the surfaces; exercised branches in J2; real-CLI
  variants remain env-gated.
- **S4/S5**: no observed null-hostility crash or silent mutation failure in any journey; UI guards
  held for every state variant driven.

## Coverage notes (§7 maintenance)

- FS-11 declares `Journeys: —` although J9 now verifies its user-visible surface (read-only
  `resource_dir` in the project form); binding it to J9 in the FS header would keep the matrix
  honest.
- The fake ACP peer reuses one `tool_call_id` (`tc_p`) across turns; per-turn unique ids would let
  permission journeys distinguish chips across turns (harness improvement, not a product issue).

## Not exercised

Real credentialed Claude/Codex chat/MCP/resume, real Claude terminal flags/hooks/xterm, and
OpenCode/OpenHands launches remain human-gated acceptance items (see HANDOFF acceptance gates).
