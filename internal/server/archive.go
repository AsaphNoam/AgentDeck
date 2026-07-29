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

func (s *Server) archiveRows(q persistarchive.Query, project string) ([]archiveAgentResult, int, string, error) {
	q.Project = project
	q.ArchivedOnly = strings.TrimSpace(q.Q) == "" && q.Active == nil
	flat, err := persistarchive.New(s.stateStore.DB()).Search(q)
	if err != nil {
		return nil, 0, "", err
	}
	rows := make([]archiveAgentResult, 0, len(flat.Results))
	for _, result := range flat.Results {
		rows = append(rows, archiveAgentResult{Result: result})
	}
	return rows, flat.Total, flat.SearchMode, nil
}

func (s *Server) archiveGroups(q persistarchive.Query) (archiveGroupsResponse, error) {
	archive := persistarchive.New(s.stateStore.DB())
	projectQuery := q
	projectQuery.ArchivedOnly = strings.TrimSpace(q.Q) == "" && q.Active == nil
	matchingProjects, mode, err := archive.Projects(projectQuery)
	if err != nil {
		return archiveGroupsResponse{}, err
	}
	projects, err := s.configStore.ListProjects()
	if err != nil {
		return archiveGroupsResponse{}, err
	}
	counts, err := s.stateStore.ArchivedAgentCounts()
	if err != nil {
		return archiveGroupsResponse{}, err
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
	if strings.TrimSpace(q.Q) == "" && q.Active == nil {
		for id, project := range projects {
			if project.Archived || counts[id] > 0 {
				ensure(id)
			}
		}
		for id := range counts {
			ensure(id)
		}
	} else {
		for _, id := range matchingProjects {
			ensure(id)
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
	rows, total, mode, err := s.archiveRows(q, projectID)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": strings.TrimSpace(q.Q), "search_mode": mode, "total": total, "limit": q.Limit, "offset": q.Offset, "results": append([]archiveAgentResult{}, rows...)})
}

func parseIntQuery(r *http.Request, key string, def int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}
