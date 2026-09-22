# Adopt modern Codex ACP capabilities

**State:** In progress
**Why:** Direct 2026-09-22 request to re-evaluate the ACP wishlist against `codex-acp` 1.12.0.
**Relevant requirements:** FS-01.R36, FS-03.R57–R61, FS-05.R38, TS-01.R35, TS-02.R35,
TS-03.R43–R44, TS-04.R61–R66, TS-06.R26, TS-08.R59, INV §1–§6, §8, §10–§13, §15–§17

## Outcome

AgentDeck uses current capability-negotiated ACP surfaces for Codex reasoning, native child
sessions, background commands and conversation cloning while keeping one provider-independent
Runtime contract. Tool names, load/fork pagination, MCP elicitation completion and file reporting
gain the newer adapter's fidelity without a direct Codex app-server path.

## Included work

Bump packaged Codex ACP/CLI to 1.12.0/0.154.0; rebase and hash-lock the still-required idle-steering
patch; add normalized capability/activity/task/fork contracts; make Clone a native conversation fork
with no settings-only fallback; render live-only collapsed reasoning, nested durable child activity
and stoppable background tasks; and consume canonical tool names plus bounded incomplete file
reports. Plans, provider management/recommendations, session goals, targeted child cancellation, and
changes to AgentDeck durable tasks/pipelines are excluded.

## Design direction

Experienced operators keep the root conversation as the focal reading path. Native children sit at
their causal point as restrained disclosures, while active background work is a compact tail summary
with state-first rows and targeted Stop. The hierarchy proves parent/child ownership and command
lifecycle without turning provider activity into AgentDeck cards or equal panels. Core and Sky &
Grove reuse the existing transcript renderers and semantic states; narrow dashboard panes remain
usable. No entrance motion, new tab, side panel, provider branding, or duplicate command output.

## How we will know it works

FS-01.A20, FS-03.A39–A42 and FS-05.A21 cover clone rollback/lineage, ephemeral reasoning, nested
children, targeted background stop, canonical metadata, replay de-duplication and tracking. TS-06.R26
adds the packaging matrix and a credentialed 1.12.0 receipt; the material transcript composition also
gets a focused real-browser pass in both appearances and the supported desktop/narrow-pane widths.

## Waiting on

Nothing.
