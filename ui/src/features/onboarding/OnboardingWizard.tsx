import { useRef, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import type { Onboarding } from "../../schemas/config";
import type { BackendType } from "../../schemas/backends";
import { usePutConfig, configErrorMessage } from "../../api/config";
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
  const [skipError, setSkipError] = useState<string | null>(null);
  const mutationClaimed = useRef(false);
  const [stepPending, setStepPending] = useState(false);
  const putConfig = usePutConfig();

  const claimMutation = () => {
    if (mutationClaimed.current) return false;
    mutationClaimed.current = true;
    setStepPending(true);
    return true;
  };
  const releaseMutation = () => {
    mutationClaimed.current = false;
    setStepPending(false);
  };

  const advance = () => setStep((s) => Math.min(s + 1, LAST_STEP));

  // Set up later is the escape hatch for someone who cannot finish now — an
  // unconfigured provider, no credentials to hand, or simply wanting to look
  // around first (FS-04.R32). It marks onboarding complete and nothing else: no
  // project is created, no backend or model catalog is written, no agent is
  // launched. A failed write leaves the wizard open with the reason, because
  // silently closing would hand back a dashboard that reopens the wizard on the
  // next poll (INV §8).
  const handleSetUpLater = () => {
    if (!claimMutation()) return;
    setSkipError(null);
    putConfig.mutate(
      { onboarding_complete: true },
      {
        onSuccess: onComplete,
        onError: (e) => setSkipError(configErrorMessage(e)),
        onSettled: releaseMutation,
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
              {step === 0 && <BackendStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={(b) => { setBackend(b); advance(); }} />}
              {step === 1 && <ProjectStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={(projectId) => { setCreatedProject(projectId); advance(); }} />}
              {step === 2 && (
                <SourceStep
                  project={createdProject}
                  backendId={backend.id}
                  backendType={backend.type}
                  claimMutation={claimMutation}
                  releaseMutation={releaseMutation}
                  onDone={advance}
                />
              )}
              {step === 3 && <LaunchStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={onComplete} initialProject={createdProject} />}
            </div>
            <div className="onboarding-actions" data-slot="footer">
              {skipError && <p className="form-error">{skipError}</p>}
              <button
                type="button"
                className="btn-link"
                onClick={handleSetUpLater}
                disabled={stepPending || putConfig.isPending}
              >
                {putConfig.isPending ? "Saving…" : "Set up later"}
              </button>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
