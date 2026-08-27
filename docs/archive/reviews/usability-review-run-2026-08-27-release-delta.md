# Usability review — changes since the previous release (v0.2.2 → v0.2.3) — 2026-08-27

## Scope and setup

- **Scope:** the 84 commits between `v0.2.2` and `v0.2.3`, restricted to what a person can see or do
  in a browser. The largest new surface is dependent work (the Tasks view, the dashboard attention
  count, the task-concurrency setting); also covered are browser-local chat drafts, the
  directory-browse control, and the projects-canvas context menu. FS-15 context links and FS-17
  agent tool results are agent-facing only (no UI diff) and are not browser-observable.
- **Browser rung:** 1 — Playwright driving the environment's cached Chromium, with console errors,
  page errors, and failed requests captured on every run.
- **Build:** `make build` (`sqlite_fts5`), the release binary at `bin/agentdeck`.
- **Fixtures:** every server ran with an isolated `AGENTDECK_HOME` under
  `.review/usability-20260826-release-delta/`, one copy and one port per journey; the deterministic
  `fakeacp` peer was exposed as `claude-agent-acp`/`codex-acp` through a PATH shim.
- **Evidence:** 58 screenshots under `.review/usability-20260826-release-delta/run/shots/`; the
  load-bearing ones are copied to
  [`usability-review-2026-08-27-release-delta-evidence/`](usability-review-2026-08-27-release-delta-evidence).

Three journey subagents were killed mid-run by an account spend limit; the journeys were re-run and
completed directly, from reset fixtures, so no result below is inherited from a partial run.

## Journey results

### JT-A — Dependent work / Tasks view (FS-16), fixture `jta`, port 4411

| Step | Result | Observation |
|---|---|---|
| 1 empty state | PASS | "No dependent work in this project yet.", styled grid, `0 need attention`, zero console errors. |
| 2 create through the UI | PASS | Row appears and reaches `running` as the fake agent starts. Findings 2 and 4 recorded here. |
| 3 arm on another task | PASS | Row shows `armed` with a "Waiting on:" line. Finding 3 recorded here. |
| 4 satisfy the prerequisite | PASS | Recording `success` moved the prerequisite to `finished` and the dependent `armed → running` within 1.2 s **with no reload**; identical after reload. |
| 5 cancel a running task | PASS | Row settled to `finished`; no stuck `running`. |
| 6 failure propagation | PASS | Live transition to `dependency_failed` carrying "a prerequisite can no longer be satisfied". |
| 7 retry a parked task | PASS (behavior) | Refused `422` naming re-arm, exactly as FS-16.A11 requires. Finding 5 records the UI offering the control. |
| 7b re-arm a parked task | PASS | Re-arming onto a satisfied prerequisite moved it to `running` — the parked state is demonstrably repairable in the real UI. |
| 8 delete | PASS | Removed and stayed removed across reload. |
| 9 restart durability | PASS | Armed stayed `armed`; the running task became `interrupted` with "the server restarted while this task was running"; the count read `1 need attention`; Retry then returned it to `running` (FS-16.A8). |
| 10 garbage input | PASS | Whitespace-only rejected with "display_name is required and bounded"; no junk task created. |

### JT-B — Dashboard attention, settings, projects canvas (FS-02, FS-04), fixture `jtb`, port 4412

| Step | Result | Observation |
|---|---|---|
| 1 zero state | PASS | On a scoped project dashboard: "0 tasks need attention", linking to `/tasks?project=…`. |
| 2 count increments | PASS | Parking a dependent moved it `0 → 1` **without a reload** (FS-02.A26). Finding 6 recorded here. |
| 3 click through | PASS | Navigates to the Tasks view already filtered to that project. |
| 4 count decrements | PASS | Resolving the attention-worthy task returned the count to zero. |
| 5 **FS-02.A24 right-click gate** | **PASS** | See below. |
| 6 task concurrency editor | PASS | Default 10 shown; `0` and `-3` disable Save with "Value must be greater than or equal to 1." and never persist; `3` saves and round-trips across reload; `default_project`, `default_role`, `onboarding_complete`, notifications, switch settings and the projects collection all survived the write — merged, not replaced (FS-04.A23). |
| 7 budget enforced | PASS | With the budget at 2 and five tasks queued, exactly 2 ran and 4 waited visibly as `ready`. |

**FS-02.A24 — the standing right-click gate PASSES in a real browser.** Viewport 1440×950; the
project grid occupied x∈[32,1408], y∈[475.5,918]. Right-clicking a card centre (163,697) opened the
card menu (Rename / Change color / Archive), *not* the create menu. Right-clicking the background at
(414,697) right of the cards, (163,935) below them, (25,925) in the shell's bottom-left padding
frame, (12,600) left padding, (1432,600) right edge, (700,462) above the grid, and (1430,940)
bottom-right corner each opened a **New project** menu, and choosing it opened a correctly styled
create modal. Evidence: `a24-padding-right-click.png`, `a24-create-modal.png`.

