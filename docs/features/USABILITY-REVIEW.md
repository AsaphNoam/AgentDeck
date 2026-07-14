# AgentDeck — Usability Review Protocol

**Verification protocol for the behavior-driven, top-to-bottom usability review.** It derives
expected behavior from feature-spec acceptance criteria, builds the real binary, starts fresh state,
and drives journeys through the actual UI. It complements the static diff review in
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) §7; it is not a third product-spec set.

> Claude Code reaches this via the `/usability-review` skill; Codex via
> `.agents/skills/usability-review`. Both land here. The workflow governs role behavior; feature
> specs govern expected product behavior; this file governs the verification method.

**Prime directive: user-impact findings are confirmed against the running app. Static sweeps produce
risk leads, not product findings, until reproduced or routed to the §8 code-review path.**

---

## 0. Why this exists — the escape post-mortem

The human's first real use of AgentDeck hit three blockers (fixed in commit `353e940`) that many
review cycles never caught:

1. **Fresh-install crash** — `layoutFromConfig` built the order slice with `append([]string(nil), …)`;
   an empty layout marshaled to JSON `null`, `CardGrid.tsx` called `order.filter(...)` → TypeError →
   dead dashboard on first launch.
2. **Environment variance** — the credential prober ran `claude auth status --no-color`; older
   Claude CLI builds don't support `--no-color`, so a valid login reported as failed.
3. **Unstyled onboarding** — wizard/dialog components referenced CSS classes (`.dialog-overlay`,
   `.wizard-*`, `.form-field`, …) that were never defined; the first-run wizard rendered as
   unstyled soup.

Why every prior review structurally could not catch them:

- **Reviews were static diff-reads.** §8 is read-only and diff-scoped; no reviewer ever built the
  binary, started the server with a fresh state dir, or rendered the UI. All three bugs are
  invisible in a diff and obvious in a browser.
- **Calibrated against specs, not users.** The code matched the spec; the spec never says "order
  must serialize as `[]` not `null`". The §8 bar — *would this cause a real problem during normal
  usage?* — was answered by reading code, not by observing behavior.
- **INVARIANTS.md is backward-looking.** Reviews sweep diffs against classes already paid for; all
  three escapes were classes not yet in the catalog.
- **The test infrastructure actively masked the bugs.** MSW mocks return the idealized contract
  (`order: []`) while the Go server emits `null` for nil slices — UI tests pass against a server
  that doesn't exist. Testing Library never evaluates CSS, so a missing stylesheet is green.
  `fakeacp` is deterministic, so real-CLI flag/version variance is never exercised.
- **Nobody owned the composed first-run path.** Each review saw one phase's diff; the fresh-install
  journey (empty config → onboarding → first launch) crosses all phases and was never any single
  review's target.

This review exists to close exactly that gap: its evidence is **observed behavior of the running
app**, composed across all phases, starting from the states a real user actually starts from.

---

## 1. Scope, bar, and severity

**The bar:** *a first-time or daily user hits this.* If an observed normal-use expectation is not
covered by an FS requirement/acceptance item, note that the requirement may be missing alongside the
finding so the fix can decide whether behavior or the specification must change.

**Severity taxonomy** (every finding gets exactly one):

| Severity | Meaning |
|---|---|
| **BLOCKER** | A core journey cannot be completed (crash, dead UI, wrongly failed gate, unusable surface). |
| **MAJOR** | The journey completes but with data loss, wrong/stale information, or a dead-end the user can't self-diagnose. |
| **MINOR** | Friction: unclear copy, missing feedback, surprising-but-recoverable behavior. |
| **POLISH** | Cosmetic. Report only when trivially fixable. |

When recording a finding: BLOCKER/MAJOR → **Must fix**; MINOR → **Worth fixing**; POLISH is worth
recording only when the fix is obvious and tiny.

