# Usability Review Run — 2026-07-26 (week of substantial change)

**Scope:** the journeys covering everything shipped since the last full matrix run (`1b039a4`,
2026-07-19) through `454f810`: annotate-and-assign, the onboarding credential repair, per-turn
transcript indexing, configurable pipeline runs, and the two fix batches. Driven against the
release-style `sqlite_fts5` binary and the untagged fallback build with isolated homes and the
repository fake ACP peer. No real provider session or credential was invoked.

**Review surface:** browser ladder **rung 1** — Playwright driving real Chromium, with DOM,
screenshot, computed-style, and console-error capture — plus loopback REST/SSE assertions, agent
MCP tool calls made with each agent's own minted token, and on-disk state checks. Evidence is under
`.review/usability-20260726-week/evidence/`. Product code and specifications were not changed.

## Executive summary

1. **BLOCKER — diff-line annotation is unreachable.** FS-13's headline capability does not work at
   all. The diff renderer emits line ids of the form `L-1`/`R-1`, and `DiffBlock.tsx` parses
   `^([LR])(\d+)$`, which never matches, so no click on a line-number gutter ever registers a
   selection. Only whole-event annotation works, in both live and archived transcripts. No test
   exercises the third-party callback contract, which is why the suite stayed green.
2. **BLOCKER — no Codex model can be assigned to a pipeline stage.** Both seeded Codex model ids
   contain a dot, and stage assignment validates model ids with the role/project slug rule, which
   forbids dots. The Start-run form offers those models and then refuses the run. This makes
   FS-14.R2's headline example — "Codex for implementation and Claude for review" — impossible on a
   stock install.
3. **MAJOR — the archive shows only the first 50 sessions and offers no pager.** With 55 sessions
   the header truthfully says "55 results" while five are unreachable from the UI. The offset
   parameter exists on the API and the page never uses it.
4. **MAJOR — three more dead ends:** the Start-run form throws away the server's field diagnostics
   and shows only "run cannot start"; a home first opened by the FTS5 build cannot launch *any*
   agent under the untagged build, failing with a raw `no such module: fts5`; and a search whose
   terms span two turns of one session answers "No results" with no hint that terms must co-occur
   in a single turn.
5. **Everything else driven this week passed.** The whole pipeline lifecycle, the annotation tray
   and its three delivery routes, archive search on both build variants, first paint on a truly
   empty home, chat round-trip and delta coalescing, all four permission outcomes including the
   historically defective cancel-while-pending case, crash/disconnect recovery, and restart
   durability across agents, layout, archive, and paused pipeline runs.
6. **Credentialed provider acceptance remains gated** and was not exercised.

## Journey results

### J14 — Configurable pipelines (`seeded/` + fake ACP, ports 47201–47206)

