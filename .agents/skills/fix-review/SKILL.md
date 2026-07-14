---
name: fix-review
description: Validate the open AgentDeck review findings, dismiss false positives, and fix real findings with regression tests and required checks. Use for "/fix-review", "fix the review findings", "address the review", or "validate and fix the findings".
---

# Fix review findings

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), the
[`spec overview`](../../../docs/specs/README.md), relevant FS/TS/INV items, and workflow
§§2–6, §8, and §10 completely, then follow the fix process.

`$ARGUMENTS` may scope the run to a finding priority or keyword; otherwise handle every open finding,
starting with **Must fix**. Update the relevant specification when a fix changes behavior or fills
missing coverage. Close with the handoff update, commit, and exact human update.
