package runtime

import "github.com/agentdeck/agentdeck/internal/state"

// ActivationKind is the code-owned contract for one host-owned turn kind: the
// fixed instruction the provider receives and the status the dashboard shows
// while that turn runs. It is the only activation data that reaches the model
// (TS-01.R21), and it lives in one table rather than as literals at the call
// site so a new kind is a row here instead of a branch inside the runtime, and
// so no kind can inherit another's instruction by accident (TS-10.R5, FS-16.R26,
// INV §2).
type ActivationKind struct {
	// Instruction is the entire prompt. It names the tool the agent should call
	// and carries no payload from the source domain.
	Instruction string
	// StatusDetail and LastTrace are the bounded, in-vocabulary status the card
	// shows for the turn (INV §8).
	StatusDetail string
	LastTrace    string
}

// activationKinds is the closed registry. An activation whose kind is absent
// here cannot start, so an unregistered kind fails loudly at its first use
// rather than prompting a model with another kind's instruction.
var activationKinds = map[string]ActivationKind{
	state.ActivationKindMail: {
		Instruction:  "You have new messages. Call the check_messages tool and handle them.",
		StatusDetail: "checking messages",
		LastTrace:    "MailActivation",
	},
	// An agent told to check its messages will do exactly that and never find its
	// task, so dependent work has its own instruction and its own status. It
	// carries no task id, arm set, or assignment text: the agent reads all of that
	// through get_assigned_task (FS-16.R26, R11).
	state.ActivationKindDependency: {
		Instruction:  "You have been assigned a task. Call the get_assigned_task tool to read your assignment, then carry it out.",
		StatusDetail: "starting assigned task",
		LastTrace:    "TaskActivation",
	},
}

// LookupActivationKind returns the contract for a kind and whether it exists.
func LookupActivationKind(kind string) (ActivationKind, bool) {
	k, ok := activationKinds[kind]
	return k, ok
}

// ActivationKinds returns every registered kind name, so a test can assert the
// registry and the state layer's kind vocabulary have not drifted apart.
func ActivationKinds() []string {
	kinds := make([]string, 0, len(activationKinds))
	for kind := range activationKinds {
		kinds = append(kinds, kind)
	}
	return kinds
}
