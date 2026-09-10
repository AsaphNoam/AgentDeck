import { describe, it, expect } from "vitest";
import { clampMenuPosition, MENU_VIEWPORT_MARGIN } from "./menuPlacement";

// J5 (FS-12.A8, INV §8): a pointer-anchored menu opened on a lower-row card
// placed its lifecycle actions past the bottom of the supported desktop floor,
// making Resume and Archive unreachable. The geometry below is the reproduced
// case: a 290px menu opened at y=640 in a 1280x720 viewport.
describe("clampMenuPosition", () => {
  const viewport = { width: 1280, height: 720 };
  const size = { width: 210, height: 290 };

  it("pulls a lower-row menu fully back into the viewport", () => {
    const { top, left } = clampMenuPosition({ x: 400, y: 640 }, size, viewport);
    expect(top + size.height).toBeLessThanOrEqual(viewport.height);
    expect(top).toBe(viewport.height - size.height - MENU_VIEWPORT_MARGIN);
    expect(left).toBe(400);
  });

  it("pulls a right-edge menu back without changing its vertical anchor", () => {
    const { top, left } = clampMenuPosition({ x: 1270, y: 100 }, size, viewport);
    expect(left + size.width).toBeLessThanOrEqual(viewport.width);
    expect(top).toBe(100);
  });

  it("leaves a menu that already fits at the pointer", () => {
    expect(clampMenuPosition({ x: 120, y: 80 }, size, viewport)).toEqual({ left: 120, top: 80 });
  });

  it("keeps a menu taller than the viewport pinned to the leading margin", () => {
    // Below this floor the menu scrolls (`.context-menu` max-block-size); it
    // must never be pushed off the top edge, where its first items would go
    // out of reach instead of its last ones.
    const tall = { width: 210, height: 900 };
    expect(clampMenuPosition({ x: 40, y: 600 }, tall, viewport).top).toBe(MENU_VIEWPORT_MARGIN);
  });

  it("clamps every menu height across the lower rows of the desktop floor", () => {
    for (const height of [120, 200, 290, 380, 460]) {
      for (const y of [480, 560, 640, 700, 719]) {
        const { top } = clampMenuPosition({ x: 200, y }, { width: 210, height }, viewport);
        expect(top).toBeGreaterThanOrEqual(MENU_VIEWPORT_MARGIN);
        expect(top + height).toBeLessThanOrEqual(viewport.height);
      }
    }
  });
});
