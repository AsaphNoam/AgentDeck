import { z } from "zod";

export const slugSchema = z.string().regex(/^[a-z0-9][a-z0-9-]{0,62}$/, {
  message: "must match ^[a-z0-9][a-z0-9-]{0,62}$",
});

export const roleSchema = z.object({
  role: slugSchema,
  title: z.string().min(1, "title is required").max(120),
  system_prompt: z.string(),
  skip_permissions: z.boolean().nullable().optional(),
});

export type RoleInput = z.infer<typeof roleSchema>;

export const roleResponseSchema = z.object({
  role: z.string(),
  title: z.string(),
  system_prompt: z.string(),
  skip_permissions: z.boolean().nullable().optional(),
});

export type RoleResponse = z.infer<typeof roleResponseSchema>;