| Step | Result | Evidence |
|---|---|---|
| 14.0 Pipelines page first paint | **PASS** — styled page, all three panels render, zero console errors. | `J14/01-pipelines.png` |
| 14.2 Run start launches exactly stage 1 | **PASS** — one live agent, correct role/backend/model, run `running`. | `J14/10-run-live-work.png` |
| 14.3 Idle without a result never advances | **PASS** — revision unchanged after the stage agent went idle. | run detail poll |
| 14.4 Authenticated result plus turn end advances once | **PASS** — advanced Work → Review only after the reporting turn ended. | MCP response `awaiting: quiescence` then advance |
| 14.5 Duplicate/stale report rejected | **PASS** — the finished stage's token is revoked (`session_unknown`); the run did not move. | MCP error body |
| 14.6 Approval-required transition pauses | **PASS** — run paused, "Needs attention · approval required", Approve/Stop offered. | `J14/11-approval-gate.png` |
| 14.7 Continue resumes into the next stage | **PASS** | run detail |
| 14.8 Stale revision rejected | **PASS** — 409 on a compare-and-swap mismatch. | HTTP status |
| 14.9 Validation failure routes to Fix | **PASS** | run detail |
| 14.10 Fix routes back to Review (repair loop) | **PASS** — visit counter incremented, bound respected. | run detail |
| 14.11 Run reaches a final outcome | **PASS** — `completed · success` after six attempts. | `J14/12-completed-run.png` |
| 14.12 Codex stage assignment | **FAIL — BLOCKER** — reproduced through the API (422) and the rendered form. | `J14/30-codex-model-options.png`, `J14/32-codex-start-result.png` |
| 14.13 Stopping a stage agent pauses the run | **PASS** — `paused · agent_stopped`, not wedged in `await_result`. | run detail |
| 14.14 Retry available after an agent stop | **PASS** | HTTP 200 |
| 14.15 Retry uses a fresh agent | **PASS** — new agent id. | run detail |
| 14.16 Blocked result pauses with an honest cause | **PASS** — "Needs attention · blocked" plus the stage's own summary. | `J14/20-blocked-run.png` |
| 14.17 Blocked Continue resumes the same agent | **PASS** — same agent id, new attempt. | run detail |
| 14.18 Concurrent same-project run requires acknowledgement | **PASS** — 409 naming each conflicting agent and run. | error envelope |
| 14.19 Acknowledged concurrent run starts | **PASS** | HTTP 201 |
| 14.20 Completed run survives a server restart | **PASS** — state, outcome, and attempt history intact. | `J14/40-after-restart.png` |
| 14.21 Every stage transcript opens from the completed run | **PASS** — all six attempts link to a rendering transcript carrying the stage assignment. | `J14/41-stage-transcript.png` |
| 14.22 Run deletion removes only the pipeline record | **PASS** — run 404s afterwards; archived session count unchanged. | `J14/42-after-delete.png` |
| 14.23 A template edit does not change an in-flight run | **PASS** — the run kept its frozen snapshot while the saved template changed. | run detail |
| 14.24 A proposal saves nothing by itself | **PASS** — proposed template still 404 after the tool returned a digest. | MCP response |
| 14.25 Only an AgentDecker session may propose | **PASS** — an implementer agent got `proposal_forbidden`. | MCP error body |
| Credentialed Codex/Claude stage execution | **SKIPPED(credentialed provider gate not authorized)** | — |

Console errors across the whole J14 journey: none, except the expected 422 resource log produced by
step 14.12's rejected start.

### J13 — Annotate & assign (`seeded/` + fake ACP ×2, port 47355)

| Step | Result | Evidence |
|---|---|---|
| 13.1 Two fake chat agents, one turn each | **PASS** | `J13/s2-chat-open.png` |
| 13.2 Live transcript: select diff lines → Annotate | **FAIL — BLOCKER** — no click on any gutter cell registers. | `J13/s2-diff-after-linenumber-clicks.png` |
| 13.2b Live transcript: select a whole event → Annotate | **PASS** — selection reflected in the tray. | `J13/s3-tray-3.png` |
| 13.3 Tray add/edit/remove, survives reload, excerpt matches the stored one | **PASS** | `J13/s3-tray-after-reload.png` |
| 13.4 Send to self | **PASS** — durable annotation card, agent runs a turn, tray clears. | `J13/s4-after-send-self.png` |
| 13.5 Send to another running chat agent | **PASS** — reserved-sender mail delivered via nudge, recipient woke, `Mail 1` badge. | `J13/s5-grid-badge.png` |
| 13.6 Send as a prefilled new launch | **PASS** — modal prefilled; Cancel preserves the tray. | `J13/s6-launch-modal-prefilled.png` |
| 13.7 Annotate an archived transcript | **PASS** for events (diff-line selection broken here too); composer absent, cards replay, tray survives reload. | `J13/s7-archive-tray-after-reload.png` |
| 13.8 Annotations searchable in Archive | **PASS** — four distinct instruction terms each returned one hit with a snippet. | `J13/s8-archive-search.png` |
| 13.9 Error paths (empty batch, blank instruction, unknown source, stopped recipient) | **PASS** — 422/404/409 each surfaced verbatim, tray preserved. | `J13/s9-blank-instruction-error.png` |

Contrary to the S5 lead, the annotation send path does **not** collapse server errors: it unwraps the
error envelope and renders the server's exact text.

### J8 — Archive & search, both build variants (ports 47801–47881)

