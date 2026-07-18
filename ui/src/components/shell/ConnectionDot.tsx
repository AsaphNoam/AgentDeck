import { useUiStore } from "../../store/uiStore";

export function ConnectionDot() {
  const connection = useUiStore((state) => state.connection);
  return (
    <span className="connection-status" data-ui="connection-indicator" data-state={connection} aria-label={`SSE ${connection}`}>
      <span className={`connection-dot ${connection}`} data-slot="indicator" />
      <span data-slot="label">{connection === "open" ? "Local link" : connection}</span>
    </span>
  );
}
