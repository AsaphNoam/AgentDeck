import { useEffect, useRef, useState } from "react";
import { CodeBlock } from "./CodeBlock";
import { renderDiagram } from "./mermaid";
import { observePresentationColors, resolvePresentationColors } from "../../../presentation/resolveColors";

let diagramSeq = 0;

/**
 * Renders one closed ```mermaid fence from an assistant message (FS-03.R37). Until the diagram is
 * available — and whenever it cannot be produced — the ordinary code block stays visible, so the
 * message never empties and the rest of the transcript keeps rendering (FS-03.R38).
 */
export function MermaidDiagram({ source }: { source: string }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [markup, setMarkup] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [showSource, setShowSource] = useState(false);
  const [appearance, setAppearance] = useState(0);

  // An already-rendered diagram carries resolved colors, so a skin change regenerates it through
  // the existing presentation observer rather than a second theme signal (TS-08.R40).
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return undefined;
    return observePresentationColors(host, () => setAppearance((generation) => generation + 1));
  }, []);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return undefined;
    let current = true;
    diagramSeq += 1;
    void renderDiagram(source, resolvePresentationColors(host), `ad-diagram-${diagramSeq}`).then((result) => {
      if (!current) return;
      setMarkup("svg" in result ? result.svg : null);
      setNote("note" in result ? result.note : null);
    });
    return () => {
      current = false;
    };
  }, [source, appearance]);

  const diagramVisible = markup !== null && !showSource;
  return (
    <div className="mermaid-diagram" ref={hostRef}>
      {diagramVisible ? (
        // The one reviewed seam for renderer-produced markup: sanitized in `renderDiagram`
        // immediately before it reaches the DOM (FS-03.R38, TS-08.R40).
        <div className="mermaid-diagram-figure" dangerouslySetInnerHTML={{ __html: markup }} />
      ) : (
        <CodeBlock language="mermaid">{source}</CodeBlock>
      )}
      {note ? <p className="mermaid-diagram-note">{note}</p> : null}
      {markup === null ? null : (
        <button
          type="button"
          className="ad-button ad-button-ghost ad-button-small"
          onClick={() => setShowSource((visible) => !visible)}
        >
          {showSource ? "Show diagram" : "Show source"}
        </button>
      )}
    </div>
  );
}