| Step | FTS5 build | Untagged build | Evidence |
|---|---|---|---|
| 8.1 Empty archive, onboarded home | **PASS** | **PASS** | `J8/fts/01-empty-archive.png` |
| 8.1b Empty archive, completely empty home | **PASS** | **PASS** | `J8/emptyhome-fts/40-empty-home-archive.png` |
| 8.2 One archived session opens read-only with the full transcript | **PASS** | **PASS** | `J8/fts/04-archived-transcript.png` |
| 8.3 Five sessions × multiple turns; counting/paging is per distinct session | **PASS** | **PASS** | `J8/fts/05-many-sessions.png` |
| 8.3b 55 sessions: paging affordance | **FAIL — MAJOR** | not re-run (same UI) | `J8/many/30-archive-55-sessions.png` |
| 8.4a–c Term in a prompt / in a reply / phrase within one turn | **PASS** | miss by design (metadata only) | `J8/fts/07-search-prompt-term.png` |
| 8.4d Two terms across two turns of one session | **FAIL — MAJOR** — "No results", no signal | same | `J8/fts/08-search-cross-turn.png` |
| 8.4e Nothing matches → honest empty state | **PASS** | **PASS** | `J8/fts/06-search-no-match.png` |
| 8.4f Metadata-only term | **PASS** — tagged `matched_in: metadata` | **PASS** | curl output |
| 8.5 Snippet shows the matched text | **PASS** | no snippet, unsignalled (MINOR) | `J8/fts/07-search-prompt-term.png` |
| 8.6 Resume from archive preserves identity/model and folds streamed deltas | **PASS** | **PASS** | `J8/fts/10-after-resume.png` |
| 8.7 Fallback build on its own native state.db | — | **PASS** | `J8/noftsnative/*` |
| 8.7b Fallback build on an FTS5-created state.db | — | **FAIL — MAJOR** — no agent can launch or resume | `J8/downgrade/21-downgrade-launch-ui.png` |
| 8.7c FTS5 build on an untagged-created state.db | **PASS** | — | `J8/upgrade/50-upgrade-archive.png` |

### J1 / J3 / J4 / J11 — core regressions (ports 47401–47410)

All **PASS**, no BLOCKER or MAJOR. Highlights: a truly empty home paints a styled shell with zero
console errors and a real empty state, and `/api/layout` returns `order: []` rather than `null`;
streamed deltas coalesce into one assistant message per turn, twice, and identically after reload;
all four permission outcomes — approve, deny, timeout, and **cancel while a permission is pending** —
resolve correctly live and after reload, with no double-fire and a 409 on re-decision; server kill
shows a reconnecting indicator and recovers without a manual reload; an agent crash and a
SIGINT-escalated cancel both land on an honest error card matching the server.

### J12 — Restart durability (ports 47601–47602)

**PASS** over the state left by the earlier journeys. Archived session counts, layout, and pipeline
runs were identical before and after a restart, with zero console errors. All three interrupted
pipeline runs came back as `paused · restart_recovery` — neither lost nor silently advanced.

## Findings

### BLOCKER — J13: diff-line annotation never registers a selection

```text
SEVERITY: BLOCKER
WHERE: J13 steps 2 and 7 (fixture seeded/, port 47355), live and archived transcripts
REPRO: open an agent transcript containing a diff → click any line-number gutter cell (either side,
       td or inner pre, including force-click)
EXPECTED: the line range is selected and an "Annotate lines N–M" button appears (FS-13.R1/R2)
OBSERVED: nothing happens on any click; the annotate button count stays 0 and the tray gains no
          entry. Only whole-event annotation is reachable.
EVIDENCE: .review/usability-20260726-week/evidence/J13/s2-diff-after-linenumber-clicks.png
```

`react-diff-viewer-continued` builds its line id as `` `${prefix}-${lineNumber}` ``
(`lib/src/index.js:118`), so the callback receives `L-1` / `R-5`. `DiffBlock.tsx:11` matches
`/^([LR])(\d+)$/`, which has no hyphen, so `chooseLine` returns on the first line every time and
`setSelection` is never called. Confirmed by reading both sides and by the browser replay. No test
in `ui/src` exercises `DiffBlock` or the `onLineNumberClick` contract — the annotation tests cover
the store, the excerpt clipping, the SSE removal path, and the endpoint, so the whole third-party
seam is untested. The fix is to accept the library's actual id format and add a `DiffBlock`
regression test that feeds it a real library-shaped id.

