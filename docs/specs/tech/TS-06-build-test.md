# TS-06 — Build, test & delivery

**Status:** Current
**Code:** `Makefile`, `go.mod`, `ui`, `internal/server/ui`, `install.sh`, `scripts`, `.github/workflows`
**Absorbed:** build/test sections in the [phase archive manifest](../../archive/phases/README.md) and contributor guidance formerly duplicated in [`CLAUDE.md`](../../../CLAUDE.md)

## 1. Scope

This spec owns supported toolchains, build tags, UI embedding, release/install constraints, GREEN
verification, spec linting, and test conventions.

## 2. Design & constraints

**R1 — The declared toolchains are authoritative.** Go follows `go.mod` (currently Go 1.25); Node
20 is the shared UI/CI baseline. Node is build-time only for a prebuilt AgentDeck binary.

**R2 — Release builds enable FTS5.** Every distributed Go build uses the `sqlite_fts5` tag. The
untagged path remains supported solely as the tested metadata-search fallback; a release command
without the tag is a defect.

**R3 — The UI is embedded, not hand-edited.** `ui/src` is the source. `make embed` builds the Vite
app and copies `ui/dist` into `internal/server/ui/dist`; agents never edit the embedded output.

**R4 — Standard targets have stable meaning.** `make build` creates the tagged binary; `make test`
runs spec lint plus both Go variants; `make dist` builds UI, refreshes embed output, and builds the
tagged binary; `make check-specs` runs the mechanical spec contract.

**R5 — GREEN is proportional but never selective.** A product-code checkpoint runs both Go test
variants and any affected UI build/tests; concurrency hot spots add focused race tests. A docs-only
spec/workflow checkpoint runs spec lint and link/reference checks plus any build/test needed to
validate claims it changed. Failures may not be hidden by removing or weakening tests.

**R6 — Acceptance tests name their contract.** New or materially touched tests that prove a feature
acceptance item include an exact `FS-nn.Ak` comment. Specs point back to load-bearing tests/code;
behavior/contract commits carry governing IDs in the subject or `Spec:` trailer.

**R7 — Spec lint enforces mechanics, review enforces truth.** Automated checks validate filenames,
headers/status, local R/A uniqueness, index parity, planned/current consistency, relative links,
citations, conflict markers, and tool-wrapper artifacts. They do not infer semantic completeness.

**R8 — CI repeats shared checks from a clean clone.** Pushes to `main` and pull requests run spec
lint, both Go variants, `go vet`, UI install/tests/build. CI uses read-only repository permissions
and cancels superseded runs; it does not rewrite embedded tracked output.

**R9 — Tests isolate user state and external providers.** Tests use temporary
`AGENTDECK_HOME`, deterministic fake ACP peers, in-process HTTP handlers, and fixtures. Credentialed
real-CLI acceptance is an explicit manual gate and never silently substitutes for automated tests.

**R10 — Distribution remains a single local binary.** The install path may install optional ACP
adapters, but the AgentDeck server/UI/MCP runtime itself requires no Node or Python process.

## 3. Interfaces & data shapes

The canonical commands are:

```sh
make check-specs
make test
cd ui && npm test && npm run build
make dist
```

The exact GREEN selection for work/review roles is defined by
[`../../features/AGENT-WORKFLOW.md`](../../features/AGENT-WORKFLOW.md); this spec owns what each
shared target guarantees.

## 4. Invariants

- **INV §6:** build/capability claims cover every runtime variant advertised.
- **INV §7:** both FTS5 and fallback readers are tested.
- **R11 — Generated output has one source.** A generated file is updated only through its generator,
  and CI/tests detect stale or hand-edited outputs where practical.

## 5. Deviations & open decisions

- Credentialed Claude, Codex, OpenCode, and OpenHands acceptance remains manual/gated. Specs label
  affected claims rather than treating fake-provider success as real-provider certification.
- Release/install documentation has historically drifted from actual optional adapter and shell-tool
  prerequisites; README and install changes must now be reviewed against R1, R2, and R10.

## 6. Traceability

- Toolchains/targets: `go.mod`, `ui/package.json`, `Makefile`, `install.sh`.
- Spec lint: `scripts/check-specs.sh`.
- CI: `.github/workflows/ci.yml`.
- Fake integration peer: `internal/runtime/testdata/fakeacp`, server integration tests.
- Generated UI guard: `.claude/hooks/guard-edit.sh`; twin-skill/spec feedback in
  `.claude/hooks/post-edit.sh`.
