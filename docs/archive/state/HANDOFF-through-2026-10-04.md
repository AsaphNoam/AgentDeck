# AgentDeck — State through 2026-10-04

Archived when `v0.9.0` was prepared. Earlier settled state remains in the preceding files in this
directory; this epoch records the work completed after `v0.8.0`.

## Release boundary

- `v0.9.0` covers 46 commits after `v0.8.0`, plus the release commit.
- The release defaults each Claude/Codex backend to the installed provider CLI with an explicit
  per-backend AgentDeck bundle, Refresh provider, and source-aware incompatibility recovery; lean
  personas with shared operating context and Claude native-preset append; Tasks as project-grouped
  work in motion; notifications that open the conversation; exact context tokens with expanded-card
  runtime metadata; and the phone desktop flow with secured agent management.
- The shipped `operating-agentdeck` package and README already matched the range (refreshed in
  `96a49f1`/`ec14587`); `install.sh` and `assemble.sh` pins were unchanged by the release cut.

## Changelog

- **2026-10-04 — Fix: installed-provider review findings.** Closed all six. Must fix: Refresh
  provider now folds only its added models into the draft and keeps concurrent edits, saving with
  the refresh's ETag (INV §3); a too-old Claude carries its selected source into the desktop
  envelope, and the phone message names the Bundle- or Installed-specific repair without paths
  (INV §8/§11). Worth fixing: Settings serializes refreshes across backends (INV §5); oversized
  `--version` output is unknown even with a valid prefix (INV §12/§16); FS-10 and the spec index
  describe the shipped Installed default and onboarding's Validate & Continue (INV §10); a
  marker-process test proves a live provider update reaches only the next start (INV §17).
  FS-09.R72 records one-refresh-at-a-time. `make test`, `make build`, full UI suite (591), UI
  build and focused providerexec `-race` pass; one earlier `make test` hit a 2 s credcheck Codex
  prober timeout that passes alone and on rerun. Unit closed; credentialed/rendered gates owed.
- **2026-10-04 — Review: installed providers and explicit bundle.** Reviewed the complete unit;
  recorded two Must-fix findings (refresh discards concurrent draft edits; incompatibility
  envelopes omit source) and four Worth-fixing findings (overlapping refresh feedback, oversized
  version output, installation-spec drift, live-update marker coverage). Medium fix model.
  Focused suites and spec/style checks pass; a temporary overlay reproduces oversized-output
  acceptance. Onboarding Validate & Continue reuse accepted with a spec wording correction.
  No product/spec edits or live-provider runs; credentialed/rendered gates remain owed.
- **2026-10-04 — Work: finished installed-provider default with explicit bundle.** TS-04.R77
  audit found no actionable provider dependency (recorded in TS-04 traceability); A37/A38/A43/A45
  verified by tests. Rendered Settings check against the built binary with a fake installed Claude
  (refresh, Bundle toggle) passed. Closure: `make test`, full UI suite (589), `make dist`, focused
  `-race` on resolver/refresh. Ready file removed; credentialed/rendered gates owed.
- **2026-10-04 — Work: provider choice in Settings and New Agent.** Slice 6 adds the Installed /
  AgentDeck bundle choice, executable path, next-start provider and Refresh provider to Settings,
  reports the provider in New Agent, and makes onboarding guidance source-aware. Full UI suite
  (589) and type check pass.
- **2026-10-04 — Work: provider refresh API.** Slice 5 adds read-only `provider_runtimes`, a bounded
  `--version` probe and the desktop-only Refresh provider endpoint with its add-only import and
  concurrency/staleness guards. Server/config/backend suites pass.
- **2026-10-04 — Work: source-aware provider recovery.** Slice 4: a recognized too-old Claude is a
  typed `provider_incompatible` naming the selected source's repair; resume refuses to replace the
  conversation on it; phone errors drop executable paths. Server/runtime suites pass.
- **2026-10-04 — Work: release runtime stops shadowing providers.** Slice 3: the wrapper only
  publishes the managed root, adapters run via private Node by absolute path, bundled Claude/Codex
  natives are verified layout entries with manifest versions, and the installer skips sign-in when
  no provider exists. Release/CLI/runtime tests pass; `assemble.sh` is syntax-checked only.
- **2026-10-04 — Work: shared provider resolver.** Slice 2 adds backend `provider_mode` and one
  resolver used by every process start, Claude terminal, readiness and `agentdeck auth` (new
  `--backend/--model`); missing/invalid/bundle-unavailable providers fail before side effects.
  Full `go test ./...` passes with no installed providers on PATH; spec check ok.
