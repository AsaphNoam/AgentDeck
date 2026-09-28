import { describe, expect, it } from "vitest";
import { classifyFileLink, resolveFromFile } from "./filePath";

describe("classifyFileLink", () => {
  // BR-6: a bare file name with a line suffix matches the URL-scheme grammar (`README.md:` reads
  // as a scheme) but is a file link; a Codex-style `:start-end` range cites its start line.
  it("treats a bare file name with a line suffix as a file link (BR-6)", () => {
    expect(classifyFileLink("README.md:12")).toEqual({ path: "README.md", line: 12 });
  });

  it("reads a :start-end line range as the start line (BR-6)", () => {
    expect(classifyFileLink("/repo/ui/src/app.css:49-125")).toEqual({ path: "/repo/ui/src/app.css", line: 49 });
  });

  it("keeps a real scheme out of the viewer", () => {
    expect(classifyFileLink("vscode:open")).toBeNull();
    expect(classifyFileLink("https://example.com:8080")).toBeNull();
  });
});

describe("resolveFromFile", () => {
  it("resolves against the viewed file's directory, leaving an escape for the server to refuse", () => {
    expect(resolveFromFile("docs/features/HANDOFF.md", "../archive/state/x.md")).toBe("docs/archive/state/x.md");
    expect(resolveFromFile("docs/features/HANDOFF.md", "./AGENT-WORKFLOW.md")).toBe("docs/features/AGENT-WORKFLOW.md");
    expect(resolveFromFile("README.md", "docs/a.md")).toBe("docs/a.md");
    expect(resolveFromFile("docs/a.md", "/abs/b.md")).toBe("/abs/b.md");
    expect(resolveFromFile("docs/a.md", "../../../etc/passwd")).toBe("../../etc/passwd");
  });
});
