import { Outlet } from "react-router-dom";
import { Header } from "./components/shell/Header";

export default function App() {
  return (
    <div className="app-shell">
      <Header />
      <main className="app-main">
        <Outlet />
      </main>
    </div>
  );
}
