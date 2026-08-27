# Render Mermaid diagrams in the chat transcript

**State:** Waiting to start
**Why:** Direct human request on 2026-08-27 — "add support for mermaid chart display in the chat window."
**Relevant requirements:** FS-03.R37, FS-03.R38, FS-03.A20–A22, FS-03.R2, FS-03.R20, TS-08.R40, TS-08.R10, TS-08.R13, TS-08.R14, TS-08.R17–R20, INV §8, INV §13

## Outcome

An agent that answers with a Mermaid diagram is read as a diagram instead of as diagram source. The
diagram matches the surrounding interface under Core and Sky & Grove, can be turned back to its
source, and behaves the same on a live stream, after a reload, and in an archived transcript.

## Included work

Included: rendering a closed ```mermaid fence inside an **assistant** message as a diagram; keeping
it a syntax-highlighted code block until the fence closes, because a partially streamed diagram is
not valid source; a per-diagram source toggle; a code-block fallback with a short note when the
source does not parse or exceeds the render budget; a theme adapter that feeds the core `--ad-*`
values into the library beside the existing `syntaxTheme` and `xtermTheme`; and one sanitizing
injection seam.

Not included: user messages, tool results, diffs, and annotations, which do not render Markdown
today; authoring, editing, exporting, or downloading diagrams; interactive diagram features (click
handlers stay disabled); any change to durable events, sequencing, fold boundaries, the transcript
endpoint, the archive, or the search index; and a Content-Security-Policy, which is recorded
separately in `docs/ideas.md` as a server-wide concern.

## How we will know it works

- **FS-03.A20** — new `ui/src/components/chat/renderers/AssistantText.test.tsx`: closed fence
  renders a diagram, open fence renders a code block, the closing delta promotes it, the source
  toggle returns the original text, and non-assistant surfaces are unchanged. Plus a
  `transcriptStore` replay case proving identical output from identical durable events.
- **FS-03.A21** — injection cases in the same test (script element, HTML label, event-handler
  attribute, click directive) render none of them and execute nothing; unparseable source leaves a
  visible code block and the following events still render; a repository check asserts the single
  injection seam.
- **FS-03.A22 / J3** — a real browser renders a diagram in a live streamed reply and in the archived
  transcript, correct under both built-in skins, with the script/HTML-label case confirmed on the
  page.
- `npm run check:styles` (Stylelint plus the presentation-contract audit) stays green, with the
  injection seam recorded in `ui/presentation-exceptions.json` with its path, rule, and reason.
- `npm run build` shows the library in its own chunk, leaving the initial bundle unchanged.

## Verified evidence behind the design

Checked against the real package and the current code, not from memory:

- `AssistantText.tsx` is the only Markdown renderer in the app, and its `code` override already
  branches on the fence's language tag — the diagram case is one more branch in one file.
- Every assistant delta rewrites the folded event's full text and re-renders the whole Markdown
  tree, with no memoization anywhere in the chat path. This is why R37 gates rendering on the closed
  fence rather than re-rendering a diagram per token.
- Mermaid 11.17.2 is ESM-only, entry `dist/mermaid.core.mjs`, 23 dependencies (d3, cytoscape, katex,
  roughjs, marked, dompurify), full minified build 3.5 MB against a current 1.8 MB app bundle —
  hence the dynamic import and separate chunk in TS-08.R40.
- Its real API supplies both primitives the design needs: `parse(text, {suppressErrors: true})` for
  the validity probe and `render(id, def)` returning `{svg, bindFunctions}` — an SVG **string**,
  which is why an injection seam exists at all.
- `securityLevel` accepts `'strict' | 'loose' | 'antiscript' | 'sandbox'`, with `strict` the default.
- `dangerouslySetInnerHTML` currently appears nowhere in `ui/src`; the seam in R38 is the first and
  is required to stay the only one.
- The presentation-contract audit is a static TypeScript/PostCSS check over `ui/src`, so it cannot
  see markup the library generates at runtime. TS-08.R40 therefore makes the sanitizing seam the
  control rather than the audit.

Security approach follows established practice rather than a new boundary. `strict` alone is not
sufficient: CVE-2026-41149 (`classDef` HTML injection in state diagrams) escaped the SVG despite
strict, and its advisory says strict does not mitigate it — fixed in 11.15.0, hence the pinned
floor. Sandbox mode alone is not sufficient either: GitLab shipped sandboxed Mermaid and still took
CVE-2026-0752, an escape-sequence bypass, and sandbox costs theming and parse-error handling. The
converged practice — Snyk Labs' diagram-renderer research and Open WebUI's own XSS fix, which
wrapped parser output in `DOMPurify.sanitize()` — is strict plus a sanitizer pass over the rendered
markup. Mermaid already bundles DOMPurify, so the second layer adds no dependency. The threat model
is real for AgentDeck specifically: this renders agent output, which is prompt-injectable from
repository content, and DeepChat took an XSS-to-RCE through exactly that path.

## Waiting on

Nothing.
