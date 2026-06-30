package server

import (
	"context"
	"errors"
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
				if err := s.registry.Stop(ctx, id); err != nil && !errors.Is(err, runtime.ErrNoHandle) {
					res.OK = false
					res.Error = err.Error()
				}
				if res.OK {
					s.cleanupMessagingMCP(id)
					s.cleanupHookSettings(id)
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