### BLOCKER — J14: no Codex model can be assigned to a pipeline stage

```text
SEVERITY: BLOCKER
WHERE: J14 step 12 (fixture seeded/, ports 47203 and 47204)
REPRO: Pipelines → Start run → choose the saved template and project → set stage 1 Backend to
       "Codex (codex)" → choose either offered model, "GPT-5.6-Sol (gpt-5.6-sol)" or
       "GPT-5.5 (gpt-5.5)" → fill the run fields → click Start run
EXPECTED: the run starts with a Codex stage, per FS-14.R2 ("Codex for implementation and Claude for
          review") and FS-14.A1/A7
OBSERVED: HTTP 422; the form shows only "run cannot start"; the diagnostic is
          "assignments.work: every stage requires a configured backend and model" even though a
          backend and model were both selected from the app's own dropdowns
EVIDENCE: .review/usability-20260726-week/evidence/J14/30-codex-model-options.png and
          32-codex-start-result.png
```

`internal/pipeline/manager.go:194` gates stage assignment on
`config.ValidSlug(assignment.Backend) || config.ValidSlug(assignment.Model)`, and `ValidSlug`
(`internal/config/validate.go:15`) is the role/project **filename** rule — `^[a-z0-9][a-z0-9-]{0,62}$`,
which deliberately excludes dots to prevent path traversal. Model ids are catalog map keys, not
filenames, and FS-09.R33 seeds Codex's default as `gpt-5.6-sol`; `gpt-5.5` fails too, so **every**
seeded Codex model is unusable and the failure is not specific to a hand-edited catalog. The existing
pipeline tests all use the model id `"gpt"`, which is slug-clean, which is why the suite stays green.
The fix is to validate a model id against the configured catalog (the run already resolves the
backend and model) rather than against the filename slug rule, and to add a regression test that
assigns a dotted seeded model id.

### MAJOR — J14: the Start-run form discards the server's field diagnostics

```text
SEVERITY: MAJOR
WHERE: J14 step 12 (fixture seeded/, port 47204)
REPRO: submit any invalid run start — a Codex model as above, an undeclared input, an over-long
       input, or an unavailable backend
EXPECTED: the offending field and reason, as the template editor already renders for template
          validation
OBSERVED: only the sentence "run cannot start"; the diagnostics array in the error envelope is
          never rendered
EVIDENCE: .review/usability-20260726-week/evidence/J14/32-codex-start-result.png
```

`ui/src/features/pipelines/RunStartForm.tsx:116-120` sets the error from `reason.message` alone.
`pipelineDiagnostics(error)` already exists in `ui/src/api/pipelines.ts:209` and
`TemplateEditor.tsx:279-281` already renders the same shape as a `pipeline-diagnostics` list, so the
fix is to call it in the run form's `onError`. TS-09 requires bounded field diagnostics precisely so
an invalid configuration "remains repairable"; dropping them at the run surface removes the user's
only way to self-diagnose. This finding compounds the blocker above but is independent of it.

### MAJOR — J8: the archive shows only the first 50 sessions with no pager

```text
SEVERITY: MAJOR
WHERE: J8 step 3b (fixture j8-many, 55 archived sessions, port 47851, FTS5 build)
REPRO: accumulate more than 50 archived sessions → open /archive
EXPECTED: every session reachable, or a next-page / load-more control
OBSERVED: the header says "55 results" and 50 rows render, with no pager anywhere; the five oldest
          sessions are unreachable from the UI
EVIDENCE: .review/usability-20260726-week/evidence/J8/many/30-archive-55-sessions.png
```

`ui/src/features/archive/ArchivePage.tsx:72` always calls `searchArchive(query, 50, 0)`. The offset
parameter that FS-05 specifies exists on the API and is never used by the page, so the archive
silently becomes lossy for any long-lived install. The count in the header is honest, which makes
the missing rows more confusing rather than less.

