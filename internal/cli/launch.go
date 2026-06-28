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
		val := func() string {
			if i+1 < len(rest) {
				i++
				return rest[i]
			}
			return ""
		}
		switch flag {
		case "--backend":
			out.Backend = val()
		case "--model":
			out.Model = val()
		case "--interface":
			out.Interface = val()
		case "--name":
			out.Name = val()
		case "--group":
			out.Group = val()
		default:
			return launchArgs{}, fmt.Errorf("unknown flag %q", flag)
		}
	}
	return out, nil
}

// runLaunch parses the launch form and POSTs it to the running dashboard's
// /api/sessions endpoint (CLI and modal produce an identical agent, §6.5).
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
