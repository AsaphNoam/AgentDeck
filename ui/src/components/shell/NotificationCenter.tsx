import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { openAgentConversation, registerConversationNavigator } from "../../lib/agentConversation";
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

  // An agent toast opens that agent's conversation (FS-02.R64); an error toast
  // names no agent and only dismisses.
  const open = () => {
    if (toast.agentId) openAgentConversation(toast.agentId);
    dismiss(toast.id);
  };

  return (
    <div className={`toast ${toast.type}`} data-ui="toast" data-state={toast.type}>
      <button type="button" data-slot="open" onClick={open}>
        <strong data-slot="title">{toast.title}</strong>
        {toast.body && <span data-slot="body">{toast.body}</span>}
      </button>
      <button type="button" data-slot="close" aria-label="Dismiss notification" onClick={() => dismiss(toast.id)}>
        ×
      </button>
    </div>
  );
}

export function NotificationCenter() {
  const toasts = useUiStore((state) => state.toasts);
  const dismiss = useUiStore((state) => state.dismissToast);
  const navigate = useNavigate();

  // Desktop notifications fire from the SSE client, outside the router tree.
  useEffect(() => {
    registerConversationNavigator(navigate);
    return () => registerConversationNavigator(null);
  }, [navigate]);

  if (toasts.length === 0) return null;
  return (
    <div className="toast-stack" role="status" aria-live="polite">
      {toasts.map((toast) => (
        <Toast key={toast.id} toast={toast} dismiss={dismiss} />
      ))}
    </div>
  );
}
