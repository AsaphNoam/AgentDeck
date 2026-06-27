import { useUiStore } from "../../store/uiStore";

export function Header() {
  const connection = useUiStore((state) => state.connection);
  return (
    <header className="app-header">
      <strong>AgentDeck</strong>
      <span className={`connection-dot ${connection}`} aria-label={`SSE ${connection}`} />
    </header>
  );
}
