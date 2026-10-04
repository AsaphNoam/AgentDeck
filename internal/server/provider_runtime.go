package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

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
func providerLaunchLayer(be config.Backend, model config.Model) (map[string]string, providerexec.Selection, *runtime.APIError) {
	sel, ok := providerexec.ForBackend(be, model)
	if !ok {
		return nil, providerexec.Selection{}, nil
	}
	if ae := providerSelectionError(sel); ae != nil {
		return nil, sel, ae
	}
	return map[string]string{sel.Provider.EnvKey: sel.Path}, sel, nil
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

// providerIncompatibleError maps a recognized provider-version incompatibility
// to its typed envelope with only the parsed versions as details (TS-04.R76).
func providerIncompatibleError(err error) *runtime.APIError {
	ae := apiError(runtime.CodeProviderIncompatible, err.Error())
	ae.Details = map[string]any{"provider": "claude"}
	var tooOld *runtime.ProviderTooOldError
	if errors.As(err, &tooOld) {
		ae.Details["version"], ae.Details["required_version"] = tooOld.Have, tooOld.Need
	}
	return ae
}

// providerErrorCodes are the envelopes that can name a desktop executable path.
var providerErrorCodes = map[string]bool{
	runtime.CodeProviderExecutableMissing: true,
	runtime.CodeProviderExecutableInvalid: true,
	runtime.CodeBundledProviderMissing:    true,
	runtime.CodeProviderIncompatible:      true,
}

// remoteProviderError rewrites a provider error envelope for the phone: no
// path, no desktop-only repair, just what happened and that the repair is in
// desktop Settings (TS-04.R76, FS-09.R74). Other bodies pass through.
func remoteProviderError(body []byte) []byte {
	var env struct {
		Error *runtime.APIError `json:"error"`
	}
	if json.Unmarshal(body, &env) != nil || env.Error == nil || !providerErrorCodes[env.Error.Code] {
		return body
	}
	details := map[string]any{}
	for _, key := range []string{"provider", "source", "version", "required_version"} {
		if v, ok := env.Error.Details[key]; ok {
			details[key] = v
		}
	}
	name := "The provider"
	switch details["provider"] {
	case "claude":
		name = "Claude Code"
	case "codex":
		name = "Codex"
	}
	var msg string
	switch env.Error.Code {
	case runtime.CodeBundledProviderMissing:
		msg = fmt.Sprintf("The AgentDeck bundle for %s is unavailable on the Mac. Change this backend's provider in AgentDeck Settings on the Mac, then retry.", name)
	case runtime.CodeProviderIncompatible:
		msg = fmt.Sprintf("%s on the Mac is too old for this request. Update it or change this backend's provider in AgentDeck Settings on the Mac, then retry.", name)
		if have, need := details["version"], details["required_version"]; have != nil && need != nil {
			msg = fmt.Sprintf("%s on the Mac is %v; this request needs %v or newer. Update it or change this backend's provider in AgentDeck Settings on the Mac, then retry.", name, have, need)
		}
	default:
		msg = fmt.Sprintf("%s is not available for this backend on the Mac. Repair it in AgentDeck Settings on the Mac, then retry.", name)
	}
	env.Error.Message, env.Error.Details = msg, details
	out, err := json.Marshal(env)
	if err != nil {
		return body
	}
	return append(out, '\n')
}

// remoteErrorWriter buffers only error responses so remoteProviderError can
// rewrite them; successful responses and streams pass straight through.
type remoteErrorWriter struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (w *remoteErrorWriter) WriteHeader(code int) {
	if code >= 400 && w.buf == nil {
		w.status, w.buf = code, &bytes.Buffer{}
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *remoteErrorWriter) Write(b []byte) (int, error) {
	if w.buf != nil {
		return w.buf.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *remoteErrorWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok && w.buf == nil {
		f.Flush()
	}
}

func (w *remoteErrorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// remoteProviderErrorFilter applies remoteProviderError to every phone route.
func remoteProviderErrorFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &remoteErrorWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.buf == nil {
			return
		}
		body := remoteProviderError(rw.buf.Bytes())
		w.Header().Del("Content-Length")
		w.WriteHeader(rw.status)
		_, _ = w.Write(body)
	})
}
