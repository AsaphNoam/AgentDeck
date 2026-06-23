import { useEffect, useState } from "react";

// Phase 0 placeholder UI: pings the Go server's health endpoint and renders the
// title. No app logic yet — later phases build the dashboard here.

interface Health {
  status: string;
  version: string;
  time: string;
}

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/health")
      .then((r) => {
        if (!r.ok) throw new Error(`health ${r.status}`);
        return r.json() as Promise<Health>;
      })
      .then(setHealth)
      .catch((e: unknown) => setError(String(e)));
  }, []);

  return (
    <main style={{ fontFamily: "system-ui, sans-serif", padding: "2rem" }}>
      <h1>AgentDeck — Phase 0</h1>
      {health && (
        <p>
          server: <strong>{health.status}</strong> · version {health.version}
        </p>
      )}
      {error && <p style={{ color: "crimson" }}>server unreachable: {error}</p>}
      {!health && !error && <p>contacting server…</p>}
    </main>
  );
}
