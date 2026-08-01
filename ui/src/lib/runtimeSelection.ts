import type { BackendsConfig } from "../schemas/backends";

export interface RuntimeSelection {
  backend: string;
  model: string;
  effort: string;
}

// Keep every backend/model picker on the same reset path. The catalog permits a
// backend's declared default model to be absent while it is being hand-edited,
// so use its first available model as the consistent fallback.
export function resetRuntimeForBackend(backends: BackendsConfig | undefined, backend: string): RuntimeSelection {
  const selectedBackend = backends?.backends[backend];
  const model = selectedBackend?.models[selectedBackend.default_model]
    ? selectedBackend.default_model
    : Object.keys(selectedBackend?.models ?? {})[0] ?? "";
  return resetRuntimeForModel(backends, backend, model);
}

export function resetRuntimeForModel(backends: BackendsConfig | undefined, backend: string, model: string): RuntimeSelection {
  return {
    backend,
    model,
    effort: backends?.backends[backend]?.models[model]?.default_effort ?? "",
  };
}