**Non-goals** (do not spend tokens here): code-quality opinions, naming/style, performance
micro-audits, redesign proposals, or unverified feature ideas. The §8 diff review and
`INVARIANTS.md` own code-level classes; this review owns *experienced behavior*.

---

## 2. Ground rules — evidence discipline

1. **Test the real binary, built the way users get it.** Use the `install.sh` / `make build`
   command line (which carries `-tags sqlite_fts5`). Run one additional pass of the search journey
   (J8) on the **untagged** fallback build — the no-FTS5 path ships too.
2. **Fresh, isolated state.** Every server instance runs with `AGENTDECK_HOME` pointing at a
   review-owned temp dir — never the developer's `~/.agentdeck/`. The orchestrator prepares three
   canonical fixtures once and copies them per journey:
   - `fresh/` — completely empty dir. The first-run state. (This is the state that shipped broken.)
   - `seeded/` — onboarded config (backends/roles/projects/layout written), no sessions.
   - `lived-in/` — seeded config **plus** archived sessions, transcripts, and tracked files,
     generated by scripted fakeacp runs.
3. **Deterministic agent backend.** Journeys that need a live agent register the env-driven fake
   ACP peer (`internal/runtime/testdata/fakeacp`, built once — see `buildFakeACP` in
   `internal/server/integration_test.go`) as a backend in the fixture config. Journeys probing
   *real*-CLI behavior (J2 credential branches) are **ENV-DEPENDENT**: when the real CLI is absent
   they report `SKIPPED(reason)`, never a silent pass.
4. **Every finding is reproducible from its report alone**: fixture + exact steps + expected vs
   observed + evidence (screenshot path, DOM snippet, curl output, or console error). No evidence →
   not a finding.
5. **Read-only toward the repo.** No product-code changes and no commits of fixes — same rule as
   §7. Findings go to the handoff (§6); the `/fix-review` loop does the fixing.

---

## 3. The journey matrix

The methodical top-to-bottom core. Each journey is a **charter**: a self-contained brief with an
entry fixture, ordered steps, expected observations, and known risks. Where a journey
touches collections, exercise the **state variants**: *empty / first item / many items / stale
(restart) items*. The fresh-install escape was structurally "the empty variant was never exercised" —
treat the empty variant as mandatory, not optional.

| # | Journey | Fixture | What must be observed |
|---|---|---|---|
| J1 | **Install & first paint** | `fresh/` | Build succeeds via the real build line; server starts; UI loads in a browser with **zero console errors** and a **styled** shell (computed styles on key elements, not just DOM presence). This journey covers the historical build and missing-style escapes; J2 covers CLI-credential variance. |
| J2 | **Onboarding wizard end-to-end** | `fresh/` | Every branch: no CLI installed → clear guidance; CLI installed but not logged in → failed check with actionable detail; logged in → pass (ENV-DEPENDENT). Back/skip/validate paths. Resulting config files on disk are sane and re-readable. |
| J3 | **First agent launch + chat round-trip** | `seeded/` + fakeacp | Create agent, send a message, streamed response renders, card status transitions (idle → busy → idle) match reality. |
| J4 | **Permission prompt flow** | `seeded/` + fakeacp | Prompt appears; approve, deny, and timeout each leave UI and server state consistent; no double-fire, no stuck prompt. |
| J5 | **Grid & layout** | `seeded/` | Reorder, density, groups, collapse; persists across page reload **and** server restart. Delete an agent that is in the saved order — grid stays sane. Empty grid renders a real empty state. |
| J6 | **Terminal runtime** | `seeded/` | Launch terminal agent (xterm.js path), type, resize, detach/reattach; output intact, keystrokes not eaten. |
| J7 | **Stop / resume / switch** | `lived-in/` + fakeacp | Each verb preserves identity, model, system prompt, add_dirs — observed from the UI and the running process, not the code (INVARIANTS §2 as behavior). |
| J8 | **Archive & search** | `lived-in/` | On the FTS5 build **and** the untagged fallback build: empty archive, one session, many sessions; search returns the right sessions; resume from archive works. |
| J9 | **Settings & config editing** | `seeded/` | Every form: edit, save, reload — saved values round-trip; seeded collections are merge-preserved, never replaced (INVARIANTS §3 as behavior); invalid input surfaces an error, never a silent no-op. |
| J10 | **Multi-agent + messaging** | `seeded/` + fakeacp ×2 | Two agents; send_message, nudge wakes the recipient, unread badges appear and clear correctly. |
| J11 | **Failure & recovery** | `seeded/` + fakeacp | Kill the server mid-session → UI shows disconnect, reconnects with accurate state (SSE reset, INVARIANTS §1). Kill the agent process → card reflects the crash. Submit garbage into every form. Stop a non-running agent. |
| J12 | **Restart durability** | state left by J3–J10 | Restart the server; everything the UI showed before (agents, layout, archive, unread state) is still true after. |

