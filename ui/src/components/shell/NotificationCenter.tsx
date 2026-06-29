import { useEffect } from "react";
import { useUiStore } from "../../store/uiStore";

export function NotificationCenter() {
  const toasts = useUiStore((state) => state.toasts);
  const dismiss = useUiStore((state) => state.dismissToast);

  useEffect(() => {
    if (toasts.length === 0) return;
    const timers = toasts.map((toast) => window.setTimeout(() => dismiss(toast.id), 6_000));
    return () => timers.forEach((timer) => window.clearTimeout(timer));
  }, [toasts, dismiss]);

  if (toasts.length === 0) return null;
  return (
    <div className="toast-stack" role="status" aria-live="polite">
      {toasts.map((toast) => (
        <button key={toast.id} type="button" className={`toast ${toast.type}`} onClick={() => dismiss(toast.id)}>
          <strong>{toast.title}</strong>
          {toast.body && <span>{toast.body}</span>}
        </button>
      ))}
    </div>
  );
}
