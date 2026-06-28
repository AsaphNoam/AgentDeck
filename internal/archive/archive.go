package archive

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type Archive struct {
	db *sql.DB
}

type Query struct {
	Q      string
	Limit  int
	Offset int
	Active *bool
}

type Response struct {
	Query   string   `json:"query,omitempty"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	Results []Result `json:"results"`
}

type Result struct {
	AgentID      string   `json:"agent_id"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Project      string   `json:"project"`
	Backend      string   `json:"backend"`
	Model        string   `json:"model"`
	Interface    string   `json:"interface"`
	Group        string   `json:"group,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	TurnCount    int      `json:"turn_count"`
	FilesTouched int      `json:"files_touched"`
	CommandsRun  int      `json:"commands_run"`
	Active       bool     `json:"active"`
	MatchedIn    []string `json:"matched_in,omitempty"`
	Snippet      string   `json:"snippet,omitempty"`

	content string
}

func New(db *sql.DB) *Archive {
	return &Archive{db: db}
}

func (a *Archive) Search(q Query) (Response, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	term := strings.TrimSpace(q.Q)
	var (
		results []Result
		total   int
		err     error
	)
	if term == "" {
		total, results, err = a.list(q)
	} else {
		total, results, err = a.search(q, ftsQuery(term), term)
	}
	if err != nil {
		return Response{}, err
	}
	return Response{Query: term, Total: total, Limit: q.Limit, Offset: q.Offset, Results: results}, nil
}

func (a *Archive) list(q Query) (int, []Result, error) {
	where, args := activeWhere(q.Active, "WHERE")
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM sessions s LEFT JOIN running r ON r.agent_id = s.agent_id`+where, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("archive: count list: %w", err)
	}
	args = append(args, q.Limit, q.Offset)
	rows, err := a.db.Query(`
SELECT s.agent_id, s.name, s.role, s.project, s.backend, s.model, s.interface, s.grp,
       s.created_at, s.updated_at, s.turn_count, s.files_touched, s.commands_run,
       (r.agent_id IS NOT NULL) AS active, '' AS snippet, '' AS content
FROM sessions s
LEFT JOIN running r ON r.agent_id = s.agent_id`+where+`
ORDER BY s.updated_at DESC, s.agent_id
LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return 0, nil, fmt.Errorf("archive: list: %w", err)
	}
	defer rows.Close()
	results, err := scanResults(rows)
	return total, results, err
}

func (a *Archive) search(q Query, match, raw string) (int, []Result, error) {
	where, args := activeWhere(q.Active, "AND")
	countArgs := append([]any{match}, args...)
	var total int
	if err := a.db.QueryRow(`
SELECT COUNT(*)
FROM sessions_fts
JOIN sessions s ON s.agent_id = sessions_fts.agent_id
LEFT JOIN running r ON r.agent_id = s.agent_id
WHERE sessions_fts MATCH ?`+where, countArgs...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("archive: count search: %w", err)
	}
	queryArgs := append([]any{match}, args...)
	queryArgs = append(queryArgs, q.Limit, q.Offset)
	rows, err := a.db.Query(`
SELECT s.agent_id, s.name, s.role, s.project, s.backend, s.model, s.interface, s.grp,
       s.created_at, s.updated_at, s.turn_count, s.files_touched, s.commands_run,
       (r.agent_id IS NOT NULL) AS active,
       snippet(sessions_fts, 7, '', '', '...', 12) AS snippet,
       sessions_fts.content
FROM sessions_fts
JOIN sessions s ON s.agent_id = sessions_fts.agent_id
LEFT JOIN running r ON r.agent_id = s.agent_id
WHERE sessions_fts MATCH ?`+where+`
ORDER BY bm25(sessions_fts, 8.0, 4.0, 4.0, 2.0, 1.0, 1.0, 1.0, 1.0), s.updated_at DESC, s.agent_id
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return 0, nil, fmt.Errorf("archive: search: %w", err)
	}
	defer rows.Close()
	results, err := scanResults(rows)
	if err != nil {
		return 0, nil, err
	}
	for i := range results {
		results[i].MatchedIn = matchedIn(results[i], raw)
		results[i].content = ""
	}
	return total, results, nil
}

func activeWhere(active *bool, prefix string) (string, []any) {
	if active == nil {
		return "", nil
	}
	if *active {
		return " " + prefix + " r.agent_id IS NOT NULL", nil
	}
	return " " + prefix + " r.agent_id IS NULL", nil
}

func scanResults(rows *sql.Rows) ([]Result, error) {
	var out []Result
	for rows.Next() {
		var r Result
		var active int
		if err := rows.Scan(&r.AgentID, &r.Name, &r.Role, &r.Project, &r.Backend, &r.Model, &r.Interface, &r.Group,
			&r.CreatedAt, &r.UpdatedAt, &r.TurnCount, &r.FilesTouched, &r.CommandsRun, &active, &r.Snippet, &r.content); err != nil {
			return nil, fmt.Errorf("archive: scan result: %w", err)
		}
		r.Active = active != 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archive: iterate results: %w", err)
	}
	return out, nil
}

func ftsQuery(q string) string {
	parts := splitTerms(q)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ReplaceAll(p, `"`, `""`)
		quoted = append(quoted, `"`+p+`"`)
	}
	return strings.Join(quoted, " ")
}

func splitTerms(q string) []string {
	var out []string
	for len(q) > 0 {
		q = strings.TrimSpace(q)
		if q == "" {
			break
		}
		if q[0] == '"' {
			end := strings.Index(q[1:], `"`)
			if end >= 0 {
				out = append(out, q[1:1+end])
				q = q[2+end:]
				continue
			}
		}
		fields := strings.Fields(q)
		if len(fields) == 0 {
			break
		}
		out = append(out, fields[0])
		q = strings.TrimPrefix(q, fields[0])
	}
	return out
}

func matchedIn(r Result, q string) []string {
	terms := splitTerms(strings.ToLower(q))
	meta := strings.ToLower(strings.Join([]string{r.Name, r.Role, r.Project, r.Group, r.Model, r.Backend}, " "))
	content := strings.ToLower(r.content)
	hitMeta, hitContent := allContain(meta, terms), allContain(content, terms)
	out := make([]string, 0, 2)
	if hitMeta {
		out = append(out, "metadata")
	}
	if hitContent {
		out = append(out, "transcript")
	}
	if len(out) == 0 {
		out = append(out, "transcript")
	}
	return out
}

func allContain(haystack string, terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		if _, err := strconv.Unquote(`"` + term + `"`); err != nil {
			return false
		}
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
