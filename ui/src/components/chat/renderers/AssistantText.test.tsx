import React from "react";
import fs from "node:fs";
import path from "node:path";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AssistantText } from "./AssistantText";
import { TranscriptView } from "../TranscriptView";
import { DIAGRAM_SOURCE_LIMIT, DIAGRAM_TOO_LARGE, DIAGRAM_UNRENDERABLE, DIAGRAM_UNSUPPORTED } from "./mermaid";

const mermaid = vi.hoisted(() => ({
  initialize: vi.fn(),
  parse: vi.fn(),
  render: vi.fn(),
}));

vi.mock("mermaid", () => ({ default: mermaid }));

const DIAGRAM = "graph TD;\n  Start-->Finish;";
const CLOSED = "Here it is:\n\n```mermaid\n" + DIAGRAM + "\n```\n";
const OPEN = "Here it is:\n\n```mermaid\n" + DIAGRAM;

beforeEach(() => {
  mermaid.initialize.mockReset();
  mermaid.parse.mockReset().mockResolvedValue(true);
  mermaid.render.mockReset().mockResolvedValue({ svg: '<svg id="d"><g class="node"><text>Finish</text></g></svg>' });
});

afterEach(cleanup);

function renderAssistant(text: string) {
  return render(<AssistantText event={{ kind: "assistant_text", seq: 3, text }} />);
}

// FS-03.A20
describe("assistant diagram rendering", () => {
  it("renders a closed mermaid fence as a diagram", async () => {
    const { container } = renderAssistant(CLOSED);

    await waitFor(() => expect(container.querySelector(".mermaid-diagram-figure svg")).not.toBeNull());
    expect(mermaid.render).toHaveBeenCalledWith(expect.any(String), DIAGRAM);
    expect(container.querySelector(".mermaid-diagram-figure")?.textContent).toContain("Finish");
  });

  it("keeps an open fence a code block and promotes it when the closing delta arrives", async () => {
    const { container, rerender } = renderAssistant(OPEN);

    expect(container.querySelector(".mermaid-diagram")).toBeNull();
    expect(container.textContent).toContain("Start-->Finish;");
    expect(mermaid.parse).not.toHaveBeenCalled();
    expect(mermaid.render).not.toHaveBeenCalled();

    rerender(<AssistantText event={{ kind: "assistant_text", seq: 3, text: CLOSED }} />);
    await waitFor(() => expect(container.querySelector(".mermaid-diagram-figure svg")).not.toBeNull());
  });

  it("returns the original source through the per-diagram toggle", async () => {
    const { container } = renderAssistant(CLOSED);
    await waitFor(() => expect(container.querySelector(".mermaid-diagram-figure svg")).not.toBeNull());

    fireEvent.click(screen.getByRole("button", { name: "Show source" }));
    expect(container.querySelector(".mermaid-diagram-figure")).toBeNull();
    expect(container.querySelector(".mermaid-diagram")?.textContent).toContain("Start-->Finish;");

    fireEvent.click(screen.getByRole("button", { name: "Show diagram" }));
    expect(container.querySelector(".mermaid-diagram-figure svg")).not.toBeNull();
  });

  it("leaves a mermaid fence in a user prompt, a tool result, and a diff unchanged", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <TranscriptView
          agentId="a1"
          events={[
            { kind: "user_text", seq: 1, text: CLOSED },
            { kind: "tool_result", seq: 2, content: CLOSED },
            { kind: "diff", seq: 3, path: "graph.md", old_text: "", new_text: CLOSED },
          ]}
          sourceActive
          annotationsEnabled={false}
          busy={false}
        />
      </QueryClientProvider>,
    );

    await waitFor(() => expect(container.textContent).toContain("Start-->Finish;"));
    expect(container.querySelector(".mermaid-diagram")).toBeNull();
    expect(mermaid.render).not.toHaveBeenCalled();
  });
});

