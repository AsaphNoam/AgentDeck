package runtime

import "github.com/AsaphNoam/Chuck/internal/state"

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
		Instruction:  "Handle the attributed peer mail supplied with this turn.",
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
	// A room turn names only the room tools. Goal, peer contributions, role and
	// remaining ceiling are pulled through read_think_tank as data, never
	// carried in the prompt (TS-14.R3, FS-21.R16).
	state.ActivationKindThinkTank: {
		Instruction: "It is your turn in a Think Tank discussion. Call read_think_tank to read the goal, " +
			"your role, your remaining turn ceiling and the new discussion, following its cursor until the " +
			"read is complete. Then call submit_think_tank_turn once with your contribution and the read_receipt.",
		StatusDetail: "taking a Think Tank turn",
		LastTrace:    "ThinkTankActivation",
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
