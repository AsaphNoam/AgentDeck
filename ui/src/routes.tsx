import { createBrowserRouter, Navigate } from "react-router-dom";
import App from "./App";
import { CardGrid } from "./components/grid/CardGrid";
import { ChatPanel } from "./components/chat/ChatPanel";
import { SettingsPage } from "./features/settings/SettingsPage";
import { OnboardingGate } from "./features/onboarding/OnboardingGate";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <OnboardingGate><CardGrid /></OnboardingGate> },
      { path: "agent/:id", element: <ChatPanel /> },
      { path: "settings", element: <SettingsPage /> },
      { path: "*", element: <Navigate to="/" replace /> },
    ],
  },
]);
