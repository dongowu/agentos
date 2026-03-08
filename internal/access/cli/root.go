package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/dongowu/agentos/internal/access"
	"github.com/spf13/cobra"
)

// APIFactory lazily resolves the local in-process API when needed.
type APIFactory func() (access.TaskSubmissionAPI, error)

type remoteAPIFactory func(serverURL, token string) access.TaskSubmissionAPI

// Root returns the root CLI command wired to either a local factory or a remote API.
func Root(localFactory APIFactory) *cobra.Command {
	return newRoot(localFactory, func(serverURL, token string) access.TaskSubmissionAPI {
		return NewHTTPTaskAPI(serverURL, token)
	})
}

func newRoot(localFactory APIFactory, remoteFactory remoteAPIFactory) *cobra.Command {
	var serverURL string
	var authToken string

	resolveAPI := func() (access.TaskSubmissionAPI, error) {
		if serverURL != "" {
			return remoteFactory(serverURL, authToken), nil
		}
		if localFactory == nil {
			return nil, fmt.Errorf("api not configured (use --server or run with controller/apiserver)")
		}
		return localFactory()
	}

	root := &cobra.Command{
		Use:   "osctl",
		Short: "AgentOS CLI",
	}
	root.PersistentFlags().StringVar(&serverURL, "server", os.Getenv("AGENTOS_SERVER_URL"), "Remote AgentOS API server URL; empty uses local embedded mode")
	root.PersistentFlags().StringVar(&authToken, "token", os.Getenv("AGENTOS_AUTH_TOKEN"), "Bearer token for authenticated AgentOS API servers")
	root.AddCommand(submitCmd(resolveAPI))
	root.AddCommand(statusCmd(resolveAPI))
	return root
}

func submitCmd(resolveAPI func() (access.TaskSubmissionAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "submit [prompt]",
		Short: "Submit a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := resolveAPI()
			if err != nil {
				return err
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

func statusCmd(resolveAPI func() (access.TaskSubmissionAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "status [task-id]",
		Short: "Get task status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := resolveAPI()
			if err != nil {
				return err
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
