import { useRef, useState } from "react";
import { useConfig } from "../../api/config";
import { OnboardingWizard } from "./OnboardingWizard";

interface OnboardingGateProps {
  children: React.ReactNode;
}

export function OnboardingGate({ children }: OnboardingGateProps) {
  const { data: config, isLoading } = useConfig();
  const [dismissed, setDismissed] = useState(false);
  const wizardOpened = useRef(false);

  if (isLoading) return <div className="gate-loading">Loading…</div>;

  const serverSatisfied =
    config?.onboarding.satisfied === true ||
    config?.onboarding_complete === true;
  if (config && !serverSatisfied) wizardOpened.current = true;

  // Backend/project writes can make the computed server gate satisfied while
  // this four-step walkthrough is still in progress. Keep an opened wizard
  // mounted until Launch completes so polling cannot eject the user mid-flow.
  if (config && !dismissed && wizardOpened.current) {
    return (
      <OnboardingWizard
        steps={config.onboarding.steps}
        onComplete={() => setDismissed(true)}
      />
    );
  }

  return <>{children}</>;
}
