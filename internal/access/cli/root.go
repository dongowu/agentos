package cli

import (
	"context"
	"fmt"

	"github.com/agentos/agentos/internal/access"
	"github.com/spf13/cobra"
)

// Root returns the root CLI command wired to the given API.
func Root(api access.TaskSubmissionAPI) *cobra.Command {
	root := &cobra.Command{
		Use:   "osctl",
		Short: "AgentOS CLI",
	}
	root.AddCommand(submitCmd(api))
	root.AddCommand(statusCmd(api))
	return root
}

func submitCmd(api access.TaskSubmissionAPI) *cobra.Command {
	return &cobra.Command{
		Use:   "submit [prompt]",
		Short: "Submit a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if api == nil {
				return fmt.Errorf("api not configured (use with controller or apiserver)")
			}
			prompt := args[0]
			resp, err := api.CreateTask(context.Background(), access.CreateTaskRequest{Prompt: prompt})
			if err != nil {
				return err
			}
			cmd.Printf("task %s created (state: %s)\n", resp.TaskID, resp.State)
			return nil
		},
	}
}

func statusCmd(api access.TaskSubmissionAPI) *cobra.Command {
	return &cobra.Command{
		Use:   "status [task-id]",
		Short: "Get task status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if api == nil {
				return fmt.Errorf("api not configured (use with controller or apiserver)")
			}
			taskID := args[0]
			resp, err := api.GetTask(context.Background(), taskID)
			if err != nil {
				return err
			}
			cmd.Printf("task %s: %s\n", resp.TaskID, resp.State)
			return nil
		},
	}
}
