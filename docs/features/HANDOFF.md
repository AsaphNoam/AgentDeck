# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** None.
- **State:** Composer file-and-command autocomplete is implemented (FS-03.R30–R34, TS-03.R24,
  TS-04.R24). In a live chat composer, `@` at a word boundary opens a file picker over the chat
  session's working directory — Git tracked/untracked/effective-ignore view when it is a worktree,
  otherwise a bounded walk; `.git` is skipped, directory symlinks are not followed, canonical
  containment is enforced, results are ranked and capped at 50 — and accepting inserts a relative
  `@path` plus a trailing space. `#` opens a command picker over the running agent's latest ACP
  `available_commands_update` snapshot and inserts `/<name>`, including a Codex `/$skill`. Both stay
  ordinary durable prompt text: no structured attachment, embedded contents, or command persistence,
  so the prompt route and `SendPrompt` are unchanged. The ACP decode boundary stores the replace-only,
  bounded snapshot (≤256 entries; name ≤256 and description/hint ≤2000 runes; nameless entries skipped)
  as live runtime state that dies with the agent, and two new session-scoped GET routes (`file-search`,
  `available-commands`) project it. Missing/unreadable directories return an empty list, unknown agents
  404, and both a stopped agent (via the running record) and a non-chat agent return the shared typed
  runtime/conflict error. FS-03 and TS-03 moved Partial→Current; TS-04 stays Partial. `make check-specs`, both Go test
  modes, focused `-race` on the snapshot, `make build`, all 227 UI tests plus the new
  `Composer.test.tsx`, presentation/style checks, and `make dist` pass. A 2026-08-15 isolated
  real-browser pass confirmed both pickers, boundary/filter/keyboard behavior, verbatim trailing-space
  submission, stopped-session failure without composer loss, and zero console errors.
- **Previous state:** Release archive packaging now preserves the package context of the three required npm
  commands (`claude-agent-acp`, `codex-acp`, and `codex`) while keeping archives symlink-free: each
  npm `.bin` symlink becomes a regular launcher that calls the private Node runtime on the original
  in-tree module, so package-relative imports still resolve after extraction. The archive round-trip
  regression fails against the old byte-flattening behavior and executes successfully now; FS-10.R3/A2
  and TS-06.R15/R21 are restored. Existing installed release directories remain immutable and need a
  newly published release plus explicit update/reinstall to receive the repair. Claude ACP startup
  hardening is implemented. The original 2026-08-10
  `initialize: runtime: transport closed` cause cannot be recovered because startup stderr was
  discarded, but the exact pinned 0.59.0 adapter now passes local ACP v1 initialize/session creation
  and an authenticated one-word streamed turn on this machine. Every ACP initialize/load/new call is
  capped at 30 seconds; early Claude exits map captured stderr to bounded resource, nested-launch,
  authentication, or runtime guidance without returning raw output, and launch/resume share fake-ACP
  exit/timeout regressions plus a focused race pass. FS-09.R48/A21 and TS-04.R22 shipped; TS-04.R10
  was split so optional-integration probing remains planned as R23. Shared checks, both Go test modes,
  source/UI builds, presentation checks, and the distribution build pass. Dashboard application logs
  now append to owner-only `$AGENTDECK_HOME/dashboard.log` for foreground and detached starts;
  foreground mode also mirrors them to stderr, while detached mode retains one redirected sink.
  The dashboard logger is the scoped process-wide `slog` default, so package-level diagnostics share
  its JSON format and level. FS-04.R41/A21 and TS-01.R15 shipped with append, permission, unavailable-
  path, and no-duplication regressions. Two chat usability items are
  also implemented: the transcript shows a **Working…** indicator at its end while the open chat agent
  is `busy` (clearing on turn end, error, or a permission pause to `waiting_input`), and transcript
  text is selectable/copyable with a distinct `.user-message::selection` colour so a highlight inside
  the user's own bubble stays visible. FS-03.R28/R29/A13/A14 shipped and FS-03 remains Current. The
  2026-08-15 browser pass confirmed the Working indicator and its error clear, plus live computed
  selection styles; browser automation could not establish an actual pointer text selection, so that
  one visual gesture remains unclaimed. Simple backend creation and
  global configuration linking is implemented. **Add backend**
  is now a provider-first dialog: choosing a type supplies a matching editable name and the canonical
  starter model, and submitting uses the new item-scoped `POST /api/backends`, which adds exactly one
  backend to the durable catalog without submitting, replacing, or discarding the browser's unsaved
  whole-catalog Settings draft. Claude/Codex creation also offers **Create and use my configuration**,
  which makes the backend durable first and then performs the normal project-free Linked connection;
  a failed connection leaves the valid backend saved, unbound, and retryable. The Settings/onboarding
  source panel replaced its project selector, discover step, and Linked/Mirrored choice with one
  **Use my … configuration** action; `project` is optional on the config-source GET/preview/refresh
  routes, and an `enable_model_sync` bind turns on continuing sync and immediately runs that
  provider's add-only import into the target backend only. Persistence is catalog-first then manifest
  with preimage restoration, the generation and SSE publish only after both writes, and one shared
  `catalogMu` serializes the full-document save, the item create, and the enabled-bind merge (the
  concurrency regression loses seven of eight entries without it). Catalog read-modify-writes now
  refuse an unreadable `backends.json` instead of overwriting it with the seed, and the Claude/Codex
  resolvers no longer treat an absent project as the server process's working directory. FS-04.R40/A20,
  FS-08.R34/A11, FS-09.R47/A20, TS-03.R22/R23, and TS-07.R16/R17/R18 shipped; FS-08.R23 is superseded
  and FS-04/TS-03 moved Partial→Current. `make check-specs`, both Go test modes, focused `-race` on the
  catalog lock, all 214 UI tests, presentation/style checks, source build, and the distribution build
  pass. The 2026-08-02 real-browser J9 pass confirmed cancel, provider-specific starter creation,
  dirty-draft preservation, existing and direct create-and-connect linking, target-only import, and
  unlink with zero browser errors; source-failure/retry and launch through the binding remain
  unexercised. Full record:
  [`../archive/reviews/usability-review-run-2026-08-02-backend-creation.md`](../archive/reviews/usability-review-run-2026-08-02-backend-creation.md).
- **Last reviewed code:** `2727ae8` (2026-08-15), the continuous range after `b6654b5`: catalog
  review fixes, ACP startup diagnostics, dashboard logging, release launcher packaging, and composer
  file/command autocomplete. Both composer findings from that review are now fixed.
- **Branch:** `main`.

## Active change

**State:** none

No implementation change is active. Do not select a waiting change without the human naming it.

## Decisions needing your input

These are product decisions needed for a future change or shipped boundaries whose reversal needs
an explicit specification update. Remove an item when the human resolves it or queues that update.

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.

## Acceptance gates

- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

The post-fix usability-review and current code-review state are committed locally on `main`; pushing
those commits to the shared `origin/main` branch needs explicit human authorization.

## Review findings

### Open findings

None.

The one-off Archive `unterminated string` 500 still did not reproduce under direct or suite coverage,
and the API-only `tmux` calls without explicit timeouts remain an unreproduced source-risk lead; they
are not promoted to findings without a repeatable failure.

## Recent changelog

- 2026-08-15 — Ran a focused real-browser usability review of the newest chat and startup work. In
  one isolated seeded home with a deterministic ACP peer, composer `@`/`#` boundary, filtering,
  keyboard acceptance, exact trailing-space submission, and stopped-session failure behavior all
  passed; a non-finishing turn showed **Working…** and cleared it after cancellation failure; a
  controlled Claude initialize resource exit surfaced bounded recovery guidance without raw stderr;
  and the foreground dashboard log appended with `0600` mode. Browser console errors: zero. The live
  browser reported selectable transcript text and contrasting computed `::selection` colours, but
  automation could not establish the pointer selection itself, so that visual gesture is recorded
  blocked rather than passed. No user-impact findings. Focused Go regressions and 15 composer/transcript
  UI tests pass. Full record:
  [`../archive/reviews/usability-review-run-2026-08-15-recent-chat-fixes.md`](../archive/reviews/usability-review-run-2026-08-15-recent-chat-fixes.md).

- 2026-08-15 — Fixed both composer autocomplete review findings (FS-03.R31/R33, TS-03.R24; INV §1);
  both restore already-specified behavior, so no specification changed. **Composer submit** now sends
  the draft exactly as displayed — rejecting only whitespace-only prompts — instead of trimming it, so
  a selected `@path` or `/$skill` and its inserted trailing space reach the optimistic event and
  `POST /prompt` verbatim. **File search** now gates on the durable running record (`ReadRunning`, the
  same seam annotate-and-assign uses), so a stopped chat agent yields the shared typed
  agent_not_running error (`409`) instead of a `200` listing of its former working directory. New
  regressions: a `Composer.test.tsx` case accepting a file and a command then asserting the request
  body keeps the trailing space, and `TestFileSearchStoppedAgent`; both were confirmed to fail against
  the pre-fix code. `make test`, `make build`, all 228 UI tests, `npm run build`, presentation/style
  checks, and `make dist` pass.

- 2026-08-15 — Reviewed the continuous range after `b6654b5` through `2727ae8` in both specification
  directions and against every invariant class. Two Must-fix autocomplete findings are open. **INV §1 /
  FS-03.R31/R33** — picker selection visibly inserts a trailing space, but Composer trims it before the
  optimistic and durable prompt paths, so neither selected `@path` nor `/$skill` is submitted exactly
  as displayed. **INV §1 / TS-03.R24** — a stopped chat agent retains its durable session snapshot and
  can still receive a `200` file-search listing, despite the specified stopped-agent conflict. The
  archive, dashboard logging, ACP startup, and command-snapshot work otherwise matches its governing
  requirements. `make check-specs` and the focused runtime and server test suites pass. No product
  code or specifications changed during review.

- 2026-08-15 — Implemented composer file-and-command autocomplete (FS-03.R30–R34, TS-03.R24,
  TS-04.R24; INV §1/§2/§7/§8). `Composer.tsx` gained one reusable keyboard-operated picker: `@` at a
  word boundary searches the chat session's working directory and inserts a relative `@path`; `#`
  searches the running agent's live ACP command snapshot and inserts `/<name>` (Codex `/$skill`
  included). Both remain ordinary durable prompt text, so the prompt route is unchanged. Two new
  session-scoped GET handlers (`file-search`, `available-commands`) sit beside
  `internal/server/files_commands.go`; `filesearch.go` does the bounded, Git-aware, symlink-safe,
  containment-checked, 50-capped listing; `acpmap.go` decodes `available_commands_update` into a
  replace-only, bounded runtime snapshot exposed by `ChatRuntime.Commands`/`Registry.Commands`. Safe
  empty/typed-error shapes keep the composer sendable for missing directories, unknown (404), and
  stopped/non-chat (conflict) agents. FS-03 and TS-03 moved Partial→Current. New regressions:
  `Composer.test.tsx`, `TestDecodeAvailableCommands`, `TestAvailableCommandsSnapshotReplaceOnly`,
  `TestFileSearch*`, and `TestAvailableCommandsSnapshotEndpoint`. `make check-specs`, both Go test
  modes, focused `-race`, `make build`, all 227 UI tests, presentation/style checks, and `make dist`
  pass. No live-browser pass was run.

- 2026-08-15 — Completed the composer file-and-command autocomplete design after the human resolved
  both review findings. File selection is intentionally a plain-text convenience: `@` searches the
  live chat session's working directory and inserts a relative `@path`, with no ACP resource block or
  embedded contents. `#` uses the same picker for every command/skill in the latest complete ACP
  `available_commands_update`; selection inserts `/<name>` (including Codex `/$<skill>`), later
  updates replace the in-memory list, and launch/resume/switch/stop boundaries reset it. Planned
  FS-03.R30–R34/A15–A17, TS-03.R24, and TS-04.R24 define the user behavior, two session-scoped reads,
  bounded Git-aware file enumeration with fallback walking, ACP decode/cache/read plumbing,
  failure behavior, and focused acceptance evidence. Pinned Claude 0.59.0, Codex 1.1.2, ACP v1, and
  current package releases were verified. The source idea was removed and
  `composer-file-and-command-autocomplete.md` is waiting to start. Both review findings are closed;
  no product code changed.

- 2026-08-15 — Reviewed the incomplete file-and-skill autocomplete design under the `/review-design`
  research and simplicity lenses; no product code, specification, or ready-change file changed.
  There is no waiting ready change yet: the interrupted design left the idea under **Ideas being
  defined** and drafted only FS-03.R30–R34/A15–A17. Two Must-fix findings are open. The draft silently
  chose plain `@path` text despite ACP v1 baseline `resource_link` support in both pinned adapters,
  and deferred the requested command/skill half despite both adapters already publishing standard
  dynamic `available_commands_update` snapshots. The recommended combined design adds no new
  package or persistence: validate and send selected files as resource links, cache the latest live
  command snapshot per session behind a read endpoint, reuse one composer picker, and invoke the
  advertised name as `/<name>`. The human still needs to decide whether `#` shows all advertised ACP
  commands (recommended) or only a provider-classifiable skill subset, and whether `@` is real file
  attachment (recommended) or cosmetic text completion. Local pinned sources, ACP v1 documentation,
  and current npm releases were checked; `make check-specs` and `git diff --check` pass.

- 2026-08-15 — Extended the §13 research lens and the design-feature launcher to claimed and
  implied limitations; no product code changed (two commits: the provider/package rule, then its
  generalization). Any design that says something cannot be done the straightforward way, silently
  takes an indirect route whose only sensible motive is such a limitation, or reads as a strange
  strategy for its stated goal must show the limitation is real — verified against the actual code,
  the pinned tool's real surface, newer releases, and better alternatives (a direct approach, an
  existing seam, an official package) — with the evidence in the change file; an unverified
  impossibility claim is a finding, and its workaround is re-judged under the over-engineering lens
  once the claim falls. The twinned design-feature skill gains the matching designer-side rule:
  never declare anything impossible from memory, verify before designing around it, record the
  evidence, and do not propose an indirect strategy whose only motive is an unverified limitation.
  Motivated by repeated real cases where a designing agent declared ACP capabilities unavailable
  that the pinned or newer Claude/Codex packages fully supported, including staying on the
  unofficial codex ACP package when the official one was better. `make check-specs`, the twin-skill
  comparison, and `git diff --check` pass.

- 2026-08-15 — Added the `/review-design` role and simplicity rules; no product code changed.
  Workflow §13 reviews a `Waiting to start` ready change before implementation through three ordered
  lenses: over-engineering (every planned requirement must earn its place from the confirmed
  outcome, a real report, or a binding invariant; "in case" machinery is a finding), maintainability
  and extension (prefer stretching an existing FS/TS area, seam, route, or pattern over minting a
  parallel mechanism — the design-level twin of INV §2 — with genuinely new interfaces justifying
  the §6 contract cost), and research (design assumptions verified against the tree and pinned tool
  versions, contradictions recorded with evidence), plus design hygiene (planned tags, invariant
  conflicts, failure/concurrency/rollback ownership, observable acceptance, silently pre-made
  product decisions). Findings use the §7 format in this file; the change stays waiting while
  Must-fix findings are open, and resolution is §11 design-feature work. The same gap was closed for
  implementation: §2 now requires the smallest satisfying change, extending existing seams before
  inventing parallel ones, and no unrequired abstraction/configurability/edge-case handling; §7 now
  treats unrequired complexity as a finding while still ignoring demands for speculative handling.
  Twinned byte-identical launchers exist in `.claude/skills/review-design/` and
  `.agents/skills/review-design/`; `AGENTS.md` and the spec README role list name the role.
  `make check-specs`, the twin-skill comparison, and `git diff --check` pass.

- 2026-08-14 — Added the `/investigate-bug` role; no product code changed. Workflow §12 defines the
  investigation process for field bug reports that cannot be live-debugged: verbatim report intake,
  spec-first classification (code defect / spec gap / works as specified), evidence-mapped diagnosis
  with **confirmed**/**probable**/**undetermined** confidence labels, and the rule that a failure
  point that logged nothing becomes its own observability finding. Findings land in this file's
  `## Review findings` in the §7 format so `/fix` consumes them unchanged; the role edits no product
  code or specs except a reproduction test committed skipped, and §8 step 1 now says a fix session
  un-skips such a test and confirms it fails before fixing. Twinned byte-identical launchers exist in
  `.claude/skills/investigate-bug/` and `.agents/skills/investigate-bug/`; `AGENTS.md` and the spec
  README's role list name the new role. `make check-specs`, the twin-skill comparison, and
  `git diff --check` pass.

