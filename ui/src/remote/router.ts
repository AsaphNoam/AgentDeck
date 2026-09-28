import { useEffect, useState } from "react";

// A minimal path router: the phone app has a handful of flat routes and the
// tailnet listener serves the phone entry for every non-API path.

export function navigate(to: string, replace = false) {
  if (replace) window.history.replaceState(null, "", to);
  else window.history.pushState(null, "", to);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

export function usePath(): string {
  const [path, setPath] = useState(() => window.location.pathname);
  useEffect(() => {
    const update = () => setPath(window.location.pathname);
    window.addEventListener("popstate", update);
    // A tapped notification asks an open app to show its card (FS-20.R21).
    const onMessage = (event: MessageEvent) => {
      const data = event.data as { kind?: string; url?: string } | undefined;
      if (data?.kind === "open" && typeof data.url === "string" && data.url.startsWith("/")) navigate(data.url);
    };
    navigator.serviceWorker?.addEventListener("message", onMessage);
    return () => {
      window.removeEventListener("popstate", update);
      navigator.serviceWorker?.removeEventListener("message", onMessage);
    };
  }, []);
  return path;
}

/** match returns the id in /<prefix>/<id>, or null. */
export function match(path: string, prefix: string): string | null {
  const parts = path.split("/").filter(Boolean);
  return parts.length === 2 && parts[0] === prefix ? decodeURIComponent(parts[1]) : null;
}
