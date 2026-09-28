# Shared creative-workspace implementation checkpoint — 2026-09-28

## Revision and environment

Implementation range: `f79fb97..ddab692` on main (two substantive commits). Final built UI includes
both commits; the build was made immediately before the second commit, so its version metadata
names the first commit plus working changes. Generated embed was committed with the second slice.

Isolated fakeACP app: `http://127.0.0.1:4528`, home
`/private/tmp/agentdeck-render-share-creative-20260928`. No real credentials or user server was
modified. Matrix-only dev view: `http://127.0.0.1:5181/__visual-matrix`. Screenshots and read-only DOM
measurements: `/private/tmp/shared-layout-evidence/`. Core restored after appearance comparisons.

## Verification completed

- Final UI suite: 482 passed, 3 existing skipped; stylelint and presentation contract pass.
- `make test`: both Go variants pass. `make dist`: final build and generated embed pass.
- Focused card/grid/nav/matrix and chat/composer/Tasks behavior checks pass, including incapable
  current model and staged capable model coverage. `git diff --check` passes.
- Actual browser viewport confirmed as 1024×900 and 1440×1000, with loaded fonts. Matched matrix
  shells/cards have identical rectangles in Core, Sky & Grove and Studio. Normal-color screenshots
  were inspected for soft depth, status salience and palette/ornament retention.
- All seven populated built destinations were captured in each appearance at both sizes:
  project dashboard, live permission receipt, completed prose/Mermaid chat, archived diff/tool chat,
  Tasks, Pipelines (empty real destination plus populated matrix timeline), and Settings.
  `routes-{core,sky-grove,studio}.json` confirms viewport, skin, loaded fonts and no page overflow.
  Independent rendered critique found no material static visual issue.
- Matrix covers remaining shared onboarding/overlay construction and navigation zero/one/five/
  overflow fixtures; real six-project overflow is keyboard reachable, focus visible, Escape closes.
  Long project labels truncate from the end while full accessible names remain.
- Expanded cards remain 640px. Long fixture header is 97.83px at 1024 and 73.91px wide; ordinary
  short header is 73.91px. Narrowest 146.66px waiting card wraps its header to 188.74px, with context,
  badge and Collapse contained. Textarea is 74px while actions remain content-sized at 36px.
  Density 1/3/6, pointer reorder, Collapse and keyboard focus were exercised; layout does not move
  with pulse animation. Focused grid tests cover stable neighboring order and persisted density.
- Tasks authoring inset is 24px, form cap 832px, subordinate signal cap 448px. Matched populated
  state fixtures and real armed/parked/interrupted rows remain readable.
- Archived tool expansion, diff, missing-file recovery and populated file viewer were inspected.
  Populated annotation tray plus file viewer was captured in all appearances at both sizes without
  page overflow (`archive-file-tray-*`). No annotation was sent. Staged OpenCode runtime selection
  removes the unsupported Session settings group; draft selection restored without Switch.
- Eighteen observations over approximately 7.2s show indicator opacity varying while label opacity
  remains 1: busy 2.4s, error/waiting 1.2s, others static. State dropdown transitions to idle/stopped
  cancel animation. Real isolated permission Deny preserves its receipt and transitions to idle.
  Reloaded live-state fixtures pulse. Timing evidence: `pulse-samples.json`.

## Approved follow-up

The operator approved both checks. Browser Send/Cancel was exercised on `a_ec51e5` in Core,
Sky & Grove and Studio with the exact prompt “Show the fixture diagram.” at confirmed 1280×720.
Each turn showed streamed prose/diagram output and Cancel, then settled with Cancel removed.
Send and Cancel each remain 36px tall. Evidence: `send-cancel-{core,sky-grove,studio}.png`.
This supplements the already captured 1024/wide geometry checks rather than claiming a new
1024 viewport. Core preference was restored.

## Required gate still open

FS-12.A31 reduced-motion check is pending, not passed. Automatic
approval review rejected a temporary `com.apple.universalaccess reduceMotion` preference write and
the exact fake-session submission, even after fakeACP source was checked. No rejected script ran.
The original Reduce Motion preference does not exist; restore by deleting the preference after an
approved temporary check. The browser adapter has no media-emulation capability.

After approval, macOS refused the direct preference write (“Could not write domain”). Computer
Use then reported the Mac locked on two attempts. The operator was asked to unlock it. Readback
still confirms the preference absent: no setting was changed or needs restoration yet. Once
unlocked, use System Settings for the approved temporary switch, inspect the actual browser media
query/static badge behavior, and restore the original off setting immediately. The exact fake
prompt was submitted through the browser only after approval.

Resume the isolated app without rebuilding unless product source changes. The archive fixture is
`a_cdbfef`; completed Mermaid fixture `a_d1706b`; denied permission fixture `a_a17655`. Rebuilding or
restarting stops live fixtures, so recreate a fake diagram session if needed. Browser tabs have the
two confirmed viewport sizes were cleaned up between turns; the new fake-check tab is 1280×720
and marked for handoff. After reduced motion passes,
reconcile planned spec markers and close the change for review. Prior A19–A23/TS-08.R68 evidence
debt, genuine finished pipeline and credentialed provider gates remain separate and unclaimed.