**Charter shape** (what the orchestrator hands each journey subagent — nothing more):

```
JOURNEY: J5 Grid & layout
FIXTURE: <path>   PORT: <port>   BINARY: <path>   FAKEACP: <env recipe or n/a>
STEPS:
  1. <step> → EXPECT <observable>
  2. ...
KNOWN RISKS: <the 1–3 invariant classes / sweep hits to watch here>
REPORT: per the format in USABILITY-REVIEW.md §5 — step results and findings only.
```

**Maintenance rule:** this matrix is only as good as its coverage. See §7.

---

## 4. Cross-cutting static sweeps

Cheap, greppable audits that may be delegated read-only. They run **first** (§5 Phase A): hits seed
which journeys get extra attention. A source-only defect routes to §8; a usability finding still
requires observed behavior.

- **S1 Serialization-contract audit.** For every JSON response struct in `internal/server`:
  nil-slice/nil-map fields (`[]T(nil)`, bare `var x []T`, `append([]T(nil), …)`, `omitempty` on
  collections) vs the TS types in `ui/src/api` vs the MSW mock fixtures. Three-way diff. Any field
  where the mock is prettier than what the real marshaler emits is a risk lead. Reproduce it in a
  journey for a usability finding, or route a source-only contract defect through §8.
- **S2 CSS wiring audit.** Set of classNames referenced in `ui/src/**/*.tsx` vs selectors defined
  in the stylesheets. Report both directions: referenced-but-undefined (the wizard escape) and
  defined-but-unreferenced (drift).
- **S3 External-CLI variance audit.** Every `exec.Command` of a user-machine binary: which flags
  are passed, what output format is assumed, PATH assumptions, and what happens when the tool is
  old, missing, or non-interactive. Any optional flag without a fallback is a risk lead (the
  historical `--no-color` escape); reproduce it in J2 or route a source-only defect through §8.
- **S4 Null-hostility audit.** Every `.map`/`.filter`/`.length`/spread in `ui/src` on data that
  originates from a server response, cross-checked against S1's "can this actually be null" list.
- **S5 Error-surfacing audit.** Every mutating call site in `ui/src`: `.catch → pushError` present,
  no bare `void` on mutations (extends INVARIANTS §8).
- **S6 Copy & affordance pass.** Empty-state text, button labels, tooltips vs actual behavior —
  this one runs *inside* the journeys (it needs the rendered app), not as a grep.

---

## 5. Execution architecture — token efficiency

Use bounded delegation when the environment supports it. The main reviewer owns the binary/fixture
baseline, charter scope, validation of material findings, deduplication, severity, workflow state,
and final report. It may inspect source or evidence whenever needed to validate a claim. Without
delegation, run the same phases sequentially.

**Phase A — static sweeps (parallel when available).** S1–S5 may go to bounded read-only agents,
each returning structured risk leads. Their output re-prioritizes Phase B:
a journey whose surface has sweep hits gets those hits injected as KNOWN RISKS in its charter.

