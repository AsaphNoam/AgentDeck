# Studio acceptance closure — 2026-09-29

## Environment

- Working-tree development matrix at `http://127.0.0.1:5182/__visual-matrix`.
- Isolated built application at `http://127.0.0.1:4531`, backed by the deterministic fake ACP
  runtime and a temporary `AGENTDECK_HOME`; no user configuration or real provider was used.
- Browser viewports were read from the page as 1024×900 and 1280×720.
- This closing pass is read with the prior complete route/state evidence in
  `implementation-share-creative-workspace-2026-09-28.md`; the later FS-12.R52/A26 and TS-08.R74/R79
  common-geometry contract supersedes A21/R68's original Studio-only layout oracle.

## Evidence

- Core, Sky & Grove and Studio rendered the matched deterministic matrix at both viewports. The
  matrix had no page overflow; all eight long/state dashboard cards, the 640px expanded pane,
  pipeline fixture and archive/settings columns kept `scrollWidth <= clientWidth`.
- At 1024×900, the populated built Dashboard, full agent page, Tasks, Pipelines, Archive and
  Settings routes were opened in all three appearances. Every route retained its title, focusable
  controls and reading order with no page or main-content horizontal overflow.
- A real expanded Studio dashboard card rendered the fake-ACP transcript and composer at 640px;
  the long output remained inside its 309px card, the pane owned its height, and keyboard Tab moved
  from the composer to Send with a visible 2px focus outline.
- A one-stage pipeline was created and started through the production loopback API. Its standing
  owner read the assigned execution handle and reported success through the real token-bound MCP
  `report_task_result` path. The durable run reached `completed` with final outcome `success` and
  a finished stage task. Its real timeline rendered in all three appearances at both viewports,
  including the accepted summary, with no horizontal overflow.
- Wide Settings and finished-pipeline captures were converted through the system generic-gray
  profile and reviewed together. The three appearances retain their distinct ornaments while
  sharing measures, hierarchy and control geometry, as required by the superseding shared-layout
  contract. Normal-color views retain Studio's pale canvas, forest support and restrained coral.
- The working-tree Studio canvas pseudo-element reports opacity `0.1` at 1024px; the previous
  `0.18` value is now rejected by an independent CSS contract test that also pins the required dot
  size and pitch.

## Result

FS-12.A19–A23 and TS-08.R68 are complete when read with their later R52/A26 and R74/R79
supersessions. No new rendered finding was observed. The temporary fixture, run and captures were
review-only evidence and are not product artifacts.
