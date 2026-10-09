import type { ReactNode } from "react";
import { useRef } from "react";
import * as Dialog from "@radix-ui/react-dialog";

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  confirmLabel: string;
  onCancel: () => void;
  onConfirm: () => void;
  children: ReactNode;
  pending?: boolean;
  confirmDisabled?: boolean;
  destructive?: boolean;
}

export function ConfirmDialog({
  open,
  title,
  confirmLabel,
  onCancel,
  onConfirm,
  children,
  pending = false,
  confirmDisabled = false,
  destructive = false,
}: ConfirmDialogProps) {
  const restoreFocusRef = useRef<HTMLElement | null>(null);
  return (
    <Dialog.Root open={open} onOpenChange={(next) => { if (!next) onCancel(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
        <Dialog.Content
          className="dialog-content"
          data-ui="dialog"
          data-slot="content"
          data-variant="default"
          onOpenAutoFocus={() => {
            restoreFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            restoreFocusRef.current?.focus();
            restoreFocusRef.current = null;
          }}
        >
          <Dialog.Title data-slot="title">{title}</Dialog.Title>
          <div data-slot="body">{children}</div>
          <div className="form-actions" data-slot="actions">
            <button type="button" onClick={onCancel} disabled={pending}>Cancel</button>
            <button type="button" className={destructive ? "btn-danger" : undefined} onClick={onConfirm} disabled={pending || confirmDisabled}>
              {pending ? "Working…" : confirmLabel}
            </button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
