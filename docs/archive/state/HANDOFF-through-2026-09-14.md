# AgentDeck — archived handoff state through 2026-09-14

Settled changelog entries moved out of [`../../features/HANDOFF.md`](../../features/HANDOFF.md) to
keep its session-start header inside budget. Nothing here is live state; the handoff carries the
resumable position.

## Changelog

**Changelog — 2026-09-14 (CI flake):** Fixed the intermittent `LaunchStep` onboarding test that
reddened CI on `7162f59`. `LaunchStep` disables Launch until both the roles and the projects query
resolve, but both tests awaited only the project option before clicking; when the roles response
landed second, the click hit a disabled button, no `POST /api/sessions` was sent, and the
`launchBody` wait timed out. A shared `clickLaunch()` helper now waits for the button to be enabled.
Delaying the roles handler by 300ms reproduced the CI failure verbatim and both tests pass under
that delay with the fix. Test-only: no product code changed, so the shipped `v0.5.0` artifact is
unaffected and no re-release is required. CI also warns that `actions/checkout@v4`,
`setup-go@v5` and `setup-node@v4` are being forced off deprecated Node 20; not yet breaking, not
addressed here.

**Changelog — 2026-09-14 (release: `v0.5.0`):** Refreshed the shipped `operating-agentdeck` package
for the range's agent-facing changes (FS-18.R4–R5, TS-11.R1/R8).
`references/build-and-run-pipelines.md` was rewritten onto the standing-orchestrator model: stages
are bounded durable assignments to one standing run orchestrator that alone reports each stage
outcome, run setup picks one backend/model for it while only dedicated stages select a
sub-orchestrator runtime, templates carry no conditional routing, accepted stage completion cancels
unfinished descendants before cleanup fences the next stage, Stop cancels nested delegated work, and
Continue/Retry/Replace are distinguished (FS-14.R61–R78). `references/coordinate-work.md` gained
durable waiting — `wait_for_tasks` holds an assignment open, yields its capacity slot, and resumes on
a watched revision change instead of polling — plus creator authority to inspect, retry, re-arm,
cancel, and replace work it created (FS-16.R30–R38). `SKILL.md` names both in its routing bullets.
No product code changed. README, `install.sh`, and `scripts/release/assemble.sh` were re-checked
against the range and none of their release-matched claims is falsified: the CLI install/update/auth
surface, the config schema version, and the Node and adapter pins are unchanged, and the in-range
`assemble.sh` edit already carries its own `+agentdeck.1` component suffix.
