import type { MouseEvent } from "react";
import { copyText } from "./copyText";

// Web links in rendered Markdown offer Open in new tab and Copy link on right-click
// (FS-03.R78, TS-08.R107). The anchor raises the menu itself unless an enclosing
// annotation menu claims the same right-click and composes these actions with its
// own, so one right-click never opens two menus. Every surface builds the actions
// here (INV §2).
const pending = new WeakMap<Event, string>();

// offerWebLink runs on the anchor, before any enclosing handler sees the bubble. The
// destination is the rendered, sanitized href, never the raw Markdown source.
export function offerWebLink(mouse: MouseEvent<HTMLAnchorElement>, open: (href: string) => void) {
  mouse.preventDefault();
  const native = mouse.nativeEvent;
  const href = mouse.currentTarget.href;
  pending.set(native, href);
  queueMicrotask(() => {
    if (pending.delete(native)) open(href);
  });
}

// claimWebLink is called by an annotation handler that opens its own menu for this
// right-click; it returns the link to compose, if the pointer was on one.
export function claimWebLink(mouse: MouseEvent): string | undefined {
  const href = pending.get(mouse.nativeEvent);
  pending.delete(mouse.nativeEvent);
  return href;
}

export function webLinkActions(href: string, pushError: (title: string, body: string) => void) {
  return [
    // Opened synchronously from the menu click so the browser treats it as user-initiated.
    { label: "Open in new tab", select: () => { window.open(href, "_blank", "noopener,noreferrer"); } },
    { label: "Copy link", select: () => copyText(href, pushError) },
  ];
}
