package scheduler

import (
	"errors"
	"fmt"

	"github.com/dongowu/agentos/internal/worker"
)

const errorCodeNoAvailableWorkers = "no_available_workers"

func encodeActionError(err error) (message string, code string) {
	if err == nil {
		return "", ""
	}
	message = err.Error()
	switch {
	case errors.Is(err, worker.ErrNoAvailableWorkers):
		code = errorCodeNoAvailableWorkers
	}
	return message, code
}

func decodeActionError(message, code string) error {
	if code == "" {
		if message == "" {
			return nil
		}
		return errors.New(message)
	}

	switch code {
	case errorCodeNoAvailableWorkers:
		if message == "" || message == worker.ErrNoAvailableWorkers.Error() {
			return worker.ErrNoAvailableWorkers
		}
		return fmt.Errorf("%s: %w", message, worker.ErrNoAvailableWorkers)
	default:
		if message == "" {
			return fmt.Errorf("scheduler error code: %s", code)
		}
		return errors.New(message)
	}
}
