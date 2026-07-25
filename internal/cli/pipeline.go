package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/pipeline"
)

func newPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "pipeline", Short: "Manage pipeline templates and durable runs"}
	templates := &cobra.Command{Use: "template", Short: "Inspect and validate pipeline templates"}
	templates.AddCommand(&cobra.Command{Use: "list", Short: "List pipeline templates", RunE: func(*cobra.Command, []string) error {
		return runPipelineRequest(http.MethodGet, "/api/pipelines", nil)
	}})
	var templateID, templateFile string
	validate := &cobra.Command{Use: "validate", Short: "Validate a version-1 template JSON file", RunE: func(*cobra.Command, []string) error {
		data, err := os.ReadFile(templateFile)
		if err != nil {
			return err
		}
		var template pipeline.Template
		if err := json.Unmarshal(data, &template); err != nil {
			return fmt.Errorf("decode template: %w", err)
		}
		return runPipelineRequest(http.MethodPost, "/api/pipelines/validate", map[string]any{"id": templateID, "template": template})
	}}
	validate.Flags().StringVar(&templateID, "id", "", "template id")
	validate.Flags().StringVar(&templateFile, "file", "", "template JSON file")
	_ = validate.MarkFlagRequired("id")
	_ = validate.MarkFlagRequired("file")
	templates.AddCommand(validate)
	cmd.AddCommand(templates)

	runs := &cobra.Command{Use: "run", Short: "Start and control pipeline runs"}
	var startTemplate, startName, startProject, startGoal, startRequestID string
	var startInputs, startStages []string
	var acknowledge bool
	start := &cobra.Command{Use: "start", Short: "Start a configured pipeline run", RunE: func(*cobra.Command, []string) error {
		inputs, err := parseKeyValues(startInputs)
		if err != nil {
			return err
		}
		assignments, err := parseStageAssignments(startStages)
		if err != nil {
			return err
		}
		if startRequestID == "" {
			startRequestID = newPipelineRequestID()
		}
		body := struct {
			pipeline.StartRequest
			Acknowledge bool `json:"acknowledge_shared_workspace"`
		}{StartRequest: pipeline.StartRequest{RequestID: startRequestID, TemplateID: startTemplate, DisplayName: startName, Project: startProject, Goal: startGoal, Inputs: inputs, Assignments: assignments}, Acknowledge: acknowledge}
		return runPipelineRequest(http.MethodPost, "/api/pipeline-runs", body)
	}}
	start.Flags().StringVar(&startTemplate, "template", "", "saved template id")
	start.Flags().StringVar(&startName, "name", "", "run display name")
	start.Flags().StringVar(&startProject, "project", "", "project id")
	start.Flags().StringVar(&startGoal, "goal", "", "run goal")
	start.Flags().StringVar(&startRequestID, "request-id", "", "idempotency request id")
	start.Flags().StringSliceVar(&startInputs, "input", nil, "named input as name=value (repeatable)")
	start.Flags().StringSliceVar(&startStages, "stage", nil, "runtime as stage=backend/model (repeatable)")
	start.Flags().BoolVar(&acknowledge, "ack-shared-workspace", false, "acknowledge active work sharing the project directory")
	_ = start.MarkFlagRequired("template")
	_ = start.MarkFlagRequired("project")
	_ = start.MarkFlagRequired("goal")
	runs.AddCommand(start)
	runs.AddCommand(&cobra.Command{Use: "show <run_id>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return runPipelineRequest(http.MethodGet, "/api/pipeline-runs/"+args[0], nil)
	}})
	var revision int64
	var continuation string
	continueCmd := &cobra.Command{Use: "continue <run_id>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return runPipelineRequest(http.MethodPost, "/api/pipeline-runs/"+args[0]+"/continue", map[string]any{"revision": revision, "input": continuation})
	}}
	continueCmd.Flags().Int64Var(&revision, "revision", 0, "current run revision")
	continueCmd.Flags().StringVar(&continuation, "input", "", "new input for a blocked stage")
	_ = continueCmd.MarkFlagRequired("revision")
	runs.AddCommand(continueCmd)
	for _, action := range []string{"retry", "stop"} {
		action := action
		var actionRevision int64
		actionCmd := &cobra.Command{Use: action + " <run_id>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			return runPipelineRequest(http.MethodPost, "/api/pipeline-runs/"+args[0]+"/"+action, map[string]any{"revision": actionRevision})
		}}
		actionCmd.Flags().Int64Var(&actionRevision, "revision", 0, "current run revision")
		_ = actionCmd.MarkFlagRequired("revision")
		runs.AddCommand(actionCmd)
	}
	cmd.AddCommand(runs)
	return cmd
}

func runPipelineRequest(method, path string, body any) error {
	store, err := config.New()
	if err != nil {
		return err
	}
	status, data, err := pipelineAPIRequest(dashboardPort(store), method, path, body)
	if err != nil {
		return fmt.Errorf("could not reach dashboard; run `agentdeck dashboard start` first: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("pipeline request failed (%d): %s", status, strings.TrimSpace(string(data)))
	}
	if len(bytes.TrimSpace(data)) > 0 {
		var pretty bytes.Buffer
		if json.Indent(&pretty, data, "", "  ") == nil {
			fmt.Println(pretty.String())
		} else {
			fmt.Println(string(data))
		}
	}
	return nil
}

func pipelineAPIRequest(port int, method, path string, body any) (int, []byte, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
	}
	request, err := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	return response.StatusCode, data, err
}

func parseKeyValues(values []string) (map[string]string, error) {
	out := map[string]string{}
	for _, value := range values {
		name, text, ok := strings.Cut(value, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid name=value input %q", value)
		}
		out[name] = text
	}
	return out, nil
}

func parseStageAssignments(values []string) (map[string]pipeline.RuntimeAssignment, error) {
	out := map[string]pipeline.RuntimeAssignment{}
	for _, value := range values {
		stage, runtimeValue, ok := strings.Cut(value, "=")
		backend, model, runtimeOK := strings.Cut(runtimeValue, "/")
		if !ok || !runtimeOK || stage == "" || backend == "" || model == "" {
			return nil, fmt.Errorf("invalid stage=backend/model assignment %q", value)
		}
		out[stage] = pipeline.RuntimeAssignment{Backend: backend, Model: model}
	}
	return out, nil
}

func newPipelineRequestID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("request-%d", time.Now().UnixNano())
	}
	return "request-" + hex.EncodeToString(value[:])
}
