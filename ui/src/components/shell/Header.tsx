import { Link, NavLink } from "react-router-dom";
import { ConnectionDot } from "./ConnectionDot";
import { AgentDeckMark } from "./AgentDeckMark";

export function Header() {
  return (
    <header className="app-header" data-ui="app-shell">
      <Link to="/" className="app-logo">
        <AgentDeckMark />
      </Link>
      <nav className="app-nav" data-slot="navigation" aria-label="Primary navigation">
        <NavLink to="/" end>Dashboard</NavLink>
        <NavLink to="/archive">Archive</NavLink>
        <NavLink to="/settings">Settings</NavLink>
      </nav>
      <div className="app-connection" data-slot="connection">
        <ConnectionDot />
      </div>
    </header>
  );
}