- **2026-10-04 — Work: started installed-provider default.** Slice 1 removes Codex's exact
  cache-version import gate and New Agent mismatch warning (FS-09.R71 shipped; R59/A29 retired;
  TS-06.R22 notes the now-inert wrapper export). Focused Go config/server and NewAgentModal/schema
  tests pass; spec check ok.
- **2026-10-04 — Design revision: simplify the Chuck cutover.** Revised the waiting rename unit
  for the sole operator's approved supervised cutover. FS-00.R19, FS-04.R52/A32,
  FS-10.R25–R26/A13–A14, FS-18.R18/A14, TS-02.R40–R41, TS-04.R78 and TS-11.R18 retire automatic
  home/role migration, old tmux adoption and special skill-retirement machinery. Keep source data
  recoverable, prepare role/path references offline, restart processes and rehearse once on a
  disposable copy. Retain historical annotation parsing and browser draft copy-forward. Corrected
  stale Deckhand wording to Chuck in governing rename clauses. Existing whole-root skill
  publication needs no extra cleanup mechanism. Spec lint, twin-skill comparison and whitespace
  checks pass. No product code or live data changed; implementation and the actual cutover remain
  pending, and Active change stays none.

- **2026-10-04 — Design revision: explicit provider bundles, bounded compatibility.** The waiting
  `use-installed-provider-clis.md` change now defaults each backend to Installed and offers the
  current release's single Bundle per provider. Saved overrides remain intact but inactive in
  Bundle mode; no automatic fallback, per-chat versioning or independent bundle updater. Planned
  FS-09.R75–R78/A45–A47, FS-10.R23–R24, TS-03.R54, TS-04.R75–R77 and TS-06.R30–R31 replace
  conflicting installed-only clauses with source-aware recovery and existing capability fields.
  Verification is capped at four real-provider combinations and two browser journeys in one skin;
  future bumps rerun only the affected provider smoke, not historical matrices. Larger compatibility
  machinery requires a separate scope decision. Luna checked precedence and contradictions;
  spec lint, twin-skill comparison and whitespace checks pass. Real-provider receipts remain owed.
  No product code changed. Concurrent rename edits are outside this unit.

- **2026-10-03 — Design: use installed provider CLIs.** Added waiting ready change
  `docs/ready-changes/use-installed-provider-clis.md`: local Claude/Codex by default with managed
  adapters/SDK/Node, persistent existing overrides, next-process adoption, scoped runtime metadata,
  explicit local-only refresh and provider-owned recovery. FS-09.R68–R74/A37–A44,
  FS-10.R20–R22/A10–A12, TS-03.R52–R53, TS-04.R70–R74 and TS-06.R28–R29 are planned.
  Retired the unshipped bundled-default/temporary-local draft and promoted its source idea.
  A Luna design check tightened resume failures, unsaved metadata identity and refresh races;
  fixed-adapter real-provider receipts remain pre-ship gates, not existing evidence. Spec lint,
  twin-skill comparison and whitespace checks pass. No product code changed; Active change stays none.

- **2026-10-03 — Fix: Claude 5.5 launch compatibility (Must fix).** A model the bundled Claude
  executable is too old for no longer surfaces as `model: Internal error`: a recognized
  `claude_code_version_too_old` rejection now names the bundled and required Claude Code versions
  and says to update AgentDeck, not the local `claude` CLI; other provider error data stays
  unreported (INV §8, TS-04.R9 updated). Fake-ACP launch regressions cover both paths. The
  Worth-fixing adapter-bump/override finding stays open.

- **2026-10-03 — Fix: Tasks work in motion.** Both Must-fix findings closed; the unit is closed.
  Parent rows no longer claim they "lead to" delegated children — only prerequisite successors feed
  `next` (INV §8, FS-16.R42). The projection now builds successor/indegree indexes and components
  once, orders rows with a creation-ordered ready heap plus an earliest-created cycle fallback, and
  path-compresses its union-find (INV §16, TS-08.R84); 5,000-task chain and fan-out tests pin the
  order. Specifications already required this behavior. UI, Go, spec, and build checks pass.

