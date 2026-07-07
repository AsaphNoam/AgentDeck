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

// launchArgs is the parsed `agentdeck <role>@<project> [flags]` invocation.
type launchArgs struct {
	Role      string
	Project   string
	Backend   string
	Model     string
	Interface string
	Name      string
	Group     string
	ForceNew  bool   // --new: always POST /api/sessions
	ResumeID  string // --resume <id>: explicit resume by agent_id
}

// launchBody is the POST /api/sessions request body (techspec §6.5, §7.1). Empty
// optional fields are omitted so the server applies its defaults.
type launchBody struct {
	Role      string `json:"role"`
	Project   string `json:"project"`
	Backend   string `json:"backend,omitempty"`
	Model     string `json:"model,omitempty"`
	Interface string `json:"interface,omitempty"`
	Name      string `json:"name,omitempty"`
	Group     string `json:"group,omitempty"`
}

func (a launchArgs) body() launchBody {
	return launchBody{
		Role: a.Role, Project: a.Project, Backend: a.Backend, Model: a.Model,
		Interface: a.Interface, Name: a.Name, Group: a.Group,
	}
}

// parseLaunch parses the role@project positional plus flags. The positional
// splits on the LAST "@" so a role may itself contain no "@" but the form is
// unambiguous (techspec §6.5).
func parseLaunch(args []string) (launchArgs, error) {
	if len(args) == 0 {
		return launchArgs{}, fmt.Errorf("missing <role>@<project>")
	}
	at := strings.LastIndex(args[0], "@")
	if at <= 0 || at == len(args[0])-1 {
		return launchArgs{}, fmt.Errorf("invalid launch syntax %q (expected <role>@<project>)", args[0])
	}
	out := launchArgs{Role: args[0][:at], Project: args[0][at+1:], Interface: "chat"}

	// Minimal flag parsing for the reserved launch form.
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		flag := rest[i]
		// val consumes the next token as the flag's value, requiring a present
		// non-flag operand. Without this, a value flag given last or before another
		// flag (e.g. `impl@proj --resume`) silently took "" and fell through to a
		// fresh launch instead of failing fast.
		val := func() (string, error) {
			if i+1 >= len(rest) || strings.HasPrefix(rest[i+1], "--") {
				return "", fmt.Errorf("flag %q requires a value", flag)
			}
			i++
			return rest[i], nil
		}
		var err error
		switch flag {
		case "--backend":
			out.Backend, err = val()
		case "--model":
			out.Model, err = val()
		case "--interface":
			out.Interface, err = val()
		case "--name":
			out.Name, err = val()
		case "--group":
			out.Group, err = val()
		case "--new":
			out.ForceNew = true
		case "--resume":
			out.ResumeID, err = val()
		default:
			return launchArgs{}, fmt.Errorf("unknown flag %q", flag)
		}
		if err != nil {
			return launchArgs{}, err
		}
	}
	return out, nil
}

// runLaunch parses the launch form and POSTs it to the running dashboard's
// /api/sessions endpoint (CLI and modal produce an identical agent, §6.5).
// With --resume <id> or bare-form single-inactive-match, resumes instead.
func runLaunch(args []string) int {
	la, err := parseLaunch(args)
	if err != nil {
		fmt.Println(err)
		return 2
	}

	cfgStore, cerr := config.New()
	if cerr != nil {
		fmt.Printf("config: %v\n", cerr)
		return 1
	}
	port := dashboardPort(cfgStore)

	// Explicit --resume <id>: bypass role@project launch entirely.
	if la.ResumeID != "" {
		return runResumeByID(la.ResumeID)
	}

	// --new forces a fresh launch with no inactive-session check.
	if !la.ForceNew {
		// Bare-form: check for a single inactive match with the same role@project.
		matches, merr := listInactiveSessions(port, la.Role, la.Project, la.Name)
		if merr == nil && len(matches) == 1 {
			fmt.Printf("resuming existing session %s (%s)...\n", matches[0].Name, matches[0].AgentID)
			return runResumeByID(matches[0].AgentID)
		}
		if merr == nil && len(matches) > 1 {
			fmt.Printf("multiple inactive %s@%s sessions found — specify one with --resume <id> or use --new:\n", la.Role, la.Project)
			for _, m := range matches {
				fmt.Printf("  %s  %s\n", m.AgentID, m.Name)
			}
			return 1
		}
		// No inactive match or archive unavailable → fall through to new launch.
	}

	resp, body, err := postLaunch(port, la.body())
	if err != nil {
		fmt.Printf("could not reach dashboard on 127.0.0.1:%d — run `agentdeck dashboard start` first (%v)\n", port, err)
		return 1
	}
	if resp.StatusCode != http.StatusCreated {
		fmt.Printf("launch failed (%d): %s\n", resp.StatusCode, strings.TrimSpace(string(body)))
		return 1
	}

	var out struct {
		Agent struct {
			AgentID string `json:"agent_id"`
			Name    string `json:"name"`
		} `json:"agent"`
	}
	_ = json.Unmarshal(body, &out)
	fmt.Printf("launched %s (%s) %s@%s\n", out.Agent.Name, out.Agent.AgentID, la.Role, la.Project)
	return 0
}

// postLaunch POSTs the launch body to the local dashboard.
func postLaunch(port int, b launchBody) (*http.Response, []byte, error) {
	payload, _ := json.Marshal(b)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/sessions", port)
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

// dashboardPort resolves the dashboard port from the pidfile, then config, then default.
func dashboardPort(cfgStore *config.Store) int {
	if info, ok, _ := readPidfile(cfgStore.Home()); ok && info.Port > 0 {
		return info.Port
	}
	if cfg, err := cfgStore.ReadConfig(); err == nil && cfg.Port > 0 {
		return cfg.Port
	}
	return config.DefaultConfig().Port
}
