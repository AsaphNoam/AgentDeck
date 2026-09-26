import { useEffect } from "react";
import { createPortal } from "react-dom";
import { useMenuPlacement } from "../../lib/menuPlacement";

export type PointerMenuState = {
  x: number;
  y: number;
  actions: Array<{ label: string; select: () => void }>;
};

export function PointerContextMenu({ menu, onClose }: { menu: PointerMenuState | null; onClose: () => void }) {
  const placement = useMenuPlacement(menu);

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
    <div className="context-menu" data-ui="context-menu" ref={placement.ref} style={placement.style} role="menu">
      {menu.actions.map((action) => (
        <button
          key={action.label}
          type="button"
          data-slot="item"
          onClick={() => {
            action.select();
            onClose();
          }}
        >
          {action.label}
        </button>
      ))}
    </div>,
    document.body,
  );
}
