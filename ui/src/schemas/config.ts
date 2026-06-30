import { z } from "zod";

export const onboardingStepSchema = z.object({
  done: z.boolean(),
  detail: z.string().optional(),
});

export const onboardingSchema = z.object({
  satisfied: z.boolean(),
  steps: z.object({
    backend: onboardingStepSchema,
    project: onboardingStepSchema,
    role: onboardingStepSchema,
  }),
});

export const configSchema = z.object({
  version: z.number(),
  port: z.number(),
  default_project: z.string(),
  default_role: z.string(),
  skip_permissions: z.boolean(),
  onboarding_complete: z.boolean(),
  notifications: z.object({
    desktop_enabled: z.boolean(),
    muted: z.record(z.boolean()),
  }),
  onboarding: onboardingSchema,
});

export type Config = z.infer<typeof configSchema>;
export type Onboarding = z.infer<typeof onboardingSchema>;