### MAJOR — J8: a home opened by the FTS5 build cannot launch agents under the untagged build

```text
SEVERITY: MAJOR
WHERE: J8 step 7b (fixture j8-downgrade, port 47841, untagged build)
REPRO: run the sqlite_fts5 binary once against a home, stop it, then run the untagged binary on the
       same home and launch or resume any agent
EXPECTED: the launch succeeds; the index degrades the way search already does on this build
OBSERVED: HTTP 502 and the raw internal message "runtime: index session meta: index: delete metadata
          document: no such module: fts5" printed in the launch modal. No agent can be launched or
          resumed on that home at all.
EVIDENCE: .review/usability-20260726-week/evidence/J8/downgrade/21-downgrade-launch-ui.png
```

Replayed and confirmed independently. `make build` defaults to `TAGS := sqlite_fts5`, so the shipped
release binary is unaffected and a machine that only ever runs the untagged build is also fine — the
failure needs an FTS5-first-then-untagged sequence on one home. That reachability is why this is
recorded as MAJOR rather than BLOCKER, downgrading the journey owner's original severity. It still
deserves a fix: the search path already falls back for exactly this case, the write path does not,
and the user is shown a raw SQLite module error inside a launch dialog.

### MAJOR — J8: a search spanning two turns answers "No results" with no signal

```text
SEVERITY: MAJOR
WHERE: J8 steps 4d and 4d2 (fixture j8-fts, port 47801, FTS5 build)
REPRO: search "Marlin barnacle" — an agent name plus a word from that same agent's transcript; or
       "flywheel zephyr", two words from turn 1 and turn 3 of one session
EXPECTED: the session is found, or the UI signals that terms must co-occur within one turn
OBSERVED: "No results for \"Marlin barnacle\"" — each term alone returns that session
EVIDENCE: .review/usability-20260726-week/evidence/J8/fts/08-search-cross-turn.png
```

The behaviour itself is the accepted consequence of per-turn documents and is already recorded as a
deliberate boundary, with the cross-turn alternative parked in `docs/ideas.md`. The finding is the
missing signal: narrowing by agent name plus a keyword is the most natural way to search, and the
result is an affirmatively wrong answer that a user reads as "the session is gone". This is a copy
and affordance fix, not a request to change the index design.

### Worth fixing

- **MINOR — J8:** the untagged build's search box still advertises "Search agents, roles, projects,
  transcript…" while returning nothing for any transcript-only term and never producing a snippet.
  Evidence: `J8/noftsnative/06-search-no-match.png`.
- **MINOR — J13:** nothing marks line numbers as the selection handle — computed cursor is `auto`,
  no tooltip, no aria-label, no helper copy. Even with the blocker fixed, the affordance is
  undiscoverable. Evidence: `J13/s2-chat-open.png`.
- **MINOR — J13:** with three drafts the pending tray's Send button sits below the fold (tray height
  450px, content 1165px, button at y=1509) with no sticky footer or scroll hint, and the overlay
  occludes the right half of the transcript being annotated. Evidence: `J13/s6-tray-overlay-3drafts.png`.
- **MINOR — J13:** a tray whose source agent no longer exists cannot be discarded from the UI and
  keeps one of the twenty retained-source slots for thirty days; navigating to that agent also
  raises an uncaught `no such agent` page error. Evidence: `J13/s9-ghost-source-page.png`.
- **MINOR — J4:** when a later turn reuses a tool-call id, the live view rewrites every earlier
  permission chip with the newest decision; a reload shows the correct history. Observed as
  `["denied","denied"]` live versus `["approved","denied"]` after reload. Evidence:
  `J4/04-denied.png` and `J4/05-denied-after-reload.png`.
- **MINOR — J11:** a whitespace-only agent name is accepted at launch and renders a card with no
  title, while Rename rejects the identical value with "name is required"; 5000-character and
  NUL-containing names are also accepted unvalidated. Evidence: `J11/30-whitespace-name-confirm.png`.
- **MINOR — J11:** a project saves silently with a whitespace-only title and cwd, then appears as a
  selectable target in New Agent. Evidence: `J11/31-project-whitespace.png`.
