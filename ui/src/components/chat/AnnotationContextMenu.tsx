import { PointerContextMenu } from "../ui/PointerContextMenu";
import { webLinkActions } from "../../lib/linkActions";
import { useUiStore } from "../../store/uiStore";

// link is a web link under the pointer, claimed with claimWebLink; its actions lead
// the menu so a right-click on a link keeps them beside annotation (FS-03.R78).
export type AnnotationMenuState = { x: number; y: number; label: string; annotate: () => void; copy?: () => void; link?: string };

export function AnnotationContextMenu({ menu, onClose }: { menu: AnnotationMenuState | null; onClose: () => void }) {
  const pushError = useUiStore((state) => state.pushError);
  return (
    <PointerContextMenu
      menu={menu ? {
        x: menu.x,
        y: menu.y,
        actions: [
          ...(menu.link ? webLinkActions(menu.link, pushError) : []),
          ...(menu.copy ? [{ label: "Copy selection", select: menu.copy }] : []),
          { label: menu.label, select: menu.annotate },
        ],
      } : null}
      onClose={onClose}
    />
  );
}
