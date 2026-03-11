package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/dongowu/agentos/internal/agent"
	"github.com/dongowu/agentos/internal/tool"
	_ "github.com/dongowu/agentos/internal/tool/builtin"
	"github.com/spf13/cobra"
)

var (
	serverURL string
	authToken string
)

func main() {
	root := cobra.Command{
		Use:   "claw",
		Short: "ClawOS CLI",
	}
	root.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "ClawOS server URL")
	root.PersistentFlags().StringVar(&authToken, "token", os.Getenv("AGENTOS_AUTH_TOKEN"), "Bearer token for authenticated AgentOS servers")

	root.AddCommand(runCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(toolRunCmd())
	root.AddCommand(devCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [agent.yaml] [task]",
		Short: "Run an agent with a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentPath := args[0]
			task := args[1]

			cfg, err := agent.Load(agentPath)
			if err != nil {
				return fmt.Errorf("load agent: %w", err)
			}
			result, err := submitAgentTask(cfg.Name, task)
			if err != nil {
				return err
			}
			cmd.Printf("task %s created (state: %s)\n", result.TaskID, result.State)
			cmd.Printf("check status: claw --server %s status %s\n", serverURL, result.TaskID)
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [task-id]",
		Short: "Get task status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			resp, err := doAPIRequest(http.MethodGet, "/agent/status?task_id="+taskID, nil, "")
			if err != nil {
				return fmt.Errorf("agent status: %w", err)
			}
			defer resp.Body.Close()

			var result apiResponse
			if err := decodeAPIResponse(resp, &result); err != nil {
				return err
			}
			if result.Error != "" {
				return fmt.Errorf("server: %s", result.Error)
			}
			cmd.Printf("task %s: %s\n", result.TaskID, result.State)
			return nil
		},
	}
}

func toolRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tool [tool-name] [args...]",
		Short: "Run a tool locally (no server)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]
			input := map[string]any{}
			if len(args) > 1 {
				// Join remaining args as command for shell
				cmdStr := args[1]
				for i := 2; i < len(args); i++ {
					cmdStr += " " + args[i]
				}
				input["cmd"] = cmdStr
			} else if toolName == "shell" {
				return fmt.Errorf("shell tool requires: claw tool shell <command>")
			}

			out, err := tool.Run(context.Background(), toolName, input)
			if err != nil {
				return err
			}
			if m, ok := out.(map[string]any); ok {
				if s, ok := m["stdout"].(string); ok && s != "" {
					cmd.Print(s)
				}
				if s, ok := m["stderr"].(string); ok && s != "" {
					cmd.PrintErr(s)
				}
			} else {
				cmd.Printf("%v\n", out)
			}
			return nil
		},
	}
}

type apiResponse struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
	Error  string `json:"error"`
}

type healthResponse struct {
	Status           string   `json:"status"`
	DegradedReasons  []string `json:"degraded_reasons"`
	CapacityWarnings []string `json:"capacity_warnings"`
}

type agentListResponse struct {
	Agents []string `json:"agents"`
}

type workerListResponse struct {
	Summary workerSummary    `json:"summary"`
	Workers []workerSnapshot `json:"workers"`
}

type workerSummary struct {
	Capabilities []workerCapabilitySummary `json:"capabilities"`
}

type workerCapabilitySummary struct {
	Name             string `json:"name"`
	AvailableWorkers int    `json:"available_workers"`
}

type workerSnapshot struct {
	ID string `json:"id"`
}

type devDiagnosticsResponse struct {
	SchemaVersion string             `json:"schema_version"`
	Health        healthResponse     `json:"health"`
	Ready         healthResponse     `json:"ready"`
	Workers       workerListResponse `json:"workers"`
	Agents        []string           `json:"agents"`
}

func newAPIRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, serverURL+path, body)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	return req, nil
}

func doAPIRequest(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := newAPIRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return http.DefaultClient.Do(req)
}

func submitAgentTask(agentName, task string) (*apiResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"agent": agentName,
		"task":  task,
	})
	resp, err := doAPIRequest(http.MethodPost, "/agent/run", bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}
	defer resp.Body.Close()

	var result apiResponse
	if err := decodeAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("server: %s", result.Error)
	}
	return &result, nil
}

func fetchHealth() (*healthResponse, error) {
	return fetchProbe("/health", "health")
}

