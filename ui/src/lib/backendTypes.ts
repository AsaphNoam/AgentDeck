import type { BackendType } from "../schemas/backends";

// BACKEND_TYPE_LABELS is the single display-name mapping for the backend type
// union, replacing the per-component inlined ternaries (three-way drift risk).
export const BACKEND_TYPE_LABELS: Record<BackendType, string> = {
  "claude-acp": "Claude",
  "codex-acp": "Codex / OpenAI",
  "opencode-acp": "OpenCode",
  "openhands-acp": "OpenHands",
};

// BACKEND_TYPE_OPTIONS is the ordered list for <select> options.
export const BACKEND_TYPE_OPTIONS: BackendType[] = [
  "claude-acp",
  "codex-acp",
  "opencode-acp",
  "openhands-acp",
];

// terminalSupported mirrors the server gate (internal/server/terminal.go): only
// claude-acp has a verified interactive-CLI hook path, so the New-Agent modal
// hides/disables the Terminal interface for every other backend type.
export function terminalSupported(type: BackendType): boolean {
  return type === "claude-acp";
}