- 2026-08-14 — Fixed the release archiver's npm-command flattening defect (FS-10.R3/A2,
  TS-06.R15/R21, INV §9). `CreateArchive` still emits no symlink entries, but the required private
  `claude-agent-acp`, `codex-acp`, and `codex` commands now become regular POSIX launchers that invoke
  the bundled Node runtime on their original package modules. This preserves relative ESM imports;
  the old archive copied `dist/index.js` into `.bin`, so Claude resolved `./acp-agent.js` from the
  wrong directory and every archive produced by that code carried the same latent runtime defect.
  A package/extract/execute regression was confirmed to fail against the old implementation. Focused
  release and CLI tests, the release race test, spec checks, both Go test modes, source build,
  presentation/UI build, distribution build, and whitespace checks pass. Existing installed releases
  remain unchanged until a corrected release is published and explicitly installed or updated.

- 2026-08-14 — Persisted dashboard application logs for every `dashboard start`, not only detached
  runs (FS-04.R41/A21, TS-01.R15, TS-05.R7, INV §14). Foreground mode appends JSON records to
  `$AGENTDECK_HOME/dashboard.log` and mirrors them to stderr; the detached child keeps its existing
  redirected stderr as the single sink to avoid duplicate records. Existing logs append and are
  tightened to `0600`; an unavailable log path fails startup rather than silently discarding
  diagnostics. The dashboard logger is also the scoped process-wide `slog` default. README now gives
  the exact `tail` command for collecting a shareable log file. Spec checks, focused tests/race,
  both Go test variants, source build, presentation checks, UI build, and distribution build pass.

- 2026-08-14 — Fixed a rare `TestNewAgentIDFormatUniquenessAndCollisionRetry` failure and completed
  the 2026-08-11 chat-usability record. The test drew 1000 real random agent ids and then stubbed
  `randRead` to retry into the fixed id `a_abcdef`; when the sweep happened to mint that id first
  (about 1 in 16,800 runs, 1000/2^24), every one of `NewAgentID`'s ten tries collided and the run
  failed with `could not mint unique agent_id after 10 tries`. The fixture now clears the retry
  target between the sweep and the stub. `NewAgentID` itself is unchanged and correct: the failure
  was in the fixture, not the generator, so no specification change is needed. The diagnosis was
  confirmed by a temporary reproduction that seeded `a_abcdef` and produced the identical error, and
  the fix was proven by forcing that seed against the corrected fixture. Documentation cleanup from
  the same review: the two shipped ideas (response spinner, copy-pastable messages) are removed from
  `docs/ideas.md`, the 2026-08-11 entry's idea count is corrected from eleven to nine (seven now
  remain), the current-position state records the shipped chat behavior, and FS-03 §7 cites the
  `TranscriptView` regression. `make check-specs`, both Go test modes, and whitespace checks pass.

- 2026-08-14 — Fixed the reported opaque Claude ACP startup boundary (FS-09.R48/A21,
  TS-04.R22/R23, INV §9/§12). Initialize, session load, and session new now have a shared 30-second
  per-stage deadline on launch and resume. A pre-registration Claude exit is classified from the
  captured stderr into a fixed safe recovery vocabulary, so raw stderr and the generic
  `transport closed` sentinel no longer reach the API; timeout and exit both terminate without a
  live handle. Fake-ACP exit/timeout regressions and a focused race pass succeed. The pinned
  `claude-agent-acp` 0.59.0 completed local initialize/session creation and an authenticated streamed
  one-word turn. `make check-specs`, both Go test modes, `make build`, and `make dist` pass.

- 2026-08-11 — Implemented two chat usability items (FS-03.R28/R29/A13/A14). The transcript now shows
  a waiting indicator at its end while the open chat agent is `busy`, clearing on turn end, error, or
  a permission pause to `waiting_input`. Transcript text is also selectable/copyable, with a distinct
  `.user-message::selection` colour so a highlight inside the user's own bubble (whose background is
  `--ad-highlight`, the same token the global `::selection` used) is visible rather than
  invisible-on-invisible. A `TranscriptView` regression covers the busy-only indicator. Nine
  play-session ideas were also recorded in `docs/ideas.md`; the two shipped here were left in that
  list by mistake and are now removed, leaving seven. `make check-specs`, both Go test modes,
  `make build`, all 221 UI tests, presentation/style checks, the UI build, and `make dist` pass; the
  selection colour was confirmed by the styles, not a live browser pass.

- 2026-08-02 — Fixed every open review finding. Whole-catalog Settings saves now use a strong ETag
  and reject a stale second-tab draft rather than deleting a newly created backend; creates and
  successful catalog saves return the current validator. Source-manifest unlink now uses the shared
  catalog lock, failed source pruning restores the catalog preimage, corrupted catalogs reject binds
  before preview consent is consumed, and storage errors no longer expose local paths. The UI now
  refetches the global source state after direct create-and-connect, keeps the project picker on an
  unavailable scoped dashboard, and retains a stopped builder's saved proposal session when its
  transcript read fails. Chat Back safely falls home for absent/archived projects, and the builder
  and chat presentation behavior are specified. Focused and complete Go/UI suites, spec checks,
  builds, and the generated distribution pass.

- 2026-08-02 — Usability-reviewed backend creation and global configuration linking in a real
  browser against a fresh isolated home. Cancel, provider-specific Codex starter creation,
  preservation of an unrelated dirty Settings draft, ordinary linking, direct create-and-connect,
  target-only import, and unlink all passed with zero browser errors and no new user-impact finding.
  The failure/retry branch needs a controlled unavailable-source fixture; launch through the new
  binding remains the credentialed provider gate. Full record:
  [`../archive/reviews/usability-review-run-2026-08-02-backend-creation.md`](../archive/reviews/usability-review-run-2026-08-02-backend-creation.md).

- 2026-08-02 — Reviewed the continuous range `05dff38`→`b6654b5` (Claude configured-model autosync,
  item-scoped backend creation, and global configuration linking) against FS-04/FS-08/FS-09,
  TS-03/TS-07, and every invariant class. Four Must-fix findings: stale complete-catalog save can
  erase a committed item create; create-and-connect leaves an already cached source panel falsely
  unbound; concurrent unlink can erase another completed binding; and a failed binding-prune leaves
  an incompatible catalog/manifest pair after returning 500. Two Worth-fixing findings: a corrupt
  catalog consumes preview consent before strict persistence refuses it, and new API error paths
  expose raw storage details. `make check-specs`, focused server/config tests, focused Settings tests,
  and the UI build pass; the server suite also passes when allowed its loopback test listener.

- 2026-08-02 — Implemented simple backend creation and global configuration linking
  (FS-04.R40/A20; FS-08.R2/R3/R23/R33/R34/A11; FS-09.R47/A20; TS-03.R22/R23; TS-07.R16/R17/R18;
  **INV §1/§2/§7/§8/§11/§13/§15**). Add backend became a provider-first dialog over a new
  item-scoped `POST /api/backends` that builds the type's canonical starter from the same authority
  as fresh-home seeding and never accepts the whole-catalog draft; an empty catalog makes the new
  entry default, an id reused with a different name/type is `backend_exists`, and an exact replay is
  idempotent even after an earlier connection added models. `connect_native_configuration:true` and
  the panel's single **Use my … configuration** action share one server bind seam that does an
  auto-root, user-level, Linked preview plus an `enable_model_sync` bind: it enables sync on the
  target backend only, runs that provider's existing add-only reader, writes the merged catalog
  before the manifest, restores the catalog preimage on a returned manifest error, and installs the
  generation/SSE only after both writes. Three latent defects were fixed along the way: the Claude
  and Codex resolvers canonicalized an absent project to the server's own working directory (reading
  and approving an unrelated tree), catalog read-modify-writes fell back to the seeded document on a
  corrupt `backends.json`, and the Settings editor re-seeded its draft on every background refetch.
  The two resolver regressions and the eight-way concurrent-create regression were each verified to
  fail against the previous code. FS-08.R23 is superseded by R34; FS-04 and TS-03 moved
  Partial→Current. `make check-specs`, both Go test modes, focused `-race`, all 214 UI tests,
  presentation/style checks, source build, and `make dist` pass. No real-browser pass was run.

- 2026-08-01 — Revised the simple-backend/global-source design after the human resolved every review
  decision; no product code changed. The global project-free flow is included and the shipped
  project-scoped FS-08.R2/R3/R23 text is restored until replacement ships. Add backend now uses a
  planned item-scoped, idempotent `POST /api/backends`: it builds the canonical provider starter,
  preserves unrelated whole-catalog drafts, and can create plus Linked-connect in one visible action.
  A connection failure retains the valid backend as visibly unbound and retryable. Mirrored remains
  compatible for existing bindings/explicit API callers but is not offered as recovery. Immediate
  import is target-only and provider-honest. The human accepted catalog-first best-effort persistence:
  a returned source-write error attempts restoration, while interruption may leave only safe add-only
  autosync/model residue on an unbound backend; source binding alone means connected and stable-id
  retry converges under INV §15. FS-04/FS-08/FS-09 and TS-03/TS-07 plus the ready change carry the
  complete failure, concurrency, invalidation, zero-project, replay, and browser evidence. The six
  design-review findings are resolved; documentation checks pass and the change remains waiting.

- 2026-08-01 — Reviewed the waiting simple-backend/global-source design against the current
  FS/TS requirements, implementation seams, and cited invariants. Four Must-fix design defects keep
  it from being ready: FS-08.R2/R3/R23 silently describe the unshipped global replacement as current;
  the creation dialog does not define save-before-bind, dirty whole-catalog drafts, or post-save link
  failure/Cancel ownership; Mirrored is presented as failure recovery although both modes share the
  same failing resolver path and differ only by a post-success disposable cache; and the compensated
  `config-sources.json`/`backends.json` update has no crash/rollback-failure reconciliation. Two
  Worth-fixing items narrow provider-honest target-scoped model import and add the missing durable-id,
  failure, concurrency, invalidation, and zero-project traceability. No product code or specifications
  changed during review; the findings are recorded above and the change remains waiting.

- 2026-08-01 — Designed simple backend creation and global configuration linking; no product code
  changed. Add backend will become a provider-first modal with a matching editable name and usable
  starter model, never an incomplete inline card. Claude/Codex will offer one read-only **Use my
  configuration** action during creation and on existing backends: it hides discovery/project/mode
  choices, connects a global source, immediately imports provider-supported models, and enables
  continuing add-only sync. Claude and Codex share the same flow, while their catalog metadata stays
  provider-honest. Compatibility mode appears only after a normal link fails; detached import stays
  unavailable. FS-04/FS-08/FS-09 and TS-03/TS-07 now carry the planned behavior; the ready change is
  `simple-backend-creation-and-global-source-linking.md`. Specification and whitespace checks pass.

- 2026-08-01 — Implemented Claude configured-model autosync (FS-09.R45/R46/A18/A19; TS-01.R14;
  **INV §2/§11**). An opted-in `claude-acp` backend now imports the selectors named in the
  user-level `~/.claude/settings.json` (`model`, `availableModels`, array-or-legacy-string
  `fallbackModel`) at dashboard startup, keyed by and carrying the exact selector, with friendly
  labels for the `fable`/`opus`/`sonnet`/`haiku` aliases and self-labels otherwise. The import is
  add-only and skips a selector already used as a map key or an existing entry's provider string;
  a missing/malformed/wrong-shape file is a non-fatal skip. Codex and Claude now merge through one
  shared `syncModels` add-only helper and a single `AutoSyncBackends` rewrite (`modelautosync.go`),
  with the Claude reader in `claudemodels.go`. Fresh homes seed exactly the four Claude family
  aliases with generic labels and `sonnet` as default; existing catalogs are never touched. Settings
  offers the same opt-in for Claude with restart-timing copy. `make check-specs`, both Go test modes,
  all 208 UI tests, source build, and distribution build pass. The reader/merge/persist paths and the
  seed are exercised by focused Go tests against real file I/O; no live Claude CLI was involved.

- 2026-08-01 — Fixed configuration-source linking for newly added or retargeted backends
  (FS-08.R5/R33/A10; TS-07.R15). Settings holds source-link actions until a new backend has been
  saved, an unknown backend leaves its preview token usable, and backend-catalog saves prune
  deleted or provider-incompatible bindings while retaining compatible ones. Focused server/UI
  regressions, both Go test modes, all 207 UI tests, spec checks, and the generated distribution
  build pass.

- 2026-08-01 — Reviewed the continuous range after `ca100e0` through `05dff38` in both
  specification directions and against every invariant class: the AgentDecker proposal-review fix
  (`249da5b`), the Sky & Grove visual rework (`a8d103f`), projects-home project creation (`f360216`),
  the chat Back-link change (`0fddb5a`), the scoped New Agent project lock (`24630d9`), the Claude
  configured-model design (`33806e1`, docs only), and the three chat tool-activity presentation
  commits (`f7f5262`, `a1d9446`, `05dff38`). One **Must fix** and three **Worth fixing** findings are
  recorded above. **INV §10** caught the scoped New Agent lock reaching the *unavailable*-project
  route, where the picker is hidden and every launch is rejected as `unknown project`, contradicting
  FS-02.R43's "active scoped project dashboard"; it also caught three unspecified user-visible
  behaviors — the chat Back target (no FS-03 text at all, and no test/brief/changelog for `0fddb5a`),
  the builder no longer navigating to its chat, and the builder session panel's hydrating/stopped
  presentation. **INV §7** caught the builder marking its transcript "loaded" in `.finally()` after
  swallowing the fetch error, so one failed read destroys the persisted pointer to a pending
  proposal. Clean/not applicable elsewhere: **§1** `NewAgentModal` re-applies `fixedProject` when the
  scoped route changes, `ToolRun`'s open state is keyed to its first event so a growing run keeps it,
  and the builder refetches its transcript on the running→stopped boundary; the Files tab's "Diff"
  reveal still resolves because `diff_refs` seqs come from `EvDiff` events, which grouping never
  collapses, and `CommandsTab` has no seq reveal. **§2** `shouldRenderToolResult` is the single
  omit predicate shared by `ToolResult` and the `TranscriptView` grouping, and the create modal
  reuses `ProjectForm` + `useCreateProject` rather than a second form or POST path. **§3** the create
  form submits the whole document with a server-derived empty id. **§8** create/color/archive
  mutations surface their server messages, and widening proposal discovery to every `tool_result` is
  safe because a candidate must still satisfy `ok === true`, `pipelineProposalSchema`, and a digest,
  and remains behind FS-14.R27/R30's one-time human approval. **§13** `.tool-run`,
  `.tool-run-content`, `.tool-run-event`, `.project-dashboard-header`, and the
  `.context-menu .project-color-preset` rules all resolve, `tool-run` is documented in
  `contract.json` with its `trigger`/`content` slots and collapsed/expanded states, and the shipped
  Sky & Grove palette matches TS-08.R38 value for value. **§4/§5/§6/§9/§11/§12/§14/§15** have no
  applicable surface: the range adds no teardown, concurrency, runtime/interface, durability or
  migration, server collection, external CLI, route, or external side-effect ordering. `make
  check-specs`, `make test`, all 206 UI tests, `npm run check:styles`, the UI build, and
  `git diff --check` pass; the green suite does not exercise any recorded finding. A concurrent
  session committed `05dff38` mid-review; its diff was reviewed in full and is included in the range.
  No product code or specifications changed during review.

- 2026-08-01 — Removed coloured/enclosing surfaces from chat tool activity (FS-12.R36/A12;
  **INV §13**). The collapsed summary and the calls, results, and failures disclosed from it now
  render as plain subdued text: no tint, border, box padding, or raised output panel. Disclosure,
  indentation, and semantic failure text remain. Specification/style/presentation checks, both Go
  test modes, all 206 UI tests, source and distribution builds, whitespace, and browser inspection
  of computed tool styles pass.

