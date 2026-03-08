package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	Status string `json:"status"`
}

type agentListResponse struct {
	Agents []string `json:"agents"`
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
	resp, err := doAPIRequest(http.MethodGet, "/health", nil, "")
	if err != nil {
		return nil, fmt.Errorf("health: %w", err)
	}
	defer resp.Body.Close()
	var result healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode health: %w", err)
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
	return &cobra.Command{
		Use:   "dev [agent.yaml] [task]",
		Short: "Run local dev diagnostics or submit a task quickly",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				health, err := fetchHealth()
				if err != nil {
					return err
				}
				agents, err := fetchAgents()
				if err != nil {
					return err
				}
				cmd.Printf("server: %s\n", health.Status)
				if len(agents.Agents) == 0 {
					cmd.Println("agents: (none)")
				} else {
					cmd.Printf("agents: %s\n", strings.Join(agents.Agents, ", "))
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
}
