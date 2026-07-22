# Repair onboarding credentials and defaults

**State:** Waiting to start
**Why:** The onboarding wizard cannot be skipped, exposes obsolete model strings, and does not
recognize native Claude/Codex sign-in; the installed 0.1.0 command also predates `agentdeck auth`.
**Relevant requirements:** FS-04.R32–R34, FS-04.A12–A14, FS-09.R33–R34, FS-09.A12–A13,
TS-03.R15, TS-04.R15–R16, TS-05.R3/R7/R10–R11, TS-06.R22, INV §2/§3/§8/§13/§14

## Outcome

People can set up AgentDeck later, configure a provider without typing model strings into onboarding,
and follow provider-owned Claude/Codex sign-in guidance before checking readiness. Fresh installs use
current defaults while existing backend catalogs stay untouched.

## Included work

Explicit full-wizard skip; provider guidance plus Check again; Codex native-login readiness and
API-key fallback; fresh-only `sonnet`/`gpt-5.6-sol` defaults; shared CLI/probe provider metadata;
and private-release Codex CLI verification. No embedded login terminal, server-started login process,
credential transport, new auth API route, or migration of existing backend defaults.

## How we will know it works

FS-04.A12–A14, FS-09.A12–A13; source/release command-tree and private-runtime checks; fake-provider
credential tests; onboarding UI tests; and J2.

## Waiting on

Nothing.
