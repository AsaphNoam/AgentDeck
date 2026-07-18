import { Outlet } from "react-router-dom";
import { Header } from "./components/shell/Header";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { NotificationCenter } from "./components/shell/NotificationCenter";

export default function App() {
  return (
    <div className="app-shell" data-ui="app-shell">
      <Header />
      <main className="app-main" data-slot="main">
        <ErrorBoundary label="dashboard">
          <Outlet />
        </ErrorBoundary>
      </main>
      <NotificationCenter />
    </div>
  );
}
