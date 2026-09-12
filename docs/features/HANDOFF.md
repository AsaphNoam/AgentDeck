# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-12`](../archive/state/HANDOFF-through-2026-09-12.md),
[`-11`](../archive/state/HANDOFF-through-2026-09-11.md),
[`-10`](../archive/state/HANDOFF-through-2026-09-10.md),
[`-09`](../archive/state/HANDOFF-through-2026-09-09.md),
[`-07`](../archive/state/HANDOFF-through-2026-09-07.md),
[`-06`](../archive/state/HANDOFF-through-2026-09-06.md) and
[`-03`](../archive/state/HANDOFF-through-2026-09-03.md) files, plus
[`HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None; the next role picks from the queues below.
- **Release:** `v0.4.3` is tagged and published; **Release state** and the release record carry its
  contents. `v0.4.2` and earlier are in the state archive.
- **Review units:** `file-read-nonregular-kind` is reviewed and stays open on four findings;
  `/review` has no available unit. Every other unit through this release is closed.
  `stop-telling-agents-to-poll` shipped without entering this queue on the operator's explicit
  2026-09-10 instruction; it can be added later.
- **Work units:** `persistent-pipeline-orchestration.md` is Waiting to start, including the completed
  deferred/inline mail technical contract. Its exact
  requirements and acceptance gates are in `docs/ready-changes/persistent-pipeline-orchestration.md`.
  `rename-product-to-deckhand.md` is Waiting to start: the AgentDeck → Deckhand rename with its
  one-time state migration, role rename to FirstMate, and two named read-compatibility paths.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport
  blocker; the ACP wait-list in `docs/ideas.md` holds the rest behind an adapter contract.
  Queue hygiene: `bump-pinned-acp-adapters.md` reads `State: Finished` but is still in
  `docs/ready-changes/` and absent from that directory's index; per its README a finished change's
  file is removed. Left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume; `New ideas` entries are available.
  Persistent pipeline orchestration and its mail extension are fully specified and ready to implement.
  TS-01.R31–R33, TS-02.R34 and TS-04.R53 complete shared prompt preparation, bounded batches,
  transactional budget/read settlement, uncertain-delivery recovery and deferred retention.
  The clean legacy reset, descendant cancellation, project boundaries, subordinate stage
  coordination and ordinary stop/resume remain confirmed.
  Streaming agent thinking stays part-decided (live-only decided; rendering default and whether
  `plan` ships still open). The permanently unaddressable pipeline agent is the newest `New ideas`
  entry and needs `/design-feature` before code. The Deckhand rename is fully specified and promoted
  to the work queue; no design decision remains open for it.
- **Open findings:** The separate injected-steer lifetime edge case remains outside the closed
  host-owned fallback unit. Also open: live-gate finding durability, provider-contract oracles, and
  the unverified OpenCode/OpenHands paths. See **Review findings**.
- **Bug reports:** BR-1, BR-2, and BR-3 are investigated and archived with this release; BR-3 is
  fixed and closed. BR-1's Codex model/effort defect is fixed and reviewed; BR-2 is fixed and closed.
  BR-1's still-open findings are listed above. Pinned Claude model delivery
  through `_meta` works; an ACP model `currentValue` can be stale and is no execution-model oracle.
- **State:** The file viewer's credentialed rendered forms are owed: journey J3 now carries the
  file-link steps (docked and transcript-width forms, the refusal branch, the dashboard-pane
  navigation), and none of them has been exercised against a real browser.
  Automated MCP contract verification is green. The full post-fix Claude/Codex acceptance
  matrix remains open and must not be called verified; the 2026-09-08/09 probes were limited contract
  and model-delivery checks, not that matrix. Real Claude and Codex steering is unexercised.
- **Branch:** `main`.

## Active change

**Change:** None.

**Changelog — 2026-09-12 (design-feature):** Completed the mail technical contract against existing
runtime and state seams: optional wake intent, shared bounded inline preparation, provider-result
confirmation, stable-id recovery without extra wakes, turn-budget reservation and deferred retention.
Added TS-01.R31–R33, TS-02.R34 and TS-04.R53; completed readiness references and moved the pipeline
unit to Waiting to start. Removed its completed source idea. Spec checks, twin-skill and diff checks
pass. No product code changed; implementation and provider acceptance remain future work.

**Changelog — 2026-09-12 (design-feature):** Recorded confirmed mail decisions: unread deferred
mail survives until delivery/read then uses 24-hour cleanup; uncertain delivery remains recoverable
with stable ids on a later authorized turn; inline batches contain bounded whole messages with
durable overflow. Intervention FYIs are best effort, with no atomic change/mail requirement.
Replacement context is supplied by the standing owner through assignment or ordinary mail; no
automatic inbox/history transfer. Updated FS/TS and acceptance criteria consistently. No product
question remains; the unit stays paused only for technical delivery mechanics. Spec checks, twin
skills and diff checks pass; no product code changed.

**Changelog — 2026-09-12 (design-feature):** Drafted waking/deferred durable mail and bounded
inline delivery in FS-06.R30–R36/A20–A25, FS-00.R17 and FS-14.R78/A45. Withdrew the separate
coordinator-update queue and delivery watermark in TS-09.R50 / TS-10.R36; intervention awareness
uses ordinary deferred mail. The expanded pipeline unit is paused for unread deferred-mail
retention, feature confirmation and the matching technical contract. Spec checks, twin skills and
diff checks pass; no product code changed.

**Changelog — 2026-09-12 (review):** Reviewed `file-read-nonregular-kind` (`22d77dc`), which
classifies a target through `os.Root` before opening it. Containment holds: `Root.Stat` refuses an
escaping symlink and the post-open descriptor check still guards replacement. Four findings — no
test fails without the fix and its one non-regular case silently skips on macOS; TS-05.R21 still
names `os.OpenInRoot`; an unreadable in-root file reports as outside the workspace; `filesearch.go`
keeps the superseded containment spelling. **Fix model:** medium — Codex Terra or Claude Opus.
Classes 2, 7, 8, 10, 14, 16, 17 apply; 1, 3–6, 9, 11–13, 15 have no surface. Archived three settled
entries for header budget; the slice is still over it.

**Changelog — 2026-09-12 (design-feature):** Revised persistent orchestration so the standing
agent always owns and reports the stage; configured dedicated coordinators are managed children and
the normal coordination contact, with durable awareness of material direct intervention. Cleanup
now automatically reconciles transient failures with persisted backoff and attention only for
persistent/unsafe conditions. Confirmed same-identity stop/resume and assignment re-delivery. Updated
FS-14.R61–R77/A35–A44, FS-16.R30–R38/A20–A24, TS-09.R35–R50, TS-10.R25–R37 and TS-05.R22;
R60 is superseded by R75. Promoted the source idea to the waiting ready change. Spec checks, twin
skills and diff checks pass; no product code changed and no implementation is active.

**Release state:** `v0.4.3` is published and verified on tag `8ad5261`. Release and CI runs passed,
the local distributable reports `0.4.3` with `sqlite_fts5`, and the GitHub Release carries the
darwin/arm64 archive, `install.sh`, and a manifest declaring `0.4.3` with its SHA-256.
The release shipped with five open Must-fix findings on the operator's explicit decision; all five
are now closed. The credentialed Claude and Codex journeys under
**Acceptance gates** are owed; real steering has never been exercised against a provider.

**Available by role:** `/review` has none; `/work` may take
`persistent-pipeline-orchestration` or `rename-product-to-deckhand`; `/fix` may take
`file-read-nonregular-kind` or BR-1; `/design-feature` may choose an available or
resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Acceptance gates

**Not blocking as of 2026-09-05.** These gates have not been run, and no agent may describe them as
verified, passed, or closed. The operator chose to let roles proceed with them open.

- [ ] Pinned real-provider stage-result/file-edit approval journey (FS-03.A26/J14).
- [ ] Post-fix credentialed Claude and Codex chat, MCP, resume, task, effective-model/effort, and
      reported-result checks. The historical 2026-07-26 run failed model precedence and is evidence,
      not closure for the current implementation.
- [ ] Pinned Claude terminal flags/hooks and live xterm journeys.
- [ ] Pinned OpenCode/OpenHands launch and credential checks.
- [ ] Real macOS native folder-panel checks (FS-04.A22/J2/J9/J16).
- [ ] Real-browser permission-pane and drag-refusal journeys (FS-02.A35/A43).
- [ ] Phase 7 federation matrix against real Claude and Codex installations.
- [ ] Real-browser worktree creation/launch and archive-with-uncommitted-work journeys (FS-19).
- [ ] Six-tab same-origin dashboard check against a `make dist` build (FS-02.A27).

## Blocked on human

- None.

## Review findings

### `file-read-nonregular-kind` — **Fix model:** medium — Codex Terra or Claude Opus.

- **Must fix** — the change ships with no test that fails without it, and its only non-regular case
  silently skips on macOS (**confirmed**, `INV §17`). **Where:** `22d77dc` adds no test;
  `internal/server/fileread_test.go`'s `TestFileReadRefusesNonFileAndMissing` builds its non-regular
  fixture with `net.Listen("unix", filepath.Join(root, "sock"))` under `t.TempDir()`.
  **Normal-use trigger:** running the suite on macOS. The `t.TempDir()` path exceeds the 104-byte
  `sun_path` limit, so the listen fails with `bind: invalid argument`, the case `t.Logf`s and
  returns, and the test reports PASS. Verified by running that test at `100d1cd` (pre-fix) and at
  `a632f3b` in clean worktrees: both pass, both log the skip. **Why it matters:** TS-05.R21's R11
  list requires an adversarial test per refusal class, and the non-regular class has no effective
  coverage on the development platform — the fix and the defect are indistinguishable locally. The
  FIFO hang the commit message names as the severe symptom has no test on any platform; a
  regression that restores open-then-classify would hang the suite rather than fail it.
  **Requirement:** TS-05.R21, FS-03.A37, `INV §17`. **Suggested fix/test:** build the socket under a
  short root (`os.MkdirTemp("/tmp", …)`) and fail rather than return when the platform does support
  it; add a `syscall.Mkfifo` case behind a bounded timeout. Both were confirmed to work here —
  `Root.Stat` reports a FIFO as `p---------` and non-regular, while `Root.Open` on it blocked
  indefinitely.
- **Must fix** — TS-05.R21 describes a mechanism the code no longer uses (**confirmed**,
  `INV §10`). **Where:** `docs/specs/tech/TS-05-security.md` R21 states the candidate "is then
  opened with `os.OpenInRoot`, so pathname resolution and opening are one root-confined operation"
  and that "File type, size, modification time, and content are all read from that returned
  descriptor rather than resolving the pathname again". `internal/server/fileread.go:204–227` now
  uses `os.OpenRoot` plus `Root.Stat(name)` plus `Root.Open(name)`: two resolutions, and file type
  is decided primarily by the pre-open `Root.Stat`, which is exactly "resolving the pathname
  again". **Why it matters:** the commit cites TS-05.R21 and FS-03.A37 and changed neither, so the
  security spec's containment argument now contradicts the shipped code; a later reader restoring
  "one root-confined operation" literally would reintroduce the platform-dependent verdict and the
  FIFO hang. FS-03.A37 also enumerates the refusal cases without naming the non-regular kind this
  change exists to make deterministic, while the test cites A37 for precisely that.
  **Requirement:** TS-05.R21, FS-03.A37, workflow §2.1, `INV §10`. **Suggested fix/test:** restate
  R21 as classify-through-the-root-then-open, with the post-open descriptor check as the
  replacement guard, and add the non-regular kind to A37.
- **Worth fixing** — an unreadable file inside the workspace is reported as outside it
  (**confirmed**, `INV §8`). **Where:** `internal/server/fileread.go:222–227` maps every `Root.Open`
  error that is not `os.IsNotExist` to `CodePathRefused` / "that path is outside this agent's
  working directory". **Normal-use trigger:** a root-owned or mode-`0000` file another process left
  in the working directory. Verified directly: such a file passes `Root.Stat` as regular, then
  `Root.Open` returns `openat …: permission denied` with `os.IsNotExist` false, so the viewer tells
  the person the file is outside the agent's working directory when it is plainly inside it.
  **Why it matters:** `INV §8` requires in-vocabulary data on user-facing surfaces, and this
  refusal misdirects the person to a containment problem that does not exist. The mapping entered
  in `100d1cd`, but this change keeps it as the fall-through for every stat failure.
  **Requirement:** FS-03.A37, `INV §8`. **Suggested fix/test:** branch on
  `errors.Is(err, fs.ErrPermission)` to an unreadable-file refusal and reserve path_refused for the
  root escape, which `os.Root` reports distinguishably as "path escapes from parent".
- **Worth fixing** — two containment spellings now coexist with no stated authority
  (**confirmed**, `INV §2`). **Where:** `internal/server/filesearch.go:171` `withinRoot` still uses
  `filepath.EvalSymlinks` resolve-and-recheck, the spelling the read abandoned in `100d1cd`; TS-05.R21
  previously stated the two shared one spelling under `INV §2` and that sentence was removed without
  saying which is now authoritative. **Normal-use trigger:** the composer offers a path its
  containment accepts that the read's containment then refuses. **Why it matters:** the consequence
  is bounded — `rankFiles` filters path names and never returns bytes, so the TOCTOU that motivated
  the read's change does not leak content through search — but `INV §2` exists to stop exactly this
  divergence in what "inside the root" means. **Requirement:** TS-05.R21, TS-03.R24, `INV §2`.
  **Suggested fix/test:** state in TS-05.R21 that `os.Root` is the authoritative containment for
  content reads while `withinRoot` remains a listing filter, or move the search onto `Root.Stat`
  too.

### BR-1 — **Fix model:** difficult — Codex Sol.

- **Must fix** — BR-1 finding state was not durable across concurrent roles (**confirmed**).
  **Where:** `docs/archive/reviews/live-provider-acceptance-2026-07-26.md` recorded the exact Codex
  model failure as a Must-fix on July 26, but the live acceptance session could not edit HANDOFF
  while another session owned the shared docs. The report and a contradictory effort design were
  then swept into `7d294fb` without the finding entering HANDOFF. **Why it matters:** the mandatory
  read order made the archived report invisible to every later role, so an already-detected critical
  defect was treated as an open gate for another six weeks. **Requirement:** workflow §§1.1/12,
  `INV §1`, `INV §10`. **Suggested fix/test:** a failed live gate must be recorded in HANDOFF before
  its role can close; if state-file ownership blocks that write, leave the role explicitly blocked
  rather than archiving the only finding. A commit/review that contains an acceptance report must
  reconcile every Must-fix in it with live state.
- **Must fix** — provider-contract claims can still be proved by a self-authored oracle
  (**confirmed**). **Where:** TS-04.R18 asserted a `model[effort]` request shape after inspecting
  `codex-acp`'s internal `ModelId` parser without tracing `session/new` to `threadStart`; FS-09.A15
  and `fakeacp` then checked only that AgentDeck emitted the asserted field. The fake accepts unknown
  request members that the pinned ACP decoder drops. The same oracle error recurred in the 2026-09-08
  finding-fix: it called ACP `configOptions.model.currentValue` an independent effective-model oracle
  and declared Claude `_meta` delivery broken without sending a prompt. A real prompt then showed
  stale `currentValue = opus` alongside requested Haiku/Sonnet in SDK init, assistant, and usage
  signals. **Why it matters:** design, implementation, review, and an adapter-local readback can all
  agree and remain wrong about the external provider; following the false Claude finding would add a
  redundant delivery path and change launch failure/order behavior. **Requirement:** `INV §11`,
  `INV §12`, `INV §17`. **Suggested fix/test:** require provider-behavior statements to cite a
  complete reachability trace or a credentialed prompt receipt; label ACP `currentValue` as adapter
  configuration evidence, not provider execution evidence; and add an independently derived contract
  oracle that rejects out-of-schema standard fields instead of mirroring `sessionNewParams`.
- **Worth fixing** — equivalent OpenCode/OpenHands fields remain unverified (**undetermined**).
  **Where:** neither CLI is installed. OpenHands model delivery has a separate `LLM_MODEL` env path,
  so it does not depend on the suspect ACP `model` member, but both adapters still receive an
  out-of-schema top-level `systemPrompt`; OpenCode also still depends on the top-level `model`.
  **Why it matters:** the same silent-ignore class may be live on surfaces explicitly advertised by
  AgentDeck. **Requirement:** FS-09.A6, `INV §12`. **Suggested fix/test:** keep the claims gated until
  each pinned CLI is installed and its model/prompt delivery is checked at the effective provider;
  remove any redundant unsupported top-level fields once their real mechanism is known.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
