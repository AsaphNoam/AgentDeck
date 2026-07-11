# Usability review run — 2026-07-11 (Phase 7 configuration-source linking)

This focused `/usability-review` exercised the new Configuration source panel in
the running local dashboard. It used an isolated temporary AgentDeck home and a
synthetic Claude Code root with a non-secret fixture model plus an environment
key/value pair; no user configuration or credential was read.

## Journey: link a native Claude configuration

1. Open **Settings → Backends → Claude → Configuration source**.
2. Select the seeded `my-app` project and choose **Discover native config**.
3. Verify the preview shows the inherited model and the `ANTHROPIC_API_KEY`
   *name* but not its value.
4. Choose **Link (Mirrored — compatibility)** and inspect both the resulting
   panel and `GET /api/config-sources?project=my-app`.

### Passes

- The panel is discoverable, styled, and explains that native configuration is
  read rather than copied.
- Preview and the post-link Effective view show provenance, the configured
  model, and environment-key names without exposing the fixture secret value.

### BLOCKING — Usability: Mirrored selection silently becomes Linked

**Observed in the running dashboard.** Choosing **Link (Mirrored —
compatibility)** immediately returned a bound state labelled `linked`; the GET
response persisted `"mode":"linked"`, with no mirror cache. A user explicitly
choosing the compatibility ownership mode receives a different, silent
ownership contract. This confirms the open federation-review finding of the
same name. Reproduce with the journey above. Fix by issuing a new Mirrored
preview before bind (or otherwise binding an approved Mirrored token), then
assert the panel and GET response both say `mirrored`.

### BLOCKING — Usability: a bound source has no repair path

**Observed in the running dashboard.** After binding, the only source actions
were **Refresh** and **Unlink**. There is no visible way to set an override,
reset an override to inherited, detach with an explanation/confirmation, or
repair an invalid/stale source before launching. Before binding, the detached
option is disabled as “unavailable.” This confirms the existing federation
repair-controls finding; normal users can only unlink or encounter a late
launch failure.

### Coverage limit

The onboarding wizard was already satisfied by the isolated fixture's backend
status, so its alternate-backend choice could not be browser-driven without
changing product state. The existing BLOCKING finding that the source step
hard-codes Claude remains open and needs a post-fix browser journey covering
Codex and non-federated selections.
