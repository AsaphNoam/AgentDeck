---
name: design-feature
description: Explicit invocation only. Run only when the user sends `/design-feature`. Collaboratively turn a fresh prompt idea or an entry from `docs/ideas.md` into planned AgentDeck feature and technical specifications plus a ready-to-implement change file; do not implement product code.
---

# Design an implementation-ready feature

Follow this workflow across as many turns as the conversation needs. Do not rush from an idea to a
technical design without confirming the product behavior with the user.

## 1. Choose and record the idea

Read `docs/features/HANDOFF.md`, inspect `git status` and the diff, then read `docs/specs/README.md`,
`docs/ideas.md`, `docs/ready-changes/README.md`, and `docs/features/AGENT-WORKFLOW.md` §11.

- If `$ARGUMENTS` names an entry in `docs/ideas.md`, use that entry.
- If `$ARGUMENTS` describes a fresh idea, use the prompt as the source.
- If `$ARGUMENTS` is empty, use the first real entry under `## New ideas`, ignoring the example
  code block. If that section is empty, ask the user for an idea.
- If a name could match more than one entry, ask which one the user means.

Move an existing selected entry to `## Ideas being defined`, preserving its details. Add a concise
entry there for a fresh idea. This is the only automatic selection rule; do not choose another idea.

## 2. Define the feature specification with the user

Read the relevant feature specifications, acceptance journeys, and enough shipped code to understand
where the idea fits. Ask focused questions about the user outcome, intended users, included and
excluded behavior, states, errors, compatibility, and observable acceptance. Prefer one small group
of related questions at a time.

Do not invent a product, security, privacy, compatibility, or data-retention decision. Explain any
important existing AgentDeck boundary that constrains the idea.

Draft the feature side first. Extend the best existing FS when the idea belongs to an existing
capability; create the next FS only for a genuinely separate capability. Follow the templates and ID
rules in `docs/specs/README.md`:

- describe only user- or API-visible behavior;
- add append-only R and A items and mark every unshipped item `(planned)`;
- give each acceptance item a realistic test, user journey, or manual check;
- keep scope exclusions, edge cases, and errors explicit;
- update the spec index and status when required.

Show the user a concise summary of the drafted feature behavior and remaining product questions.
Do not begin the technical specification until the user confirms the feature scope or explicitly
asks to proceed. The feature specification is done only when its behavior and acceptance criteria
are adequate for technical design.

## 3. Define the technical specification with the user

Read the relevant TS files, matching `INVARIANTS.md` classes, code boundaries, data shapes, external
protocols, and applicable architectural rationale. Design the smallest coherent architecture that
satisfies the confirmed feature specification.

Extend the best existing TS when possible; create the next TS only for a separate architectural
area. Add planned, append-only requirements covering the required interfaces, persistence, security,
failure handling, compatibility, rollout, and verification. Keep implementation sequencing out of
the specification.

When the design has a non-trivial tradeoff, present the viable options, consequences, and a
recommendation, then wait for the user's decision. Always ask before choosing a security, privacy,
data-retention, compatibility, destructive-migration, or externally visible protocol boundary. Do
not interrupt the user for routine, reversible implementation mechanics.

## 4. Make the feature ready to implement

When both specification sides are complete:

1. Check that FS, TS, and invariant references agree; all unshipped requirements remain `(planned)`.
2. Create `docs/ready-changes/<descriptive-name>.md` using its template. Include the outcome, scope,
   acceptance evidence, exact FS/TS/INV references, and `Waiting to start` state.
3. Remove the source entry from `docs/ideas.md`; preserve its origin and useful context in the ready
   change file.
4. Do not make it active in `HANDOFF.md` and do not write product code.
5. Run `make check-specs`, the twin-skill comparison, and `git diff --check`.
6. Update the handoff changelog and add the exact plain-language final update to `BRIEFS.md`.

If a material product or technical decision remains unresolved, do not create a ready change. Keep
the entry under `Ideas being defined`, record what is needed, and tell the user plainly.