- 2026-08-01 — Collapsed uninterrupted chat tool activity (FS-03.R26/A11;
  FS-12.R35; TS-08.R39; **INV §1/§8/§13**). Consecutive `tool_call`/`tool_result`
  records now render as one closed **Ran _n_ tools** row; opening it restores the individual calls,
  non-empty results, and failures. A non-tool event starts a new run. Successful no-payload results
  now render nowhere, including orphaned legacy records; the former **Completed** presentation is
  retired. Grouping occurs only in `TranscriptView`, so persistence, live/replay input, and expanded
  per-event annotation anchors are unchanged. Focused regressions cover expansion, empty-result
  omission, boundaries, and per-event annotation; specification/style/presentation checks, both Go test modes, all 206 UI
  tests, source and distribution builds, whitespace, and a real-browser visual fixture pass with no
  browser errors.

- 2026-08-01 — Implemented compact chat tool activity (FS-03.R25/A10; FS-12.R34;
  TS-08.R1/R5/R7; **INV §8/§13**). A terminal tool result whose completed payload is empty is
  now a compact, labelled **Completed** outcome rather than a blank block; an empty `content`
  field no longer hides a meaningful error. Tool calls/results use subdued neutral rows, while
  diffs, fenced code, commands, and Terminal retain their technical surfaces. The visual matrix
  covers call, non-empty result, empty completion, and failure. Specification/style/presentation
  checks, both Go test modes, all 203 UI tests, source and distribution builds, whitespace, and a
  real-browser visual fixture pass with no browser errors.

- 2026-08-01 — Designed Claude configured-model autosync; no product code changed. An opted-in
  `claude-acp` backend will add valid selectors from the user-level Claude `model`,
  `availableModels`, and `fallbackModel` settings at dashboard startup. Sync is add-only, keeps
  existing entries/defaults/frozen sessions authoritative, discovers no effort levels, and reads no
  project/policy/env/private-cache/binary/network/session source. Fresh homes will seed the portable
  `fable`/`opus`/`sonnet`/`haiku` aliases with generic labels and Sonnet as default; existing catalogs
  are never migrated. FS-09 gained planned R45/R46/A18/A19 and TS-01 gained planned R14. The idea
  moved to the waiting `claude-configured-model-autosync.md` change; documentation checks pass.

- 2026-08-01 — Fixed scoped New Agent launches (FS-02.R43/A25; **INV §1/§8/§10**). A New agent action on `/project/:id` now fixes the launch target to that route project and omits the project picker, while the general modal and other prefilled launch surfaces retain their existing chooser behavior. The fixed project is reapplied if the scoped route changes before submission, preventing a mounted modal from carrying the prior project. Focused regressions cover the hidden picker and submitted project from both the modal and scoped grid. `make check-specs`, both Go test modes, all 200 UI tests, source and distribution builds, presentation/style checks, and whitespace checks pass. No real-browser pass was run; component coverage stands in for J5.

- 2026-08-01 — Implemented "create a project from the projects page" (FS-02.R41/R42/A24; **INV
  §1/§8/§10/§13**). The projects home now shows a persistent **New project** header button and a
  background right-click **New project** menu (a right-click on a project card still opens that card's
  menu, never the background one), both opening one modal that reuses the Settings `ProjectForm` and
  the existing `POST /api/projects` create path (server-derived id, six-preset color, non-blocking
  `cwd` warning). A valid submission closes the modal and the card appears from the shared project
  query with no reload; an API/validation failure keeps the modal open with the server message; Cancel
  or Escape closes with no change. No new API, storage, id-derivation, or color rule; no create surface
  on the scoped `/project/:id` route. FS-02.R41/R42/A24 shipped and FS-02 moved Partial→Current. A new
  `ProjectDashboard` regression covers A24 (both entry points, card-menu exclusion, live create,
  error, cancel). `make check-specs`, all 197 UI tests, both Go test modes, source and distribution
  builds, presentation/style checks, and whitespace checks pass. A real-browser J5 pass remains for
  usability review; component coverage stands in for it here.

- 2026-08-01 — Designed "create a project from the projects page"; no product code changed. The
  projects home gains a persistent **New project** button and a background right-click **New project**
  menu, both opening one modal that reuses the Settings project form and the existing
  `POST /api/projects` create path (server-derived id, six-preset color, non-blocking `cwd` warning);
  a valid submission adds the card live. FS-02 gained R41/R42/A24 (all planned) and moved to Partial;
  the create surface is projects-home-only and the background menu never fires on a project card. No
  technical-spec change is needed — the route, hooks, form, and `.context-menu`/dialog presentation
  already exist. The idea moved from `docs/ideas.md` to the waiting
  `create-project-from-projects-page.md` change; `make check-specs` passes.

- 2026-08-01 — Reworked Sky & Grove's visual contract and bundled CSS (FS-12.R28/R31/A10;
  TS-08.R33 retired, R38 shipped; **INV §10/§13**). The palette now separates an atmospheric blue
  canvas, blue-white surface hierarchy, and evergreen action/structure colors; smaller radii and
  controlled shadows recover Core's discipline; low-contrast off-canvas contour linework replaces
  the oversized rings; and removing the card-leaf override restores the semantic state strip.
  Settings preview copy and the xterm palette fixture follow the single palette authority. Core's
  marker-free token values remain unchanged. The presentation/style contract, all 194 UI tests,
  both Go modes, source/distribution builds, paired browser matrix, and browser console check pass.

- 2026-08-01 — Fixed the AgentDecker builder findings (FS-14.R27/A10; **INV §1/§13/§10**): proposal
  discovery now validates every self-identifying terminal tool result instead of trusting the
  ACP display title/category; the builder stays on Pipelines where its approval controls live; and a
  stopped builder refetches and retains a transcript that contains a pending proposal, while stale
  proposal-free sessions still expire. Focused UI regressions cover generic ACP labels and the
  stopped-builder recovery. The project-menu swatches regain their round border/hover treatment and
  the visual matrix now renders that menu picker. `make check-specs`, the focused UI tests/build, and
  the full required verification suite pass. The real credentialed adapter round-trip remains a
  release gate, not a claim from this fixture coverage.

- 2026-08-01 — Targeted usability review of the Create-with-AgentDecker pipeline flow (J14, FS-14
  §4.2): 2 Must-fix, 1 Worth-fixing, queued above. The live round-trip could not be run — `fakeacp`
  cannot issue a real `propose_pipeline_template` MCP call, so no fixture exercises a real
  MCP-tool-named transcript event. Leading cause: proposal detection matches on the ACP-derived
  tool-call `name`/`title`, which never carries the tool name (`name` = ACP kind `"other"`), so the
  proposal panel likely never renders with the real adapter (needs a live-adapter check). Secondary:
  launch navigates to the chat while the only approval surface is on Pipelines, stranding the user.
  Run record:
  [`../archive/reviews/usability-review-run-2026-08-01-agentdecker-builder.md`](../archive/reviews/usability-review-run-2026-08-01-agentdecker-builder.md).

- 2026-08-01 — Usability review of the per-chat runtime picker (FS-03.R23/R24/A9): PASS, no findings.
  Built the real binary, seeded an isolated home, and drove the running chat header in headless
  Chrome via `fakeacp` (reached through `claude-agent-acp`/`codex-acp` PATH shims). Confirmed the
  running agent renders backend/model/effort selects; a differing selection reveals Switch; applying
  re-seeds the picker from the authoritative record; a backend change resets model and effort to the
  new defaults; a cross-backend switch applies via the switch-runtime path (server record verified);
  and a stopped agent shows read-only static identity with no picker. Zero console/page errors
  throughout. This clears the previously blocked runtime-switch browser journey. Run record:
  [`../archive/reviews/usability-review-run-2026-08-01-runtime-picker.md`](../archive/reviews/usability-review-run-2026-08-01-runtime-picker.md).

- 2026-08-01 — Reviewed the continuous range after `9c6a637` through `ca100e0` (the per-chat runtime
  picker; `b28a96c` is a prior review-state commit) in both specification directions and against every
  invariant class. Code and specs agree: FS-03.R23/R24/A9 and the R1 update describe the shipped
  running-chat header picker, its explicit Switch, and the stopped/archived static identity, and no
  uncovered behavior shipped. **INV §2** is load-bearing and correct — the change extracts
  `resetRuntimeForBackend`/`resetRuntimeForModel` into one `runtimeSelection.ts` and routes the chat
  header, New Agent modal, and dashboard Switch dialog through it, replacing three drifting inline
  copies behavior-equivalently (and more correctly for an absent `default_model`). Clean/not applicable
  elsewhere: §1 the re-seed effect republishes the picker from the authoritative agent record on
  switch/hydration; §8 a rejected/`no_change` switch surfaces its server message via `role="alert"` and
  resets to the live identity; §10 the ready-change file is removed, the index and FS-03 Traceability
  updated, no drift; §13 `.chat-runtime-picker`/`.chat-runtime-switch` and the reused
  `.form-field`/`.form-error` all resolve; §5 the `switching` guard plus the server per-agent switch
  lock prevent double-submit and the chat-only payload omits `interface`, which the server preserves as
  the current value; §3/§4/§6/§7/§9/§11/§12/§14/§15 have no applicable surface. No new finding. `make
  check-specs` passes and the 35 affected chat/switch/launch UI tests pass. No product code or
  specifications changed during review.

- 2026-08-01 — Implemented the per-chat runtime picker. A running chat header now offers catalog
  backend/model/effort selects and an explicit Switch action that calls the existing runtime-switch
  path; a shared reset helper keeps the header, New Agent modal, and dashboard Switch dialog aligned.
  Rejected switches surface their server message and reset to the live agent identity, catalog-missing
  identities remain visible but cannot submit until a listed target is selected, and stopped/archived
  views retain static identity. FS-03.R23/R24/A9 are shipped and FS-03 is Current. Focused coverage,
  all 191 UI tests, both Go test modes, specification/presentation/source/distribution builds, and an
  isolated browser launch→model-switch→stop pass succeed.

- 2026-08-01 — Reviewed the continuous range after `b8e31fb` through `9c6a637` in both specification
  directions and against every invariant class: the effort-lifecycle fix (`bd5ba71`) and the project
  context-menu/preset-color feature (`9c6a637`); `8b95f90` is a prior review-state commit. The effort
  fix correctly resolves its three predecessor findings — **INV §10/FS-09.R4/R42** ordinary resume now
  skips catalog re-validation and reuses the frozen effort (verified the resume path keeps the snapshot
  value untouched); **INV §2** `SupportsEffort` derives backend capability from each adapter's
  `EffortDelivery` (behaviour-equivalent to the removed allowlist); **INV §2/§9** migration v13 adds
  `pipeline_attempts.effort` with the version guard bumped, insert/select/scan updated in lockstep
  (placeholder count verified), and `stageExecution` reads the frozen attempt effort. One
  **Worth fixing** finding: the shared `.context-menu button` CSS out-specifies the preset swatches, so
  swatches inside the project menu render squarish and borderless (fill and selected outline survive) —
  cosmetic, INV §13/§10. Clean/not applicable elsewhere: §1 the color update republishes optimistically
  and rolls back on error, the portal menu carries no cross-boundary derived state; §2 one
  `ProjectColorPicker` and one `projectColors` palette feed Settings/onboarding/menu, the portal menu
  matches `CardContextMenu` exactly; §3 `ProjectForm` merges the seeded color via local state; §4/§5/§6
  add no teardown, concurrency, or runtime; §7 no new read loops; §8 the color mutation surfaces its
  error and invalidates; §10 R38/R39/R40 wiring reaches every surface and `AgentCard` already sets
  `--ad-project-accent`; §11 no server serialization change; §14 no new route. `make check-specs`, the
  config/state/pipeline/backend Go tests, the frozen-effort resume integration test, and the four
  changed UI test files pass. No product code or specifications changed during review.

- 2026-07-31 — Implemented project context menus and preset colors. Active project cards now open a
  cursor-positioned portal menu with Rename, Archive, and an immediate six-swatch color picker;
  unavailable-project cards still offer no menu. Slate, Blue, Green, Amber, Rose, and Violet live in
  one frontend palette and replace all R/G/B input fields in Settings and onboarding, with Slate the
  new-project default. The existing RGB API/storage contract remains unchanged, including support for
  historical and non-UI colors. Agent and home project cards now use their project accent for a soft
  surface wash and border while preserving the left edge and agent-state top bar. Focused component
  and visual-matrix regressions, `make check-specs`, the full tests, source build, distribution build,
  and whitespace checks pass. FS-02.R38–R40/A22–A23, FS-04.R39/A19, and TS-08.R37 are current.

- 2026-07-31 — Designed the project context-menu and preset-color feature; no product code changed.
  The active-project card's inline right-click expand becomes a cursor-positioned portal menu that
  reuses the agent menu's presentation, and project color is chosen from a fixed six-accent palette
  (Slate default, Blue, Green, Amber, Rose, Violet) shown as inline swatches in that menu and in the
  Settings/onboarding project forms, replacing the free-form R/G/B inputs everywhere. The accent also
  gains visual weight: each card in a project reads as gently mono-colored via a `color-mix` background
  wash (~9% into `--ad-surface-panel`) and tinted border (~50% into `--ad-border-strong`), keeping the
  7px left stripe and the agent-state top bar, bounded for contrast across presets and skins.
  Storage/API stay the existing RGB triple: no schema change, no migration, no server-side enum
  enforcement, and old colors stay valid. FS-02 gained R38/R39/R40/A22/A23, FS-04 gained R39/A19, and
  TS-08 gained R37 (all planned); the three specs moved Current→Partial and the index tracks it. Items
  #1 (create from the projects page) and #3 (visual rework) stayed raw ideas; #2/#4 moved to the
  waiting `project-context-menu-and-preset-colors.md` change. `make check-specs` passes.

- 2026-07-31 — Fixed all three effort-lifecycle review findings. **INV §10 /
  FS-09.R4/R42** — ordinary resume now trusts and re-applies the effort frozen with the agent;
  only an explicit effort override or `config_refresh` validates a newly resolved value against
  the current catalog, with a fake-ACP launch → settings edit → stop → resume regression. **INV
  §2** — backend effort capability now derives from each adapter's `EffortDelivery` contract in
  the config validator rather than a duplicate type allowlist. **INV §2/§10** — migration v13
  adds `pipeline_attempts.effort`; every attempt stores its execution level and stage launch,
  continuation, and recovery read that frozen field instead of live run assignments. TS-01.R12,
  TS-02.R18, and TS-09.R24 now state the delivery and persistence boundaries. `make check-specs`,
  both Go test modes, `make build`, `make dist`, and whitespace checks pass; the first distribution
  attempt encountered npm's `ENOTEMPTY` cleanup race and the immediate retry passed.

- 2026-07-31 — Reviewed the committed range after `7fc5158` (the agent-effort-selection
  implementation and its four bundled review fixes in `de8634f`; the surrounding commits are
  docs/design only) in both specification directions and against every invariant class. One
  **Must fix** and two **Worth fixing** findings are recorded above. **INV §10 / FS-09.R42/R4** —
  plain resume re-validates the frozen effort against the current catalog, so removing a model's
  effort level in Settings blocks resume of an unchanged frozen session, which R4 forbids and R42's
  reject-points (launch/switch/pipeline start, not resume) exclude; pipeline `ContinueStage`
  correctly does not re-validate. **INV §2** — `ValidateModelEffort`'s hardcoded backend allowlist
  duplicates each adapter's `EffortDelivery`, and `PipelineAttemptRecord` omits effort so reconcile
  reads a continued stage's level from live assignments. Clean/not applicable elsewhere: §1 effort is
  re-applied at resume/switch and republished; §2 `resolveEffort`/`deliveredModelID` are single seams
  and the Codex suffix is composed once for new/load; §3 the ModelRow editor spreads `...model` and
  Settings saves the whole document; §4 `applyPostSessionEffort` shuts down before registration on
  failure; §5 adds no concurrency; §6 joins no new runtime, only the `EffortDelivery` adapter method;
  §7 the archive/session scans extend existing checked loops; §8 effort renders in-vocabulary and
  validation errors surface as field errors; §9 migration v12 is forward-only `NOT NULL DEFAULT ''`
  with the version guard bumped and every read site updated in lockstep; §11 the seed `[]string{}`
  and read normalization agree so the round-trip is stable and `"efforts":[]` is served; §12 external
  CLIs are unchanged; §13 the switch effort control reuses `.form-field`/`.form-error`; §14 the
  effort fields ride existing `localOnly` routes; §15 the post-session step precedes registration.
  A Sonnet subagent ran a parallel correctness pass and independently reached the same Must fix.
  `make check-specs` passes; no product code or specifications changed during review.