func fetchReady() (*healthResponse, error) {
	return fetchProbe("/ready", "ready")
}

func fetchProbe(path, label string) (*healthResponse, error) {
	resp, err := doAPIRequest(http.MethodGet, path, nil, "")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	defer resp.Body.Close()
	var result healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return &result, nil
}

func fetchAgents() (*agentListResponse, error) {
	resp, err := doAPIRequest(http.MethodGet, "/agent/list", nil, "")
	if err != nil {
		return nil, fmt.Errorf("agent list: %w", err)
	}
	defer resp.Body.Close()
	var result agentListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode agent list: %w", err)
	}
	return &result, nil
}

func fetchAvailableWorkers() (*workerListResponse, error) {
	resp, err := doAPIRequest(http.MethodGet, "/v1/workers?available_only=true", nil, "")
	if err != nil {
		return nil, fmt.Errorf("workers: %w", err)
	}
	defer resp.Body.Close()
	var result workerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode workers: %w", err)
	}
	return &result, nil
}

func decodeAPIResponse(resp *http.Response, out *apiResponse) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("server returned empty response")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w: %s", err, string(body))
	}
	if resp.StatusCode >= 400 && out.Error == "" {
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func devCmd() *cobra.Command {
	var output string
	var sections []string
	var requireReady bool
	var requiredCapabilities []string

	cmd := &cobra.Command{
		Use:   "dev [agent.yaml] [task]",
		Short: "Run local dev diagnostics or submit a task quickly",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if output != "text" && output != "json" {
					return fmt.Errorf("unsupported output format %q (expected text or json)", output)
				}
				if len(sections) > 0 && output != "json" {
					return fmt.Errorf("section requires --output json")
				}
				normalizedSections, err := normalizeDevSections(sections)
				if err != nil {
					return err
				}
				normalizedCapabilities, err := normalizeDevCapabilityRequirements(requiredCapabilities)
				if err != nil {
					return err
				}
				health, err := fetchHealth()
				if err != nil {
					return err
				}
				ready, err := fetchReady()
				if err != nil {
					return err
				}
				workers, err := fetchAvailableWorkers()
				if err != nil {
					return err
				}
				agents, err := fetchAgents()
				if err != nil {
					return err
				}
				if output == "json" {
					if err := renderDevDiagnosticsJSON(cmd, devDiagnosticsResponse{
						SchemaVersion: "v1",
						Health:        *health,
						Ready:         *ready,
						Workers:       *workers,
						Agents:        append([]string(nil), agents.Agents...),
					}, normalizedSections); err != nil {
						return err
					}
					return validateDevDiagnosticsRequirements(*ready, *workers, requireReady, normalizedCapabilities)
				}
				cmd.Printf("server: %s\n", health.Status)
				cmd.Printf("readiness: %s", ready.Status)
				if len(ready.DegradedReasons) > 0 {
					cmd.Printf(" (%s)", strings.Join(ready.DegradedReasons, ", "))
				}
				cmd.Print("\n")
				if len(ready.CapacityWarnings) > 0 {
					cmd.Printf("capacity warnings: %s\n", strings.Join(ready.CapacityWarnings, ", "))
				}
				if len(workers.Workers) == 0 {
					cmd.Println("available workers: (none)")
				} else {
					ids := make([]string, 0, len(workers.Workers))
					for _, worker := range workers.Workers {
						ids = append(ids, worker.ID)
					}
					cmd.Printf("available workers: %s\n", strings.Join(ids, ", "))
					if len(workers.Summary.Capabilities) > 0 {
						capabilities := make([]string, 0, len(workers.Summary.Capabilities))
						for _, capability := range workers.Summary.Capabilities {
							capabilities = append(capabilities, fmt.Sprintf("%s=%d", capability.Name, capability.AvailableWorkers))
						}
						cmd.Printf("available by capability: %s\n", strings.Join(capabilities, ", "))
					}
				}
				if len(agents.Agents) == 0 {
					cmd.Println("agents: (none)")
				} else {
					cmd.Printf("agents: %s\n", strings.Join(agents.Agents, ", "))
				}
				if err := validateDevDiagnosticsRequirements(*ready, *workers, requireReady, normalizedCapabilities); err != nil {
					return err
				}
				cmd.Println("next: claw dev <agent.yaml> \"<task>\"")
				return nil
			}
			if len(args) != 2 {
				return fmt.Errorf("dev mode requires either no args or: claw dev <agent.yaml> <task>")
			}
			cfg, err := agent.Load(args[0])
			if err != nil {
				return fmt.Errorf("load agent: %w", err)
			}
			result, err := submitAgentTask(cfg.Name, args[1])
			if err != nil {
				return err
			}
			cmd.Printf("task %s created (state: %s)\n", result.TaskID, result.State)
			cmd.Printf("check status: claw --server %s status %s\n", serverURL, result.TaskID)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "text", "Output format for diagnostics mode: text or json")
	cmd.Flags().StringSliceVar(&sections, "section", nil, "With --output json, emit only selected sections (repeatable or comma-separated): health, ready, workers, or agents")
	cmd.Flags().BoolVar(&requireReady, "require-ready", false, "With diagnostics mode, return a non-zero exit when readiness is degraded")
	cmd.Flags().StringSliceVar(&requiredCapabilities, "require-capability", nil, "With diagnostics mode, require available workers for these capabilities (repeatable or comma-separated)")
	return cmd
}

