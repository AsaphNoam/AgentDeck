// BACKEND_SUPPORT_WIRE mirrors the server's backend_support response field for
// FS-09.A30's matrix (see internal/server/config_endpoint_test.go). MSW doubles
// include it so UI tests run against the payload the server actually produces.
export const BACKEND_SUPPORT_WIRE = {
  "claude-acp": {
    chat: { available: true, effort: true, fast: true },
    terminal: { available: true, effort: true, fast: false },
  },
  "codex-acp": {
    chat: { available: true, effort: true, fast: true },
    terminal: { available: false, effort: false, fast: false },
  },
  "opencode-acp": {
    chat: { available: true, effort: false, fast: false },
    terminal: { available: false, effort: false, fast: false },
  },
  "openhands-acp": {
    chat: { available: true, effort: false, fast: false },
    terminal: { available: false, effort: false, fast: false },
  },
};
