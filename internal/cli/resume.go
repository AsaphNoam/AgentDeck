package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
)

// runResumeByID POSTs to /api/sessions/{id}/resume and prints the result.
func runResumeByID(agentID string) int {
	cfgStore, cerr := config.New()
	if cerr != nil {
		fmt.Printf("config: %v\n", cerr)
		return 1
	}
	port := dashboardPort(cfgStore)

	resp, body, err := postResume(port, agentID, nil)
	if err != nil {
		fmt.Printf("could not reach dashboard on 127.0.0.1:%d — run `agentdeck dashboard start` first (%v)\n", port, err)
		return 1
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("resume failed (%d): %s\n", resp.StatusCode, strings.TrimSpace(string(body)))
		return 1
	}

	var out struct {
		Agent struct {
			AgentID string `json:"agent_id"`
			Name    string `json:"name"`
			Role    string `json:"role"`
			Project string `json:"project"`
		} `json:"agent"`
	}
	_ = json.Unmarshal(body, &out)
	fmt.Printf("resumed %s (%s) %s@%s\n", out.Agent.Name, out.Agent.AgentID, out.Agent.Role, out.Agent.Project)
	return 0
}

// postResume POSTs to /api/sessions/{id}/resume with an optional body.
func postResume(port int, agentID string, body map[string]string) (*http.Response, []byte, error) {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/sessions/%s/resume", port, agentID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data, nil
}

// archiveResult is one session from GET /api/archive.
type archiveResult struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Project string `json:"project"`
	Active  bool   `json:"active"`
}

// listInactiveSessions queries the archive endpoint for inactive sessions
// matching the given role and project.
func listInactiveSessions(port int, role, project string) ([]archiveResult, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/archive?active=false&limit=200", port)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var ar struct {
		Results []archiveResult `json:"results"`
	}
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, err
	}

	var matches []archiveResult
	for _, r := range ar.Results {
		if !r.Active && r.Role == role && r.Project == project {
			matches = append(matches, r)
		}
	}
	return matches, nil
}
