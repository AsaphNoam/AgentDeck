import { launchDefaultAgent } from "../../api/client";

export function EmptyState() {
  return (
    <section className="empty-state">
      <h1>No running agents</h1>
      <button type="button" onClick={() => void launchDefaultAgent()}>
        New Agent
      </button>
    </section>
  );
}
