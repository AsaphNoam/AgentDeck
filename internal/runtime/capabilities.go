package runtime

import "encoding/json"

// Capability negotiation is bilateral and extension-scoped (TS-04.R62): the
// client offers only what AgentDeck implements, and a capability is on only
// when the peer's initialize response advertises its matching surface. Nothing
// here reads the backend type or an adapter version (INV §11/§12).

// AIR is the `_meta.jetbrains.air` extension the Codex adapter uses for
// optional surfaces.
const (
	airVersion           = 1
	airAsyncTasks        = "asyncTasks"
	airFileChangeReports = "agentFileChangeReport"
	maxAirCapabilities   = 32
)

// clientOffer is the set of optional surfaces AgentDeck advertises. Each flag
// turns on only together with the runtime handling for the events it unlocks,
// so an adapter never switches away from a form AgentDeck already renders.
type clientOffer struct {
	Subagents         bool
	AsyncTasks        bool
	FileChangeReports bool
}

// offered is what this build implements.
var offered = clientOffer{Subagents: true, AsyncTasks: true}

// clientCapabilitiesFor builds the initialize `clientCapabilities` object.
func clientCapabilitiesFor(o clientOffer) map[string]any {
	caps := map[string]any{}
	if o.Subagents {
		caps["subagents"] = map[string]any{}
	}
	var air []string
	if o.AsyncTasks {
		air = append(air, airAsyncTasks)
	}
	if o.FileChangeReports {
		air = append(air, airFileChangeReports)
	}
	if len(air) > 0 {
		caps["_meta"] = map[string]any{"jetbrains": map[string]any{"air": map[string]any{
			"version": airVersion, "capabilities": air,
		}}}
	}
	return caps
}

// negotiateCapabilities maps an initialize response into the normalized value.
// Absent, malformed, or wrongly-typed advertisements read as unsupported.
func negotiateCapabilities(initRes json.RawMessage, o clientOffer) SessionCapabilities {
	var body struct {
		AgentCapabilities struct {
			SessionCapabilities map[string]json.RawMessage `json:"sessionCapabilities"`
		} `json:"agentCapabilities"`
		Meta struct {
			JetBrains struct {
				Air struct {
					Version      json.RawMessage `json:"version"`
					Capabilities json.RawMessage `json:"capabilities"`
				} `json:"air"`
			} `json:"jetbrains"`
		} `json:"_meta"`
	}
	if json.Unmarshal(initRes, &body) != nil {
		return SessionCapabilities{}
	}
	session := body.AgentCapabilities.SessionCapabilities
	air := decodeAir(body.Meta.JetBrains.Air.Version, body.Meta.JetBrains.Air.Capabilities)
	asyncTasks := o.AsyncTasks && air[airAsyncTasks]
	return SessionCapabilities{
		Fork:               isObject(session["fork"]),
		Subagents:          o.Subagents && isObject(session["subagents"]),
		BackgroundTasks:    asyncTasks,
		BackgroundTaskStop: asyncTasks,
		FileChangeReports:  o.FileChangeReports && air[airFileChangeReports],
	}
}

func decodeAir(version, capabilities json.RawMessage) map[string]bool {
	var v int
	if json.Unmarshal(version, &v) != nil || v < airVersion {
		return nil
	}
	var list []string
	if json.Unmarshal(capabilities, &list) != nil || len(list) > maxAirCapabilities {
		return nil
	}
	out := make(map[string]bool, len(list))
	for _, name := range list {
		out[name] = true
	}
	return out
}

func isObject(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	return len(raw) > 0 && json.Unmarshal(raw, &obj) == nil && obj != nil
}
