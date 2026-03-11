package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/dongowu/agentos/internal/access"
	"github.com/spf13/cobra"
)

// APIFactory lazily resolves the local in-process API when needed.
type APIFactory func() (access.TaskSubmissionAPI, error)

type remoteAPIFactory func(serverURL, token string) access.TaskSubmissionAPI
type remoteWorkersFactory func(serverURL, token string) WorkerAPI

type workerListJSONEnvelope struct {
	SchemaVersion string           `json:"schema_version"`
	Summary       *WorkerSummary   `json:"summary,omitempty"`
	Workers       []WorkerSnapshot `json:"workers,omitempty"`
}

// Root returns the root CLI command wired to either a local factory or a remote API.
func Root(localFactory APIFactory) *cobra.Command {
	return newRoot(localFactory, func(serverURL, token string) access.TaskSubmissionAPI {
		return NewHTTPTaskAPI(serverURL, token)
	})
}

func newRoot(localFactory APIFactory, remoteFactory remoteAPIFactory, workerFactories ...remoteWorkersFactory) *cobra.Command {
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
	resolveWorkers := func() (WorkerAPI, error) {
		if serverURL == "" {
			return nil, fmt.Errorf("workers command requires --server")
		}
		if len(workerFactories) > 0 && workerFactories[0] != nil {
			return workerFactories[0](serverURL, authToken), nil
		}
		return NewHTTPTaskAPI(serverURL, authToken), nil
	}

	root := &cobra.Command{
		Use:   "osctl",
		Short: "AgentOS CLI",
	}
	root.PersistentFlags().StringVar(&serverURL, "server", os.Getenv("AGENTOS_SERVER_URL"), "Remote AgentOS API server URL; empty uses local embedded mode")
	root.PersistentFlags().StringVar(&authToken, "token", os.Getenv("AGENTOS_AUTH_TOKEN"), "Bearer token for authenticated AgentOS API servers")
	root.AddCommand(submitCmd(resolveAPI))
	root.AddCommand(statusCmd(resolveAPI))
	root.AddCommand(workersCmd(resolveWorkers))
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

func workersCmd(resolveWorkers func() (WorkerAPI, error)) *cobra.Command {
	var availableOnly bool
	var status string
	var capability string
	var output string
	var summaryOnly bool
	var workersOnly bool
	var noCapabilitySummary bool
	var noWorkers bool
	var sortBy string
	var limit int
	var unschedulableOnly bool
	var requireCount int
	var requireAvailableCount int
	var requireLoadThreshold string
	var requiredWorkers []string
	var requiredCapabilityCounts []string
	var requiredCapabilityAvailableCounts []string
	var requiredCapabilityOnlineCounts []string
	var requiredCapabilityBusyCounts []string
	var requiredCapabilityOfflineCounts []string
	var requiredStatusCounts []string

	cmd := &cobra.Command{
		Use:   "workers",
		Short: "Inspect worker capacity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if output != "table" && output != "json" {
				return fmt.Errorf("unsupported output format %q (expected table or json)", output)
			}
			if sortBy != "" && !isSupportedWorkerSort(sortBy) {
				return fmt.Errorf("unsupported sort mode %q (expected id, load, or status)", sortBy)
			}
			if limit < 0 {
				return fmt.Errorf("limit must be >= 0")
			}
			if requireCount < 0 {
				return fmt.Errorf("require-count must be >= 0")
			}
			if requireAvailableCount < 0 {
				return fmt.Errorf("require-available-count must be >= 0")
			}
			normalizedLoadThreshold, err := normalizeLoadThreshold(requireLoadThreshold)
			if err != nil {
				return err
			}
			normalizedRequiredWorkers, err := normalizeWorkerRequirements(requiredWorkers)
			if err != nil {
				return err
			}
			normalizedCapabilityRequirements, err := normalizeCapabilityCountRequirements(requiredCapabilityCounts)
			if err != nil {
				return err
			}
			normalizedCapabilityAvailableRequirements, err := normalizeCapabilityCountRequirements(requiredCapabilityAvailableCounts)
			if err != nil {
				return err
			}
			normalizedCapabilityOnlineRequirements, err := normalizeCapabilityCountRequirements(requiredCapabilityOnlineCounts)
			if err != nil {
				return err
			}
			normalizedCapabilityBusyRequirements, err := normalizeCapabilityCountRequirements(requiredCapabilityBusyCounts)
			if err != nil {
				return err
			}
			normalizedCapabilityOfflineRequirements, err := normalizeCapabilityCountRequirements(requiredCapabilityOfflineCounts)
			if err != nil {
				return err
			}
			normalizedStatusRequirements, err := normalizeStatusCountRequirements(requiredStatusCounts)
			if err != nil {
				return err
			}
			if summaryOnly && workersOnly {
				return fmt.Errorf("summary-only and workers-only are mutually exclusive")
			}
			if output != "json" && (summaryOnly || workersOnly) {
				return fmt.Errorf("summary-only and workers-only require --output json")
			}
			if output != "table" && (noCapabilitySummary || noWorkers) {
				return fmt.Errorf("no-capability-summary and no-workers require --output table")
			}
			api, err := resolveWorkers()
			if err != nil {
				return err
			}
			resp, err := api.ListWorkers(context.Background(), WorkerListFilters{
				AvailableOnly: availableOnly,
				Status:        status,
				Capability:    capability,
			})
			if err != nil {
				return err
			}
			resp = transformWorkerList(resp, workerViewOptions{
				SortBy:            sortBy,
				Limit:             limit,
				UnschedulableOnly: unschedulableOnly,
			})
			switch output {
			case "json":
				if err := renderWorkersJSON(cmd, resp, summaryOnly, workersOnly); err != nil {
					return err
				}
				return validateWorkerRequirements(
					resp,
					requireCount,
					requireAvailableCount,
					normalizedLoadThreshold,
					normalizedRequiredWorkers,
					normalizedCapabilityRequirements,
					normalizedCapabilityAvailableRequirements,
					normalizedCapabilityOnlineRequirements,
					normalizedCapabilityBusyRequirements,
					normalizedCapabilityOfflineRequirements,
					normalizedStatusRequirements,
				)
			default:
				cmd.Printf(
					"summary: total=%d online=%d busy=%d offline=%d available=%d\n",
					resp.Summary.Total,
					resp.Summary.Online,
					resp.Summary.Busy,
					resp.Summary.Offline,
					resp.Summary.AvailableWorkers,
				)
				if !noCapabilitySummary {
					for _, capability := range resp.Summary.Capabilities {
						cmd.Printf(
							"capability %s: total=%d online=%d busy=%d offline=%d available=%d\n",
							capability.Name,
							capability.Total,
							capability.Online,
							capability.Busy,
							capability.Offline,
							capability.AvailableWorkers,
						)
					}
				}
				if !noWorkers && len(resp.Workers) == 0 {
					cmd.Println("workers: (none)")
					return validateWorkerRequirements(
						resp,
						requireCount,
						requireAvailableCount,
						normalizedLoadThreshold,
						normalizedRequiredWorkers,
						normalizedCapabilityRequirements,
						normalizedCapabilityAvailableRequirements,
						normalizedCapabilityOnlineRequirements,
						normalizedCapabilityBusyRequirements,
						normalizedCapabilityOfflineRequirements,
						normalizedStatusRequirements,
					)
				}
				if err := renderWorkersTable(cmd, resp.Summary.Capabilities, resp.Workers, !noCapabilitySummary, !noWorkers); err != nil {
					return err
				}
				return validateWorkerRequirements(
					resp,
					requireCount,
					requireAvailableCount,
					normalizedLoadThreshold,
					normalizedRequiredWorkers,
					normalizedCapabilityRequirements,
					normalizedCapabilityAvailableRequirements,
					normalizedCapabilityOnlineRequirements,
					normalizedCapabilityBusyRequirements,
					normalizedCapabilityOfflineRequirements,
					normalizedStatusRequirements,
				)
			}
		},
	}
	cmd.Flags().BoolVar(&availableOnly, "available", false, "Only show schedulable workers")
	cmd.Flags().StringVar(&status, "status", "", "Only show workers with this status")
	cmd.Flags().StringVar(&capability, "capability", "", "Only show workers advertising this capability")
	cmd.Flags().StringVar(&output, "output", "table", "Output format: table or json")
	cmd.Flags().BoolVar(&summaryOnly, "summary-only", false, "With --output json, emit only the summary section")
	cmd.Flags().BoolVar(&workersOnly, "workers-only", false, "With --output json, emit only the worker list section")
	cmd.Flags().BoolVar(&noCapabilitySummary, "no-capability-summary", false, "With --output table, hide the capability summary block")
	cmd.Flags().BoolVar(&noWorkers, "no-workers", false, "With --output table, hide the worker table block")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort returned workers by id, load, or status")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit the number of returned workers after filtering and sorting (0 means no limit)")
	cmd.Flags().BoolVar(&unschedulableOnly, "unschedulable-only", false, "Only show workers that are not currently schedulable")
	cmd.Flags().IntVar(&requireCount, "require-count", 0, "Return a non-zero exit unless at least this many workers remain after filtering")
	cmd.Flags().IntVar(&requireAvailableCount, "require-available-count", 0, "Return a non-zero exit unless the emitted summary contains at least this many schedulable workers")
	cmd.Flags().StringVar(&requireLoadThreshold, "require-load-threshold", "", "Return a non-zero exit unless every emitted worker has normalized load <= this threshold")
	cmd.Flags().StringSliceVar(&requiredWorkers, "require-worker", nil, "Return a non-zero exit unless these worker IDs remain after filtering (repeatable or comma-separated)")
	cmd.Flags().StringSliceVar(&requiredCapabilityCounts, "require-capability-count", nil, "Return a non-zero exit unless the emitted worker set contains at least these capability counts (repeatable or comma-separated capability=count)")
	cmd.Flags().StringSliceVar(&requiredCapabilityAvailableCounts, "require-capability-available-count", nil, "Return a non-zero exit unless the emitted capability summary contains at least these schedulable worker counts (repeatable or comma-separated capability=count)")
	cmd.Flags().StringSliceVar(&requiredCapabilityOnlineCounts, "require-capability-online-count", nil, "Return a non-zero exit unless the emitted capability summary contains at least these online worker counts (repeatable or comma-separated capability=count)")
	cmd.Flags().StringSliceVar(&requiredCapabilityBusyCounts, "require-capability-busy-count", nil, "Return a non-zero exit unless the emitted capability summary contains at least these busy worker counts (repeatable or comma-separated capability=count)")
	cmd.Flags().StringSliceVar(&requiredCapabilityOfflineCounts, "require-capability-offline-count", nil, "Return a non-zero exit unless the emitted capability summary contains at least these offline worker counts (repeatable or comma-separated capability=count)")
	cmd.Flags().StringSliceVar(&requiredStatusCounts, "require-status-count", nil, "Return a non-zero exit unless the emitted worker set contains at least these status counts (repeatable or comma-separated status=count)")
	return cmd
}

