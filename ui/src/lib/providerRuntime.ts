import type { BackendType, ProviderRuntime } from "../schemas/backends";

// Claude/Codex are the only backends whose provider executable AgentDeck
// selects (FS-09.R68/R75). Other backends have no provider source.
export function hasProviderSource(type: BackendType): type is "claude-acp" | "codex-acp" {
  return type === "claude-acp" || type === "codex-acp";
}

export function providerName(type: BackendType): string {
  return type === "codex-acp" ? "Codex" : "Claude Code";
}

// The executable-override key the managed adapter honors (FS-09.R76).
export function providerExecutableKey(type: BackendType): string {
  return type === "codex-acp" ? "CODEX_PATH" : "CLAUDE_CODE_EXECUTABLE";
}

// Official installation entry points; AgentDeck never installs a provider.
export function providerInstallURL(type: BackendType): string {
  return type === "codex-acp" ? "https://developers.openai.com/codex/cli" : "https://code.claude.com/docs/en/setup";
}

const SOURCE_LABELS: Record<ProviderRuntime["source"], string> = {
  detected: "Installed",
  ambient: "Installed (dashboard environment path)",
  backend: "Installed (backend path)",
  model: "Installed (model path)",
  bundled: "AgentDeck bundle",
};

// describeProviderRuntime is the one wording for a saved backend/model's
// next-start provider, shared by Settings and New Agent (FS-09.R72). A missing
// version is "not checked", never an incompatibility.
export function describeProviderRuntime(type: BackendType, rt: ProviderRuntime): { summary: string; problem?: string } {
  const name = providerName(type);
  const parts = [`${SOURCE_LABELS[rt.source]} ${name}`];
  if (rt.path) parts.push(rt.path);
  if (rt.state === "available") {
    parts.push(rt.version ? `version ${rt.version}${rt.checked_at ? `, last checked ${new Date(rt.checked_at).toLocaleTimeString()}` : ""}` : "version not checked");
  }
  const summary = parts.join(" · ");
  switch (rt.state) {
    case "available":
      return { summary };
    case "bundle_unavailable":
      return { summary, problem: `The AgentDeck bundle for ${name} is not available in this installation. Choose Installed provider, or reinstall AgentDeck.` };
    case "missing":
      return rt.source === "detected"
        ? { summary, problem: `${name} was not found. Install it, set its executable path, or choose AgentDeck bundle.` }
        : { summary, problem: `The ${name} executable path was not found. Fix or clear it, or choose AgentDeck bundle.` };
    default:
      return { summary, problem: `The ${name} executable path is not usable. Use an absolute path or a command name, or choose AgentDeck bundle.` };
  }
}
