import { Link, NavLink } from "react-router-dom";
import { ConnectionDot } from "./ConnectionDot";
import { AgentDeckMark } from "./AgentDeckMark";
import { ActiveProjectNav } from "./ActiveProjectNav";

export function Header() {
  return (
    <header className="app-header" data-ui="app-shell" data-slot="header">
      <Link to="/" className="app-logo" data-slot="brand">
        <AgentDeckMark />
      </Link>
      <nav className="app-nav" data-slot="navigation" aria-label="Primary navigation">
        <NavLink to="/" end>Dashboard</NavLink>
        <NavLink to="/tasks">Tasks</NavLink>
        <NavLink to="/pipelines">Pipelines</NavLink>
        <NavLink to="/archive">Archive</NavLink>
        <NavLink to="/settings">Settings</NavLink>
      </nav>
      <ActiveProjectNav />
      <div className="app-connection" data-slot="connection">
        <ConnectionDot />
      </div>
    </header>
  );
}