- **2026-10-03 — Review: lean personas and shared operating context.** No findings; the unit is
  closed. Fresh and upgraded role seeding, exact-only retained-role migration, the runtime-only
  overlay across every lifecycle composer, and Claude's shared new/load native-preset builder match
  the governing requirements. The already-recorded manual role scenarios and credentialed Claude
  fresh/resume check remain acceptance gates, not code-review failures. INV 1–3, 7–12, 15 and 17
  applied without a violation; INV 4–6, 13–14 and 16 had no applicable changed surface. Spec lint
  and the focused config, runtime and server suites pass. **Fix model:** trivial/easy — Claude
  Sonnet or Codex Luna.

- **2026-10-03 — Review: notifications open the agent's conversation.** The implementation matches
  the notification-navigation requirements, but one Worth-fixing coverage gap keeps the unit open:
  the toast test substitutes a placeholder route and does not prove the specified stale-agent
  **Agent not found** recovery. INV 2, 8, 10 and 13 applied without a violation; INV 17 applies to
  the finding; INV 1, 3–7, 9, 11–12 and 14–16 had no applicable changed surface. The 31 focused UI
  tests, style/presentation checks, production UI build and diff check pass; A46's real-browser
  toast check and manual macOS desktop-notification click remain owed. **Fix model:** trivial/easy —
  Claude Sonnet or Codex Luna.

- **2026-10-03 — Work: notifications open the agent's conversation.** Agent toasts (all four types)
  open `/agent/<id>` and dismiss; a × control only dismisses; error/pipeline toasts stay
  non-navigating. Desktop Web Notification click focuses, navigates, and closes. One helper,
  `ui/src/lib/agentConversation.ts`, builds the path for both; `NotificationCenter` registers the
  router's navigate for the SSE client (importing the router from `sse.ts` would cycle). FS-02 is
  now Current. `make test`, `make build`, UI tests and `make dist` pass. INV 2, 8, 13 apply and were
  checked; no other class has a surface. Owed: A46's real-browser toast check and manual macOS
  desktop click.

- **2026-10-03 — Review: Tasks as project-grouped work in motion.** Two Must-fix findings keep the
  unit open: delegation successors are incorrectly labelled as sequential “leads to” work, and the
  graph projection is quadratic instead of the specified O(tasks + links) bound. INV 8, 16 and 17
  applied to the findings; INV 1–2, 7, 10–11 and 13 applied without another violation; INV 3–6,
  9, 12, 14–15 had no applicable changed surface. The 40 focused Tasks UI tests, Go-produced task
  wire fixture check and style/presentation contract pass. **Fix model:** medium — Codex Terra or
  Claude Opus.

- **2026-10-03 — Work: Tasks as project-grouped work in motion.** Tasks opens on All projects
  (≤4 project reads in flight), each project showing related-work groups built only from
  task-result prerequisites and parent lineage, with typed links, branches/joins, unavailable
  references, quiet creator/assignee labels and collapsed settled history. Detail expands inline
  with existing controls; run-lineage work withholds stage-restricted controls until run detail
  confirms ownership. Create and Fire signal are closed bottom disclosures with explicit projects.
  The UI now parses `waiting`, lineage, outputs and cleanup flags against a Go-marshalled fixture.
  Rendered in Core, Sky & Grove and Studio at 1024/1440px against that fixture; a live multi-agent
  run through the built binary was not done. `make test`, UI tests and `make dist` pass.

- **2026-10-03 — Work: lean personas and shared operating context.** Fresh homes seed
  AgentDecker, Implementer, Reviewer and Researcher with lean FS-18.R16 prompts; `pm`/`teammate`
  are no longer seeded or migrated, and existing files, defaults and references are untouched. The
  previous retained prompts joined the exact-match digests. Every lifecycle composer now appends a
  runtime-only standing context, with the skill pointer still package-gated. Claude chat new/load
  send the native preset with an append. `make build` passes; `make test` passed except one
  `sqlite_fts5` run of `TestCoalescedMailProducesOnePromptAndIsNeverReplayed` (unread count read
  before completion projected it), which then passed 20/20 in each variant — an existing timing
  flake, not fixed here. A12 and A13's credentialed receipt remain owed gates.

- **2026-10-02 — Design: lean personas and shared operating context.** User confirmed four
  roles, coordination within AgentDecker and preservation of existing PM/Teammate roles/references.
  FS-04.R50–R51/A30–A31, FS-18.R15–R17/A11–A13, TS-11.R15–R17 and TS-04.R69 specify lean
  personas, internal/external Researcher, runtime-only shared guidance and additive Claude prompt
  delivery. Luna research and pinned adapter/SDK evidence are retained in
  `docs/ready-changes/lean-personas-and-operating-context.md`, waiting to start; source idea promoted.
  No product code changed or active work selected. Spec, twin-skill and diff checks pass.

