import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { PhoneApp } from "./PhoneApp";
import "../styles/remote.css";

// The phone entry served only by the tailnet listener (TS-13.R14). It shares
// ui/src/api and appearance palettes with the desktop, not its shell or SharedWorker.
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: true, staleTime: 0 } },
});

if ("serviceWorker" in navigator) {
  void navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => undefined);
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <PhoneApp />
    </QueryClientProvider>
  </React.StrictMode>,
);
