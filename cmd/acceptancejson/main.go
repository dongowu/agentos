package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type auditEnvelope struct {
	Records []map[string]any `json:"records"`
}

type actionAuditRecord struct {
	ExitCode int `json:"exit_code"`
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: acceptancejson <command> [args]")
	}
	input, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	switch args[0] {
	case "task-audit-count":
		count, err := taskAuditCount(input)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, count)
		return err
	case "task-audit-last-action-id":
		actionID, err := taskAuditLastActionID(input)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, actionID)
		return err
	case "action-audit-exit-code":
		exitCode, err := actionAuditExitCode(input)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, exitCode)
		return err
	case "global-audit-match-count":
		if len(args) != 3 {
			return errors.New("usage: acceptancejson global-audit-match-count <task-id> <tenant-id>")
		}
		count, err := globalAuditMatchCount(input, args[1], args[2])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, count)
		return err
	case "task-audit-last-worker-id":
		workerID, err := taskAuditLastWorkerID(input)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, workerID)
		return err
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func taskAuditCount(input []byte) (int, error) {
	records, err := decodeAuditRecords(input)
	if err != nil {
		return 0, err
	}
	return len(records), nil
}

func taskAuditLastActionID(input []byte) (string, error) {
	records, err := decodeAuditRecords(input)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", errors.New("no audit records returned")
	}
	actionID, _ := records[len(records)-1]["action_id"].(string)
	if actionID == "" {
		return "", errors.New("last audit record missing action_id")
	}
	return actionID, nil
}

func actionAuditExitCode(input []byte) (int, error) {
	var record actionAuditRecord
	if err := json.Unmarshal(input, &record); err != nil {
		return 0, err
	}
	return record.ExitCode, nil
}

func globalAuditMatchCount(input []byte, taskID, tenantID string) (int, error) {
	records, err := decodeAuditRecords(input)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, record := range records {
		recordTaskID, _ := record["task_id"].(string)
		recordTenantID, _ := record["tenant_id"].(string)
		if recordTaskID == taskID && recordTenantID == tenantID {
			count++
		}
	}
	return count, nil
}

func taskAuditLastWorkerID(input []byte) (string, error) {
	records, err := decodeAuditRecords(input)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", errors.New("no audit records returned")
	}
	workerID, _ := records[len(records)-1]["worker_id"].(string)
	if workerID == "" {
		return "", errors.New("last audit record missing worker_id")
	}
	return workerID, nil
}

func decodeAuditRecords(input []byte) ([]map[string]any, error) {
	var envelope auditEnvelope
	if err := json.Unmarshal(input, &envelope); err != nil {
		return nil, err
	}
	if envelope.Records == nil {
		return []map[string]any{}, nil
	}
	return envelope.Records, nil
}
