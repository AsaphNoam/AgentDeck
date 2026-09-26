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
  const [step, setStep] = useState<WizardStep>(() => !steps.backend.done ? "backend" : !steps.project.done ? "project" : "resume");
  const [createdProject, setCreatedProject] = useState<string | undefined>(undefined);
  const [backend, setBackend] = useState<{ id: string; type: BackendType }>(() => configuredBackendEntry && configuredBackend
    ? { id: configuredBackend, type: configuredBackendEntry.type }
    : { id: "claude", type: "claude-acp" });
  const [skipError, setSkipError] = useState<string | null>(null);
  const mutationClaimed = useRef(false);
  const [stepPending, setStepPending] = useState(false);
  const userAdvanced = useRef(false);
  const putConfig = usePutConfig();

  // A returning wizard has no backend-selection step to report its choice. Wait
  // for the saved catalog, then resume at Config only for a supported provider.
  useEffect(() => {
    if (!steps.backend.done || !steps.project.done || !backendsData || userAdvanced.current) return;
    const selected = configuredBackendEntry && configuredBackend
      ? { id: configuredBackend, type: configuredBackendEntry.type }
      : { id: "claude", type: "claude-acp" as BackendType };
    setBackend(selected);
    setStep(supportsConfigSource(selected.type) ? "source" : "launch");
  }, [steps.backend.done, steps.project.done, backendsData, configuredBackend, configuredBackendEntry]);

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
    userAdvanced.current = true;
    setCreatedProject(projectId);
    setStep(supportsConfigSource(backend.type) ? "source" : "launch");
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
    userAdvanced.current = true;
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
              {(["backend", "project", ...(supportsConfigSource(backend.type) ? ["source"] : []), "launch"] as const).map((key) => {
                const label = key === "backend" ? "Backend" : key === "project" ? "Project" : key === "source" ? "Config" : "Launch";
                const activeIndex = step === "resume" ? -1 : (["backend", "project", ...(supportsConfigSource(backend.type) ? ["source"] : []), "launch"] as string[]).indexOf(step);
                const index = (["backend", "project", ...(supportsConfigSource(backend.type) ? ["source"] : []), "launch"] as string[]).indexOf(key);
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
              {step === "backend" && <BackendStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={(b) => { userAdvanced.current = true; setBackend(b); setStep("project"); }} />}
              {step === "project" && <ProjectStep claimMutation={claimMutation} releaseMutation={releaseMutation} onDone={continueFromProject} />}
              {step === "source" && (
                <SourceStep
                  backendId={backend.id}
                  backendType={backend.type}
                  claimMutation={claimMutation}
                  releaseMutation={releaseMutation}
                  onDone={() => { userAdvanced.current = true; setStep("launch"); }}
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
