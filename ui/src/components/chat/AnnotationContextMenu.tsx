import { PointerContextMenu } from "../ui/PointerContextMenu";

export type AnnotationMenuState = { x: number; y: number; label: string; annotate: () => void; copy?: () => void };

export function AnnotationContextMenu({ menu, onClose }: { menu: AnnotationMenuState | null; onClose: () => void }) {
  return (
    <PointerContextMenu
      menu={menu ? {
        x: menu.x,
        y: menu.y,
        actions: [
          ...(menu.copy ? [{ label: "Copy selection", select: menu.copy }] : []),
          { label: menu.label, select: menu.annotate },
        ],
      } : null}
      onClose={onClose}
    />
  );
}
