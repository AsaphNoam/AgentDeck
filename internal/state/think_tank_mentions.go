package state

import (
	"encoding/json"
	"sort"
	"unicode/utf8"
)

// ThinkTankMaxMentions bounds the selected addressees of one shared message
// (TS-14.R25).
const ThinkTankMaxMentions = 32

// ThinkTankMention is one picker-selected addressee: a participant id over a
// UTF-8 byte range of the submitted body. Labels are presentation only.
type ThinkTankMention struct {
	AgentID string `json:"agent_id"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
}

// ThinkTankAddressee is the snapshot of a mention recorded with the input and
// its published entry, readable by every participant.
type ThinkTankAddressee struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
}

type thinkTankMentionSnapshot struct {
	Addressees []ThinkTankAddressee `json:"addressees"`
}

// ThinkTankEntryAddressees returns the addressees recorded in an input or
// entry context; contexts without mentions return none.
func ThinkTankEntryAddressees(context string) []ThinkTankAddressee {
	if context == "" {
		return nil
	}
	var c thinkTankMentionSnapshot
	if json.Unmarshal([]byte(context), &c) != nil {
		return nil
	}
	return c.Addressees
}

func sortedThinkTankMentions(mentions []ThinkTankMention) []ThinkTankMention {
	out := append([]ThinkTankMention{}, mentions...)
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

func sameThinkTankMentions(context string, mentions []ThinkTankMention) bool {
	got := ThinkTankEntryAddressees(context)
	want := sortedThinkTankMentions(mentions)
	if len(got) != len(want) {
		return false
	}
	for i, a := range got {
		if a.AgentID != want[i].AgentID || a.Start != want[i].Start || a.End != want[i].End {
			return false
		}
	}
	return true
}

// thinkTankMentionContext validates mentions against the body and the room's
// live participants, then snapshots them. Judges, departed members and
// unknown ids cannot be addressed; deleted agents are refused by the caller.
func thinkTankMentionContext(d ThinkTankDetail, body string, mentions []ThinkTankMention) (string, error) {
	if len(mentions) > ThinkTankMaxMentions {
		return "", thinkTankInvalid("a message can address at most %d participants", ThinkTankMaxMentions)
	}
	members := map[string]ThinkTankMember{}
	for _, m := range d.Members {
		members[m.AgentID] = m
	}
	out := thinkTankMentionSnapshot{Addressees: []ThinkTankAddressee{}}
	prevEnd := 0
	for _, m := range sortedThinkTankMentions(mentions) {
		if m.Start < prevEnd || m.Start >= m.End || m.End > len(body) ||
			!utf8.RuneStart(body[m.Start]) || (m.End < len(body) && !utf8.RuneStart(body[m.End])) {
			return "", thinkTankInvalid("mention ranges must be ordered, non-overlapping character ranges of the message")
		}
		prevEnd = m.End
		member, ok := members[m.AgentID]
		if !ok || member.Role != ThinkTankRoleParticipant {
			return "", thinkTankInvalid("%s is not a participant", m.AgentID)
		}
		if member.State == ThinkTankMemberDeparted {
			return "", thinkTankConflict("%s has left the discussion", member.AgentName)
		}
		out.Addressees = append(out.Addressees, ThinkTankAddressee{AgentID: member.AgentID, Name: member.AgentName,
			Project: member.Project, Start: m.Start, End: m.End})
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	if len(body)+len(raw) > ThinkTankMaxTextBytes {
		return "", thinkTankInvalid("message exceeds %d bytes", ThinkTankMaxTextBytes)
	}
	return string(raw), nil
}
