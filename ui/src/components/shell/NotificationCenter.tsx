import { useEffect } from "react";
import { useUiStore, type ToastItem } from "../../store/uiStore";

// Toast owns its own 6s auto-dismiss timer, keyed to its id, so a newly pushed
// toast doesn't restart the timers of older ones (the previous single effect
// depended on the whole toasts array, resetting every timer on each new toast so
// older toasts lingered).
function Toast({ toast, dismiss }: { toast: ToastItem; dismiss: (id: string) => void }) {
  useEffect(() => {
    const timer = window.setTimeout(() => dismiss(toast.id), 6_000);
    return () => window.clearTimeout(timer);
  }, [toast.id, dismiss]);

  return (
    <button type="button" className={`toast ${toast.type}`} onClick={() => dismiss(toast.id)}>
      <strong>{toast.title}</strong>
      {toast.body && <span>{toast.body}</span>}
    </button>
  );
}

export function NotificationCenter() {
  const toasts = useUiStore((state) => state.toasts);
  const dismiss = useUiStore((state) => state.dismissToast);

  if (toasts.length === 0) return null;
  return (
    <div className="toast-stack" role="status" aria-live="polite">
      {toasts.map((toast) => (
        <Toast key={toast.id} toast={toast} dismiss={dismiss} />
      ))}
    </div>
  );
}