// FS-03.A21
describe("assistant diagram safety", () => {
  it("strips scripts, HTML labels, handlers, and links from the rendered markup", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("no request expected"));
    mermaid.render.mockResolvedValue({
      svg:
        '<svg id="d"><script>window.__diagramScript = true;</script>' +
        '<foreignObject><div onmouseover="window.__diagramHandler = true">label</div></foreignObject>' +
        '<g onclick="window.__diagramHandler = true"><a href="javascript:window.__diagramHandler = true"><text>Finish</text></a></g>' +
        '<image href="https://example.invalid/pixel.png"></image>' +
        '<g style="fill:url(https://example.invalid/fill.svg);stroke: red"><text>Styled</text></g>' +
        '<style>@import url(https://example.invalid/a.css); #d { background: url("https://example.invalid/b.png"); }</style>' +
        "</svg>",
    });

    const { container } = renderAssistant(
      "```mermaid\ngraph TD;\n  Start-->Finish;\n  click Start \"javascript:alert(1)\"\n```\n",
    );

    await waitFor(() => expect(container.querySelector(".mermaid-diagram-figure svg")).not.toBeNull());
    const figure = container.querySelector(".mermaid-diagram-figure") as Element;
    expect(figure.querySelector("script")).toBeNull();
    expect(figure.querySelector("foreignObject")).toBeNull();
    expect(figure.querySelector("image")).toBeNull();
    expect(figure.querySelector("a")).toBeNull();
    expect(figure.innerHTML).not.toContain("onclick");
    expect(figure.innerHTML).not.toContain("onmouseover");
    expect(figure.innerHTML).not.toContain("javascript:");
    expect(figure.innerHTML).not.toContain("example.invalid/fill.svg");
    expect(figure.querySelector("style")?.textContent).not.toContain("example.invalid");
    expect(fetchSpy).not.toHaveBeenCalled();
    expect((window as unknown as Record<string, unknown>).__diagramScript).toBeUndefined();
    expect((window as unknown as Record<string, unknown>).__diagramHandler).toBeUndefined();
    fetchSpy.mockRestore();
  });

  it("refuses an external-image node before the renderer runs and makes no request", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("no request expected"));
    const { container } = renderAssistant(
      '```mermaid\nflowchart TD\n  A@{ img: "https://example.invalid/pixel.png" }\n```\n',
    );

    await screen.findByText(DIAGRAM_UNSUPPORTED);
    expect(mermaid.parse).not.toHaveBeenCalled();
    expect(mermaid.render).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(container.querySelector("img, image")).toBeNull();
    expect(container.textContent).toContain("example.invalid");
    fetchSpy.mockRestore();
  });

  it("keeps the code block and the following events when the source is over the size bound", async () => {
    const oversized = "graph TD;\n".padEnd(DIAGRAM_SOURCE_LIMIT + 1, "A");
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client}>
        <TranscriptView
          agentId="a1"
          events={[
            { kind: "assistant_text", seq: 1, text: "```mermaid\n" + oversized + "\n```\n" },
            { kind: "assistant_text", seq: 2, text: "Following event." },
          ]}
          sourceActive
          annotationsEnabled={false}
          busy={false}
        />
      </QueryClientProvider>,
    );

    await screen.findByText(DIAGRAM_TOO_LARGE);
    expect(mermaid.render).not.toHaveBeenCalled();
    expect(screen.getByText("Following event.")).toBeInTheDocument();
  });

  it("keeps the code block with a note when the source does not parse", async () => {
    mermaid.parse.mockResolvedValue(false);
    const { container } = renderAssistant("```mermaid\nnot a diagram\n```\n");

    await screen.findByText(DIAGRAM_UNRENDERABLE);
    expect(mermaid.render).not.toHaveBeenCalled();
    expect(container.textContent).toContain("not a diagram");
  });

  it("keeps the code block when the renderer itself fails", async () => {
    mermaid.initialize.mockImplementation(() => {
      throw new Error("unresolvable presentation values");
    });
    const { container } = renderAssistant(CLOSED);

    await screen.findByText(DIAGRAM_UNRENDERABLE);
    expect(container.textContent).toContain("Start-->Finish;");
    expect(container.querySelector(".mermaid-diagram-figure")).toBeNull();
  });

  it("keeps renderer-produced markup behind a single insertion seam", () => {
    const root = path.resolve(__dirname, "../../..");
    const seams: string[] = [];
    const walk = (dir: string) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(full);
        else if (/(?<!\.test)\.tsx?$/.test(entry.name) && /dangerouslySetInnerHTML|\.innerHTML\s*=/.test(fs.readFileSync(full, "utf8"))) {
          seams.push(path.relative(root, full));
        }
      }
    };
    walk(root);

    expect(seams).toEqual(["components/chat/renderers/MermaidDiagram.tsx"]);
  });
});
