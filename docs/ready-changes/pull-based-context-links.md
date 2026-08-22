# Pull-based context links

**State:** Waiting to start
**Why:** Direct `/design-feature Pull-based context links` request confirmed on 2026-08-22. The
human approved target-neutral canonical references, separate grants/attachments/personal state, and
durable participant-based work authorization semantics for the follow-on orchestration features.
All 2026-08-22 design review findings are incorporated. The human deliberately chose no owner
management surface or automatic grant expiry for the current local, single-user product.
**Relevant requirements:** FS-15.R1–R17, FS-15.A1–A7, TS-01.R18/R22–R23, TS-02.R24,
TS-04.R28, TS-05.R16; existing boundaries FS-00.R13–R15, FS-06.R24–R27, FS-11.R10,
TS-09.R8–R9/R14, INV §1, INV §2, INV §5, INV §7, INV §8, INV §9, INV §11, INV §12,
INV §13, INV §15.

## Outcome

Agents can give immutable transcript spans and accepted pipeline-attempt reports stable reusable
reference ids, grant another chat agent access without copying source content, discover ad-hoc direct
shares, and retrieve only the chosen context through bounded token-authorized reads. References,
grant presentation, personal visibility, and future work attachments remain distinct, and no context
operation starts a model turn.

## Included work

Add the in-process `internal/contextref` service; a forward-only migration for canonical references,
direct grants, and personal hidden preferences; deterministic paged transcript/report renderers;
and the five token-scoped MCP tools in TS-04.R28. Reuse the existing address-matching helper,
transcript and pipeline authorities, MCP session identity, and owner-only state store while adding
one shared Go normalized-event projection, oversized-record diagnostics, and the context-specific
durable chat-recipient query required by TS-01.R22. Cover canonicalization,
restart durability, authorization, lifecycle/tombstones, bounds, atomicity, and absence of
mail/activation/prompt/transcript/SSE side effects.

Excluded: work/task objects, dependencies, reassignment machinery, host-managed waiting, assignment
APIs, workflow graphs/DSLs, dashboard or REST context management, grant-expiry timers, MCP resource
registration, generic blob or artifact storage, authored summaries, pipeline named values,
tracked/search projections, and workspace/project-resource files. The next feature designs attach
these references rather than expanding this change into orchestration.

## How we will know it works

FS-15.A1–A7: state/service canonicalization and exhaustive normalized-event projection tests;
token-bound real-MCP and fake-ACP integration; immutable pipeline-report and post-quiescence
coverage; bounded cursor traversal including unknown/oversized markers; adversarial identity and
authorization tests; stop/archive/delete/tombstone behavior; and provider-prompt, mailbox,
activation, transcript, SSE, log, and mail-budget non-mutation assertions. Shared checks per TS-06
and workflow §2.

## Waiting on

Nothing.
