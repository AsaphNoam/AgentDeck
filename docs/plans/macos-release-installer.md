# Plan — macOS release installer (disposable sequencing)

Sequencing only; the specs (FS-10, TS-05.R12, TS-06.R13–R21, INV §9/§10) are the source of truth.
Delete this file when the change completes (git keeps it).

## Design anchors (reversible local choices — flag for review)

- **Application root** resolves to `~/Library/Application Support/AgentDeck`, overridable via
  `AGENTDECK_APP_ROOT` (mirrors `AGENTDECK_HOME`) so tests and non-darwin dev machines exercise the
  install/update transaction. Distinct from `$AGENTDECK_HOME` (TS-06.R16).
- **Wrapper** (`bin/agentdeck`) is a POSIX `sh` script (no compiled dependency): it prepends
  `runtime/node/bin` and `runtime/node_modules/.bin` then `exec`s `libexec/agentdeck` (TS-06.R15).
- **Release coordinates** default to GitHub repo `AsaphNoam/AgentDeck` (module path stays
  `github.com/agentdeck/agentdeck`), overridable for tests via a metadata-fetcher interface.
- **Transaction reuse (INV §2):** the bootstrap shell installer and `agentdeck update` share one
  Go stage→verify→activate core (`internal/release`); shell only downloads + checksums + hands off.
- **Durability (INV §9):** atomic rename for `current`/`previous` pointer swaps, fsync file + parent
  dir; serialization via a lock file in the app root; pointer swap never touches a running dashboard.

## Slices (each: spec-first if behavior changes, implement, test both Go variants, commit)

1. `internal/release` layout+activation core — app root, versions/, current/previous, activate,
   rollback, serialize lock. (TS-06.R16,R18; FS-10.R10)
2. Archive layout + `manifest.json` + SHA-256 & internal-manifest/layout verification + staging.
   (TS-06.R15,R17; TS-05.R12)
3. Wrapper script + shim install. (TS-06.R15,R16)
4. `agentdeck release install` internal cmd + `scripts/release/install.sh` bootstrap + darwin/arm64
   guard. (FS-10.R1–R4,R8; A1,A5)
5. `agentdeck update [--check|--yes|--rollback]` with fetcher interface + corrupt-download recovery.
   (FS-10.R7,R8,R13,R14; TS-06.R18,R19; A4)
6. `agentdeck auth claude|codex` delegation boundary. (FS-10.R5,R11; TS-06.R20; A3)
7. Interactive UX: PATH prompt, start+open dashboard, `--no-start`/non-interactive. (FS-10.R6,R12; A5)
8. `scripts/release/assemble.sh` + `.github/workflows/release.yml`. (TS-06.R14,R21)
9. README release-install docs; flip `(planned)` tags across FS-10/TS-06/TS-05; Traceability;
   handoff + brief. (FS-10.A6,R9; TS-05.R12)

## Gated (manual, not automatable here)

Real GitHub Release publish, real Node distribution download in CI, credentialed Claude/Codex
sign-in and chat. Kept behind the fetcher/adapter interfaces; tests use local fake archives + fake
adapters.
