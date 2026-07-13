# AgentDeck implementation queue

This directory is the dedicated home for **specified features that are ready to implement but have
not started yet**. It is a delivery queue, not a product specification: the linked FS/TS requirements
and acceptance criteria remain authoritative.

Each ready item has one small work package: `W-<number>-<slug>.md`. It exists only after the feature
has an adequate FS/TS delta and the user has made clear that implementation is wanted. A package
moves through these states:

```text
Ready → Active → Shipped | Paused | Retired
```

- **Ready** — eligible for implementation; sits in this directory and is not in the handoff.
- **Active** — one agent has picked it up. `HANDOFF.md` points to the package and records only the
  current checkpoint/resumption state.
- **Shipped** — remove the package; Git history and the Current FS/TS record the result.
- **Paused** — retain the package with the blocker or decision needed.
- **Retired** — remove it after recording the reason in the product backlog or governing spec.

## Work-package shape

```md
# W-<number> — <short title>

**Status:** Ready | Active | Paused
**Source:** I<n> | B<n> | G<n> from `docs/product-backlog.md` (or direct human request)
**Governing contracts:** FS-nn.Rk, TS-nn.Rk, INV §n

## Intended outcome
<one user-visible outcome>

## Scope
<included and explicitly excluded work>

## Acceptance
<linked A-items, tests, journeys, or manual gates>

## Dependencies / decisions
<only items that block implementation>
```

Do not turn a package into a duplicate spec or a detailed implementation plan. A large active item
may still use a disposable `docs/plans/<change>.md` for sequencing.

## Queue

No ready work packages.