### JT-C — Chat drafts and directory browsing (FS-03, FS-04), fixture `jtc`, ports 4413/4415

| Step | Result | Observation |
|---|---|---|
| 1 draft survives navigation | PASS | Exact text restored after navigating to the dashboard and back. |
| 2 draft survives reload | PASS | Restored verbatim after a full page reload. |
| 3 no cross-contamination | PASS | Two agents kept their own distinct drafts; each composer restored only its own. |
| 4 accepted send clears it | PASS | Composer emptied and the text did **not** resurrect after navigation or reload — no double-send risk. |
| 5 stopped agent | PASS | No draft leaked into another composer; no console error. |
| 6 browse control, Settings | PASS | Two enabled **Browse…** controls (cwd and the pending `add_dirs` entry) in the project form, and two more in the New project modal. |
| 7 onboarding wizard | PASS | On a genuinely un-onboarded home the wizard renders fully styled (`dialog-content onboarding-wizard`, 24 px padding, real font, rounded buttons) with zero console errors — the historical unstyled-wizard escape does not recur. |
| 8 native folder panel | SKIPPED(native macOS panel is not driveable headlessly) | Not claimed as verified. See the gate note below. |
| 9 invalid working directory | **FAIL** | Finding 1. |

These three journeys collectively exercise FS-03.A19's "manual reload/navigation check", which is
the verification that item names and that no automated suite performs.

## Findings

Severity counts: **0 BLOCKER, 1 MAJOR, 4 MINOR, 1 POLISH.**

```
SEVERITY: MAJOR
WHERE: JT-C step 9 (fixture jtc, port 4413)
REPRO: Settings → Projects → Edit → set the working directory to /definitely/not/a/real/path/xyz → Update
EXPECTED: the save explains that the directory does not exist (FS-04.A22 surface; the server
          returns warnings:[{code:"cwd_not_found", message:"directory … does not exist yet"}] and
          ProjectForm.tsx:107 has the markup "⚠ … (save still succeeds)")
OBSERVED: dialog closes, no alert, no inline warning, nothing anywhere in the page; the bogus path
          is persisted. ProjectsEditor.tsx:69-72 calls setWarnings(resp.warnings) and setOpen(false)
          in the same success handler, unmounting the only component that renders the warning.
          Same on the create path (:78-81). Pre-existing, not a regression in this release.
EVIDENCE: cwd-warning-swallowed.png; curl PUT /api/projects/my-app returns the warning verbatim
```

```
SEVERITY: MINOR
WHERE: JT-A step 2 (fixture jta, port 4411)
REPRO: create a task with target "Launch a new agent"; let it reach running
EXPECTED: the row lets a person reach the agent doing the work (FS-16.A8)
OBSERVED: the meta row reads only "launches implementer" — never the assigned agent id, name, or a
          link, even while running and even though the API carries assigned_agent_id (a_6eceb6).
          TasksPage.tsx:136 shows the assignee only when target_kind === "agent", and then as a raw
          id. There is no route from a running task to its conversation.
EVIDENCE: tasks-row-no-assignee.png
```

```
SEVERITY: MINOR
WHERE: JT-A step 3 (fixture jta, port 4411)
REPRO: create task B armed on task A's success; read B's row while both are on screen
EXPECTED: the view says what B waits on in terms a person recognises (FS-16.R14/A8)
OBSERVED: "Waiting on: task tk_8caec11d865acf33 → success" — a raw durable id, while the
          prerequisite's display name "Run the build" is rendered two rows away. waitingOn()
          (TasksPage.tsx:39-47) formats arm.source_id directly. The product elsewhere forbids this:
          J5 requires each card show the project title, not its durable id.
EVIDENCE: waiting-on-durable-id.png
```

```
SEVERITY: MINOR
WHERE: JT-A step 2 (fixture jta, port 4411)
REPRO: open /tasks, fill only name and instruction, submit without touching the role select
EXPECTED: the form defaults to the configured default_role ("implementer"), as NewAgentModal does
OBSERVED: it preselects roleNames[0] — "agentdecker", the internal AgentDeck-expert role — because
          TasksPage.tsx:181 is `role || roleNames[0]` and ignores config.default_role.
          NewAgentModal.tsx:53-64 deliberately prefers the configured default. A person who does not
          notice the dropdown silently assigns work to the wrong kind of agent.
EVIDENCE: tasks-row-no-assignee.png shows the resulting "launches agentdecker"; GET /api/config
          reports default_role "implementer"
```