- 2026-07-30 — Designed the per-chat runtime picker feature; no product code changed. The chat
  header's static `backend · model · effort` becomes inline backend/model/effort selects for a
  running chat agent, applied through an explicit **Switch** action that reuses the existing
  switch-runtime path (FS-01.R13/R30 — native resume or primer). Interface stays in the dashboard
  dialog; a stopped agent shows static text. FS-03 gained R23/R24/A9 (planned) and moved to Partial;
  R1 now names effort in the runtime identity. No technical-spec change is needed — switch-runtime
  already accepts backend/model/effort over existing architecture. The idea moved to the waiting
  `per-chat-runtime-picker.md` change; `make check-specs` and whitespace checks pass.

- 2026-07-30 — Fixed all four agent-effort-selection review findings (committed in `de8634f`
  alongside the implementation). **INV §11** — the `DefaultBackends` seed now declares `Efforts: []string{}`
  for the OpenCode/OpenHands models, so the seed matches the read-time normalization, the
  `TestRoundTripConfigObjects` round-trip is green, and a fresh install writes `"efforts":[]` rather
  than the `"efforts":null` the read normalization exists to prevent. **INV §4** (FS-09.A15/R40) —
  new chat-runtime tests dump the Claude post-session `session/set_config_option` params on success
  and prove a rejected option fails the launch with no running agent registered. **INV §10**
  (FS-09.R37/R38/A14) — the specs' cited effort coverage now exists: the Codex catalog `efforts`
  import (`ReadCodexModelCatalog`), the New Agent effort control, and the Settings model effort
  editor all gained assertions. **INV §10** (FS-12.R32/A11) — Appearance Settings no longer
  pre-checks Core when a warning is present, so clicking Core saves `""` and repairs a hand-edited
  unsupported skin id straight to Core; a regression covers the repair. No specification change was
  needed: the seed and Core-repair fixes restore already-specified behavior, and the effort tests
  fill the coverage the "Verify by" lists already promised. `make test`, `make build`,
  `make check-specs`, all 185 UI tests, the UI build, and whitespace checks pass.

- 2026-07-30 — Reviewed the uncommitted agent-effort-selection change (FS-01.R30/A14, FS-08.R31/A8,
  FS-09.R35–R42/A14–A15, FS-14.R31/A11, TS-01.R12, TS-02.R18, TS-03.R19, TS-04.R18–R19, TS-07.R14,
  TS-09.R24) in both specification directions and against every invariant class. One **Must fix**
  and two **Worth fixing** findings are recorded above. **INV §11** caught the `Efforts`
  nil-vs-`[]` read/write asymmetry that fails `TestRoundTripConfigObjects`, reddening `make test`
  and persisting `"efforts":null`. **INV §4/§10** caught the untested Claude post-session delivery
  and its failure-teardown, and spec "Verify by" lists citing effort tests that do not exist. Clean
  or not applicable elsewhere: **§1** effort is re-applied at resume/switch and the Codex suffix is
  composed once by `deliveredModelID` for both `sessionNewParams`/`sessionLoadParams`; **§2** the
  load-bearing shared seams (`resolveEffort`, `deliveredModelID`, `ValidateModelEffort`) are single
  authorities used by launch/switch/pipeline; **§3** the ModelRow editor merge-preserves the model;
  **§5** adds no concurrency; **§6** joins no new runtime, only an `EffortDelivery` adapter method;
  **§7** the archive/session scans extend existing checked loops; **§8** effort renders in-vocabulary
  and validation errors surface as field errors; **§9** migration v12 is forward-only `NOT NULL
  DEFAULT ''` with the version-guard test updated; **§12** a provider-rejected level fails closed and
  is deliberately not retried bare; **§13** the new controls reuse `.form-field`/`.form-error`;
  **§14** effort rides existing `localOnly` routes; **§15** the post-session step precedes agent
  registration. `make check-specs` and all 180 UI tests pass; the config Go suite fails on the
  recorded round-trip regression. No product code or specifications were changed during review.

- 2026-07-30 — Implemented agent effort selection. Backends now declare provider-native effort
  levels/defaults; Codex autosync imports cache levels; the shared resolver freezes the resolved
  value through launch, resume, switch, archive, CLI, federation, and per-stage pipeline runs.
  Claude chat applies its setting post-session with failure cleanup, Codex chat uses the model
  suffix, and Claude terminal uses argv. FS-01.R30/A14, FS-08.R31/A8, FS-09.R35–R42/A14–A15,
  FS-14.R31/A11, TS-01.R12, TS-02.R18, TS-03.R19, TS-04.R18–R19, TS-07.R14, and TS-09.R24 are
  current; real provider honoring remains gated in FS-09.A16 and TS-07.R12.

- 2026-07-30 — Reviewed the continuous range after `0f52f89` through `7fc5158` in both specification
  directions and against every invariant class: the Sky & Grove appearance and the archive-retry,
  project-rename field-error, native-dialog guard-bypass, and lifecycle-dialog specification fixes.
  Code and specs agree; FS-04.R38/A18, FS-12.R27–R33/A9–A11, TS-02.R21, TS-03.R21, and TS-08.R30–R36
  match the shipped appearance preference, Core fallback, checker skin rules, and live xterm
  recoloring. One **Worth fixing** finding: the Appearance radio control cannot save Core to repair a
  hand-edited unsupported skin id because Core already renders checked as the effective fallback
  (**INV §10**, FS-12.R32/A11). Clean/not applicable elsewhere: §1 AppearanceRoot/xterm republish on
  the `data-skin` boundary and disconnect the observer on cleanup; §2 one `BUILT_IN_SKINS` allowlist
  and one `resolvePresentationColors` helper, checker-enforced; §3 PUT `/api/config` stays a partial
  merge; §4 terminal teardown disposes observer/subs/ws/term; §5/§6 add no concurrency or runtime;
  §7 `handleGetConfig` falls back on `ErrNotFound`/`ErrCorrupt` and 500s only on unexpected reads;
  §8 the save error rolls back the optimistic cache and toasts, and rename now keeps field-level
  detail; §9 the field is an additive version-1 preference with no migration and `omitempty` Core
  omission; §11 the new fields are scalars with a lockstep Go/TS/manifest id set; §12 no external
  CLI; §13 the presentation checker passes with every new class resolved; §14 the config route keeps
  `localOnly`; §15 the server writes durably before responding and the injected `writeConfig` failure
  preserves the prior choice. Specification checks, the presentation contract check, the Go config and
  server suites, and the appearance/terminal/visual-matrix UI tests pass; the green suite does not
  exercise the recorded finding. No product code or specifications changed during review.

- 2026-07-30 — Implemented the confirmed Sky & Grove built-in appearance. Core remains the
  no-marker default/fallback; Settings writes the global preference through the existing config
  route with optimistic rollback; unknown/corrupt/read-failure paths remain usable; the statically
  bundled skin, finite presentation contract, paired visual matrix, and in-place xterm recoloring
  ship together. All required checks and isolated real-browser switch/reload/onboarding evidence
  pass; FS-04.R38/A18, FS-12.R27–R33/A9–A11, TS-02.R21, TS-03.R21, and TS-08.R30–R36 are current.
- 2026-07-30 — Completed the confirmed Sky & Grove feature design; no product code changed. The
  technical plan uses the existing config route and atomic version-1 `config.json` store, an
  optimistic React Query projection, one root `data-skin` marker, a statically bundled `ad-skins`
  stylesheet, version-2 presentation manifest/checker rules, live xterm recoloring, paired visual
  fixtures, and Core fallback on missing/unknown/failed configuration. The source idea moved to the
  waiting `add-sky-grove-skin.md` change; specification and whitespace checks pass.

- 2026-07-30 — Completed the archive/native-dialog review-fix run: all five Must-fix findings and
  five Worth-fixing findings are fixed or correctly closed. Specification checks, both Go test
  variants, UI tests/build, source build, distribution build, and whitespace checks pass.

- 2026-07-30 — Began the requested Sky & Grove theme design; no product code changed. FS-12 and
  FS-04 now carry planned behavior for the first optional built-in skin, a Settings appearance
  selector, a durable global preference, complete surface/integration coverage, safe Core fallback,
  locally bundled assets, and explicit exclusions. Technical design waits for human confirmation of
  the proposed behavior, so the idea remains under definition and no ready change exists yet.

- 2026-07-30 — Closed the native-dialog ready-change audit finding: history confirms that the
  claimed waiting file never existed, and a retroactive file would be misleading. The gap remains a
  historical record; future behavior changes must use the existing spec-first planned-state and
  ready-change workflow before implementation.

- 2026-07-30 — Closed native-dialog guard bypasses (INV §10): the presentation check now rejects
  `globalThis`, computed-property, and static-alias calls as well as the original bare/window forms;
  its fixtures exercise every equivalent call shape.

- 2026-07-30 — Corrected the shipped lifecycle dialog specification (INV §10): Rename and runtime
  switching now consistently name their core application dialogs, with no stale browser-prompt
  wording left in FS-01.

- 2026-07-30 — Fixed project Rename validation feedback (INV §8/§10): the dialog now preserves the
  server's structured field-level message instead of showing only `HTTP 400`; the overlong-title
  component regression passes.

- 2026-07-30 — Fixed per-project Archive retry visibility (INV §8/§10): a failed independent agent
  page now remains visibly failed beneath its project and offers Retry, while successful rows stay
  rendered for later-page failures. The focused Archive UI regression proves recovery without a
  search change or reload.

- 2026-07-30 — Fixed unexpected project-read errors failing open (INV §7, also §5/§8): Resume,
  Switch runtime, agent Restore, and pipeline start now route their archived-project check through a
  shared `projectArchiveGate` that fails closed with an internal error when the definition is corrupt
  or otherwise unreadable, while a missing definition still proceeds as unavailable-but-active and
  agent Archive stays available for cleanup. The human chose the fail-closed corrupt-project policy;
  FS-05.R34 and TS-03.R20 now record it. The four-path regression was verified to fail against the
  old code (restore even fail-opened to 200, clearing the archive flag); both Go test variants,
  `make build`, focused `-race`, and whitespace checks pass.

- 2026-07-29 — Fixed failed project-archive compensation reporting (INV §15, also §5): if project
  publication and exact flag rollback both fail, the project remains active, the server publishes
  the durable agent flags that remain, and the response reports both causes. TS-02 now records this
  coherent independent-agent-archive fallback, and the injected dual-failure regression passes.

- 2026-07-29 — Fixed idempotent Archive transition races (INV §5/§15): agent and project Archive
  now acquire their exclusive transition claim before the authoritative archived-state read and
  no-op decision. Restore barrier regressions prove competing Archive requests receive the specific
  retryable conflict, then succeed after Restore releases its claim; focused server tests pass.

- 2026-07-29 — Fixed per-project Archive search races (INV §1/§8): each project pager now aborts
  and generation-checks superseded requests, cleanup cancels work across query/project boundaries,
  and the parent supplies a stable error callback. A delayed-old/fast-new regression proves the
  newest query remains rendered; the focused Archive UI suite passes.

- 2026-07-29 — Fixed grouped Archive pagination (INV §7/§8/§10): top-level pages now serialize
  project summaries with non-null empty agent lists, archived counts use a grouped durable query, and
  per-project archive filtering/count/limit/offset execute in one durable search instead of loading
  the full agent corpus. The 201-session regression now bounds group response size and still reaches
  the final project row at offset 200; focused archive/server tests pass.

_(Newest first; durable product truth is in FS/TS and history is in git.)_

- 2026-07-29 — Independently reviewed the continuous range after `dc04dbd` through `70afbe8`,
  covering the completed project-dashboard/archive lifecycle, its review/usability records, and the
  native-dialog replacement in both specification directions and against every invariant class.
  Five Must-fix findings remain: grouped Archive pages still materialize and embed the unbounded
  agent corpus (**INV §7/§8/§10**); per-project search requests can race stale rows into the current
  query (**INV §1/§8**); idempotent Archive bypasses transition claims (**INV §5/§15**); failed
  project-archive compensation is discarded (**INV §15**); and unexpected project-read errors fail
  open on Resume, Switch, pipeline control, and agent Restore (**INV §7/§5/§8**). Four Worth-fixing
  findings cover lost project-title field errors (**INV §8/§10**), contradictory FS-01 prompt text
  and bypassable native-dialog guard checks (**INV §10**), and the missing committed ready-change
  trail. Clean/not applicable: §2 shared construction, §3 persisted/form merging, §4 teardown, §6
  runtime/interface adoption, §9 durability primitives/migration, §11 collection nullability, §12
  external CLIs, §13 CSS selectors, and §14 loopback security. Specification checks, both Go test
  variants, all 170 UI tests, presentation checks, and focused backend/UI suites pass; the green
  tests do not cover the recorded failures. No product code or specifications changed during review.

- 2026-07-29 — Completed the native-dialog replacement. Agent rename, runtime switch, move to group,
  stop/release-group, project rename/color/archive, role/project delete, and in-use force-delete now
  use application dialogs. The static presentation contract rejects first-party native
  `prompt`/`confirm`; dialog regressions cover submission, cancellation, field errors, catalog-driven
  runtime selection, resource-retention copy, and force deletion. FS-01.R32/A16, FS-02.R37/A21,
  FS-04.R37/A17, FS-12.R26/A8, and TS-08.R29 are shipped. Specification checks, the full Go test
  matrix, all 170 UI tests, the UI build, distribution build, and whitespace checks pass. The
  in-app browser could not reach an isolated loopback server that passed a local HTTP health check,
  so the required browser smoke pass is recorded as an automation limitation rather than a product pass.

- 2026-07-29 — Designed the native-prompt/confirm replacement with the human; no product code changed.
  The seven `window.prompt` inputs and eight `confirm()` dialogs across the dashboard, project cards,
  and Settings become styled application dialogs that validate, state consequences, and cancel
  cleanly, issuing today's requests with no API/storage/route change. FS-12.R26/A8 own the umbrella
  (superseding R19's carve-out); FS-01.R32/A16 the rename and cancellable catalog-driven switch;
  FS-02.R37/A21 the move-to-group combobox, stop/release-group, and project card actions; FS-04.R37/A17
  the delete/force-delete-in-use/archive confirmations; and TS-08.R29 the presentation-contract guard
  that forbids native prompt/confirm in `ui/src`. FS-02/FS-04/FS-12/TS-08 move to Partial while those
  items are unshipped. The idea was promoted to the waiting `replace-native-dialogs.md` change;
  `make check-specs`, the twin-skill comparison, and whitespace checks pass.

- 2026-07-29 — Fixed every project-dashboard/grouped-Archive review finding and completed the
  active change. **INV §2/§4/§5/§15** now protect archive/restore claims, live-orphan reaping,
  compensation, and pipeline starts; **INV §1/§3/§8/§10** keep project/Archive state and shared
  layout coherent; **INV §7/§10/§11** cover the complete grouped-search corpus, independent paging,
  documented action responses, and Settings/selector boundaries. The Codex profile statement now
  accurately describes the visibility guarantee for new versus already-running children. Focused
  Go/UI regressions, specification checks, and the normal build/test/distribution verification are
  recorded with this change.

- 2026-07-29 — Independently re-reviewed the uncommitted project-dashboard/grouped-Archive
  implementation after the first pass, from the planned FS/TS requirements through every changed
  backend/UI seam and every invariant class. Seven Must-fix and nine Worth-fixing dashboard findings
  are recorded above; the prior Codex/traceability findings remain unchanged. **INV §2/§4/§5/§15**
  caught Archive bypassing the shared orphan reaper, the missing agent transition claim, destructive
  config-write compensation, a non-waiting project claim, and pipeline transitions taking their lease
  only after durable mutation. Those gaps can leave a live archived agent, erase an earlier individual
  archive, or create a paused run after project archival. **INV §1/§3/§8/§10** caught successful
  project actions leaving the catalog stale and scoped drag replacing the global layout. **INV
  §7/§10** reconfirmed the 200-session truncation and found that the UI never uses the per-project
  agent endpoint, so independent paging is absent at both layers. The remaining spec gaps cover the
  group Load-more gate, card and Settings surfaces, archived-project readiness/selectors, project-read
  errors, Resume precedence, project-action response shape, and stopped-search routing. Clean/not
  applicable: §6 adds no runtime/interface; §9's migration is forward-only with a non-null default
  and version guard; §11's new server collections are non-null (although the Archive UI tests still
  mock the retired flat response); §12 adds no external CLI; §13 all new literal classes resolve; and
  §14 every new route inherits `localOnly` and project paths are slug-validated. `make check-specs`,
  both Go test variants, all 163 UI tests, presentation/source/UI builds, `make build`, and `make dist`
  pass. The new project dashboard, archive actions, transition barriers, grouped response, rollback,
  and per-project paging have essentially no focused coverage, so the green suite does not exercise
  the defects. No product code or specifications changed during review.