type workerViewOptions struct {
	SortBy            string
	Limit             int
	UnschedulableOnly bool
}

func renderWorkersJSON(cmd *cobra.Command, resp *WorkerListResponse, summaryOnly, workersOnly bool) error {
	payload := workerListJSONEnvelope{
		SchemaVersion: "v1",
		Summary:       &resp.Summary,
		Workers:       resp.Workers,
	}
	switch {
	case summaryOnly:
		payload.Workers = nil
	case workersOnly:
		payload.Summary = nil
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func isSupportedWorkerSort(sortBy string) bool {
	switch sortBy {
	case "id", "load", "status":
		return true
	default:
		return false
	}
}

func isSupportedWorkerStatus(status string) bool {
	switch status {
	case "online", "busy", "offline":
		return true
	default:
		return false
	}
}

func normalizeWorkerRequirements(requiredWorkers []string) ([]string, error) {
	if len(requiredWorkers) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(requiredWorkers))
	normalized := make([]string, 0, len(requiredWorkers))
	for _, raw := range requiredWorkers {
		for _, workerID := range strings.Split(raw, ",") {
			workerID = strings.TrimSpace(workerID)
			if workerID == "" {
				return nil, fmt.Errorf("worker values must not be empty")
			}
			if _, ok := seen[workerID]; ok {
				continue
			}
			seen[workerID] = struct{}{}
			normalized = append(normalized, workerID)
		}
	}
	return normalized, nil
}

type capabilityCountRequirement struct {
	Name     string
	MinCount int
}

type statusCountRequirement struct {
	Name     string
	MinCount int
}

func normalizeCapabilityCountRequirements(rawRequirements []string) ([]capabilityCountRequirement, error) {
	if len(rawRequirements) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(rawRequirements))
	requirements := make([]capabilityCountRequirement, 0, len(rawRequirements))
	for _, raw := range rawRequirements {
		for _, entry := range strings.Split(raw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				return nil, fmt.Errorf("capability count values must not be empty")
			}
			name, countRaw, ok := strings.Cut(entry, "=")
			name = strings.TrimSpace(name)
			countRaw = strings.TrimSpace(countRaw)
			if !ok || name == "" || countRaw == "" {
				return nil, fmt.Errorf("capability count values must use capability=count")
			}
			count, err := strconv.Atoi(countRaw)
			if err != nil || count < 0 {
				return nil, fmt.Errorf("capability counts must be integers >= 0")
			}
			if _, exists := seen[name]; exists {
				return nil, fmt.Errorf("duplicate capability count requirement for %q", name)
			}
			seen[name] = struct{}{}
			requirements = append(requirements, capabilityCountRequirement{Name: name, MinCount: count})
		}
	}
	return requirements, nil
}

