# Per-chat runtime picker (backend/model/effort)

**State:** Waiting to start
**Why:** Human request (design-feature, 2026-07-30), from the `docs/ideas.md` "Per-chat model
picker" idea. The chat header shows the runtime identity as static text; the only way to change a
running agent's runtime is the dashboard card's right-click Switch dialog. This adds an in-context
picker so the runtime can be changed without leaving the chat.
**Relevant requirements:** FS-03.R1, FS-03.R23, FS-03.R24, FS-03.A9; FS-01.R13, R15, R26, R30, R32;
FS-09.R37; INV §1, §2, §8, §13.

## Outcome

In an agent's chat view, a person can change a running agent's backend, model, and effort directly
from the header and apply it with one explicit Switch action — instead of navigating back to the
dashboard grid and using the card's right-click Switch dialog. History is preserved by the existing
switch-runtime path (native resume or primer).

## Included work

- Replace the static `backend · model · effort` header text (`ui/src/components/chat/ChatPanel.tsx`)
  with inline backend/model/effort selects for a **running** chat agent, populated from the backend
  catalog (`useBackends`).
- Reset logic identical to the dashboard Switch dialog and New Agent modal: choosing a backend resets
  model to the backend default and effort to the model default; the effort select shows only for a
  model that declares efforts (FS-09.R37). **Reuse a shared helper rather than copying the reset
  logic a third time** (INV §2) — factor the backend→model→effort reset used by
  `CardContextMenu.tsx` and `NewAgentModal.tsx` into one place and call it here.
- A selection that differs from the agent's current runtime reveals an explicit **Switch** control
  that calls the existing `switchRuntime` client (`ui/src/api/client.ts`) with
  `{backend, model, effort}` (interface unchanged). No change applies until Switch is activated.
- Surface `no_change`/rejected/rolled-back switch errors (FS-01.R26) as an actionable message and
  restore the picker to the agent's current runtime (INV §8). After a successful switch, re-seed the
  picker from the updated agent state (INV §1 — republish derived state across the switch boundary).
- When the agent is not running (stopped/archived under FS-05), the header stays static text with no
  picker or Switch control. If the current backend/model is absent from the live catalog, the picker
  shows the current identity without breaking and keeps Switch gated on a listed backend/model.
- Any new header classNames get defined selectors (INV §13); reuse existing `.form-field`/select
  styling where possible.

### Not included

- No interface (chat↔terminal) switching from the header — that stays in the dashboard Switch dialog
  (FS-01.R15/R32), because switching to terminal replaces the whole chat panel.
- No new HTTP endpoint, request field, persistence, or migration: switch-runtime already accepts
  backend/model/effort and performs native-resume/primer + rollback (FS-01.R30, shipped). **No
  technical-spec change is required** — this is a UI surface over existing architecture.
- No apply-on-change behavior; the switch is always an explicit, confirmed action.

## How we will know it works

- FS-03.A9: a new chat-header component test under `ui/src/components/chat/` proving, for a running
  fake agent, that the header renders backend/model/effort selects; choosing a backend resets model
  and effort to defaults; the effort select appears only for an effort-capable model; a differing
  selection reveals **Switch** and calls `POST /api/sessions/{id}/switch-runtime` with the chosen
  runtime; and a stopped agent renders static identity with no picker.
- A component test for the error path: a rejected/`no_change` switch surfaces an error and restores
  the picker to the current runtime.
- Standard checks: `make test`, `cd ui && npm test && npm run build`, `make build`, `make check-specs`.
- Manual/browser: on a running agent, change the model in the header, click Switch, and confirm the
  transcript continues and the header reflects the new runtime; confirm a stopped agent shows no
  picker.

## Waiting on

Nothing — product scope and all three product decisions (inline header dropdowns; backend/model/
effort only; explicit Switch confirmation) are confirmed. Ready to start.
