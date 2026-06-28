import { Outlet } from "react-router-dom";
import { Header } from "./components/shell/Header";
import { ErrorBoundary } from "./components/ErrorBoundary";

export default function App() {
  return (
    <div className="app-shell">
      <Header />
      <main className="app-main">
        <ErrorBoundary label="dashboard">
          <Outlet />
        </ErrorBoundary>
      </main>
    </div>
  );
}
