import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import type { Onboarding } from "../../schemas/config";
import { BackendStep } from "./steps/BackendStep";
import { ProjectStep } from "./steps/ProjectStep";
import { SourceStep } from "./steps/SourceStep";
import { LaunchStep } from "./steps/LaunchStep";

// The optional Config (federation) step lives between Project and Launch. It is
// purely client-side and skippable, so it is not tracked in the server-side
// onboarding step flags; a returning user who finished project setup resumes at
// it and can simply Continue.
const LAST_STEP = 3;

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

  const advance = () => setStep((s) => Math.min(s + 1, LAST_STEP));

  return (
    <Dialog.Root open modal>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay onboarding-overlay" />
        <Dialog.Content
          className="dialog-content onboarding-wizard"
          onInteractOutside={(e) => e.preventDefault()}
          onEscapeKeyDown={(e) => e.preventDefault()}
          aria-describedby={undefined}
        >
          <Dialog.Title>Welcome to AgentDeck</Dialog.Title>
          <div className="wizard-progress">
            {["Backend", "Project", "Config", "Launch"].map((label, i) => (
              <div
                key={label}
                className={`wizard-step-indicator ${i < step ? "done" : i === step ? "active" : ""}`}
              >
                {label}
              </div>
            ))}
          </div>
          {step === 0 && <BackendStep onDone={advance} />}
          {step === 1 && <ProjectStep onDone={(projectId) => { setCreatedProject(projectId); advance(); }} />}
          {step === 2 && <SourceStep project={createdProject} onDone={advance} />}
          {step === 3 && <LaunchStep onDone={onComplete} initialProject={createdProject} />}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
