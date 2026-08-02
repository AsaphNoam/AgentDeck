# Usability review run — 2026-08-02 — backend creation and configuration linking

**Scope:** FS-04.R40/A20, FS-08.R33/R34/A11, and FS-09.R47 for the current backend creation and global configuration-linking flow.

**Result:** PASS for the exercised browser journey. No new user-impact findings. Existing durability findings in `HANDOFF.md` were not reclassified by this run.

## Harness

- **Binary:** `make build` (`bin/agentdeck`, `sqlite_fts5`), current `main` at `6f7bf78`.
- **Fixture:** fresh isolated `AGENTDECK_HOME` at `/private/tmp/agentdeck-usability-20260802.EZSFi4`; loopback server on port 4394. The review created and removed only state under that home. Native Claude and Codex setup was read from the machine's existing user-level locations; neither source was modified.
- **Browser:** in-app Browser Playwright surface (protocol rung 1), driven against `http://127.0.0.1:4394`.
- **Evidence:** browser DOM snapshots and final loopback API responses captured during the run. Browser console warning/error entries: `[]`.

## Journey results

| Journey | Steps | Result |
|---|---|---|
| J1 Install & first paint (build portion) | `make build` produced the tagged binary; fresh server started and Settings rendered in the browser. | PASS |
| J9 Settings & config — Add backend | The dialog opened; choosing Codex changed the editable suggested name to `Codex / OpenAI` and the saved item had starter model `gpt-5.6-sol`. Cancel closed the dialog and left the durable catalog at its original four entries. | PASS |
| J9 Settings & config — dirty draft | An unsaved `Claude` → `Draft Claude` Settings edit remained visible after creating a separate Codex backend. The durable catalog retained `Claude`, while the new `codex-openai` item was present. | PASS |
| J9 Settings & config — existing backend link | A newly created Codex backend opened one project-free **Use my Codex configuration** control. Linking immediately displayed `ok` / `linked`, enabled model sync, and added four configured models to that target. | PASS |
| J9 Settings & config — direct create and connect | **Create and use my configuration** created `Review connected Claude`; its expanded source panel immediately displayed `ok` / `linked`, the user-level source path, and no Mirrored recovery action. | PASS |
| J9 Settings & config — unlink | Unlink on the temporary Claude backend immediately restored the ordinary **Use my Claude Code configuration** action. Final API state retained only the intentionally linked temporary Codex binding. | PASS |

## Final API evidence

`GET /api/backends` after the journey returned keys `claude`, `codex`, `codex-openai`, `opencode`, `openhands`, and `review-connected-claude`; the durable Claude name remained `Claude`, proving the separate dirty editor value was not saved by item creation. `codex-openai` retained its expected `gpt-5.6-sol` default.

`GET /api/config-sources` returned one healthy linked Codex binding for `codex-openai`; both Claude and Codex source candidates were healthy. The direct-created Claude backend was deliberately unlinked before this final read.

## Not exercised

- Connection failure followed by retry: both available native sources were healthy. A controlled malformed/unavailable source fixture is needed to observe this browser branch without modifying personal provider setup.
- Launch through the new binding and real-provider honoring: this remains the credentialed provider acceptance gate recorded in `HANDOFF.md`; no provider session was started.
- The pre-existing concurrent/stale-write durability findings are API/interleaving defects, not failures that this single-browser journey can prove or close.
