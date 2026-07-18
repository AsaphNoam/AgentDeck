import { ConfigSourcePanel } from "../../settings/ConfigSourcePanel";
import type { BackendType } from "../../../schemas/backends";

interface SourceStepProps {
  // The project just created in the previous step; federation is project-scoped.
  project?: string;
  // The backend chosen in the Backend step, so this step links the RIGHT provider
  // (Claude vs Codex) instead of always assuming Claude.
  backendId: string;
  backendType: BackendType;
  onDone: () => void;
}

// Only Claude Code / Codex have a native configuration to federate; OpenCode and
// OpenHands are configured directly in Settings.
const FEDERATED: Partial<Record<BackendType, true>> = { "claude-acp": true, "codex-acp": true };

// SourceStep is an OPTIONAL onboarding step: it lets a new user link their native
// Claude Code / Codex configuration up front, but linking can equally be done
// later in Settings, so the step is always skippable. It reuses the same
// ConfigSourcePanel as Settings so there is one federation UI, not two.
export function SourceStep({ project, backendId, backendType, onDone }: SourceStepProps) {
  const federated = !!FEDERATED[backendType];
  return (
    <div className="onboarding-step source-step" data-ui="onboarding" data-slot="step" data-variant="source">
      <h3>Link your CLI configuration (optional)</h3>
      {federated ? (
        <>
          <p className="source-hint">
            AgentDeck can read your existing Claude Code or Codex setup — model, instructions and tooling —
            so agents launch with your real configuration. Nothing is copied or modified. You can also do
            this anytime from Settings → Backends.
          </p>
          <ConfigSourcePanel
            backendId={backendId}
            backendType={backendType}
            initialProjectId={project}
            defaultOpen
          />
        </>
      ) : (
        <p className="source-hint">
          This backend is configured directly in Settings → Backends — there is no external CLI
          configuration to link. You can continue.
        </p>
      )}

      <div className="onboarding-actions" data-slot="actions">
        <button type="button" onClick={onDone}>
          Continue
        </button>
      </div>
    </div>
  );
}
