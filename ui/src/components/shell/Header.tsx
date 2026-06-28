import { Link } from "react-router-dom";
import { useUiStore } from "../../store/uiStore";

export function Header() {
  const connection = useUiStore((state) => state.connection);
  return (
    <header className="app-header">
      <Link to="/" className="app-logo">
        <strong>AgentDeck</strong>
      </Link>
      <nav className="app-nav">
        <Link to="/settings">Settings</Link>
      </nav>
      <span className={`connection-dot ${connection}`} aria-label={`SSE ${connection}`} />
    </header>
  );
}