- 2026-07-29 — Reviewed the range after `547ca43` through `dc04dbd` plus the uncommitted project-
  dashboard/grouped-archive working tree, in both specification directions and against every
  invariant class. One Must-fix and four Worth-fixing findings are recorded above. **INV §7/§10**
  caught the grouped Archive fetching a fixed 200-session slice and paginating the projection, so a
  corpus over 200 sessions truncates per-project rows and search while the archived-agent count
  still reports the true number (FS-05.R36/A19). **INV §10** caught the Archive **Load more** gate
  comparing agent-row count to the group total (hidden past 50 groups) and the project cards missing
  R30 color/per-state summary and R34 Rename/Change color actions. **INV §15/§5** caught the Codex
  profile publish (`e021ce3`) claiming an atomic per-entry swap it does not perform. Clean/not
  applicable elsewhere: §2 launch/resume/switch/pipeline all route project-start gating and archived
  checks through the shared `launchAgent`/`composeChildEnv` seams and `codexHomeEnv` stays the final
  env layer; §1 the scoped dashboard and archive lists derive from the live agent/project queries;
  §3 the project archive bit is preserved across ordinary `PUT /api/projects` edits; §4/§5 the
  project-start lease and `beginProjectArchive` form one atomic claim under `archiveMu` spanning
  registry registration, and switch rollback tears down registration; §8 archive/restore actions
  surface errors through the envelope and toasts; §11 every grouped/agent collection marshals
  non-null; §13 all new dashboard/archive classNames resolve; §14 new routes inherit `localOnly`
  and slug-validate the project path. The subagent-assisted `e021ce3` correctness pass found the
  transaction/rollback, symlink safety, owner-only modes, prune scoping, switch ordering, and env
  composition otherwise correct. `go build ./...`, the server/state/pipeline Go tests, and
  `make check-specs` pass. No product code or specifications changed during review.

- 2026-07-28 — Validated and fixed all nine specification-review findings in the waiting project-
  dashboard/grouped-Archive design; no product code changed. Stale confirmation deviations now record
  the chosen boundaries. FS-05.R36/A19 define independent project-group and agent pagination, retain
  `active` compatibility and all-session full-text search, and schedule the conflicting flat-list
  requirements/acceptance criteria for retirement only when the replacement ships while retaining
  R4/A2's non-null guarantee. Archived agents restore before Resume; archived defaults remain dormant
  and are excluded from process selectors; TS-01.R13 adds an exclusive project/agent transition claim
  held through archive commit/compensation; unavailable cards/routes and archived routes have complete
  presentation rules; and FS-14's preamble now acknowledges planned requirements. `make check-specs`
  and whitespace checks pass. The ready change remains waiting to start with no unresolved decision.

- 2026-07-28 — Fixed the five Codex-isolation findings and both scoped follow-ups. The private
  setup mirror now selects an explicit setup allowlist, preserves executable setup assets as
  owner-only, rejects unsafe profile destinations/source overlap before mutation, and stages a full
  generation with its manifest before transactional publication; manifest failure restores the old
  profile. Refresh stays locked through child process creation, and unsafe Codex switches are
  rejected before stopping a live agent while failed rollback registrations are torn down. The
  environment-composition matrix now covers the shared launch/resume/switch/rollback composer and
  the stale federation qualifier is removed. Regression coverage includes runtime-state exclusion,
  executable mode, source/destination safety, manifest rollback, concurrent-start barrier, and
  unsafe-switch preservation. `make check-specs`, both Go test variants, `make build`, `make dist`,
  and whitespace checks pass.

- 2026-07-28 — Reviewed the continuous range after `a574076` through `547ca43`, including the
  previously reviewed AgentDecker catalog-membership fix and the new Codex session-isolation feature.
  Five Must-fix and three Worth-fixing findings are recorded above. **INV §12/§10** caught the
  open-ended setup mirror copying the pinned/current provider's real state and stripping executable
  setup modes; **INV §5/§15** caught the partial-generation refresh/start window; **INV §14**
  caught the destination-symlink escape; and **INV §4/§15** caught the failed-switch rollback
  leak. **INV §2** confirmed one final child-env helper is correctly shared by launch, resume, and
  switch but found the promised path matrix unpinned; **INV §10** also found one stale "planned"
  spec qualifier. Clean/not applicable on the remaining classes: §1 is covered by the refresh
  boundary finding; §3 adds no persisted form/one-shot field; §6 adds no runtime/interface; §7
  surfaces changed read failures; §8 uses existing bounded start errors; §9 adds no PID/protocol/
  SQLite migration and its durability concern is covered by the staged-generation finding; §11
  adds no API collection and writes a non-null manifest list; §13 adds no Codex UI/CSS selector;
  the remaining §14/§15 surfaces are owner-only and pre-spawn. Specification checks, both Go test
  variants, focused config/runtime/server race tests, all 163 UI tests, presentation/source/UI
  builds, the distribution build, and whitespace checks pass. The credentialed packaged-Codex gate
  remains unrun and correctly gated. No product code or specifications changed during review.

- 2026-07-28 — Shipped Codex session isolation (FS-08.R32/A9, FS-09.R43/R44/A17, TS-02.R19,
  TS-04.R20/R21). **INV §2** is load-bearing: one `codexHomeEnv` layer, appended last to the shared
  `composeEnv` in launch/resume/switch, points every codex-acp child at `<home>/codex` and overrides
  any ambient/backend/model `CODEX_HOME`; `spawnCmd` reads that composed value back as the single
  source of truth and refreshes the profile before spawn, so a resumed session never opens a
  different store. The new `internal/config/codexprofile.go` provisions the `0700` profile and, under
  a package mutex, one-way mirrors the personal `${CODEX_HOME:-~/.codex}` top-level setup into
  owner-only files while excluding session/history entries, dereferencing in-root symlinks, failing
  on a root-escaping one, and pruning managed copies of removed personal setup via
  `cache/codex-profile.json` without touching the child's own session data. **INV §12** shapes the
  gate: the packaged `codex-acp` honoring a non-default `CODEX_HOME` and the refreshed setup stays a
  credentialed live-CLI acceptance, and fake-ACP tests assert only the composed env and refresh
  contract. Config profile-refresh, env-composition, and a codex spawn-refresh test were added; both
  Go test variants, `make build`, `make check-specs`, focused `-race` on the config/runtime refresh,
  and whitespace checks pass. No UI changed. AgentDeck's own process `CODEX_HOME` is untouched, so
  federation discovery and model autosync still read the real home.

- 2026-07-28 — Refined the planned Codex session isolation with the human; no product code changed.
  `CODEX_HOME=<agentdeck-home>/codex` remains the always-on, new-launch-only boundary, but the
  private profile now refreshes personal Codex setup before every launch/resume/switch rather than
  symlinking only auth/config: auth, configuration, skills, agents, rules, plugins, and MCP setup
  are copied one way; session/history data is excluded; source additions/edits/removals apply at the
  next child start; and AgentDeck never writes to the personal home. This keeps AgentDeck Codex
  capable with the user's normal setup while its sessions stay private. Pre-existing AgentDeck Codex
  sessions may no longer native-resume, as accepted by the human. FS-08 gained R32/A9; FS-09 R43/R44
  and A17, TS-02 R19, TS-04 R20/R21, and the ready change now define the profile-refresh boundary,
  final `CODEX_HOME` precedence, managed-path cleanup, source-link safety, and the exact packaged
  live-CLI gate. `make check-specs` and whitespace checks pass; no product code or tests changed.

- 2026-07-27 — Fixed the AgentDecker builder's stale-project launch. **INV §10** found the wiring
  incomplete: readiness treated any non-empty `project` state as launchable, so a project deleted
  from Settings or another tab after the builder opened left the controlled select showing no
  selection while the launch still posted the removed id and was rejected. Readiness now requires the
  selection to be a current catalog member (`projects.data?.[project]`), and a selection that leaves
  the catalog is cleared so the seed effect and launch gate re-evaluate against the live catalog.
  This restores FS-14.R26's "launch limited to a listed project" boundary, so no specification change
  was needed. The catalog-refresh regression was verified to fail against the old code; specification
  checks, both Go test variants, all 163 UI tests, the UI/distribution builds, and whitespace checks
  pass.

- 2026-07-27 — Reviewed the continuous 12-hour range from `cc9d498` through `a574076`, including
  all committed UI/runtime/test/specification changes and the clean working tree. Code and
  specifications agree on the repaired review batch, Dashboard presentation/Resume, builder picker,
  annotation context menu, and the planned effort-selection change; every invariant class was
  swept. One Must-fix remains: a selected project that disappears after the builder opens can still
  be launched because readiness checks only for a non-empty id rather than a current catalog member
  (FS-14.R26). One Worth-fixing process finding remains: the three new behavior changes were shipped
  without the required ready-change/active-plan trail, so spec-first sequencing is not auditable.
  `make check-specs`, both Go test variants, the full 162-test UI suite, the source build, the
  release build, and whitespace checks pass. No product code or specifications changed during this
  review.

- 2026-07-27 — Designed launch-time effort selection with the human; no product code changed.
  Verified both pinned adapters rather than assuming: `codex-acp` 1.1.2 encodes effort **inside** the
  ACP model id as `model[effort]` (so no new protocol field is needed), and its per-model levels are
  already published in the `models_cache.json` that FS-09.R28 autosync reads; `claude-agent-acp`
  0.62.0 accepts effort only as a post-`session/new` configuration option seeded from the native
  settings chain AgentDeck must not write; and Claude Code 2.1.202 takes `--effort` for terminal
  launches. FS-09.R35–R42/A14–A16 now specify optional per-model `efforts`/`default_effort`,
  rejection of bracketed provider model strings, add-only autosync of levels, the claude/codex-only
  capability boundary, the Claude post-session teardown, precedence, and undeclared-level rejection.
  FS-01.R30/A14 add the launch field, `--effort`, and effort's place in runtime identity; FS-14.R31/A11
  add per-stage run assignment; FS-08.R31/A8 make the long-inert effort override real. TS-01.R12 pins
  one `resolveEffort` on the shared composition seam, TS-02.R18 keeps `backends.json` at version 2 and
  adds a forward-only `sessions.effort` column, TS-03.R19 adds optional fields to existing routes only,
  TS-04.R18–R19 pin the three adapter-declared delivery mechanisms and fail-closed rejection, TS-07.R14
  feeds the federation override into the resolver, and TS-09.R24 snapshots per-stage effort.
  **INV §2** is load-bearing (the Codex suffix must be composed once for both `sessionNewParams` and
  `sessionLoadParams`), with **INV §4** on the pre-registration teardown, **INV §1** on re-applying
  effort at resume, and **INV §12** deliberately departed from (fail closed, never retry bare).
  Recorded FS-08's pre-existing spec/code mismatch as a deviation rather than silently fixing it.
  FS-01, FS-14, TS-02, TS-03, and TS-09 move to Partial while their planned items are unshipped. The
  source idea was promoted to the waiting `agent-effort-selection.md` change; specification,
  twin-skill, and whitespace checks pass.

- 2026-07-27 — Gave the AgentDecker builder its own project selector. **INV §10** found the
  shipped wiring incomplete: the builder launches an ordinary chat agent, which needs a real project
  directory, but exposed only backend and model and sent `default_project` unconditionally — so the
  seeded `my-app` produced a launch the page could not repair. **INV §2** supplied the fix shape: the
  builder now uses the same existence-checked default seed and project `<select>` as `RunStartForm`,
  rather than a second, weaker way of choosing the same thing. Readiness follows the selected
  project, so a default naming a removed project holds the launch closed instead of enabling a button
  whose only outcome is a rejected launch. FS-14.R26/A10 gained the project choice and that
  boundary. Both regressions were verified to fail against the old code; all 158 UI tests, both Go
  test variants, specification checks, the UI/source/distribution builds, and whitespace checks pass.
  A real-browser check on an isolated home confirmed the picker renders and behaves in both
  directions with no console error.

- 2026-07-26 — Added direct Dashboard Resume. **INV §10** found no new invariant boundary: the
  existing resume operation already restores the frozen session snapshot, but was unnecessarily
  reachable only through Archive. A stopped card's right-click menu now offers **Resume**, while a
  running card omits it; failures use the established error toast. FS-02.R27–R28/A14 pin menu
  visibility, the endpoint, and retryable failure behavior. Focused menu regressions, all 156 UI
  tests, both Go test variants, specification checks, and the distribution build pass.

- 2026-07-26 — Restored visible Dashboard state-badge labels. **INV §10** found a presentation
  selector that matched every `span` inside a badge, converting its label into a state-coloured
  rectangle. The selector now targets only `.ad-badge-indicator`, while the base badge variant
  supplies its state-coloured text. FS-02.R3's existing vocabulary is restored; all six labels,
  the visual matrix, the full 153-test UI suite, both Go test variants, and the release build pass.
  A browser check confirms Busy, Idle, Waiting, Done, Error, and Unknown render as readable labels.

- 2026-07-26 — Clarified Dashboard card identity and context. **INV §10** found no additional
  invariant breach: the problem was an explicit but misleading presentation rule. FS-02.R25 now
  resolves the subtitle's durable project id through the current project configuration and falls
  back safely if that definition is absent; FS-02.R26 replaces the blank zero-context track with a
  visible `0% context used` label. Card/grid/context regressions cover title, fallback, and zero;
  the visual matrix renders the zero state and human-readable project label; and J5 now requires
  those observations. Specification checks, both Go test variants, all 152 UI tests, source/UI and
  distribution builds, and a real-browser visual-matrix check pass.

- 2026-07-26 — Retried the blocked browser validation without changing product code or specifications.
  **No invariant class** applied because no completed browser path produced a finding. Native confirmation
  completed, unlike the earlier stalled run, but that listener belonged to an older review home and was
  excluded as evidence. A new isolated J7 listener rendered the stopped-agent dashboard, transcript,
  and Archive list, after which the in-app browser repeatedly lost its tab after page transitions.
  J5 restart/delete and J6–J14 remain unconfirmed, not passed. `make check-specs` and whitespace
  checks pass; the full result is
  [`../archive/reviews/usability-review-run-2026-07-26-browser-retry.md`](../archive/reviews/usability-review-run-2026-07-26-browser-retry.md).

