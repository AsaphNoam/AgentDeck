package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/runtime"
)

const releaseGroupWorkers = 4

type groupActionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type groupActionResult struct {
	AgentID string            `json:"agent_id"`
	OK      bool              `json:"ok"`
	Error   *groupActionError `json:"error,omitempty"`
}

type groupActionRequest struct {
	Group *string `json:"group"`
}
type groupActionResponse struct {
	Project string              `json:"project"`
	Group   string              `json:"group"`
	Results []groupActionResult `json:"results"`
}

var errGroupLifecycleBusy = errors.New("a lifecycle transition is already in progress")

func normalizeGroup(raw string) (string, *runtime.APIError) {
	group := strings.TrimSpace(raw)
	if group == "" {
		return "", nil
	}
	if group == "_ungrouped" {
		return "", apiError(runtime.CodeInvalidGroupName, "invalid group name")
	}
	return group, nil
}

func (s *Server) handleProjectGroupAction(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	if !config.ValidSlug(project) {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid project"))
		return
	}
	var body groupActionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.Group == nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	group, ae := normalizeGroup(*body.Group)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	if group == "" {
		writeAPIError(w, apiError(runtime.CodeGroupNotFound, "group is required"))
		return
	}
	agents, err := s.stateStore.ListAgents()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	var ids []string
	for _, a := range agents {
		if a.Project == project && !a.Archived && strings.TrimSpace(a.Group) == group {
			ids = append(ids, a.AgentID)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		writeAPIError(w, apiError(runtime.CodeGroupNotFound, "no agents in group: "+group))
		return
	}

	archive := strings.HasSuffix(r.URL.Path, "/archive")
	results, err := s.groupAction(r.Context(), project, group, ids, archive)
	if errors.Is(err, errGroupLifecycleBusy) {
		// Release is all-or-none with respect to lifecycle contention: a 200
		// never leaves a claimed member running while its peers were stopped.
		writeAPIError(w, apiError(runtime.CodeConflict, err.Error()))
		return
	}
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, groupActionResponse{Project: project, Group: group, Results: results})
}

func (s *Server) groupAction(ctx context.Context, project, group string, ids []string, archive bool) ([]groupActionResult, error) {
	projects := make(map[string]string, len(ids))
	archiveClaimed := make([]string, 0, len(ids))
	if archive {
		for _, id := range ids {
			agent, err := s.stateStore.ReadAgent(id)
			if err != nil || agent.Archived || s.beginAgentArchive(ctx, agent.Project, id) != nil {
				for _, claimed := range archiveClaimed {
					s.endAgentArchive(projects[claimed], claimed)
				}
				return nil, errGroupLifecycleBusy
			}
			projects[id] = agent.Project
			archiveClaimed = append(archiveClaimed, id)
		}
	}
	claimed := make([]string, 0, len(ids))
	startLeased := make([]string, 0, len(ids))
	for _, id := range ids {
		if !s.claimLifecycle(id) {
			for _, claimedID := range claimed {
				s.releaseLifecycle(claimedID)
			}
			for _, archivedID := range archiveClaimed {
				s.endAgentArchive(projects[archivedID], archivedID)
			}
			return nil, errGroupLifecycleBusy
		}
		claimed = append(claimed, id)
	}
	if !archive {
		for _, id := range ids {
			agent, err := s.stateStore.ReadAgent(id)
			if err != nil {
				for _, leased := range startLeased {
					s.releaseAgentStart(projects[leased], leased)
				}
				for _, claimedID := range claimed {
					s.releaseLifecycle(claimedID)
				}
				return nil, errGroupLifecycleBusy
			}
			if ae := s.acquireAgentStart(agent.Project, id); ae != nil {
				for _, leased := range startLeased {
					s.releaseAgentStart(projects[leased], leased)
				}
				for _, claimedID := range claimed {
					s.releaseLifecycle(claimedID)
				}
				return nil, errGroupLifecycleBusy
			}
			projects[id] = agent.Project
			startLeased = append(startLeased, id)
		}
	}
	for _, id := range ids {
		agent, err := s.stateStore.ReadAgent(id)
		if err != nil || agent.Project != project || agent.Archived || strings.TrimSpace(agent.Group) != group {
			for _, leased := range startLeased {
				s.releaseAgentStart(projects[leased], leased)
			}
			for _, claimedID := range claimed {
				s.releaseLifecycle(claimedID)
			}
			for _, archivedID := range archiveClaimed {
				s.endAgentArchive(projects[archivedID], archivedID)
			}
			return nil, errGroupLifecycleBusy
		}
	}
	defer func() {
		for _, id := range startLeased {
			s.releaseAgentStart(projects[id], id)
		}
		for _, id := range claimed {
			s.releaseLifecycle(id)
		}
		for _, id := range archiveClaimed {
			s.endAgentArchive(projects[id], id)
		}
	}()
	results := make([]groupActionResult, len(ids))
	jobs := make(chan int)
	workers := len(ids)
	if workers > releaseGroupWorkers {
		workers = releaseGroupWorkers
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				id := ids[idx]
				var ae *runtime.APIError
				if ctx.Err() != nil {
					ae = apiError(runtime.CodeConflict, "group action was cancelled")
				}
				if ae == nil && archive {
					ae = s.prepareArchiveMember(ctx, id)
				} else if ae == nil {
					ae = s.stopAgentClaimed(ctx, id)
				}
				result := groupActionResult{AgentID: id, OK: ae == nil}
				if ae != nil {
					result.Error = &groupActionError{Code: ae.Code, Message: ae.Message}
				}
				results[idx] = result
			}
		}()
	}
	for i := range ids {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if archive {
		stopped := make([]string, 0, len(ids))
		for i, result := range results {
			if result.OK {
				stopped = append(stopped, ids[i])
			}
		}
		if len(stopped) > 0 {
			if err := s.setAgentsArchived(stopped, true); err != nil {
				for _, id := range stopped {
					_, _ = s.stateMgr.Touch(id)
					for i := range ids {
						if ids[i] == id {
							results[i] = groupActionResult{AgentID: id, Error: &groupActionError{Code: runtime.CodeInternal, Message: "stopped but could not be archived: " + err.Error()}}
						}
					}
				}
			} else {
				for _, id := range stopped {
					_, _ = s.stateMgr.Touch(id)
				}
			}
		}
	}
	return results, nil
}

// releaseAgents reserves every member before stopping any of them. This keeps
// Release group faithful to its all-members contract when another lifecycle
// transition is in progress: it returns conflict with no partial release.
