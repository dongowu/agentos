package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/dongowu/agentos/internal/agent"
	"github.com/dongowu/agentos/internal/tool"
	_ "github.com/dongowu/agentos/internal/tool/builtin"
	"github.com/spf13/cobra"
)

var serverURL string

func main() {
	root := cobra.Command{
		Use:   "claw",
		Short: "ClawOS CLI",
	}
	root.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "ClawOS server URL")

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

			body, _ := json.Marshal(map[string]string{
				"agent": cfg.Name,
				"task":  task,
			})
			resp, err := http.Post(serverURL+"/agent/run", "application/json", bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("agent run: %w", err)
			}
			defer resp.Body.Close()

			var result apiResponse
			if err := decodeAPIResponse(resp, &result); err != nil {
				return err
			}
			if result.Error != "" {
				return fmt.Errorf("server: %s", result.Error)
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
			resp, err := http.Get(serverURL + "/agent/status?task_id=" + taskID)
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
		Use:   "dev",
		Short: "Start local dev agent (placeholder)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("claw dev: start clawd server first, then use claw run")
			return nil
		},
	}
}