- 2026-07-26 — Fixed all seven open review findings. **INV §4** now copies the launch generation
  onto a resumed chat runtime, so a resumed or switch-resumed crash drops registry ownership, tears
  down hook/MCP/settings registration, allows an immediate re-resume, and pauses its pipeline stage
  as `agent_crash`; the resume→crash regression asserts the delivered generation and a matching
  pipeline test pins that an empty generation is ignored. **INV §1** makes the missing-source
  annotation recovery wait for completed agent hydration, so a deep link or reload shows a loading
  state instead of offering to discard live drafts, and classifies pipeline attempt transcripts and
  the persisted AgentDecker builder by `running` rather than identity presence, so finished attempts
  reach Archive and a stopped builder's browser key expires. **INV §1** also selects the newest
  session metadata for the archived header, so a switched session shows the identity Resume will
  restore. **INV §10** makes Archive paging render every matching session exactly once: a repeated
  row proves the `updated_at` ordering shifted, so the first page is refetched and whatever moved
  into it is recovered. **INV §8** routes both onboarding steps through the shared config error
  parser, so a repairable 400 names its field and reason instead of `Error: HTTP 400`. FS-13.R16/A8
  and FS-05.R28/A12 gained the new hydration and reachability boundaries; the other five fixes
  restore existing requirements. Every regression was verified to fail against the old code.
  Specification checks, both Go variants, 149 UI tests, presentation/source/UI builds, the
  distribution build, focused `-race` on the resume crash path, and whitespace checks pass.

- 2026-07-26 — Reviewed the continuous range after `ccc2b50` through `cc9d498` in both
  specification directions and against every invariant class. Two Must-fix and five Worth-fixing
  findings are recorded above. **INV §1/§2/§4** caught the missing generation on chat Resume and the
  pre-hydration destructive annotation recovery; **INV §1/§10** caught identity presence being used
  as liveness in two pipeline links, stale archived session metadata, and mutable offset paging;
  **INV §8** promoted the two carried onboarding `HTTP 400` leads after confirming the shared parser
  is bypassed. Clean on the remaining changed surfaces: §3 seeded forms remain merge-preserving; §5
  onboarding/index/pipeline claims are serialized; §6 added no new runtime or driver and its changed
  lifecycle contract is covered by the Resume finding; §7 readers check their real error signals;
  §9 FTS downgrade and cancellation cause handling are bounded; §11 collections and client schemas
  stay non-null/in lockstep; §12 the Claude optional-flag retry is fixed-command and fail-closed;
  §13 new selectors and presentation states resolve; §14 adds no route and existing routes remain
  under `localOnly`; §15 annotation/index/notification side effects follow their durable writes.
  The one-off Archive 500 and untimed tmux calls remain unconfirmed, not findings. Specification
  checks, both Go variants, 139 UI tests, source/UI/presentation builds, the distribution build, and
  whitespace checks pass. No product code or specifications were changed.

- 2026-07-26 — Confirmed the `/fix` queue is empty. **No invariant class** applied: every
  recorded Must-fix and Worth-fixing item is already resolved, no active change is selected, and
  the worktree is clean. `make check-specs` passes; no product or specification change was needed.

- 2026-07-26 — Re-ran the post-fix usability matrix at `c59dd2c`. **No invariant class** applied
  because the completed browser paths produced no finding. J1–J4 and the exercised J5 layout paths
  passed in the release build with new isolated homes and zero unexpected console errors; the
  onboarding/provider fallback and all four permission labels were confirmed live and after reload.
  The in-app browser stalled on J5's native confirm, then the execution account refused further
  loopback servers after reaching its usage limit. J5 restart/delete and J6–J14 are recorded blocked,
  not passed. Static serialization/CSS/null sweeps were clean; external-CLI and mutation-error hits
  remain source leads only. All 139 UI tests, presentation checks/build, specification checks, and
  focused tagged/untagged recent-fix regressions pass. Full report:
  [`../archive/reviews/usability-review-run-2026-07-26-rerun.md`](../archive/reviews/usability-review-run-2026-07-26-rerun.md).

- 2026-07-26 — Distinguished permission outcomes. **No invariant class** applied; FS-03.R4/R17/A6
  now retain the runtime's approved, denied, cancelled, and timed-out outcomes through both live and
  replay transcript projection. Permission chips render **Approved**, **Denied**, **Cancelled**, or
  **Timed out** with declared presentation states. Store and renderer regressions cover all four;
  unknown legacy decisions remain conservatively denied.

- 2026-07-26 — Identified archived transcripts before Resume. **No invariant class** applied;
  FS-05.R31/A15 now require the read-only header to show recorded name, project, backend/model, and
  creation date. The view requests the transcript's existing session metadata instead of relying on
  live-agent state, retains a loading fallback, and renders the identity alongside its archived
  state. A UI regression verifies the metadata query and every header field.

- 2026-07-26 — Identified cancellation-escalation exits. **INV §8** now marks the exact turn when a
  peer ignores cooperative cancellation and receives fallback SIGINT; if that signal ends the
  process, the durable fatal error, error status, and turn-end reason identify Cancel as the cause
  while ordinary crash diagnostics remain unchanged. FS-01.R7 and FS-03.R9/A5 pin the outcome. The
  hung-peer regression now verifies running-row removal, status detail, transcript error, and reason.

- 2026-07-26 — Rejected blank project identity fields. **INV §8** now treats whitespace-only title
  and cwd values as missing in the shared project validator used by both create and update, so such
  a project cannot enter the New Agent target list. A field-specific config regression requires
  both diagnostics. No FS/TS delta was needed because the fix restores FS-04.R7.

- 2026-07-26 — Unified agent display-name validation. **INV §8** now routes launch, rename, and
  identity updates through one trim/nonblank/NUL-free/256-character boundary; omitted launch names
  still receive a curated suggestion, while explicit whitespace can no longer create a titleless
  card. FS-01.R4/R8/R22/A1/A6 pin the contract. Focused regressions cover the Unicode limit and
  launch rejection for whitespace, NUL, and overlong input.

- 2026-07-26 — Aligned live and replayed permission resolution. **INV §2** now makes the live
  resolver stop at the newest matching permission request, exactly like the durable transcript
  fold, so a reused tool-call id cannot rewrite earlier chips. A store regression covers both an
  incoming live resolution and the optimistic user-action path. No FS/TS delta was needed because
  the fix restores FS-03.R4/R12/A6.

- 2026-07-26 — Added stale annotation-tray recovery. **INV §1** now gives a missing-agent route a
  direct discard action with its retained draft count, and SSE reconnect contains the corresponding
  missing-transcript rejection instead of surfacing an uncaught error. FS-13.R16/A8 pin the
  recovery boundary; route and reconnect regressions cover it without treating archived sources
  as deleted.

- 2026-07-26 — Kept annotation delivery controls visible. **No invariant class** applied;
  FS-13.R18/A10 now split the bounded pending tray into an independently scrolling draft body and
  a fixed delivery footer, so detailed drafts cannot push target selection, errors, or Send below
  the visible edge. A three-draft UI regression verifies the body/footer boundary.

- 2026-07-26 — Made diff-line selection discoverable. **No invariant class** applied;
  FS-13.R17/A9 now require a visible instruction and pointer cursor on selectable line numbers.
  `DiffBlock` explains that line-number clicks select a range, the renderer theme supplies the
  pointer affordance, and the existing library-shaped selection regression now checks the helper.

- 2026-07-26 — Made Archive search capability honest. **INV §8** now reports
  `search_mode` on every Archive response from one shared SQLite capability detector; the metadata-
  only path removes transcript from the placeholder and explains that transcript text/snippets are
  unavailable, while full-text mode retains the one-turn guidance. FS-05.R30/A14, TS-02.R10, and
  TS-03.R18 pin the API/UI contract. Tagged, untagged, server, and UI regressions cover both modes.

- 2026-07-26 — Explained Archive search document boundaries. **INV §8** now gives
  the search input an accessible scope note stating that all terms must match one metadata record,
  transcript turn, or annotation and are not combined across turns, so an empty result no longer
  implies absence from the whole session. FS-05.R29/A13 specify the affordance and its UI regression.

- 2026-07-26 — Kept lifecycle usable across an FTS5 capability downgrade. **INV §8**
  now probes an existing virtual search table before each derived-document boundary; when the
  SQLite driver reports the module unavailable, AgentDeck skips only the FTS document and still
  commits authoritative session metadata, counters, and turn rollups. FS-01.R29/A13 and TS-02.R10
  now pin that degradation and its bounded error surface. Focused tests simulate the unavailable
  writer in both SQLite build variants.

- 2026-07-26 — Added Archive paging. **INV §10** now exposes **Load more** whenever
  the reported match total exceeds the rendered rows, requests the next offset, appends the page,
  and preserves the first page if a later request fails. FS-05.R28/A12 now specify the UI contract,
  and TS-03 no longer records pagination as incomplete. A two-page UI regression verifies offset
  progression and complete reachability.

- 2026-07-26 — Fixed run-start validation feedback. **INV §8** now routes rejected
  start errors through the shared pipeline diagnostic parser and renders each field and repairable
  message alongside the summary, matching the template editor. A UI regression submits a valid
  form, injects an assignment diagnostic, and verifies the field and message remain visible. No
  FS/TS delta was needed because the fix restores TS-09.R3/R23 and FS-14.R18.

- 2026-07-26 — Fixed Codex pipeline assignments. **INV §2** no longer applies the
  role/project filename slug rule to backend and model catalog keys before the shared lifecycle
  validator checks the configured catalog, so seeded `gpt-5.6-sol` assignments now start and reach
  the ordinary launch path. A manager regression starts a run with that exact model id. No FS/TS
  delta was needed because the fix restores FS-14.R2/A1 and FS-09.R33.

- 2026-07-26 — Fixed diff-line annotation selection. **INV §10** now parses the
  `react-diff-viewer-continued` line-id contract (`L-<line>` / `R-<line>`) instead of a format the
  dependency never emits, restoring FS-13.R1/R2 in live and archived transcripts. A component
  regression drives a library-shaped `L-2` id through selection and verifies the captured anchor
  and excerpt. No FS/TS delta was needed because the fix restores existing requirements.

- 2026-07-26 — Drove the week's shipped work through the real rendered UI: J14 pipelines, J13
  annotations, J8 archive/search on both build variants, J1/J3/J4/J11 core regressions, and J12
  restart durability, against isolated homes and the fake ACP peer. Stage results were reported
  through the local MCP endpoint with each stage agent's own minted token while its turn was held
  open, so runs advanced through genuine turn quiescence. Sixteen findings are recorded above, six
  Must fix. The two that matter most are unreachable shipped capabilities rather than regressions:
  **INV §10** — `DiffBlock` parses a line id format the diff library does not emit, so FS-13's
  diff-line annotation has never worked in a browser and no test covers that third-party seam;
  **INV §2** — pipeline stage assignment validates model ids with the role/project filename slug
  rule, which forbids dots, so both seeded Codex models are rejected and FS-14.R2's own
  Codex-plus-Claude example is impossible. **INV §10** also caught the archive rendering only its
  first 50 sessions while reporting the true total, and **INV §8** caught three dead ends: dropped
  run-start diagnostics, a raw `no such module: fts5` blocking every launch on a downgraded home,
  and cross-turn searches answering "No results" with no signal. Clean on the rest: the full
  pipeline lifecycle including approval gates, repair loops, revision compare-and-swap, stop/
  blocked/retry recovery, shared-workspace confirmation, restart recovery and run deletion; the
  annotation tray, its three delivery routes and archive searchability; both archive build variants;
  a styled empty home with zero console errors; delta coalescing; all four permission outcomes
  including cancel-while-pending; and restart durability across agents, layout, archive and paused
  runs. Static sweeps were clean on serialization, CSS wiring and null hostility. The full report is
  [`../archive/reviews/usability-review-run-2026-07-26-week.md`](../archive/reviews/usability-review-run-2026-07-26-week.md).
  No product code or specifications were changed; credentialed providers remain gated.

- 2026-07-26 — Fixed the J2 Claude readiness blocker. **INV §12** now recognizes common
  `unknown`/`unrecognized`/`unsupported`/`invalid` option or flag diagnostics, Go's undefined-flag
  wording, and unexpected-argument wording only when the output also names `-no-color`; it then
  retries the same fixed status argv without the optional flag. Unrelated status/auth failures do
  not enter the fallback, and raw output remains behind the bounded result vocabulary. No FS/TS
  delta was needed because the fix restores FS-04.R17/R34/A14, FS-09.A5, and TS-04.R15. The focused
  regression, specification checks, both Go variants, source/presentation/UI builds, the
  distribution build, and whitespace checks pass. Credentialed providers remain gated.

- 2026-07-26 — Replayed J2 onboarding through the real rendered UI against two fresh isolated homes.
  Set up later closes the modal by pointer click, persists completion, leaves the seeded backend
  catalog/defaults and project set unchanged, and launches no session. Missing-adapter, signed-out,
  and ready Check again transitions work; the ordinary project, optional Config, first fake-ACP
  launch, completion write, and restart reread also pass with no browser console error. **INV §12**
  found one reproducible blocker: a ready Claude adapter that rejects optional `--no-color` as
  `unknown flag` rather than the one hard-coded `unknown option` phrase is wrongly reported failed,
  so the wizard cannot advance. The full report is
  [`../archive/reviews/usability-review-run-2026-07-26-j2.md`](../archive/reviews/usability-review-run-2026-07-26-j2.md).
  No product code or specifications were changed; credentialed providers remain gated.

- 2026-07-26 — Fixed all seven open review findings. **INV §4** now carries the concrete launch
  generation through every runtime exit and tells pipeline-owned stops from ordinary user stops, so
  stopping a stage card pauses the run as `agent_stopped` with Retry available instead of wedging
  `await_result`. **INV §8** moved the immutable stage-reporting protocol and every declared output
  name outside the variable prompt budget, and pipeline notifications now use the run display name
  plus final outcome. **INV §10** routes stopped attempt transcripts to Archive; **INV §1** drops a
  persisted AgentDecker builder id after live-agent hydration proves it stale. **INV §5** gives Set
  up later, backend validation, project creation, source actions, and first launch/completion one
  synchronous UI mutation claim. **INV §2/§5** makes turn-end and annotation consumption plus flush
  one sequence-bounded index operation; later-sequence text remains buffered for the next immutable
  document. No FS/TS delta was needed because each fix restores existing requirements. Regression
  coverage includes the ordinary stage stop endpoint, a maximum-size Unicode input, route/lifecycle
  presentation, notification payloads, four deferred onboarding mutations, and deterministic index
  interleavings. Specification checks, both Go variants, 126 UI tests, source/UI builds, focused race
  tests, the distribution build, and whitespace checks pass.

- 2026-07-26 — Reviewed the continuous range after `eb63dd5` through `ccc2b50` — the six-finding fix
  batch and the whole configurable-pipeline-runs implementation — in both specification directions
  and swept the diff against every invariant class. FS-14.R1–R30 and TS-09.R1–R23 match the code:
  templates are filename-addressed version-1 JSON behind one canonical validator, runs are frozen
  snapshots advanced only by an explicit authenticated result plus a persisted turn boundary, every
  transition is a revision compare-and-swap, and the shipped surfaces (REST, SSE, MCP tools, CLI,
  Pipelines page) all exist. Three must-fix and two worth-fixing findings are recorded above.
  **INV §4** caught that a solicited stop drops the launch generation before the exit callback, so an
  ordinary Stop on a stage agent wedges its run with no valid recovery action. **INV §8** caught the
  assignment renderer clipping its own mandatory reporting instruction when a named value approaches
  the value bound, and the notification builder naming runs by opaque id with no terminal outcome.
  **INV §10** caught attempt-history transcripts linking to the live agent route rather than the
  archive. **INV §1** caught the builder agent id persisted in `localStorage` with no boundary
  cleanup. Clean on the remaining classes: §2 manual and pipeline launch/resume/stop share
  `launchAgent`/`composeResumeSpecWithGeneration` and one validator; §3 run snapshots are frozen and
  template PUT is a deliberate whole-document replace; §5 every mutation is a revision CAS and the
  report/quiescence writes are single row-count-checked transactions; §6 stage agents are ordinary
  chat agents and the `Lifecycle` seam carries the checklist; §7 all four new list queries check
  `rows.Err()` and malformed runs/templates are isolated per entity; §9 launch/continue/stop
  corroborate ownership through `Owns` plus PID-checked reaping and startup never invents a result;
  §11 `NormalizeTemplate`, `nonEmptyJSON`, and literal-initialised slices keep every collection
  non-null; §12 the range adds no external CLI invocation and tightens two; §13 all 62 static plus 6
  dynamic Pipelines class names resolve; §14 all 13 new routes register inside the `localOnly` mux
  and template files keep `ValidSlug` on read/write/delete; §15 report, values, and action intent all
  commit before the tool returns or a process starts. Specification checks, both Go test variants,
  120 UI tests, and source/UI builds pass; the open paths need stop-injection, prompt-bound, and
  archive-link coverage. No product code or specifications were changed.

