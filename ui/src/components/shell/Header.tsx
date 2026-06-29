import { Link } from "react-router-dom";
import { ConnectionDot } from "./ConnectionDot";

export function Header() {
  return (
    <header className="app-header">
      <Link to="/" className="app-logo">
        <strong>AgentDeck</strong>
      </Link>
      <nav className="app-nav">
        <Link to="/archive">Archive</Link>
        <Link to="/settings">Settings</Link>
      </nav>
      <ConnectionDot />
    </header>
  );
}
