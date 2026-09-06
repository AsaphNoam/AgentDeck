import { describe, expect, it, vi } from "vitest";
import type { PresentationColors } from "../../../presentation/resolveColors";
import { renderDiagram } from "./mermaid";

const colors: PresentationColors = {
  background: "#101820",
  surface: "#18242f",
  foreground: "#f4f7fa",
  muted: "#91a0ad",
  accent: "#4da3ff",
  selection: "#7bc4ff",
  success: "#5bc886",
  error: "#ff6b6b",
  line: "#71808d",
  fontFamily: "monospace",
};

// FS-03.R37/A22, TS-08.R40, INV §8/§17: this deliberately uses the real pinned Mermaid
// producer so a mock cannot omit the generated theme stylesheet or its local marker references.
describe("real Mermaid sanitization", () => {
  it("retains theme contrast and safe fragment references without requesting the network", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("no request expected"));
    // jsdom has no layout engine; Mermaid needs these SVG measurements to finish producing its
    // otherwise deterministic markup. The sanitizer assertions below use Mermaid's real output.
    Object.defineProperty(SVGElement.prototype, "getBBox", {
      configurable: true,
      value: () => ({ x: 0, y: 0, width: 120, height: 24 }),
    });
    Object.defineProperty(SVGElement.prototype, "getComputedTextLength", {
      configurable: true,
      value: () => 80,
    });

    const result = await renderDiagram("flowchart TD\n  A[Start] --> B[Finish]", colors, "ad-diagram-real-theme");

    expect(result).toHaveProperty("svg");
    const document = new DOMParser().parseFromString("svg" in result ? result.svg : "", "image/svg+xml");
    const theme = document.querySelector("style")?.textContent ?? "";
    expect(theme.length).toBeGreaterThan(100);
    expect(theme).toContain(colors.foreground);
    expect(theme).toContain(colors.surface);
    expect(theme).toMatch(/url\(#[^)]+\)/);
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });
});
