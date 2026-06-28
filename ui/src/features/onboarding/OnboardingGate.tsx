import { useState } from "react";
import { useConfig } from "../../api/config";
import { OnboardingWizard } from "./OnboardingWizard";

interface OnboardingGateProps {
  children: React.ReactNode;
}

export function OnboardingGate({ children }: OnboardingGateProps) {
  const { data: config, isLoading } = useConfig();
  const [dismissed, setDismissed] = useState(false);

  if (isLoading) return <div className="gate-loading">Loading…</div>;

  const satisfied =
    dismissed ||
    config?.onboarding.satisfied === true ||
    config?.onboarding_complete === true;

  if (config && !satisfied) {
    return (
      <OnboardingWizard
        steps={config.onboarding.steps}
        onComplete={() => setDismissed(true)}
      />
    );
  }

  return <>{children}</>;
}
