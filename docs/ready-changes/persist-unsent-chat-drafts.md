# Persist unsent chat drafts

**State:** Waiting to start
**Why:** The direct request to preserve text typed in a chat across navigation and refresh.
**Relevant requirements:** FS-03.R36/A19, TS-01.R17, INV §1

## Outcome

Each chat restores its unsent text in the same browser after navigation, refresh, or reopening
AgentDeck, without sending or syncing that text.

## Included work

Add one browser-local draft store used directly by the existing composer. Keep the 20 most recently
edited per-agent drafts without time expiry; clear on accepted send, manual emptying, or the existing
agent-deletion event. Browser-storage failure leaves the live composer usable. No server endpoint,
database/config field, migration, background cleanup, timer, or cross-device sync is included.

## How we will know it works

FS-03.A19 component/store/deletion-event tests cover per-chat restore, navigation/reload persistence,
accepted and rejected sends, manual clearing, twenty-entry pruning, malformed/unavailable storage,
and deletion cleanup; a manual browser check covers navigation and reload.

## Waiting on

Nothing.
