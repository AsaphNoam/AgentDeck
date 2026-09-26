import { useEffect, useRef, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import type { Onboarding } from "../../schemas/config";
import type { BackendType } from "../../schemas/backends";
import { useBackends, usePutConfig, configErrorMessage } from "../../api/config";
import { BackendStep } from "./steps/BackendStep";
import { ProjectStep } from "./steps/ProjectStep";
import { SourceStep, supportsConfigSource } from "./steps/SourceStep";
import { LaunchStep } from "./steps/LaunchStep";

type WizardStep = "backend" | "project" | "source" | "launch" | "resume";

interface OnboardingWizardProps {
  steps: Onboarding["steps"];
  onComplete: () => void;
}

export function OnboardingWizard({ steps, onComplete }: OnboardingWizardProps) {
  const { data: backendsData } = useBackends();
  const configuredBackend = Object.entries(backendsData?.backends ?? {}).find(([, backend]) => backend.default)?.[0]
    ?? Object.keys(backendsData?.backends ?? {})[0];
  const configuredBackendEntry = configuredBackend ? backendsData?.backends[configuredBackend] : undefined;
  const configuredType = configuredBackendEntry?.type;
  const [step, setStep] = useState<WizardStep>(() => !steps.backend.done ? "backend" : !steps.project.done ? "project" : "resume");
  const [createdProject, setCreatedProject] = useState<string | undefined>(undefined);
  // A returning wizard has no backend-selection step to report its choice, so
  // it uses the saved catalog's backend — never a hard-coded provider (FS-04.R49).
  const [chosenBackend, setChosenBackend] = useState<{ id: string; type: BackendType } | null>(null);
  const backend = chosenBackend ?? (configuredBackend && configuredType ? { id: configuredBackend, type: configuredType } : undefined);
  const hasSource = !!backend && supportsConfigSource(backend.type);
  const [skipError, setSkipError] = useState<string | null>(null);
  const mutationClaimed = useRef(false);
  const [stepPending, setStepPending] = useState(false);
  const putConfig = usePutConfig();

  // Resume waits for the saved catalog, then offers Config only for a supported provider.
  useEffect(() => {
    if (step !== "resume" || !backendsData) return;
    setStep(configuredType && supportsConfigSource(configuredType) ? "source" : "launch");
  }, [step, backendsData, configuredType]);

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

  const continueFromProject = (projectId: string) => {
    setCreatedProject(projectId);
    setStep(!backend ? "resume" : hasSource ? "source" : "launch");
  };

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
          <div className="onboarding-flow" data-ui="onboarding" data-variant={step === "resume" ? "backend" : step}>
            <Dialog.Title>Welcome to AgentDeck</Dialog.Title>
            <div className="wizard-progress" data-slot="progress">
              {(["backend", "project", ...(hasSource ? ["source"] : []), "launch"] as const).map((key) => {
                const label = key === "backend" ? "Backend" : key === "project" ? "Project" : key === "source" ? "Config" : "Launch";
                const activeIndex = step === "resume" ? -1 : (["backend", "project", ...(hasSource ? ["source"] : []), "launch"] as string[]).indexOf(step);
                const index = (["backend", "project", ...(hasSource ? ["source"] : []), "launch"] as string[]).indexOf(key);
                return <div
                  key={key}
                  className={`wizard-step-indicator ${index < activeIndex ? "done" : index === activeIndex ? "active" : ""}`}
                  data-state={index < activeIndex ? "complete" : index === activeIndex ? "current" : "upcoming"}
                >
                  {label}
                </div>
              })}
            </div>
            <div data-slot="content">
              {step === "resume" && <p className="wizard-step-desc">Loading your configured backend…</p>}
              {step === "backend" && <BackendStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={(b) => { setChosenBackend(b); setStep("project"); }} />}
              {step === "project" && <ProjectStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={continueFromProject} />}
              {step === "source" && backend && (
                <SourceStep
                  backendId={backend.id}
                  backendType={backend.type}
                  claimMutation={claimMutation}
                  releaseMutation={releaseMutation}
                  onDone={() => setStep("launch")}
                />
              )}
              {step === "launch" && <LaunchStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={onComplete} initialProject={createdProject} />}
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
