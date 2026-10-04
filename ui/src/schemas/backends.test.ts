import { describe, expect, it } from "vitest";
import {
  backendsResponseSchema,
  editableBackendsConfig,
  launchSupportFor,
  parseBackendSupport,
  withBackendSupport,
} from "./backends";
import { BACKEND_SUPPORT_WIRE } from "../test/backendSupport";

const catalog = {
  version: 2 as const,
  backends: {
    claude: { name: "Claude", type: "claude-acp" as const, default: true, default_model: "d", models: { d: { name: "D", model: "m" } } },
  },
};

// FS-09.A30 / TS-03.R47
describe("backend_support parsing", () => {
  it("parses the server response through the real schema and keeps every entry", () => {
    const wire = { ...catalog, codex_runtime: { catalog_status: "compatible" }, backend_support: BACKEND_SUPPORT_WIRE };
    expect(backendsResponseSchema.safeParse(wire).success).toBe(true);
    const res = withBackendSupport(wire);
    expect(res.backend_support).toEqual(BACKEND_SUPPORT_WIRE);
    expect(res.backends).toBe(catalog.backends);
  });

  it("drops only the malformed entry and distinguishes missing from all-false", () => {
    const support = parseBackendSupport({
      ...BACKEND_SUPPORT_WIRE,
      "claude-acp": { chat: BACKEND_SUPPORT_WIRE["claude-acp"].chat, terminal: { available: "yes" } },
      "codex-acp": null,
    });
    expect(launchSupportFor(support, "claude-acp", "terminal")).toBeUndefined();
    expect(launchSupportFor(support, "claude-acp", "chat")).toEqual({ available: true, effort: true, fast: true });
    expect(launchSupportFor(support, "codex-acp", "chat")).toBeUndefined();
    expect(launchSupportFor(support, "opencode-acp", "terminal")).toEqual({ available: false, effort: false, fast: false });
  });

  it("treats a missing or null field as unknown support without failing the catalog", () => {
    expect(withBackendSupport({ ...catalog }).backend_support).toEqual({});
    expect(withBackendSupport({ ...catalog, backend_support: null }).backend_support).toEqual({});
  });

  it("the PUT projection excludes response-only fields", () => {
    const response = { ...catalog, credentials: {}, codex_runtime: { catalog_status: "compatible" as const }, backend_support: BACKEND_SUPPORT_WIRE };
    expect(JSON.parse(JSON.stringify(editableBackendsConfig(response)))).toEqual(catalog);
  });
});

// TS-03.R52, FS-09.A43 — provider runtime metadata is response-only and
// tolerantly parsed.
describe("provider_runtimes parsing", () => {
  it("keeps valid entries, drops malformed ones, and never reaches the PUT body", async () => {
    const { parseProviderRuntimes, withBackendSupport, editableBackendsConfig } = await import("./backends");
    const parsed = parseProviderRuntimes({
      claude: { sonnet: { source: "detected", state: "available", path: "/bin/claude" }, bad: { source: "nope" } },
      codex: null,
    });
    expect(parsed).toEqual({ claude: { sonnet: { source: "detected", state: "available", path: "/bin/claude" } } });
    const res = withBackendSupport({ ...catalog, provider_runtimes: { claude: { sonnet: { source: "bundled", state: "bundle_unavailable" } } } });
    expect(res.provider_runtimes.claude.sonnet.state).toBe("bundle_unavailable");
    expect(withBackendSupport({ ...catalog }).provider_runtimes).toEqual({});
    expect(JSON.parse(JSON.stringify(editableBackendsConfig(res)))).toEqual(catalog);
  });
});