func normalizeStatusCountRequirements(rawRequirements []string) ([]statusCountRequirement, error) {
	if len(rawRequirements) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(rawRequirements))
	requirements := make([]statusCountRequirement, 0, len(rawRequirements))
	for _, raw := range rawRequirements {
		for _, entry := range strings.Split(raw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				return nil, fmt.Errorf("status count values must not be empty")
			}
			name, countRaw, ok := strings.Cut(entry, "=")
			name = strings.TrimSpace(name)
			countRaw = strings.TrimSpace(countRaw)
			if !ok || name == "" || countRaw == "" {
				return nil, fmt.Errorf("status count values must use status=count")
			}
			if !isSupportedWorkerStatus(name) {
				return nil, fmt.Errorf("unsupported worker status requirement %q (expected online, busy, or offline)", name)
			}
			count, err := strconv.Atoi(countRaw)
			if err != nil || count < 0 {
				return nil, fmt.Errorf("status counts must be integers >= 0")
			}
			if _, exists := seen[name]; exists {
				return nil, fmt.Errorf("duplicate status count requirement for %q", name)
			}
			seen[name] = struct{}{}
			requirements = append(requirements, statusCountRequirement{Name: name, MinCount: count})
		}
	}
	return requirements, nil
}

func normalizeLoadThreshold(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return nil, fmt.Errorf("require-load-threshold must be a float >= 0")
	}
	return &value, nil
}