- 2026-07-25 — Shipped configurable pipeline runs (FS-14, TS-09). Added reusable model-neutral
  version-1 JSON templates with bounded repairable validation; forward-only SQLite run, attempt,
  value, and idempotency state; a compare-and-swap sequential reconciler driven by explicit scoped
  stage results and persisted turn quiescence; shared manual/pipeline launch, resume, stop, teardown,
  and generation ownership; restart/crash/blocked/approval/retry/loop/stop recovery; exact
  AgentDecker template/run proposals; and local REST, revisioned SSE, notifications, and thin CLI
  controls. Added the Pipelines page with structured editing, run setup/supervision/history,
  advisory shared-workspace confirmation, transcript/agent links, notification settings, and
  ordinary-agent pipeline associations. Malformed run detail is isolated in summaries/startup,
  collection shapes stay non-null, and all routes inherit Host/Origin enforcement. FS-14/TS-09 and
  their TS-01–TS-05/TS-08/FS-12 deltas are Current; J14 now owns the composed usability charter.
  Specification checks, both Go variants, 120 UI tests, presentation/UI builds, focused race tests,
  distribution build, whitespace checks, and an isolated real-browser Pipelines pass succeed. The
  browser pass caught and fixed form overflow and empty-run layout defects. Credentialed providers
  remain the existing manual gate, and the two unrelated review findings remain open.

- 2026-07-25 — Defined and received final human confirmation for the feature-side scope of
  configurable pipeline runs. Draft FS-14 now specifies model-neutral
  generic stage templates, run-time backend/model assignment, named opaque-text inputs/outputs,
  sequential outcome routing with approval gates and bounded repair loops, explicit authenticated
  stage results, restart-safe attempts, safe blocked/retry identity, retained run history, and
  visible shared-workspace concurrency. The run-start editor changes only the run name, project,
  goal, declared input values, and per-stage backend/model; structure and stage semantics remain
  template-owned. A dedicated Pipelines page supports manual creation and a
  provider/model-selected AgentDecker builder. AgentDecker can propose exact Save or Start actions;
  each executes only after a separate one-time UI confirmation, as a soft interaction guard under
  the existing same-user local-API trust model. TS-09 and small TS-01–TS-05 deltas now specify one
  in-process transactional reconciler, JSON templates, SQLite run state, scoped MCP result/proposal
  tools, shared lifecycle services, REST/SSE/CLI surfaces, restart recovery, and the Pipelines UI—no
  queue, second service, parallel graph engine, or new authentication layer. Effort remains the first
  separate idea. The source idea was promoted to the waiting `configurable-pipeline-runs.md` change;
  specification/whitespace checks and the Claude/Codex design-skill comparison pass, and no product
  code changed.

- 2026-07-25 — Fixed six of eight open review findings. **INV §15** closed: live annotations
  now use AppendAnnotationAndSync to persist and flush index before publishing, ensuring
  append failures block delivery and retries do not duplicate. **INV §15** closed: session
  upsert and metadata document replacement moved into one atomic SQLite transaction. **INV §12**
  closed: Codex prober reserves bounded 2-second deadline for native login so hung CLI cannot
  exhaust API-key fallback path; added regression test with hanging Codex CLI. **INV §8/§12**
  closed: Claude status failures return only bounded vocabulary ("status_check_failed") instead
  of raw output containing account identity. **INV §8** closed: sign-in error message updated
  to remove misleading dashboard reference. **INV §10** closed: FS-13 spec citation updated
  from retired R5 to active R25-R27. Specification checks, both Go test variants, 113 UI tests,
  source/UI builds pass. Two must-fix findings remain (onboarding wizard race, index boundaries
  atomicity).

- 2026-07-25 — Reviewed the continuous range after `8b84e4f` through `eb63dd5` in both spec
  directions and swept every invariant class. Six must-fix and two worth-fixing findings are recorded
  above. **INV §15** caught that live annotation persistence still hides append/sync failures and that
  metadata replacement is split from the session transaction. **INV §2/§5** caught non-atomic index
  document boundaries; §5 also caught the competing onboarding completion actions. **INV §8/§12**
  caught provider-output leakage, the exhausted Codex fallback deadline, and misleading sign-in copy;
  **INV §10** caught FS-13's citation to retired search behavior. Clean on the remaining classes:
  §1 annotation-tray lifecycle cleanup is bounded; §3 onboarding merge-preserves catalogs; §4 the
  installer owns both temporary artifacts on all exits; §6 has no new interface/runtime/driver; §7
  the new streaming/SQL readers check iteration errors; §9 migrations preserve legacy content and
  probe liveness is bounded overall; §11 collection shapes stay non-null; §13 all new UI selectors
  resolve; §14 adds no route and the existing annotation route remains under `localOnly`.
  Specification lint, both Go variants, 113 UI tests, source/UI builds, and distribution build pass;
  the open paths need new injected-failure and interleaving coverage.

- 2026-07-25 — Replaced whole-session transcript indexing with immutable turn documents. **INV §10**
  keeps raw NDJSON authoritative and the FTS projection rebuildable: a migration splits the old row
  into current metadata plus a preserved legacy content document, while new turns and annotation
  flushes append deterministic documents and metadata updates replace only metadata. **INV §5** no
  longer requires restart seeding because no replace-style transcript accumulator exists; the only
  buffer is the current turn and it is cleared after commit. **INV §2/§7** route live and reindex
  through the same event/document helpers, with `Reader.ForEach` making reindex and sequence recovery
  streaming rather than whole-session reads. Archive FTS groups document hits back to one session,
  counts/paginates distinct sessions, chooses the best transcript snippet, and intentionally requires
  every term or phrase inside one document. Existing fallback metadata search is unchanged. FS-05 and
  TS-02 return to Current; the more complete cross-turn/size-bounded alternative and its evidence
  threshold remain in `ideas.md`. Specification checks, both Go variants, focused index `-race`,
  source build, and distribution build pass.

- 2026-07-25 — Repaired onboarding credentials and defaults, the last waiting ready change.
  **INV §2** was the load-bearing find: `agentdeck auth` and the credential prober each carried their
  own copy of the provider commands, and the CLI's copy was simply wrong — `codex-acp` is a stdio ACP
  server that ignores argv, so `agentdeck auth codex` started a server and hung instead of signing
  anyone in. One `internal/backend/providerauth` table now owns both verbs for both providers, with a
  test asserting login and readiness share an executable. Codex readiness asks that CLI for
  `login status` before falling back to the API key, so a ChatGPT-signed-in user with no
  `OPENAI_API_KEY` is ready (FS-09.R34); verified against the real signed-in CLI and through
  `PUT /api/backends` on an isolated home, where the gate went from held-closed to satisfied.
  **INV §12** shapes the failure vocabulary: an uninterrogable CLI reports skipped, never failed, and
  "not logged in" is matched before "logged in" so the substring read cannot invert. The wizard gained
  the **Set up later** completion path (FS-04.R32) and lost both model-identifier fields (R33),
  submitting the seeded catalog unchanged per **INV §3**; provider sign-in is now named guidance plus
  Check again over the same backend save (R34, TS-03.R15). Fresh homes seed `sonnet`/`gpt-5.6-sol`
  aliases rather than dated pins (FS-09.R33), and re-seeding still leaves an existing catalog
  byte-for-byte intact. TS-06.R22 pins `@openai/codex` as a direct release dependency — the version
  was already resolved transitively, so the lockfile moved three lines — validated before packaging
  and proven to resolve with no global Codex installed. FS-04, TS-03, and TS-06 flip to Current.
  Specification checks, both Go test variants, focused `-race`, 113 UI tests, source/UI/dist builds
  pass. J2 in a real browser is the remaining unproven surface.

- 2026-07-25 — Fixed all five open findings. **INV §15** (Must fix): the annotations endpoint
  delivered reserved mail or started the prompt turn before appending the source annotation event, so
  any append failure returned 500 after an irreversible effect and the preserved tray's retry
  delivered a second copy. The handler now validates the target, composes its payload, appends the
  durable event, and only then delivers through one deferred closure; FS-13.R5 and TS-03.R14 pin the
  ordering, including the honest residue that a delivery failure after the append records a second
  annotation event on retry. `TestAnnotationAppendFailureDeliversNoMailAndRetrySendsOnce` blocks the
  transcript directory and asserts one mail row across the retry (new FS-13.A7). **INV §2**: the
  duplicated `clipExcerpt` collapsed into `ui/src/lib/annotations.ts` with the server marker
  authoritative, and both local `AnnotationDraft` re-declarations now import the canonical type.
  **INV §1**: pending trays are dropped when an agent is deleted, and expire/cap on rehydration
  (30 days, 20 sources) — new FS-13.R16/A8, since nothing on the server owns that state and the live
  agent list deliberately excludes archived sources. **INV §4**: the piped installer's temporary
  bootstrap is now removed by the lock-holding child, the last process that reads it, guarded to the
  installer's own `mktemp` name; the piped-install regression runs under a private `TMPDIR` and
  asserts nothing is left behind. Specification checks, both Go test variants, focused `-race` on the
  annotation path, 107 UI tests, source build, and distribution build pass.

- 2026-07-25 — Audited the prior two weeks of fixes against the invariant catalog and kept only
  repeatable, cross-cutting lessons. New INV §15 requires local durable state before releasing an
  external peer or observable side effect, with atomicity, rollback, or idempotency for retryable
  multi-store mutations; the open annotation partial-delivery finding now cites it. INV §2 now names
  live/replay event projection as one logical artifact and records the shared transcript reducer.
  INV §11 records the second null-collection recurrence from incomplete nested backend maps and
  adds server validation plus `?? {}` to the canonical pattern. Release-only and provider-specific
  fixes stayed in their narrower FS/TS requirements and regression tests rather than bloating INV.
  Documentation checks and whitespace validation pass; no product code changed.

