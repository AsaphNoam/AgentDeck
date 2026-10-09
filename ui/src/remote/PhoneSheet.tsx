import type { ReactNode } from "react";
import * as Dialog from "@radix-ui/react-dialog";

export function PhoneSheet({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  return <Dialog.Root open onOpenChange={(open) => { if (!open) onClose(); }}>
    <Dialog.Portal>
      <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
      <Dialog.Content className="dialog-content phone-sheet" data-ui="dialog" data-slot="content" data-variant="default" aria-describedby={undefined}>
        <div className="phone-section-title"><Dialog.Title data-slot="title">{title}</Dialog.Title><Dialog.Close className="phone-sheet-close" aria-label="Close dialog">×</Dialog.Close></div>
        {children}
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>;
}
