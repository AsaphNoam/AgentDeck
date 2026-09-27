import { describe, expect, it } from "vitest";
import { classifyFileLink } from "./filePath";

describe("classifyFileLink", () => {
  // BR-6 reproductions, skipped until /fix. A bare file name with a line suffix matches the
  // URL-scheme grammar (`README.md:` reads as a scheme), so the link is dropped as a dead anchor;
  // a Codex-style `:start-end` line range stays part of the name, so the server reports the file
  // missing.
  it.skip("treats a bare file name with a line suffix as a file link (BR-6)", () => {
    expect(classifyFileLink("README.md:12")).toEqual({ path: "README.md", line: 12 });
  });

  it.skip("reads a :start-end line range as the start line (BR-6)", () => {
    expect(classifyFileLink("/repo/ui/src/app.css:49-125")).toEqual({ path: "/repo/ui/src/app.css", line: 49 });
  });
});
