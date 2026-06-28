import { createBrowserRouter, Navigate } from "react-router-dom";
import App from "./App";

function GridPlaceholder() {
  return (
    <section className="placeholder-view">
      <h1>Agents</h1>
      <p>Live dashboard data is connected. Card grid lands in subphase 2.5.</p>
    </section>
  );
}

function ChatPlaceholder() {
  return (
    <section className="placeholder-view">
      <h1>Chat</h1>
      <p>Transcript state is connected. Full chat panel lands in subphase 2.6.</p>
    </section>
  );
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <GridPlaceholder /> },
      { path: "agent/:id", element: <ChatPlaceholder /> },
      { path: "*", element: <Navigate to="/" replace /> },
    ],
  },
]);
