import { ConfigSourcePanel } from "../../settings/ConfigSourcePanel";

interface SourceStepProps {
  // The project just created in the previous step; federation is project-scoped.
  project?: string;
  onDone: () => void;
}

// SourceStep is an OPTIONAL onboarding step: it lets a new user link their native
// Claude Code / Codex configuration up front, but linking can equally be done
// later in Settings, so the step is always skippable. It reuses the same
// ConfigSourcePanel as Settings so there is one federation UI, not two.
export function SourceStep({ project, onDone }: SourceStepProps) {
  return (
    <div className="onboarding-step source-step">
      <h3>Link your CLI configuration (optional)</h3>
      <p className="source-hint">
        AgentDeck can read your existing Claude Code or Codex setup — model, instructions and tooling —
        so agents launch with your real configuration. Nothing is copied or modified. You can also do
        this anytime from Settings → Backends.
      </p>

      <ConfigSourcePanel
        backendId="claude"
        backendType="claude-acp"
        initialProjectId={project}
        defaultOpen
      />

      <div className="onboarding-actions">
        <button type="button" onClick={onDone}>
          Continue
        </button>
      </div>
    </div>
  );
}