- 2026-07-24 — Closed the routing hole that let the invariant sweep be skipped. The read order in
  every launcher said to read "the relevant FS/TS/INV items named by the handoff/request", so when no
  INV item was named an agent resolved that to nothing and never opened the file where the sweep
  instruction lives. `/review`, `/work`, and `/fix` (both twin copies) now name `INVARIANTS.md` as an
  unconditional read and state the sweep, the §6 checklist, and the changelog class tag directly.
  `check-specs.sh` gained a `## Review findings` check: each bullet starts with **Must fix**/**Worth
  fixing**, and a bullet citing code carries an `INV §n` tag or the literal `(no invariant class)`.
  Both rules were already documented (workflow §7 and INVARIANTS.md); the check only makes omitting
  them fail a command the loop already runs, and it verifies the tag rather than the thinking. It
  runs on the full sweep and on any `--file` HANDOFF edit, so the post-edit hook catches it too.
  Applying it retro-tagged the open installer finding as INV §4 — `exec` replacing the process image
  so the EXIT trap never fires is a cleanup that misses an exit path. Not done, and still open for a
  decision: the workflow-text guards against inheriting a prior review's framing and against
  test-coverage findings standing in for defect findings.

- 2026-07-24 — Re-reviewed the same range after `61b234d` through `8b84e4f` at the human's request,
  this time sweeping the diff against every INVARIANTS class as `INV` requires of `/review`. The
  earlier pass checked only FS/TS requirement conformance and test coverage, so it missed four
  defects now recorded above: an unrolled-back partial delivery (INV §4, breaking FS-13.R5), two
  duplicated-helper drifts (INV §2), and an unbounded per-agent tray with no lifecycle cleanup
  (INV §1). Clean on the remaining classes: §13 all 15 new annotation selectors resolve; §14 the new
  route inherits `localOnly`; §5 the self-target idle pre-check is advisory but `SendPrompt` claims
  `turnActive` atomically; §10 both call sites gate `sourceActive`/`annotationsEnabled` correctly
  (FS-13.R15); §11 an empty batch cannot marshal to `null`. Spec conformance findings from the first
  pass stand unchanged; no product code or specs were edited.

- 2026-07-24 — Reviewed the continuous range after `61b234d` through `8b84e4f`, validating
  the annotate-and-assign implementation. All specification requirements (FS-13.R1-R15,
  FS-06.R21, TS-02.R14-R15, TS-03.R14) match the code in both directions. Required tests pass:
  UI store persistence, message delivery and budget integrity, diff/event selection and clipping,
  archive indexing, endpoint validation and routing. Go test suite (both FTS5 and non-FTS5 variants),
  UI tests (105 tests), and full builds pass with no regressions. J13 real-browser usability journey
  documented but pending. Recorded no findings — superseded by the re-review above, which found four.

- 2026-07-24 — Implemented annotate-and-assign. Live and archived chat transcripts now support
  diff-line and event annotations in a browser-local pending tray, delivery to the current agent,
  another running chat agent, or a prefilled new-agent launch, durable annotation cards, reserved
  dashboard-user mail, and archive indexing. The new regression coverage verifies validation,
  mail-size clipping, no-budget reserved mail, durable delivery, and annotation search; spec, UI,
  both Go test variants, source build, and distribution build pass. J13 is the remaining real-browser
  usability journey for this new surface.

- 2026-07-22 — Reviewed the continuous range after `61b234d` through `ef4ee18` (the current-history
  span since the last review; `61b234d` is the rehashed equivalent of the old `4195ed0` marker). The
  shipped product code — the permission busy-before-release race fix and cancelled-decision emission,
  the transcript-replay assistant-delta folding, the onboarding wizard latch, and the release-archive
  symlink dereference — matches its requirements in both directions (FS-03.R4/R9/A4/A5/A6, FS-04.R23,
  INV §9). The design/spec-only commits (annotate-and-assign, onboarding-credentials) carry consistent
  `(planned)` tags and ship no code. One Worth-fixing finding recorded: the piped installer leaks its
  temporary bootstrap file because `exec` discards the cleanup trap. Spec check, Go build, and the
  touched runtime/release/cli package tests pass.

- 2026-07-22 — The human accepted the current local-API trust and child-environment inheritance
  boundaries for now, and moved those plus the terminal-capability boundary to the known-issues
  backlog. Codex remains supported for chat; its terminal interface is intentionally rejected until
  a Codex-specific interactive-CLI hook/flag path is verified.

- 2026-07-22 — Defined the waiting onboarding-credentials change with the human. It adds an
  explicit Set up later completion path; removes onboarding model fields; gives Claude/Codex
  provider-owned sign-in guidance plus Check again; treats Codex native login or API key as ready;
  updates fresh-only defaults to `sonnet`/`gpt-5.6-sol`; preserves existing `backends.json`; and
  pins a private Codex CLI readiness probe. There is no embedded terminal, dashboard-started login,
  credential transport, or new auth API. The observed `agentdeck auth` failure is an installed
  v0.1.0 binary predating that command, not absent current source. FS-04, FS-09, TS-03, TS-04, and
  TS-06 are planned/Partial; `repair-onboarding-credentials.md` waits to start. Spec, twin-skill,
  and whitespace checks pass; no product code changed.

- 2026-07-21 — Published the verified piped-installer fix in GitHub Release v0.1.1. The tag's
  Apple-silicon release workflow completed successfully: it assembled the private runtime, passed
  the release transaction/bootstrap and fresh-install checks, and uploaded the archive,
  `manifest.json`, and corrected `install.sh`. The documented `releases/latest/download/install.sh`
  endpoint now serves the fixed bootstrap. The two pending `main` commits (the installer fix and
  the previously committed annotate-and-assign specification work) are pushed to `origin/main`.

- 2026-07-21 — Fixed the documented `curl | bash` release installer path. Its lock re-exec had
  treated `bash` as the script pathname, causing the lock-holding child to resume midway through
  the pipe with helpers such as `die` and `on_path` undefined. A piped invocation now first writes
  an owner-only executable temporary bootstrap, then safely re-execs that complete file under the
  lock. The new fake-release regression exercises the exact pipe → lock → install sequence;
  specification checks, the full Go test suite, source build, and distribution build pass. The
  v0.1.0 release asset remains unchanged until a new release is published.

- 2026-07-20 — Defined the annotate-and-assign feature with the human: new planned FS-13
  (diff-line and transcript-event selection in live and archived transcripts, a per-browser pending
  tray, batch send to the current agent, another chat agent, or a new prefilled launch, a durable
  structured `annotation` transcript event, archive search), planned FS-06 reserved user-sender
  mail (no turn-budget consumption, unforgeable), planned TS-02 annotation-event and user-mail
  persistence, and the planned TS-03 annotations batch endpoint. The human confirmed all four scope
  decisions (surfaces, batch tray, new-task-as-prefilled-launch, mail delivery) in conversation.
  The ready change `annotate-and-assign.md` is waiting to start; no product code changed.
  Specification, twin-skill, and whitespace checks pass.

- 2026-07-19 — Re-ran every runnable non-credentialed journey J1–J12 against the release-style
  build with isolated homes. The cancelled-permission prompt now resolves live and after reload;
  approve, deny, the real timeout, double-fire rejection, grid/restart, both archive search builds,
  Settings, messaging, recovery, and durability passed with no new finding. J6 and credentialed
  provider branches remain human-gated. The in-app browser cannot execute native prompt/confirm
  dialogs, so affected J5/J7/J9 UI actions are recorded as blocked while their backing operations
  and rendered results passed. Full report:
  [`../archive/reviews/usability-review-run-2026-07-19-post-fix.md`](../archive/reviews/usability-review-run-2026-07-19-post-fix.md).

- 2026-07-19 — Fixed the worth-fixing J4 finding: cancelling a turn with a pending permission now
  emits and persists a `permission_resolved` (decision `cancelled`), matching the deny and timeout
  paths. The withheld prompt renders a resolved chip on the live view and after reload instead of
  leaving Approve/Deny clickable forever (which returned `409 permission already resolved`). FS-03.R9
  and A5 pin the behavior; `TestCancelDuringPendingPermission` now asserts the emitted event.
  Specification checks, both Go test variants (incl. focused `-race` on the permission path), and the
  source build pass. No open review findings remain.

- 2026-07-19 — Drove the full non-credentialed usability matrix J1–J12 (J6 and the credentialed J2
  branch skipped as gated) with Playwright against the real binary, then re-verified J3/J4 on a
  rebuild at `c64d7bf`. Browser-level confirmation that the permission-deny race fix holds (3/3
  deny turns return to idle) and that reloaded transcripts coalesce streamed deltas like live chat.
  One new Worth-fixing finding recorded above (cancel-during-pending leaves a stale actionable
  permission prompt). All other journeys passed, including grid/layout persistence, resume/switch
  identity, both archive-search builds, settings round-trips, MCP messaging/nudge/unread, failure
  recovery, and restart durability. Full report:
  [`../archive/reviews/usability-review-run-2026-07-19.md`](../archive/reviews/usability-review-run-2026-07-19.md).

- 2026-07-18 — Fixed the permission-denial completion race: the runtime now records the temporary
  resolved/busy state before responding to ACP, so a fast peer can only write the final idle status
  through normal `turn_end` completion. The same ordering protects timeout resolution. A two-agent
  HTTP/SSE fake-ACP regression asserts idle after each denied `turn_end`; specification checks, both
  Go test variants, source build, and distribution build pass.

- 2026-07-18 — Re-ran the complete non-credentialed usability matrix after the onboarding and
  transcript-replay fixes. Both fixes now pass in the real built app: polling no longer ejects the
  wizard, and Archive/resume folds streamed replies exactly like live chat. Grid reorder/restart,
  tagged and fallback Archive search, Settings round-trips, two-agent MCP messaging and unread
  durability, fake live xterm input/resize/reattach, reconnect, crash recovery, and the presentation
  matrix passed. Found one new must-fix defect: a fast permission denial can race `turn_end` and
  overwrite idle back to busy, leaving the composer stuck on Cancel. Full report:
  [`../archive/reviews/usability-review-run-2026-07-18-post-fix.md`](../archive/reviews/usability-review-run-2026-07-18-post-fix.md).

- 2026-07-18 — Fixed both must-fix usability findings. An opened onboarding wizard is now latched
  until successful Launch completion, so the 10-second config refresh cannot replace Project or
  Config with the Dashboard after backend validation. Full transcript replay now uses the same
  consecutive-assistant folding helper as live Server-Sent Events, so Archive and resume keep one
  streamed reply in one message. FS-03, FS-04, and FS-05 now pin the behavior; focused gate, store,
  and Archive regressions pass along with the specification checks, 104 UI tests, both Go test
  variants, source/UI builds, and the distribution build.

- 2026-07-18 — Re-ran the behavior-driven usability review after an interrupted run left no durable
  checkpoint. The tagged production binary, untagged archive fallback, isolated fake-ACP homes, and
  development visual matrix covered first paint, onboarding, launch/chat, permission approve/deny,
  layout/restart, Archive/search/resume, dense Settings, disconnect/reconnect, and agent crash.
  Found two must-fix defects: config polling ejects a fresh user from the four-step wizard after
  Backend succeeds, and Archive/resume renders each stored assistant stream delta as a separate
  message. The redesigned presentation otherwise remained coherent and the presentation checks,
  101 UI tests, production UI build, spec check, and tagged/untagged Go builds passed. J6 live
  terminal and J10 messaging remain unexercised. Full report:
  [`../archive/reviews/usability-review-run-2026-07-18.md`](../archive/reviews/usability-review-run-2026-07-18.md).

- 2026-07-18 — Reviewed the continuous range after `87d6251` through `4195ed0`: the Codex
  role-prompt delivery fix, the installer/usability fixes, and the full core-interface redesign.
  The redesign is behavior-preserving presentation only — screens, data, routes, and actions are
  unchanged, the development-only visual matrix stays out of the production bundle, and third-party
  renderers read the shared semantic values. The two fixes match their specifications (Codex config
  overlay, corrupt-backend fallback, persisted/searchable user prompts, installer flag
  preservation). FS-12/TS-08 and the touched FS/TS agree with the code in both directions.
  Specification, presentation, UI (101 tests), and Go checks pass. No findings.

- 2026-07-18 — Shipped the product-native core interface across the shell, Dashboard, agent screen,
  Archive, Settings, onboarding, overlays, and third-party renderers. Layered semantic CSS, local
  fonts and mark, shared presentation primitives, stable future-skin hooks, a development-only
  visual matrix, Stylelint, and the TSX/CSS contract checker now form one maintained presentation
  authority. Real-browser review covered baseline/high-variance fixtures and the embedded release;
  it found and fixed hidden Settings panels consuming layout space. Specification, UI, both Go
  variants, source build, and distribution checks pass.

- 2026-07-18 — Finished the core frontend feature design after the human selected layered plain CSS.
  TS-08 now pins the cascade, exact core values/fonts/assets, stable manifest-backed skin hooks,
  third-party renderer adapters, migration sequencing, and unattended-maintenance safeguards:
  Stylelint, cross-TSX/CSS contract checks, stale-exception rejection, pretest/prebuild enforcement,
  deterministic visual fixtures, and local frontend agent instructions. The source idea moved to the
  ready change `redesign-core-interface.md`; specification and whitespace checks pass.

- 2026-07-18 — The human confirmed the product-native, presentation-only FS-12 behavior. Audited
  the current styling/build/component seams and added draft TS-08 with common constraints for local
  assets, third-party renderer theming, data-driven inline styles, shared presentation primitives,
  stable future-skin hooks, and visual/style-contract verification. Technical completion is paused
  on the A/B/C presentation-contract choice; specification checks pass.

- 2026-07-18 — Revised the planned frontend behavior after human feedback. FS-12 now makes the
  default presentation AgentDeck's product-native core, removes all Field Atlas metaphors, and
  excludes responsive/zoom/keyboard/accessibility expansion, new recovery states, and dedicated
  browser-dialog replacements. It retains a distinctive editorial/technical visual direction and a
  future-skin compatibility boundary. Specification checks pass; technical design still waits for
  confirmation of this revision.

- 2026-07-17 — Started the requested frontend redesign definition. Added planned FS-12 for a
  cross-product Field Atlas interface covering the shell, dashboard, agent workspace, archive,
  settings, onboarding, overlays, accessibility, responsive behavior, and the future-skin product
  boundary. Technical architecture and ready-change creation wait for human confirmation of the
  visual direction, desktop floor, and dedicated-dialog scope. Specification checks pass.

- 2026-07-17 — Audited every entry under `Known things to improve` against the current
  specifications, implementation, and focused tests. Removed fixed Codex-role, user-prompt, and
  installer claims; removed vague or unreachable subclaims; and narrowed partially fixed entries to
  their evidenced remainder. The installer lock re-exec preserves no-start/non-interactive flags and
  no longer blocks release; live-provider acceptance remains gated.

- 2026-07-16 — Codex chat now receives the frozen composed project/role prompt through the
  official `codex-acp` `CODEX_CONFIG.developer_instructions` overlay on launch and resume; invalid
  overlays fail before spawn, unrelated config remains intact, and Codex no longer receives the
  unsupported generic ACP `systemPrompt`. Runtime regression tests plus `make check-specs`,
  `make test`, and `make build` pass. A real authenticated Codex role-adherence new-turn/resume
  check remains an explicit acceptance gate.

- 2026-07-16 — Fixed all recorded installer and usability findings: the locked bootstrap preserves
  no-start/non-interactive choices under a pseudo-terminal test; incomplete hand-edited backend
  catalogs fall back safely with the filename in diagnostics and the UI guards null collections;
  accepted user prompts are sequenced, persisted, replayed, and indexed; onboarding names useful
  credential recovery steps; and the config-source panel has its missing styles. Specifications,
  focused Go/UI tests, `make test`, `make build`, and `make dist` pass.

- 2026-07-16 — Usability review drove J1–J3, J5, J8 (tagged + untagged), and J9 (incl. FS-11) against
  the real binary in a browser. Four findings recorded: a hand-edited incomplete `backends.json`
  crashes the whole dashboard (new, Must fix); user prompts are never persisted to the transcript so
  archives are one-sided and user text is unsearchable (Must fix, extends a known advisory); credential
  failures show raw codes with a misleading hint (Worth fixing); the config-source panel is unstyled
  (Worth fixing). J1/J3/J5/J8/J9 core paths and the full onboarding walk passed with zero console
  errors; FS-11's read-only resource_dir surfaces correctly. J4/J6/J7/J10/J11 were not exercised. Full
  report: [`../archive/reviews/usability-review-run-2026-07-16.md`](../archive/reviews/usability-review-run-2026-07-16.md).
- 2026-07-16 — Review found no unreviewed product code after the recorded project-resources review boundary. The installer flag-preservation finding remains the only open review finding.
- 2026-07-16 — Review through `87d6251` found the project shared-resources work sound: launch,
  resume, and switch inject the owner-only resource directory through one shared helper; project
  responses expose only the path and never the contents; and the specifications match the code in
  both directions. No new findings. The open installer flag-preservation regression still stands.
  Spec checks and the targeted config/server tests pass.
- 2026-07-15 — Shipped project shared resources (FS-11 Current): every project gets an
  AgentDeck-owned owner-only `project-resources/{id}/` directory outside its repository, created on
  project creation and lazily before launch, injected into launch/resume/switch as
  `AGENTDECK_PROJECT_RESOURCES` + an add_dir + a composed instruction, exposed as a read-only
  `resource_dir` in project responses and Settings, and retained on project deletion. FS-11, TS-02,
  TS-03, and TS-05 flip to Current. `make check-specs`, `make test`, `make build`, `ui` test/build,
  and `make dist` pass.
- 2026-07-15 — Review through `ccd0a51` found that the release-installer lock re-exec loses
  explicit no-start/non-interactive flags. Specification, test, build, and distribution checks pass,
  but the existing non-terminal bootstrap test does not cover the interactive trigger.
- 2026-07-15 — Renamed the explicit review command to `/review` in the Codex and Claude skill
  copies; it retains the same unreviewed-range review behavior.
- 2026-07-15 — Renamed the explicit build/finding-fix commands to `/work` and `/fix`. `/work`
  now finds the sole waiting ready change (or asks the user to choose when several wait), so an
  explicit request no longer reports no work while implementable work is available.
- 2026-07-15 — Defined the waiting project shared-resources change: every project will receive an
  AgentDeck-owned owner-only folder outside its repository, injected consistently into agent
  launches and retained after project deletion. It is ready to start and is not active work.
- 2026-07-15 — Fixed the release-path review findings (INV §9): bootstrap and updater lock claims
  now cover resolution/download through activation, the stable shim is fsynced then atomically
  renamed, and the arm64 macOS release workflow runs release/CLI coverage plus a bootstrap journey.
  `make check-specs`, `make test`, `make build`, and `make dist` passed.
- 2026-07-15 — Review through `d260f93` recorded three must-fix macOS release defects: full-operation
  installer/update contention is not serialized, the stable shim is written in place, and release CI
  omits required delivery checks. Shared specification, Go (both variants), build, and distribution
  checks passed.
- 2026-07-15 — Shipped the Apple-silicon macOS GitHub Releases installer: verified private Node and
  Claude/Codex ACP runtime, guided sign-in, stable shim, explicit update/rollback, no-start mode,
  release assembly/publish workflow, and release documentation. Automated checks are green; real
  provider sign-in remains credential-gated.
- 2026-07-15 — Claude chat and credential checks now target the pinned official
  `@agentclientprotocol/claude-agent-acp` package; source installs enforce its Node 22 floor.
- 2026-07-15 — Defined the waiting macOS arm64 GitHub Releases installer change: a private Node and
  Claude/Codex ACP runtime, optional guided sign-in, explicit update/rollback, checksums, and no
  signing/notarization. It is ready to start and does not make the release installer active yet.
- 2026-07-15 — Added a collaborative feature-design workflow that turns one idea into confirmed
  planned specifications and a ready change without starting implementation.
- 2026-07-14 — Codex backends can opt into `autosync_models`: on startup AgentDeck add-only merges
  the Codex CLI's `models_cache.json` into the catalog (FS-09.R28/A8). Claude autosync stays an idea.
- 2026-07-15 — Confirmed detached federation import remains deliberately unshipped: `detach=true`
  returns `501 not_implemented`; source assets remain reference-only until a verified provider launch-
  injection design exists. It is a known capability gap, not a human decision awaiting resolution.
- 2026-07-14 — New Agent modal now defaults the name to just the (capitalized) role instead of
  `Role-project` (FS-01.R1 auto-suggest; format not pinned).
- 2026-07-14 — Project ids are now server-derived from the title (`slug(title)-<timestamp>`); the
  Settings and onboarding project forms no longer ask for a slug (FS-04.R31/A11).
- 2026-07-14 — Replaced letter-number future-work labels with plain-language ideas, known
  improvements, ready changes, and current-change records.
- 2026-07-14 — Limited Claude and Codex workflow skills to their explicit slash-command triggers.
- 2026-07-14 — Added archive notices explaining that old process labels are historical and must
  not be followed; older live briefs now carry the same context.
- 2026-07-14 — Removed repeated user-intent classification from agent instructions; only the
  no-self-prioritization rule remains.
- 2026-07-14 — Simplified agent instructions: removed specialist process labels while keeping
  stable requirement IDs and plain-language human updates.
- 2026-07-13 — SDD foundation complete: authoritative FS/TS/INV contracts, lifecycle, archive
  manifest, requirement-link lint, local hook, CI, role workflows, and verification landed.
- 2026-07-14 — Changes waiting to start moved out of the handoff; the handoff now records only the
  change in progress.
- 2026-07-12 — Federation bindings hydrate on restart so watch/sweep detects external edits.
- 2026-07-12 — Restart-orphaned runtimes are reaped by Stop/Switch/Release.
- 2026-07-12 — Onboarding completion write failures remain visible and retryable.
- 2026-07-12 — Canonical Phase 0–7 usability review recorded; no remaining usability BLOCKER.
- 2026-07-12 — End-to-end code review through `4036e78` recorded two restart blockers, since fixed.
- 2026-07-12 — Untagged Archive search falls back to metadata `LIKE` when FTS5 is unavailable.
- 2026-07-11 — Configuration federation 7.5–7.7 shipped with resolver, manager, API, launch, and UI.
