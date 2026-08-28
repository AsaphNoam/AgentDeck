import { createBrowserRouter, Navigate } from "react-router-dom";
import App from "./App";
import { ChatPanel } from "./components/chat/ChatPanel";
import { SettingsPage } from "./features/settings/SettingsPage";
import { OnboardingGate } from "./features/onboarding/OnboardingGate";
import { ArchivePage } from "./features/archive/ArchivePage";
import { ArchiveAgentPage } from "./features/archive/ArchiveAgentPage";
import {
  PipelineRunPage,
  PipelinesIndex,
  PipelinesLayout,
  PipelineTemplatePage,
  RunsPage,
  TemplatesPage,
} from "./features/pipelines/PipelinesPage";
import { TasksPage } from "./features/tasks/TasksPage";
import { ProjectDashboard, ScopedProjectDashboard } from "./features/dashboard/ProjectDashboard";

const developmentRoutes = import.meta.env.DEV
  ? [
      {
        path: "__visual-matrix",
        lazy: async () => {
          const { VisualMatrix } = await import("./presentation/VisualMatrix");
          return { Component: VisualMatrix };
        },
      },
    ]
  : [];

export const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <OnboardingGate><ProjectDashboard /></OnboardingGate> },
	  { path: "project/:project", element: <ScopedProjectDashboard /> },
      { path: "agent/:id", element: <ChatPanel /> },
      { path: "archive", element: <ArchivePage /> },
      { path: "archive/:id", element: <ArchiveAgentPage /> },
      {
        path: "pipelines",
        element: <PipelinesLayout />,
        children: [
          { index: true, element: <PipelinesIndex /> },
          { path: "runs", element: <RunsPage /> },
          { path: "runs/:runID", element: <PipelineRunPage /> },
          { path: "templates", element: <TemplatesPage /> },
          { path: "templates/:templateID", element: <PipelineTemplatePage /> },
        ],
      },
      { path: "tasks", element: <TasksPage /> },
      { path: "settings", element: <SettingsPage /> },
      ...developmentRoutes,
      { path: "*", element: <Navigate to="/" replace /> },
    ],
  },
]);
