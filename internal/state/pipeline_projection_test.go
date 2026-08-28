package state

import (
	"strings"
	"testing"
	"time"
)

func TestPipelineDelegatedTasksUsesCreatorWindowsAndIndex(t *testing.T) {
	st, _ := newTestStore(t)
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	for _, agent := range []Agent{
		{AgentID: "a_stage", Name: "Stage", Role: "implementer", Project: "app", Interface: "chat", CreatedAt: base},
		{AgentID: "a_delegate", Name: "Delegate", Role: "implementer", Project: "app", Interface: "chat", CreatedAt: base},
	} {
		if err := st.WriteAgent(agent); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.WriteStatus(Status{AgentID: "a_stage", State: "busy", Detail: "stage preview"}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteStatus(Status{AgentID: "a_delegate", State: "idle", Detail: "delegate preview"}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteRunning(RunningEntry{AgentID: "a_delegate", PID: 1, Interface: "chat", StartedAt: base}); err != nil {
		t.Fatal(err)
	}
	insert := func(id, creator, generation, assignee string, at time.Time) {
		t.Helper()
		if _, err := st.DB().Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, state,
  created_by_kind, created_by_agent_id, created_by_generation, assigned_agent_id, created_at, updated_at)
VALUES (?, 'app', ?, '', 'agent', 'finished', 'agent', ?, ?, ?, ?, ?)`,
			id, id, creator, generation, assignee, formatTime(at), formatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 22; i++ {
		insert("first-"+string(rune('a'+i)), "a_stage", "g1", "a_delegate", base.Add(time.Duration(i+1)*time.Second))
	}
	insert("second", "a_stage", "g1", "a_delegate", base.Add(31*time.Second))
	insert("second-hop", "a_delegate", "g2", "a_stage", base.Add(32*time.Second))
	next := base.Add(30 * time.Second)
	windows := []PipelineAttemptWindow{
		{AttemptID: "pa_1", AgentID: "a_stage", AgentGeneration: "g1", CreatedAt: base, NextCreatedAt: &next},
		{AttemptID: "pa_2", AgentID: "a_stage", AgentGeneration: "g1", CreatedAt: next},
	}
	tasks, err := st.PipelineDelegatedTasks("app", windows, 20)
	if err != nil {
		t.Fatal(err)
	}
	var first, second []PipelineDelegatedTask
	for _, task := range tasks {
		switch task.AttemptID {
		case "pa_1":
			first = append(first, task)
		case "pa_2":
			second = append(second, task)
		default:
			t.Fatalf("unexpected attempt %q", task.AttemptID)
		}
	}
	if len(first) != 20 || first[0].DelegatedTotal != 22 || first[0].DelegatedRunningCount != 1 {
		t.Fatalf("first projection = %+v", first)
	}
	if len(second) != 1 || second[0].TaskID != "second" || second[0].DelegatedTotal != 1 {
		t.Fatalf("second projection = %+v", second)
	}
	if first[0].Agent.Name != "Delegate" || !first[0].Agent.Running || !first[0].Agent.StatusFound {
		t.Fatalf("delegate snapshot = %+v", first[0].Agent)
	}

	query, args := pipelineDelegatedTaskQuery("app", windows, 20)
	rows, err := st.DB().Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	usedIndex := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		usedIndex = usedIndex || strings.Contains(detail, "idx_tasks_project_creator_created")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !usedIndex {
		t.Fatal("delegated-task query did not use idx_tasks_project_creator_created")
	}
}

func TestPipelineAgentSnapshotsAndDelegatesPreserveMissingState(t *testing.T) {
	st, _ := newTestStore(t)
	now := time.Now().UTC()
	if err := st.WriteAgent(Agent{AgentID: "a_status_missing", Name: "Known", Role: "implementer", Project: "app", Interface: "chat", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	snapshots, err := st.PipelineAgentSnapshots([]string{"a_status_missing", "a_missing"})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshots["a_status_missing"]; !got.IdentityFound || got.StatusFound {
		t.Fatalf("known snapshot = %+v", got)
	}
	if _, ok := snapshots["a_missing"]; ok {
		t.Fatalf("missing identity produced a snapshot: %+v", snapshots)
	}
	if tasks, err := st.PipelineDelegatedTasks("app", nil, 20); err != nil || tasks == nil {
		t.Fatalf("empty projection = %#v err=%v, want [] nil", tasks, err)
	}
}
