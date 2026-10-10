# Continue chats after a quota reset

**State:** Waiting to start
**Why:** Direct `/design-feature` request on 2026-10-09; global default-on, all-agent coverage and
the exact continuation notice confirmed, with API/retention approval on 2026-10-10.
**Relevant requirements:** FS-01.R40–R45/A23–A28, FS-04.R53/A33, TS-02.R45, TS-03.R60,
TS-04.R86–R88, TS-10.R39–R44; owner exceptions in FS-03/FS-14/FS-16/FS-21 §6 and TS-09/TS-14 §5;
TS-01.R19–R21/R31–R32; INV §1, §2, §3, §4, §5, §7, §8, §9, §10, §11, §12, §14, §15, §16, §17.

## Outcome

Every quota-interrupted chat shows its quota limit and known reset, and Chuck schedules the same
unfinished conversation/assignment for continuation when Auto continue is enabled (default on).
Actual continuation appends **Chuck continued this conversation when the quota reset** once.

## Included work

Normalize provider terminal evidence, persist one bounded recovery record per conversation,
extend the existing dispatcher and owner-specific continuation seams, preserve manual control and
restart safety, and wire the setting/indicator/cancellation through current desktop/phone/API
clients. The operator waived API backwards compatibility: replace the implicit Cancel body with
the explicit turn/quota target and update current callers, rather than adding legacy aliases.
No duplicate durable task, new conversation, general scheduler, permission change or inferred
result. One recovery record lasts until conversation deletion; notices use ordinary retention.

Reset availability is provider-dependent. The operator requested ACP gaps remain TBD/wishlisted:
Codex quota indication works with typed failures, but no schedule is invented without a trustworthy
reset. No new reset-forwarding adapter patch, provider-login side client or date parser is included.
The separately requested full 5-hour/weekly subscription views remain a design in `docs/ideas.md`.

## How we will know it works

FS-01.A23–A28/FS-04.A33 cover provider wire classification, known/unknown reset, same conversation
and all owned-work phases, manual/global cancellation races, overdue/crash recovery, notice
idempotence and honest packaged/installed provider evidence. Run the applicable TS-06 closure
matrix, focused real-browser desktop/phone journeys and serialized real Go API/client checks.
Fake fixtures cannot satisfy the bounded live-provider gate.

## Verified provider boundary

`scripts/release/package.json`/lock pin Claude ACP 0.85.1 and Codex ACP 2.1.1. Both expose
AIR `sessionFailure`, whose pinned error/limit/empty-actions policy distinguishes account quota
from rate-limit retry and context/session new-session policies; its wire omits the internal kind.
Chuck currently neither negotiates it (`internal/runtime/capabilities.go`) nor decodes prompt
metadata (`acpmap.go`). Claude forwards rejected-window resets in
`usage_update._meta["_claude/rateLimit"]` after assistant usage is known. Codex privately consumes
`account/rateLimits/read`/`updated`, forwarding no typed reset; `/status` returns text.
Claude `/usage` similarly queues a conversation turn and renders text, so neither command is a
passive paired-window read. Missing first-request/reset evidence remains unknown. The pinned
Claude live SDK timestamp is a bare numeric declaration: verify its epoch unit with primary
producer/provider evidence before using it for a schedule; an unverified value stays unknown.

Evidence: pinned package `dist/session-failure-extension.js`, `dist/acp-agent.js:4919` and Codex
`dist/index.js:30460` under `scripts/release/node_modules`; reviewed assembly pins/patch boundary
in `scripts/release/assemble.sh`. Official
[failure contract](https://github.com/agentclientprotocol/claude-agent-acp/blob/main/docs/session-failure-extension.md),
[Claude releases](https://github.com/agentclientprotocol/claude-agent-acp/releases),
[Codex package](https://github.com/agentclientprotocol/codex-acp/blob/main/package.json) and
[Codex releases](https://github.com/agentclientprotocol/codex-acp/releases) were checked for a
replacement surface. Reverify on adapter updates before changing the verified support set.

## Waiting on

None for quota indication/recovery using available reset evidence. Full passive subscription
snapshots and Codex structured reset forwarding remain separate ACP wishlist dependencies.