func isSupportedDevSection(section string) bool {
	switch section {
	case "health", "ready", "workers", "agents":
		return true
	default:
		return false
	}
}

func normalizeDevSections(sections []string) ([]string, error) {
	if len(sections) == 0 {
		return nil, nil
	}

	selected := make(map[string]struct{}, len(sections))
	for _, raw := range sections {
		for _, section := range strings.Split(raw, ",") {
			section = strings.TrimSpace(section)
			if section == "" {
				return nil, fmt.Errorf("section values must not be empty")
			}
			if !isSupportedDevSection(section) {
				return nil, fmt.Errorf("unsupported section %q (expected health, ready, workers, or agents)", section)
			}
			selected[section] = struct{}{}
		}
	}

	normalized := make([]string, 0, len(selected))
	for _, section := range []string{"health", "ready", "workers", "agents"} {
		if _, ok := selected[section]; ok {
			normalized = append(normalized, section)
		}
	}
	return normalized, nil
}

func normalizeDevCapabilityRequirements(capabilities []string) ([]string, error) {
	if len(capabilities) == 0 {
		return nil, nil
	}

	selected := make(map[string]struct{}, len(capabilities))
	normalized := make([]string, 0, len(capabilities))
	for _, raw := range capabilities {
		for _, capability := range strings.Split(raw, ",") {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				return nil, fmt.Errorf("capability values must not be empty")
			}
			if _, ok := selected[capability]; ok {
				continue
			}
			selected[capability] = struct{}{}
			normalized = append(normalized, capability)
		}
	}
	return normalized, nil
}

func validateDevDiagnosticsRequirements(ready healthResponse, workers workerListResponse, requireReady bool, requiredCapabilities []string) error {
	failures := make([]string, 0, 2)
	if requireReady && ready.Status != "ok" {
		failures = append(failures, fmt.Sprintf("readiness requirement failed: status=%s", ready.Status))
	}

	if missing := missingCapabilities(requiredCapabilities, workers.Summary.Capabilities); len(missing) > 0 {
		failures = append(failures, fmt.Sprintf("required capabilities unavailable: %s", strings.Join(missing, ", ")))
	}

	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

func missingCapabilities(required []string, available []workerCapabilitySummary) []string {
	if len(required) == 0 {
		return nil
	}

	availableByCapability := make(map[string]int, len(available))
	for _, capability := range available {
		availableByCapability[capability.Name] = capability.AvailableWorkers
	}

	missing := make([]string, 0, len(required))
	for _, capability := range required {
		if availableByCapability[capability] <= 0 {
			missing = append(missing, capability)
		}
	}
	return missing
}

func renderDevDiagnosticsJSON(cmd *cobra.Command, resp devDiagnosticsResponse, sections []string) error {
	var payload any = resp
	if len(sections) > 0 {
		selected := map[string]any{"schema_version": resp.SchemaVersion}
		for _, section := range sections {
			switch section {
			case "health":
				selected["health"] = resp.Health
			case "ready":
				selected["ready"] = resp.Ready
			case "workers":
				selected["workers"] = resp.Workers
			case "agents":
				selected["agents"] = resp.Agents
			}
		}
		payload = selected
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
