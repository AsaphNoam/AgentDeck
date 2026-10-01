# AgentDeck — State through 2026-10-01

Archived when `v0.8.0` was prepared. Earlier settled state remains in the preceding files in this
directory; this epoch records the work completed after `v0.7.0`.

## Release boundary

- `v0.8.0` covers 13 commits after `v0.7.0`, including the post-publication state commit.
- The release adds unrestricted on-demand viewing of readable local UTF-8 text files from chat,
  source and rendered-Markdown file-selection annotations through the existing tray and delivery
  flow, and correct Claude model application after native session resume.
- The shipped `operating-agentdeck` reference was refreshed with the local-file viewing and
  annotation boundary. README install/command claims and the pinned Node, Claude ACP, Codex ACP,
  and Codex CLI versions remained current.
- `make test`, all 562 UI tests, the presentation contract, and `make dist VERSION=0.8.0` passed.
  The first parallel UI run had one timing miss in `NewAgentModal`; that test passed in isolation
  and the complete serial rerun passed.
- The release build refreshed the generated embedded UI and produced a `sqlite_fts5` binary that
  reports version `0.8.0`.

## Settled changes

- **Open and annotate any local text file.** Chat file links now accept absolute, relative,
  symlinked, and `.git` paths under the approved same-machine trust policy while retaining
  regular-file, UTF-8, size, loopback-origin, and remote-route boundaries. Source and rendered file
  selections become backward-compatible file anchors in the shared annotation tray. Review found
  and the fix closed path-identity, link-authority, remote-denial, integration, persistence, and
  legacy round-trip gaps.
- **Claude same-backend model switching.** Claude chat now applies and verifies the selected model
  after session creation or load, so a resumed provider transcript cannot silently restore its old
  model over the operator's choice. The investigation, regression, review, and fix are closed.
- **Exact context and expanded-card runtime metadata.** Design completed and the ready change
  `show-exact-context-and-runtime-metadata.md` remains available for implementation after this
  release.

## State carried forward

- Work units: `show-exact-context-and-runtime-metadata.md` and `rename-product-to-deckhand.md` are
  ready; `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
- Review and fix queues are empty.
- Credentialed Claude/Codex, Codex 1.12.0, real-tailnet/device, six-tab shared-stream, J14, and
  Sky & Grove with Codex capability gates remain owed as listed in the live handoff.
- API/model error compatibility, failed pipeline-stage chat behavior, presentation radii, the
  paused direct-action spec alignment, injected-steer lifetime behavior, and silent Files/Commands
  tab copying remain future decisions or investigation notes.

