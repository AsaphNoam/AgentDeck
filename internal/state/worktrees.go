package state

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AgentDeck-owned worktree ownership (TS-12.R2). A project_worktrees row exists
// exactly for checkouts AgentDeck created, and its presence is the sole
// ownership test: a project whose cwd merely lies inside someone else's
// worktree has no row and therefore no deletion path at all (FS-19.R4).
//
// The row is written only after the checkout exists and removed only after the
// checkout is gone (INV §15), so a crash can leave a checkout that is not
// recorded as owned — inert, treated as external — but never a record that
// authorizes deleting something AgentDeck did not create.

// setupOutputLimit bounds the stored tail of a setup run so one noisy bootstrap
// cannot grow the state database without limit (TS-12.R5).
const setupOutputLimit = 64 * 1024

// ProjectWorktree is one ownership record.
type ProjectWorktree struct {
	Project      string     `json:"project"`
	RepoPath     string     `json:"repo_path"`
	Branch       string     `json:"branch"`
	CheckoutPath string     `json:"checkout_path"`
	CreatedAt    time.Time  `json:"created_at"`
	SetupOK      *bool      `json:"setup_ok"`
	SetupAt      *time.Time `json:"setup_at"`
	SetupOutput  string     `json:"setup_output"`
}

// InsertProjectWorktree records ownership of a freshly created checkout. A
// second insert for the same project conflicts rather than silently re-pointing
// an existing record at another checkout.
func (s *Store) InsertProjectWorktree(w ProjectWorktree) error {
	created := w.CreatedAt
	if created.IsZero() {
		created = timeNow()
	}
	_, err := s.db.Exec(
		`INSERT INTO project_worktrees(project, repo_path, branch, checkout_path, created_at, setup_ok, setup_at, setup_output)
		 VALUES (?, ?, ?, ?, ?, NULL, NULL, '')`,
		w.Project, w.RepoPath, w.Branch, w.CheckoutPath, formatTime(created),
	)
	if err != nil {
		return fmt.Errorf("state: insert project worktree %s: %w", w.Project, err)
	}
	return nil
}

// ReadProjectWorktree returns the ownership row for a project, or ErrNotFound
// when the project owns no checkout.
func (s *Store) ReadProjectWorktree(project string) (ProjectWorktree, error) {
	row := s.db.QueryRow(
		`SELECT project, repo_path, branch, checkout_path, created_at, setup_ok, setup_at, setup_output
		 FROM project_worktrees WHERE project = ?`, project)
	w, err := scanProjectWorktree(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectWorktree{}, ErrNotFound
	}
	if err != nil {
		return ProjectWorktree{}, fmt.Errorf("state: read project worktree %s: %w", project, err)
	}
	return w, nil
}

// ListProjectWorktrees returns every ownership row keyed by project id. It is
// the subprocess-free source for the projects-list enrichment (TS-12.R6).
func (s *Store) ListProjectWorktrees() (map[string]ProjectWorktree, error) {
	rows, err := s.db.Query(
		`SELECT project, repo_path, branch, checkout_path, created_at, setup_ok, setup_at, setup_output
		 FROM project_worktrees`)
	if err != nil {
		return nil, fmt.Errorf("state: list project worktrees: %w", err)
	}
	defer rows.Close()
	out := map[string]ProjectWorktree{}
	for rows.Next() {
		w, err := scanProjectWorktree(rows)
		if err != nil {
			return nil, fmt.Errorf("state: scan project worktree: %w", err)
		}
		out[w.Project] = w
	}
	// rows.Err() is the only iteration-failure signal; without it a mid-iteration
	// failure silently truncates the map (INV §7).
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate project worktrees: %w", err)
	}
	return out, nil
}

// RecordProjectWorktreeSetup stores the outcome of a setup run on the ownership
// row so the warning survives the request that produced it (TS-12.R5). The
// captured output is clipped to the stored tail bound.
func (s *Store) RecordProjectWorktreeSetup(project string, ok bool, output string) error {
	if len(output) > setupOutputLimit {
		output = output[len(output)-setupOutputLimit:]
	}
	okValue := 0
	if ok {
		okValue = 1
	}
	res, err := s.db.Exec(
		`UPDATE project_worktrees SET setup_ok = ?, setup_at = ?, setup_output = ? WHERE project = ?`,
		okValue, formatTime(timeNow()), output, project,
	)
	if err != nil {
		return fmt.Errorf("state: record worktree setup %s: %w", project, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("state: record worktree setup %s: %w", project, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProjectWorktree removes an ownership row. It runs only after the
// checkout it describes is gone (TS-12.R7); a missing row is tolerated.
func (s *Store) DeleteProjectWorktree(project string) error {
	if _, err := s.db.Exec(`DELETE FROM project_worktrees WHERE project = ?`, project); err != nil {
		return fmt.Errorf("state: delete project worktree %s: %w", project, err)
	}
	return nil
}

func scanProjectWorktree(row rowScanner) (ProjectWorktree, error) {
	var (
		w         ProjectWorktree
		createdAt string
		setupOK   sql.NullInt64
		setupAt   sql.NullString
		setupOut  sql.NullString
	)
	if err := row.Scan(&w.Project, &w.RepoPath, &w.Branch, &w.CheckoutPath, &createdAt, &setupOK, &setupAt, &setupOut); err != nil {
		return ProjectWorktree{}, err
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return ProjectWorktree{}, wrapTimeErr("project_worktrees.created_at", err)
	}
	w.CreatedAt = created
	if setupOK.Valid {
		ok := setupOK.Int64 != 0
		w.SetupOK = &ok
	}
	at, err := parseOptionalTime(setupAt)
	if err != nil {
		return ProjectWorktree{}, wrapTimeErr("project_worktrees.setup_at", err)
	}
	w.SetupAt = at
	w.SetupOutput = setupOut.String
	return w, nil
}
