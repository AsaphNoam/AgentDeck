import { ConfigSourcePanel } from "../../settings/ConfigSourcePanel";
import type { BackendType } from "../../../schemas/backends";

interface SourceStepProps {
  // The backend chosen in the Backend step, so this step links the RIGHT provider
  // (Claude vs Codex) instead of always assuming Claude.
  backendId: string;
  backendType: BackendType;
  onDone: () => void;
  claimMutation?: () => boolean;
  releaseMutation?: () => void;
}

// Only Claude Code / Codex have a native configuration to federate; OpenCode and
// OpenHands are configured directly in Settings.
const FEDERATED: Partial<Record<BackendType, true>> = { "claude-acp": true, "codex-acp": true };
export const supportsConfigSource = (backendType: BackendType) => !!FEDERATED[backendType];

// SourceStep is an OPTIONAL onboarding step: it lets a new user link their native
// Claude Code / Codex configuration up front, but linking can equally be done
// later in Settings, so the step is always skippable. It reuses the same
// ConfigSourcePanel as Settings so there is one federation UI, not two.
export function SourceStep({ backendId, backendType, onDone, claimMutation, releaseMutation }: SourceStepProps) {
  const handleContinue = () => {
    if (claimMutation && !claimMutation()) return;
    releaseMutation?.();
    onDone();
  };
  return (
    <div className="onboarding-step source-step" data-ui="onboarding" data-slot="step" data-variant="source">
      <h3>Link your CLI configuration (optional)</h3>
      <p className="source-hint">
        AgentDeck can read your existing {backendType === "claude-acp" ? "Claude Code" : "Codex"} setup —
        model, instructions and tooling — so agents launch with your real configuration. Nothing is
        copied or modified. You can also link it later in Settings → Backends.
      </p>
      <ConfigSourcePanel
        backendId={backendId}
        backendType={backendType}
        compactOnboarding
        claimMutation={claimMutation}
        releaseMutation={releaseMutation}
      />

      <div className="onboarding-actions" data-slot="actions">
        <button type="button" onClick={handleContinue}>
          Continue
        </button>
      </div>
    </div>
  );
}
