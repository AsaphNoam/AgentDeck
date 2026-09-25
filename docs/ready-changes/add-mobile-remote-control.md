# Add mobile remote control

**State:** Waiting to start
**Why:** 2026-09-25 `/design-feature` request: keep orchestrating from a phone while the Mac keeps
working — a remote control like Claude Code Remote Control or Codex in the ChatGPT app, using
Nodeterm's mobile companion (https://www.nodeterm.dev/docs/remote/mobile-companion/) as a reference
model rather than a specification.
**Relevant requirements:** FS-20.R1–R29, FS-00.R18, TS-13.R1–R14, TS-02.R37, TS-03.R46, TS-05.R23,
TS-06.R27, TS-08.R73, INV §2, §4, §5, §8, §10, §14, §15, §16

## Outcome

A person pairs an Android phone or iPhone with their Mac, then — anywhere their phone has Tailscale —
sees what needs them, answers permissions and questions with full context, continues and steers
conversations, redirects tasks and pipeline runs, starts new work against configured projects, and
gets attention-only push notifications. The Mac stays the only runtime and authority; AgentDeck adds
no cloud service or account.

## Included work

Settled decisions (user, 2026-09-25): Tailscale instead of a built relay, embedded as a `tsnet` node
(accepting the measured ~15–22 MiB binary growth and a Go ≥ 1.26.6 toolchain bump); AgentDeck QR
pairing on top of tailnet identity; an installable web app served by the Mac rather than an Expo or
native app; an allowlist of existing `/api` routes rather than a dedicated phone API; a hashed,
sliding, node-bound `HttpOnly` cookie credential; approvals only inside the app; optional
keep-awake; pairings that do not expire and no phone limit.

Included: the Remote settings section, embedded node lifecycle, tailnet listener guard and
allowlist, pairing/revocation, the phone web app (Home, decision cards, conversations, task/run
actions, New work), Web Push, and keep-awake. Excluded: any relay or public exposure, native apps,
biometric app lock, terminal on the phone, and every desktop-only surface in FS-20.R16.

Evidence already gathered is recorded in TS-13 §1 (tsnet v1.102.5 source lines, toolchain and size
measurements, Apple's separate Home Screen storage). The Go Web Push library is an implementation
choice; verify its `aes128gcm` and Apple push-service behavior before adopting it.

Sequencing note: if `rename-product-to-deckhand` ships first, the node hostname, cookie-facing copy,
and the **Ask AgentDecker** label follow the renamed product and resident-operator role (FS-00.R16,
FS-18.R14).

## How we will know it works

FS-20.A1–A7 through server tests with a fake listener and fake `WhoIs`, UI tests, and a fakeACP
browser pass at a 390px phone viewport; FS-20.A1, A5, A6, and A8 manual gates on a real tailnet with
a real Android phone and a real iPhone; the TS-13.R5 route-inventory allowlist test; the closure
matrix in AGENT-WORKFLOW §2, including both Go test variants and focused `-race` coverage of pairing
claims, device revocation, and node enable/disable generations.

## Waiting on

Nothing.
