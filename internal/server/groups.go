package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

const releaseGroupWorkers = 4

type releaseGroupResult struct {
	AgentID string `json:"agent_id"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleReleaseGroup(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	agents, err := s.stateStore.ListAgents()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	var ids []string
	for _, a := range agents {
		if a.Group == group {
			ids = append(ids, a.AgentID)
		}
	}
	if len(ids) == 0 {
		writeAPIError(w, apiError(runtime.CodeGroupNotFound, "no agents in group: "+group))
		return
	}

	results := s.releaseAgents(r.Context(), ids)
	writeJSON(w, http.StatusOK, map[string]any{"group": group, "stopped": results})
}

func (s *Server) releaseAgents(ctx context.Context, ids []string) []releaseGroupResult {
	results := make([]releaseGroupResult, len(ids))
	jobs := make(chan int)
	workers := releaseGroupWorkers
	if len(ids) < workers {
		workers = len(ids)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				id := ids[idx]
				res := releaseGroupResult{AgentID: id, OK: true}
				// Release runs the shared stop-and-teardown seam, so a member
				// whose start/stop transition is already in flight is reported
				// as not stopped instead of tearing down that transition's
				// registration (FS-01.R34, INV §2/§4).
				if ae := s.stopAgent(ctx, id); ae != nil {
					res.OK = false
					res.Error = ae.Message
				}
				results[idx] = res
			}
		}()
	}
	for i := range ids {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}
