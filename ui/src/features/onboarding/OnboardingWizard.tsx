import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import type { Onboarding } from "../../schemas/config";
import type { BackendType } from "../../schemas/backends";
import { usePutConfig, configErrorMessage } from "../../api/config";
import { useUiStore } from "../../store/uiStore";
import { BackendStep } from "./steps/BackendStep";
import { ProjectStep } from "./steps/ProjectStep";
import { SourceStep } from "./steps/SourceStep";
import { LaunchStep } from "./steps/LaunchStep";

// The optional Config (federation) step lives between Project and Launch. It is
// purely client-side and skippable, so it is not tracked in the server-side
// onboarding step flags; a returning user who finished project setup resumes at
// it and can simply Continue.
const LAST_STEP = 3;
const STEP_VARIANTS = ["backend", "project", "source", "launch"] as const;

function initialStep(steps: Onboarding["steps"]): number {
  if (!steps.backend.done) return 0;
  if (!steps.project.done) return 1;
  return 2;
}

interface OnboardingWizardProps {
  steps: Onboarding["steps"];
  onComplete: () => void;
}

export function OnboardingWizard({ steps, onComplete }: OnboardingWizardProps) {
  const [step, setStep] = useState(() => initialStep(steps));
  const [createdProject, setCreatedProject] = useState<string | undefined>(undefined);
  // The backend chosen in step 0, so the federation Config step targets the right
  // provider (default Claude only until the user picks otherwise).
  const [backend, setBackend] = useState<{ id: string; type: BackendType }>({ id: "claude", type: "claude-acp" });
  const putConfig = usePutConfig();
  const pushError = useUiStore((state) => state.pushError);

  const advance = () => setStep((s) => Math.min(s + 1, LAST_STEP));

  // Skip setup: leave onboarding without launching an agent (FS-04.R32). Marking
  // onboarding_complete forces the gate satisfied (R22), so the user is not
  // re-gated and can finish configuration in Settings. A failed write keeps the
  // wizard open with the error surfaced rather than silently claiming completion.
  const handleSkip = () => {
    putConfig.mutate(
      { onboarding_complete: true },
      {
        onSuccess: onComplete,
        onError: (e) => pushError("Failed to skip setup", configErrorMessage(e)),
      },
    );
  };

  return (
    <Dialog.Root open modal>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay onboarding-overlay" data-ui="dialog" data-slot="overlay" />
        <Dialog.Content
          className="dialog-content onboarding-wizard"
          data-ui="dialog"
          data-variant="onboarding"
          onInteractOutside={(e) => e.preventDefault()}
          onEscapeKeyDown={(e) => e.preventDefault()}
          aria-describedby={undefined}
        >
          <div className="onboarding-flow" data-ui="onboarding" data-variant={STEP_VARIANTS[step]}>
            <Dialog.Title>Welcome to AgentDeck</Dialog.Title>
            <div className="wizard-progress" data-slot="progress">
              {["Backend", "Project", "Config", "Launch"].map((label, i) => (
                <div
                  key={label}
                  className={`wizard-step-indicator ${i < step ? "done" : i === step ? "active" : ""}`}
                  data-state={i < step ? "complete" : i === step ? "current" : "upcoming"}
                >
                  {label}
                </div>
              ))}
            </div>
            <div data-slot="content">
              {step === 0 && <BackendStep onDone={(b) => { setBackend(b); advance(); }} />}
              {step === 1 && <ProjectStep onDone={(projectId) => { setCreatedProject(projectId); advance(); }} />}
              {step === 2 && (
                <SourceStep
                  project={createdProject}
                  backendId={backend.id}
                  backendType={backend.type}
                  onDone={advance}
                />
              )}
              {step === 3 && <LaunchStep onDone={onComplete} initialProject={createdProject} />}
            </div>
            <div className="wizard-skip" data-slot="actions">
              <button type="button" className="wizard-skip-button" onClick={handleSkip} disabled={putConfig.isPending}>
                {putConfig.isPending ? "Skipping…" : "Skip setup"}
              </button>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
