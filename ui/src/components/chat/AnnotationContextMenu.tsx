import { useEffect } from "react";
import { createPortal } from "react-dom";

export type AnnotationMenuState = { x: number; y: number; label: string; annotate: () => void };

export function AnnotationContextMenu({ menu, onClose }: { menu: AnnotationMenuState | null; onClose: () => void }) {
  useEffect(() => {
    if (!menu) return;
    const onPointerDown = (event: MouseEvent) => {
      if (!(event.target as HTMLElement)?.closest(".context-menu")) onClose();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("mousedown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [menu, onClose]);

  if (!menu) return null;

  return createPortal(
    <div className="context-menu" data-ui="context-menu" style={{ left: menu.x, top: menu.y }} role="menu">
      <button
        type="button"
        data-slot="item"
        onClick={() => {
          menu.annotate();
          onClose();
        }}
      >
        {menu.label}
      </button>
    </div>,
    document.body,
  );
}
