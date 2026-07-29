package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	persistarchive "github.com/agentdeck/agentdeck/internal/archive"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

type archiveAgentResult struct {
	persistarchive.Result
	Archived bool `json:"archived"`
}

type archiveProjectGroup struct {
	Project            string               `json:"project"`
	Title              string               `json:"title"`
	Color              [3]int               `json:"color"`
	ProjectStatus      string               `json:"project_status"`
	ArchivedAgentCount int                  `json:"archived_agent_count"`
	Results            []archiveAgentResult `json:"results"`
}

type archiveGroupsResponse struct {
	Query      string                `json:"query,omitempty"`
	SearchMode string                `json:"search_mode"`
	Total      int                   `json:"total"`
	Limit      int                   `json:"limit"`
	Offset     int                   `json:"offset"`
	Results    []archiveProjectGroup `json:"results"`
}

func parseArchiveQuery(r *http.Request) (persistarchive.Query, *runtime.APIError) {
	limit, err := parseIntQuery(r, "limit", 50)
	if err != nil || limit < 1 || limit > 200 {
		return persistarchive.Query{}, apiError(runtime.CodeValidation, "limit must be between 1 and 200")
	}
	offset, err := parseIntQuery(r, "offset", 0)
	if err != nil || offset < 0 {
		return persistarchive.Query{}, apiError(runtime.CodeValidation, "offset must be >= 0")
	}
	q := persistarchive.Query{Q: r.URL.Query().Get("q"), Limit: limit, Offset: offset}
	switch r.URL.Query().Get("active") {
	case "":
	case "true":
		v := true
		q.Active = &v
	case "false":
		v := false
		q.Active = &v
	default:
		return persistarchive.Query{}, apiError(runtime.CodeValidation, "active must be true or false")
	}
	return q, nil
}

func (s *Server) archiveRows(q persistarchive.Query, project string) ([]archiveAgentResult, string, error) {
	// Grouping is a server projection. Search remains authoritative for the
	// all-session corpus; archive state only controls the ordinary no-filter view.
	fetch := q
	fetch.Limit, fetch.Offset, fetch.Project = 200, 0, project
	flatResults := make([]persistarchive.Result, 0)
	mode := "metadata"
	for {
		flat, err := persistarchive.New(s.stateStore.DB()).Search(fetch)
		if err != nil {
			return nil, "", err
		}
		mode = flat.SearchMode
		flatResults = append(flatResults, flat.Results...)
		if len(flatResults) >= flat.Total || len(flat.Results) == 0 {
			break
		}
		fetch.Offset += len(flat.Results)
	}
	agents, err := s.stateStore.ListAgents()
	if err != nil {
		return nil, "", err
	}
	archived := make(map[string]bool, len(agents))
	for _, agent := range agents {
		archived[agent.AgentID] = agent.Archived
	}
	rows := make([]archiveAgentResult, 0, len(flatResults))
	for _, result := range flatResults {
		row := archiveAgentResult{Result: result, Archived: archived[result.AgentID]}
		if strings.TrimSpace(q.Q) == "" && q.Active == nil && !row.Archived {
			continue
		}
		rows = append(rows, row)
	}
	return rows, mode, nil
}

func (s *Server) archiveGroups(q persistarchive.Query) (archiveGroupsResponse, error) {
	rows, mode, err := s.archiveRows(q, "")
	if err != nil {
		return archiveGroupsResponse{}, err
	}
	projects, err := s.configStore.ListProjects()
	if err != nil {
		return archiveGroupsResponse{}, err
	}
	agents, err := s.stateStore.ListAgents()
	if err != nil {
		return archiveGroupsResponse{}, err
	}
	counts := map[string]int{}
	for _, agent := range agents {
		if agent.Archived {
			counts[agent.Project]++
		}
	}
	groups := map[string]*archiveProjectGroup{}
	ensure := func(id string) *archiveProjectGroup {
		if group := groups[id]; group != nil {
			return group
		}
		group := &archiveProjectGroup{Project: id, Title: id, ProjectStatus: "missing", Results: []archiveAgentResult{}}
		if project, ok := projects[id]; ok {
			group.Title, group.Color = project.Title, project.Color
			if project.Archived {
				group.ProjectStatus = "archived"
			} else {
				group.ProjectStatus = "active"
			}
		}
		group.ArchivedAgentCount = counts[id]
		groups[id] = group
		return group
	}
	for _, row := range rows {
		ensure(row.Project).Results = append(ensure(row.Project).Results, row)
	}
	if strings.TrimSpace(q.Q) == "" && q.Active == nil {
		for id, project := range projects {
			if project.Archived || counts[id] > 0 {
				ensure(id)
			}
		}
	}
	ordered := make([]archiveProjectGroup, 0, len(groups))
	for _, group := range groups {
		ordered = append(ordered, *group)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := strings.ToLower(ordered[i].Title), strings.ToLower(ordered[j].Title)
		if a == b {
			return ordered[i].Project < ordered[j].Project
		}
		return a < b
	})
	total := len(ordered)
	start := q.Offset
	if start > total {
		start = total
	}
	end := start + q.Limit
	if end > total {
		end = total
	}
	return archiveGroupsResponse{Query: strings.TrimSpace(q.Q), SearchMode: mode, Total: total, Limit: q.Limit, Offset: q.Offset, Results: append([]archiveProjectGroup{}, ordered[start:end]...)}, nil
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	q, ae := parseArchiveQuery(r)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	resp, err := s.archiveGroups(q)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleArchiveProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	if !config.ValidSlug(projectID) {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid project"))
		return
	}
	q, ae := parseArchiveQuery(r)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	rows, mode, err := s.archiveRows(q, projectID)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	filtered := make([]archiveAgentResult, 0)
	filtered = append(filtered, rows...)
	if strings.TrimSpace(q.Q) == "" && q.Active == nil {
		filtered = filtered[:0]
		for _, row := range rows {
			if row.Archived {
				filtered = append(filtered, row)
			}
		}
	}
	total := len(filtered)
	start := q.Offset
	if start > total {
		start = total
	}
	end := start + q.Limit
	if end > total {
		end = total
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": strings.TrimSpace(q.Q), "search_mode": mode, "total": total, "limit": q.Limit, "offset": q.Offset, "results": append([]archiveAgentResult{}, filtered[start:end]...)})
}

func parseIntQuery(r *http.Request, key string, def int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}
