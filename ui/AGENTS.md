# AgentDeck UI presentation rules

Read `docs/specs/features/FS-12-application-interface.md` and
`docs/specs/tech/TS-08-frontend-presentation.md` before changing presentation code.

## Dependency direction

Presentation is a leaf. Feature components own state, data, mutations, routing, Radix behavior,
drag behavior, and terminal lifecycle. CSS, visual primitives, and decorative assets may receive
those values; they must not fetch, persist, derive product meaning, or alter event behavior.

Use the cascade in `src/styles/index.css`: reset → tokens → base → components → features →
integrations → skins. Core remains complete without a skin marker. The only production skin is a
statically imported file declared by `src/presentation/contract.json`; its rules stay inside
`ad-skins` and are scoped to the matching root `data-skin` id or the Settings appearance preview.
Do not add another id, skin file, provider, browser-storage preference, dynamic loader, external
asset, or arbitrary skin code without first updating FS-12, TS-08, and the versioned contract.

## Choosing a value

1. Reuse a semantic `--ad-*` token from `styles/tokens.css`.
2. If the value is a reusable semantic role, add one core raw value and one semantic alias, then
   update `src/presentation/contract.json` when skins may override the alias.
3. Keep component-local calculations close to the owning selector and derive them from tokens.
4. Do not add raw colors, font families, shadows, radii, or spacing to feature CSS or TSX.

## Stable hooks and exceptions

Public presentation hooks use product-native `data-ui`, `data-slot`, `data-state`, and
`data-variant` names from `src/presentation/contract.json`. Do not treat implementation class names
as public hooks, and do not add an undocumented hook. Product text, DOM order, actions, and state
meaning never come from CSS.

Dynamic feature values may use inline styles only when an exact, justified entry exists in
`presentation-exceptions.json`. The checker rejects both new unlisted styles and stale exceptions.
Do not add inline disable comments or broad file/rule suppressions.

Skin raw palette tokens use the private `--ad-<skin-id>-*` namespace and may be defined and used
only in that skin's declared stylesheet. Active skin rules may override only public semantic tokens
from the contract or approved public hooks. Core token definitions stay in `styles/tokens.css`;
never move them into a skin or make Core depend on `[data-skin]`.

## Required checks

Run `npm run check:styles` for presentation edits. `npm test` and `npm run build` run it
automatically. A visual change also needs the deterministic development matrix and a real-browser
review of the affected surfaces. Never edit `internal/server/ui/dist`; regenerate it with
`make embed`.