- **MINOR — J3/J11:** cancelling a turn against a peer that ignores cancellation escalates correctly
  and unblocks the UI in about three seconds, but the transcript shows only "process exited" and the
  card goes to error — nothing ties the outcome to the user's Cancel or says the agent is now dead.
  Evidence: `J3/22-after-cancel.png`.
- **POLISH — J4:** a cancelled and a timed-out permission both render the chip "DENIED". This matches
  the two-state chip FS-03 specifies, so it is spec-conformant, but it misreports what happened.
  Evidence: `J4/07-after-cancel-live.png`.
- **POLISH — J8:** an opened archived transcript's header shows only "Archived session · read-only ·
  Resume" with no name, project, model, or date, so with several similar sessions the user cannot
  tell which one they opened — and Resume is one click away. Evidence: `J8/fts/04-archived-transcript.png`.

### Unconfirmed

- **J11 archive search 500.** One run showed the raw server error `archive: count search:
  unterminated string` with a console 500 after a sequence of whitespace, 2000-character, and
  quote-containing queries. An identical replay logged only 200s, and direct API probes of the same
  inputs all returned 200. Recorded as unconfirmed, not as fact. Evidence: `J11/32-archive-search.png`
  and the non-reproducing `J11/40-archive-repro.png`.

## Static sweeps

- **S1 serialization contract:** clean. Every pipeline, annotation, archive, and onboarding
  collection is initialised to a non-nil empty value before marshaling, and the Pipelines UI sits
  behind a zod schema boundary. No `null`-collection lead survived.
- **S2 CSS wiring:** clean in both directions. `npm run check:styles` passes, and an independent
  extraction of every `className` in `ui/src` found no referenced-but-undefined class and no
  genuine orphan. The repo has no `clsx`/`classnames` usage that could hide a dynamic class from
  the contract checker.
- **S3 external-CLI variance:** the widened `--no-color` retry from `454f810` is correct and now
  requires both the flag substring and unsupported-argument vocabulary; no other prober passes an
  optional flag. One residual lead: the tmux terminal driver shells out with no timeout anywhere,
  unlike the iTerm2 driver which bounds every call at four seconds. J6 is gated, so this was not
  reproduced and is recorded as a source risk lead for the code-review path, not a usability finding.
- **S4 null hostility:** clean; every server-derived collection dereference in `ui/src` is either
  guarded or provably non-null.
- **S5 error surfacing:** the pipeline and annotation mutations are well wired. Two onboarding
  regressions were found by reading and are recorded as leads because this run did not inject the
  server failure needed to observe them: `BackendStep.tsx:122` and `ProjectStep.tsx:48` both use
  `setError(String(e))`, which renders "Error: HTTP 400" instead of the server's reason. The same
  mistake was previously fixed in five sibling editors, and the run-start finding above is the same
  class observed live.

## Coverage notes

- **Journeys driven:** J1, J3, J4, J8 (both build variants), J11, J12, J13, J14. All are
  distinguishable in the tables above between passing and unexercised.
- **Not driven:** J2 (replayed separately on 2026-07-26 and unchanged since, except that its
  reported blocker is fixed), J5 grid/layout and J7 stop/resume/switch (unchanged this week and
  covered by the 2026-07-19 matrix), J6 terminal, J9 settings, and J10 messaging beyond the
  annotation delivery paths J13 exercised.
- **Gated:** every credentialed provider branch. No authenticated Claude or Codex session, API key,
  or login flow was invoked; all agent behaviour came from the bounded fake ACP peer.
- **Harness note:** stage results were delivered by calling `report_pipeline_stage_result` over the
  local MCP endpoint with each stage agent's own minted token — the same authenticated path a real
  stage agent uses — while that agent's turn was held open by a pending permission, so the run
  advanced through genuine turn quiescence rather than a synthetic shortcut.
- **Matrix maintenance (§7):** the J14 charter matched the shipped surface and needed no change. Two
  gaps are worth adding to the matrix later: J8 does not currently name the >50-session variant that
  exposed the pagination defect, and no journey covers switching build variants on one home.
