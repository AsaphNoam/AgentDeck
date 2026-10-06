import React, { useState } from "react";
import fs from "node:fs";
import path from "node:path";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AutoGrowTextarea } from "./AutoGrowTextarea";

afterEach(cleanup);

// jsdom does no layout, so content height is simulated as 20px per line.
function measureLines(el: HTMLTextAreaElement) {
  Object.defineProperty(el, "scrollHeight", { configurable: true, get: () => 20 * Math.max(1, el.value.split("\n").length) });
}

function Controlled({ maxHeight }: { maxHeight?: string }) {
  const [value, setValue] = useState("");
  return <AutoGrowTextarea aria-label="field" maxHeight={maxHeight} value={value} onChange={(event) => setValue(event.target.value)} />;
}

describe("AutoGrowTextarea (FS-02.A48)", () => {
  it("grows to show every line and shrinks back when lines are deleted", () => {
    render(<Controlled />);
    const field = screen.getByLabelText("field") as HTMLTextAreaElement;
    measureLines(field);

    fireEvent.change(field, { target: { value: Array.from({ length: 10 }, (_, i) => `line ${i}`).join("\n") } });
    expect(field.style.height).toBe("200px");
    expect(field.style.overflowY).toBe("hidden");

    fireEvent.change(field, { target: { value: "one" } });
    expect(field.style.height).toBe("20px");
  });

  it("caps growth at the given max height and scrolls beyond it", () => {
    render(<Controlled maxHeight="40vh" />);
    const field = screen.getByLabelText("field") as HTMLTextAreaElement;
    expect(field.style.maxHeight).toBe("40vh");
    expect(field.style.overflowY).toBe("auto");
  });

  it("refits uncontrolled fields on input and forwards the element ref", () => {
    const ref = React.createRef<HTMLTextAreaElement>();
    let inputs = 0;
    render(<AutoGrowTextarea aria-label="field" ref={ref} onInput={() => { inputs += 1; }} />);
    const field = screen.getByLabelText("field") as HTMLTextAreaElement;
    expect(ref.current).toBe(field);
    measureLines(field);

    field.value = "a\nb\nc";
    fireEvent.input(field);
    expect(field.style.height).toBe("60px");
    expect(inputs).toBe(1);
  });

  it("leaves an unmeasurable (hidden) field at its natural height", () => {
    render(<AutoGrowTextarea aria-label="field" defaultValue="text" />);
    expect((screen.getByLabelText("field") as HTMLTextAreaElement).style.height).toBe("auto");
  });
});

// Every multi-line field in ui/src goes through the shared component (TS-08.R88, INV §2).
it("is the only place ui/src renders a raw textarea", () => {
  const src = path.resolve(__dirname, "../..");
  const offenders: string[] = [];
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (/\.tsx$/.test(entry.name) && !/\.test\.tsx$|AutoGrowTextarea\.tsx$/.test(entry.name) && /<textarea\b/.test(fs.readFileSync(full, "utf8"))) {
        offenders.push(path.relative(src, full));
      }
    }
  };
  walk(src);
  expect(offenders).toEqual([]);
});
