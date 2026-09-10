import { useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";

/** Gap kept between a menu edge and the viewport edge when clamping. */
export const MENU_VIEWPORT_MARGIN = 8;

export interface MenuAnchor {
  x: number;
  y: number;
}

// A type alias, not an interface: the project augments `CSSProperties` with a
// Radix custom-property index signature, and only an alias gets the implicit
// index signature that lets this be spread into a `style` prop.
export type MenuPosition = { left: number; top: number };

/**
 * clampMenuPosition keeps a fixed-position menu fully inside the viewport.
 *
 * A pointer near the bottom or right edge would otherwise place lifecycle
 * actions past the viewport, where they cannot be reached at all (J5,
 * FS-12.A8, INV §8). The menu is shifted back rather than flipped, so its
 * item order never depends on where the pointer happened to be. A menu taller
 * or wider than the viewport pins to the leading margin and relies on its own
 * scrolling.
 */
export function clampMenuPosition(
  anchor: MenuAnchor,
  size: { width: number; height: number },
  viewport: { width: number; height: number },
  margin = MENU_VIEWPORT_MARGIN,
): MenuPosition {
  const clamp = (value: number, extent: number, available: number) =>
    Math.max(margin, Math.min(value, available - extent - margin));
  return {
    left: clamp(anchor.x, size.width, viewport.width),
    top: clamp(anchor.y, size.height, viewport.height),
  };
}

/**
 * useMenuPlacement measures a pointer-anchored menu and returns the position
 * style that keeps it on screen. The measurement runs in a layout effect, so
 * the corrected position is applied before the browser paints and the menu
 * never visibly jumps.
 */
export function useMenuPlacement(anchor: MenuAnchor | null) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [placed, setPlaced] = useState<MenuPosition | null>(null);

  useLayoutEffect(() => {
    const node = ref.current;
    if (!anchor || !node) {
      setPlaced(null);
      return;
    }
    const rect = node.getBoundingClientRect();
    setPlaced(
      clampMenuPosition(
        anchor,
        { width: rect.width, height: rect.height },
        { width: window.innerWidth, height: window.innerHeight },
      ),
    );
    // Keyed on the coordinates rather than the anchor object so a caller that
    // passes a fresh literal each render cannot drive a measure/set loop.
  }, [anchor?.x, anchor?.y]); // eslint-disable-line react-hooks/exhaustive-deps

  const style: CSSProperties | undefined =
    placed ?? (anchor ? { left: anchor.x, top: anchor.y } : undefined);
  return { ref, style };
}