- **2026-10-02 — Design: notifications open the agent's conversation.** User widened the approval
  idea to every agent notification and chose the full conversation route, with the toast body
  opening and a close control dismissing. FS-02.R64/A46 and TS-03.R51 specify client-only routing
  over the existing `agent_id` payload; `docs/ready-changes/notifications-open-conversation.md` is
  waiting to start; source idea promoted. No product code changed or active work selected.

- **2026-10-02 — Design: Tasks as project-grouped work in motion.** User confirmed project-first
  connected task rows with creator as supporting context. FS-16.R41–R45/A27–A29 and TS-08.R82–R85
  specify active relationships, waiting/cleanup visibility, secondary completed history and bottom
  manual creation. `docs/ready-changes/tasks-work-in-motion.md` is waiting to start; source idea
  promoted. Existing API/query seams suffice. Incumbent browser tab returned HTTP 504; implementation
  owes the recorded before/after rendered gates. No product code changed or active work selected.

- **2026-10-02 — Review: exact context tokens and expanded-card runtime metadata.** No findings;
  the unit is closed. **Fix model:** trivial/easy — Claude Sonnet or Codex Luna. The ACP decode,
  durable status/session tuple, turn rollup and reindex paths, resume/switch restoration,
  `AgentState` projection, shared meter, and expanded-card runtime identity match the named
  requirements. The rendered matrix at 1024×900 and 1280×720 kept long metadata, state, Collapse,
  and the exact-token meter separated without overflow in Core, Sky & Grove, and Studio. INV
  1–3, 5, 7–11, 13, and 16–17 applied without a violation; 4, 6, 12, 14, and 15 had no applicable
  changed surface. Both Go variants, the tagged build, all 558 UI tests, the style contract, and
  the production UI build pass.

- **2026-10-02 — Work: exact context tokens and expanded-card runtime metadata.** ACP
  `usage_update` decodes one reading (capped percentage plus the reported used/size pair; a
  missing, non-integer or out-of-range pair rejects the update). Migration 35 adds nullable pairs to
  `status` and `sessions`; status writes, turn_end, rollup, reindex, resume and switch carry the
  pair with the percentage, and a percentage-only hook clears it. `AgentState` gains optional
  `context_used`/`context_size`; `ContextBar` renders `12,345 / 200,000 tokens · 6% context used`
  on the chat header and expanded card, and the expanded card shows backend · model · effort. A
  real-browser check of the visual matrix at 1024/1440 in all three appearances found the longer
  label running under the state badge; the expanded action side now wraps the meter onto its own
  row, verified overlap-free. INV 1–3, 7–8, 10–11, 13, 16–17 applied; 4–6, 9, 12, 14–15 had no
  changed surface. A follow-up live-app pass (`go run ./scripts/stress-fixture`: production server
  and embedded UI with the fake ACP) showed `44,000 / 200,000 tokens · 22% context used` on the
  expanded project card and the agent screen, kept it across reload, showed `claude · haiku ·
  medium` on the expanded card, and no meter on collapsed cards. No paid real-provider run.

- **2026-10-02 — Fix: phone desktop flow and agent management.** Open phone agent and project
  screens now enter the archived state when the desktop archives them (INV §1 republish derived state; project
  configuration refetches on the live revision); **Open diff** shows exactly the requested diff
  (INV §8); the switch-runtime form resets effort to the chosen model's default and follows live
  runtime changes, and the immediate Effort control uses the live model (INV §1/§3). FS-20 A3/A4/A9 and
  traceability plus TS-13's runtime-options shape and R17 trace now describe the shipped phone (INV
  §17). An empty project archived on the desktop still reaches an open phone screen only on refresh
  (FS-20 §6). The UI-coverage Worth-fixing item stays open.
- **2026-10-02 — Review: phone desktop flow and agent management.** Four Must-fix behavior/spec
  defects and one Worth-fixing acceptance-coverage gap keep the unit open. The tailnet field filters,
  tracked-file no-follow guard, task-route denial, narrowed attention projection, runtime-option
  secrecy/defaults, shared desktop/phone dashboard helper, and presentation hooks otherwise match
  the selected requirements. INV 1–4, 7–8, 10–11, and 13–17 were reviewed; 5–6, 9, and 12 had no
  applicable changed surface. Focused remote server tests, 21 focused UI tests, the presentation
  contract, and the range whitespace check pass.