func validateWorkerRequirements(resp *WorkerListResponse, requireCount, requireAvailableCount int, requireLoadThreshold *float64, requiredWorkers []string, capabilityRequirements, capabilityAvailableRequirements, capabilityOnlineRequirements, capabilityBusyRequirements, capabilityOfflineRequirements []capabilityCountRequirement, statusRequirements []statusCountRequirement) error {
	failures := make([]string, 0, 2)
	if requireCount > 0 && len(resp.Workers) < requireCount {
		failures = append(failures, fmt.Sprintf("worker count requirement failed: expected at least %d, got %d", requireCount, len(resp.Workers)))
	}
	if requireAvailableCount > 0 && resp.Summary.AvailableWorkers < requireAvailableCount {
		failures = append(
			failures,
			fmt.Sprintf(
				"available worker count requirement failed: expected at least %d, got %d",
				requireAvailableCount,
				resp.Summary.AvailableWorkers,
			),
		)
	}
	if requireLoadThreshold != nil {
		for _, worker := range resp.Workers {
			load := workerNormalizedLoad(worker)
			if load > *requireLoadThreshold {
				failures = append(
					failures,
					fmt.Sprintf(
						"load threshold requirement failed: worker %s load %.2f exceeds %.2f",
						worker.ID,
						load,
						*requireLoadThreshold,
					),
				)
			}
		}
	}
	if missing := missingWorkers(resp.Workers, requiredWorkers); len(missing) > 0 {
		failures = append(failures, fmt.Sprintf("required workers missing: %s", strings.Join(missing, ", ")))
	}
	for _, requirement := range capabilityRequirements {
		actual := capabilityCount(resp.Summary.Capabilities, requirement.Name)
		if actual < requirement.MinCount {
			failures = append(
				failures,
				fmt.Sprintf(
					"capability count requirement failed: %s expected at least %d, got %d",
					requirement.Name,
					requirement.MinCount,
					actual,
				),
			)
		}
	}
	for _, requirement := range capabilityAvailableRequirements {
		actual := capabilityAvailableCount(resp.Summary.Capabilities, requirement.Name)
		if actual < requirement.MinCount {
			failures = append(
				failures,
				fmt.Sprintf(
					"capability available count requirement failed: %s expected at least %d, got %d",
					requirement.Name,
					requirement.MinCount,
					actual,
				),
			)
		}
	}
	for _, requirement := range capabilityOnlineRequirements {
		actual := capabilityOnlineCount(resp.Summary.Capabilities, requirement.Name)
		if actual < requirement.MinCount {
			failures = append(
				failures,
				fmt.Sprintf(
					"capability online count requirement failed: %s expected at least %d, got %d",
					requirement.Name,
					requirement.MinCount,
					actual,
				),
			)
		}
	}
	for _, requirement := range capabilityBusyRequirements {
		actual := capabilityBusyCount(resp.Summary.Capabilities, requirement.Name)
		if actual < requirement.MinCount {
			failures = append(
				failures,
				fmt.Sprintf(
					"capability busy count requirement failed: %s expected at least %d, got %d",
					requirement.Name,
					requirement.MinCount,
					actual,
				),
			)
		}
	}
	for _, requirement := range capabilityOfflineRequirements {
		actual := capabilityOfflineCount(resp.Summary.Capabilities, requirement.Name)
		if actual < requirement.MinCount {
			failures = append(
				failures,
				fmt.Sprintf(
					"capability offline count requirement failed: %s expected at least %d, got %d",
					requirement.Name,
					requirement.MinCount,
					actual,
				),
			)
		}
	}
	for _, requirement := range statusRequirements {
		actual := workerStatusCount(resp.Summary, requirement.Name)
		if actual < requirement.MinCount {
			failures = append(
				failures,
				fmt.Sprintf(
					"status count requirement failed: %s expected at least %d, got %d",
					requirement.Name,
					requirement.MinCount,
					actual,
				),
			)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

func missingWorkers(workers []WorkerSnapshot, requiredWorkers []string) []string {
	if len(requiredWorkers) == 0 {
		return nil
	}

	present := make(map[string]struct{}, len(workers))
	for _, worker := range workers {
		present[worker.ID] = struct{}{}
	}

	missing := make([]string, 0, len(requiredWorkers))
	for _, workerID := range requiredWorkers {
		if _, ok := present[workerID]; !ok {
			missing = append(missing, workerID)
		}
	}
	return missing
}

func capabilityCount(capabilities []WorkerCapabilitySummary, capabilityName string) int {
	for _, capability := range capabilities {
		if capability.Name == capabilityName {
			return capability.Total
		}
	}
	return 0
}

func capabilityAvailableCount(capabilities []WorkerCapabilitySummary, capabilityName string) int {
	for _, capability := range capabilities {
		if capability.Name == capabilityName {
			return capability.AvailableWorkers
		}
	}
	return 0
}

func capabilityOnlineCount(capabilities []WorkerCapabilitySummary, capabilityName string) int {
	for _, capability := range capabilities {
		if capability.Name == capabilityName {
			return capability.Online
		}
	}
	return 0
}

func capabilityBusyCount(capabilities []WorkerCapabilitySummary, capabilityName string) int {
	for _, capability := range capabilities {
		if capability.Name == capabilityName {
			return capability.Busy
		}
	}
	return 0
}

func capabilityOfflineCount(capabilities []WorkerCapabilitySummary, capabilityName string) int {
	for _, capability := range capabilities {
		if capability.Name == capabilityName {
			return capability.Offline
		}
	}
	return 0
}

func workerStatusCount(summary WorkerSummary, status string) int {
	switch status {
	case "online":
		return summary.Online
	case "busy":
		return summary.Busy
	case "offline":
		return summary.Offline
	default:
		return 0
	}
}

func workerNormalizedLoad(worker WorkerSnapshot) float64 {
	if worker.MaxTasks <= 0 {
		return 1
	}
	return float64(worker.ActiveTasks) / float64(worker.MaxTasks)
}

func transformWorkerList(resp *WorkerListResponse, opts workerViewOptions) *WorkerListResponse {
	workers := append([]WorkerSnapshot(nil), resp.Workers...)
	for i := range workers {
		if len(workers[i].Capabilities) == 0 {
			continue
		}
		workers[i].Capabilities = append([]string(nil), workers[i].Capabilities...)
		sort.Strings(workers[i].Capabilities)
	}
	if opts.UnschedulableOnly {
		filtered := make([]WorkerSnapshot, 0, len(workers))
		for _, worker := range workers {
			if isWorkerUnschedulable(worker) {
				filtered = append(filtered, worker)
			}
		}
		workers = filtered
	}

	switch opts.SortBy {
	case "id":
		sort.SliceStable(workers, func(i, j int) bool {
			return workers[i].ID < workers[j].ID
		})
	case "load":
		sort.SliceStable(workers, func(i, j int) bool {
			left := workerLoadRank(workers[i])
			right := workerLoadRank(workers[j])
			if left == right {
				return workers[i].ID < workers[j].ID
			}
			return left > right
		})
	case "status":
		sort.SliceStable(workers, func(i, j int) bool {
			left := workerStatusRank(workers[i].Status)
			right := workerStatusRank(workers[j].Status)
			if left == right {
				return workers[i].ID < workers[j].ID
			}
			return left > right
		})
	}

	if opts.Limit > 0 && len(workers) > opts.Limit {
		workers = workers[:opts.Limit]
	}

	return &WorkerListResponse{
		Summary: summarizeWorkers(workers),
		Workers: workers,
	}
}

func summarizeWorkers(workers []WorkerSnapshot) WorkerSummary {
	summary := WorkerSummary{Total: len(workers)}
	capabilitySummary := make(map[string]*WorkerCapabilitySummary)

	for _, worker := range workers {
		switch worker.Status {
		case "online":
			summary.Online++
		case "busy":
			summary.Busy++
		case "offline":
			summary.Offline++
		}
		schedulable := isWorkerSchedulable(worker)
		if schedulable {
			summary.AvailableWorkers++
		}

		for _, name := range worker.Capabilities {
			entry, ok := capabilitySummary[name]
			if !ok {
				entry = &WorkerCapabilitySummary{Name: name}
				capabilitySummary[name] = entry
			}
			entry.Total++
			switch worker.Status {
			case "online":
				entry.Online++
			case "busy":
				entry.Busy++
			case "offline":
				entry.Offline++
			}
			if schedulable {
				entry.AvailableWorkers++
			}
		}
	}

	names := make([]string, 0, len(capabilitySummary))
	for name := range capabilitySummary {
		names = append(names, name)
	}
	sort.Strings(names)
	summary.Capabilities = make([]WorkerCapabilitySummary, 0, len(names))
	for _, name := range names {
		summary.Capabilities = append(summary.Capabilities, *capabilitySummary[name])
	}
	return summary
}

func isWorkerSchedulable(worker WorkerSnapshot) bool {
	return worker.Status == "online" && worker.MaxTasks > 0 && worker.ActiveTasks < worker.MaxTasks
}

func isWorkerUnschedulable(worker WorkerSnapshot) bool {
	return !isWorkerSchedulable(worker)
}

func workerLoadRank(worker WorkerSnapshot) float64 {
	if worker.MaxTasks <= 0 {
		return 1e9
	}
	return float64(worker.ActiveTasks) / float64(worker.MaxTasks)
}

func workerStatusRank(status string) int {
	switch status {
	case "offline":
		return 3
	case "busy":
		return 2
	case "online":
		return 1
	default:
		return 0
	}
}

func renderWorkersTable(cmd *cobra.Command, capabilities []WorkerCapabilitySummary, workers []WorkerSnapshot, showCapabilitySummary, showWorkers bool) error {
	if showCapabilitySummary && len(capabilities) > 0 {
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "CAPABILITY\tTOTAL\tONLINE\tBUSY\tOFFLINE\tAVAILABLE"); err != nil {
			return err
		}
		for _, capability := range capabilities {
			if _, err := fmt.Fprintf(
				tw,
				"%s\t%d\t%d\t%d\t%d\t%d\n",
				capability.Name,
				capability.Total,
				capability.Online,
				capability.Busy,
				capability.Offline,
				capability.AvailableWorkers,
			); err != nil {
				return err
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		if showWorkers {
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return err
			}
		}
	}

	if !showWorkers {
		return nil
	}

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tSTATUS\tLOAD\tADDR\tCAPABILITIES"); err != nil {
		return err
	}
	for _, worker := range workers {
		capabilities := "-"
		if len(worker.Capabilities) > 0 {
			capabilities = strings.Join(worker.Capabilities, ",")
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%d/%d\t%s\t%s\n",
			worker.ID,
			worker.Status,
			worker.ActiveTasks,
			worker.MaxTasks,
			worker.Addr,
			capabilities,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