**Phase B — journeys (parallel where independent).** Assign one owner per journey and batch
tightly-coupled pairs (J3+J4) when useful. Isolation rule: **each journey gets its own port
and its own copy of its fixture dir**, so journeys run in parallel without interference (J12 is the
exception: it deliberately reuses state left by earlier journeys, so it runs last, serially).
A charter hands the subagent exactly the block in §3 — fixture path, port, binary path, fakeacp
recipe, step list, and report schema. The owner reads the relevant FS acceptance items and
may inspect evidence/source needed to validate a finding.

**Browser protocol.** The UI is a SPA; DOM-level checks need a real browser. The fallback ladder,
in order — the report must state which rung was used:
1. Playwright driving the environment's Chromium (preferred: screenshots + console-error capture);
2. `curl`-level API assertions plus a targeted `npm run test` render for the specific component;
3. API assertions alone (mark all visual steps `BLOCKED(no browser)` — never inferred-pass).

Screenshots at each step go to a run directory; reports carry **paths**, not images.

**Report format.** Per step: `PASS | FAIL | BLOCKED(reason) | SKIPPED(reason)`.
Per finding, max ~6 lines:

```
SEVERITY: MAJOR
WHERE: J5 step 3 (fixture seeded/, port 4381)
REPRO: reorder cards A,B,C → restart server → reload
EXPECTED: order A,B,C preserved
OBSERVED: order reset to creation order
EVIDENCE: run/J5/step3-after-restart.png
```

No narration, no logs, no code excerpts. The orchestrator rejects malformed reports (does not
re-run them); it re-runs only FAILs that lack evidence.

**Budget rules.**
- Inside a subagent: one repro attempt + one confirm attempt per finding, then report and move on.
- A subagent that gets BLOCKED (server won't start, fixture broken) reports and **stops** — it
  never debugs the app or the harness; that's the orchestrator's call.
- The orchestrator's final report is severity-ordered with a top-5 executive summary; everything
  below MINOR is an appendix.

**Verification of the review itself.** Before reporting, the orchestrator spot-replays every
BLOCKER's repro steps. A BLOCKER the orchestrator cannot reproduce is downgraded to *unconfirmed*
and flagged as such — never silently reported as fact.

---

## 6. Reporting & handoff

- Write every **Must fix** and **Worth fixing** finding (after the §1 severity mapping) to
  `## Review findings` in [`HANDOFF.md`](HANDOFF.md), using the format in AGENT-WORKFLOW §7 so the
  existing `/fix-review` (§8) process can act on it. Prefix the
  title with the journey/sweep id (`J1`, `S2`) so the fix agent can find the repro in the run
  report.
- If a finding suggests a new systemic class, record it as a candidate finding. Usability review
  does not edit specs/INV; fix-review validates and updates the technical-spec appendix.
- Save the full run summary (journey results, all findings, evidence paths) as a file, and report to
  the human via the **human update** (AGENT-WORKFLOW §6): a focused, plain-language entry in
  [`BRIEFS.md`](BRIEFS.md) — what was driven, severity counts, each blocker as one plain sentence,
  link to the run summary file — pasted as the end-of-turn message. Do not list every lower-priority
  item in the update; those live in the handoff and the run file.
- If a journey is all-PASS, say so in the matrix — an unexercised journey and a passing journey
  must be distinguishable.
- Commit the state updates together on `main` (`usability review: <journey> — state recorded`) and push
  at session exit (AGENT-WORKFLOW §6) — product code stays untouched.

---

## 7. Maintenance — the matrix must track the product

When an FS adds or changes a user-facing acceptance item, the same change adds or extends a
journey charter here when browser verification is appropriate. The reviewer's first step each run
compares this matrix with the feature-spec index and relevant A-items, then records uncovered
normal-use surfaces as possible missing requirements before running anything.
