package server

import (
	"fmt"

	"github.com/agentdeck/agentdeck/internal/backend/providerexec"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

// providerInstallGuidance is the official installation entry point for each
// provider; AgentDeck never installs or updates a provider itself (FS-09.R77).
var providerInstallGuidance = map[string]string{
	"claude": "https://code.claude.com/docs/en/setup",
	"codex":  "https://developers.openai.com/codex/cli",
}

// providerLaunchLayer resolves the provider before any launch side effect and
// returns the final child-env layer that pins the adapter to the selected
// executable, overwriting every inherited/backend/model override value so
// nothing can redirect the child (TS-04.R75). Other backend types get nil.
func providerLaunchLayer(be config.Backend, model config.Model) (map[string]string, string, *runtime.APIError) {
	sel, ok := providerexec.ForBackend(be, model)
	if !ok {
		return nil, "", nil
	}
	if ae := providerSelectionError(sel); ae != nil {
		return nil, "", ae
	}
	return map[string]string{sel.Provider.EnvKey: sel.Path}, sel.Path, nil
}

// providerSelectionError maps an unusable selection to its typed, source-aware
// error (FS-09.R77, TS-04.R76). Guidance names a repair; it never switches the
// source, installs anything, or retries.
func providerSelectionError(sel providerexec.Selection) *runtime.APIError {
	if sel.Available() {
		return nil
	}
	p := sel.Provider
	var code, msg string
	switch sel.State {
	case providerexec.StateBundleUnavailable:
		code = runtime.CodeBundledProviderMissing
		msg = fmt.Sprintf("The AgentDeck bundle for %s is not available in this AgentDeck installation. Choose Installed provider for this backend in Settings, or reinstall AgentDeck.", p.Name)
	case providerexec.StateMissing:
		if sel.Source == providerexec.SourceDetected {
			code = runtime.CodeProviderExecutableMissing
			msg = fmt.Sprintf("%s is not installed or not on AgentDeck's PATH. Install it (%s), set its executable path in Settings, or choose AgentDeck bundle for this backend.", p.Name, providerInstallGuidance[p.ID])
		} else {
			code = runtime.CodeProviderExecutableMissing
			msg = fmt.Sprintf("The %s executable set by %s (%s) was not found. Fix or clear that path in Settings, or choose AgentDeck bundle for this backend.", p.Name, overrideLabel(sel), sel.Override)
		}
	default:
		code = runtime.CodeProviderExecutableInvalid
		msg = fmt.Sprintf("The %s executable set by %s (%s) is not a usable executable. Use an absolute path or a command name, or choose AgentDeck bundle for this backend.", p.Name, overrideLabel(sel), sel.Override)
	}
	ae := apiError(code, msg)
	ae.Details = map[string]any{"provider": p.ID, "source": sel.Source}
	if sel.Path != "" {
		ae.Details["path"] = sel.Path
	}
	return ae
}

func overrideLabel(sel providerexec.Selection) string {
	switch sel.Source {
	case providerexec.SourceModel:
		return "this model's " + sel.Provider.EnvKey
	case providerexec.SourceBackend:
		return "this backend's " + sel.Provider.EnvKey
	default:
		return "the dashboard environment's " + sel.Provider.EnvKey
	}
}