```
SEVERITY: MINOR
WHERE: JT-A step 7 (fixture jta, port 4411)
REPRO: park a task by failing its prerequisite, then press Retry on the parked row
EXPECTED: controls offered on a row can succeed for that row's state
OBSERVED: Retry is rendered for dependency_failed as well as interrupted, and for dependency_failed
          it is always refused: 422 "state: this task is parked by an unsatisfiable prerequisite;
          re-arm it instead". The server is right (FS-16.A11) and the message names the fix, but the
          UI offers a button that can never work while Re-arm — the control that does — sits in the
          same row without prominence. TasksPage.tsx renders Retry on both states.
EVIDENCE: parked-retry-refused.png
```

```
SEVERITY: POLISH
WHERE: JT-B step 2 (fixture jtb, port 4412)
REPRO: park exactly one task, open the scoped project dashboard
EXPECTED: "1 task needs attention"
OBSERVED: "1 task need attention" — CardGrid.tsx:35 pluralises the noun but not the verb:
          `${count} task${count === 1 ? "" : "s"} need attention`
EVIDENCE: attention-count-grammar.png
```

## Static sweeps

- **S1 serialization contract** — clean for the release delta. Every collection on the new tasks API
  is built from `make([]T, 0, …)` or `[]T{}` and re-read through `ReadTask` before it is returned, so
  no field can marshal to `null`; the zod schemas add `.nullable().optional().default([])` on top.
  The new config and directory-picker fields are scalars.
- **S2 CSS wiring** — clean, both directions. Every class referenced by the new
  `TasksPage`/`TaskConcurrencyEditor`/`BrowseDirectoryButton` components resolves, including the
  dynamically composed badge variants, and every selector added in `tasks.css` is referenced.
- **S3 external-CLI variance** — the delta adds one user-machine exec, the directory picker. It runs
  the fixed `/usr/bin/osascript`, never through PATH or a shell, matches only the stable numeric
  cancel code `-128` rather than localized text, and returns one opaque failure by design
  (TS-05.R15). No optional-flag fallback risk of the historical `--no-color` kind.
- **S4 null hostility** — clean; every server-origin collection in the changed UI is guarded.
- **S5 error surfacing** — clean; every mutation in the changed UI has a catch that reaches
  user-visible state. Note this sweep passed while finding 1 still slipped through: the mutation's
  *error* path is wired, but its *warning* path is rendered into a component the same handler
  unmounts, which no grep-shaped sweep detects.

## Journey-matrix coverage (§7)

Comparing the matrix against the feature-spec index before running showed that the three specs that
shipped in this release cite **no journey at all**: FS-16 (dependent work), FS-15 (context links) and
FS-17 (agent tool results). FS-16.A8 — the Tasks view — names "UI tests plus restart and deletion
integration tests" and nothing browser-driven, and FS-02.A26's attention count names dashboard UI
tests only. Every other `J<n>` cited across FS-02/03/04/06 resolves to a real matrix row.

The three journeys run here (JT-A, JT-B, JT-C) are added to the §3 matrix as J15–J17 so the matrix
tracks the product, as §7 requires.

## Acceptance gates

- **FS-02.A24 — closed.** Verified in a real browser at eight distinct background points including
  the shell's padding frame, plus the card-menu contrast and the create modal. Details above.
- **FS-04.A22 — still open, narrowed.** A real browser confirms the **Browse…** controls are present
  and enabled for `cwd` and the pending `add_dirs` entry in both the Settings project form and the
  New project modal, and that the onboarding wizard renders styled. What remains unverified is the
  part the gate actually names: that the native macOS folder panel opens *in front*, selects, and
  cancels. That panel is server-side `osascript` and blocks on real pointer input, so it cannot be
  driven headlessly; it needs a human at the machine.

## Notes that are not findings

- On a fresh `AGENTDECK_HOME` the onboarding wizard did not appear, because the review's PATH shim
  makes the fake adapter satisfy the backend credential probe. Re-run without the shim, the same
  build reports `cred check skipped: cli_not_installed` and shows the wizard. A fixture artifact,
  confirmed and discarded rather than reported.
- "Set up later" on the wizard's backend step dismisses the whole wizard and marks onboarding
  complete rather than skipping one step. This is deliberate and specified (FS-04.R32), and is
  outside this release's delta.
- A task that finishes with outcome `failure` is not counted as needing attention, by FS-02.A26's
  explicit wording. Its badge renders in the neutral tone but carries the literal text "FAILURE",
  so it is not silently indistinguishable.
- `PUT /api/projects/{id}` ignores an `archived` field in the body; archiving has its own route.
  Not a defect — the wrong endpoint was used during setup.
