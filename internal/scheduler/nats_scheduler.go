package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dongowu/agentos/pkg/taskdsl"
	"github.com/nats-io/nats.go"
)

const (
	subjectDispatch = "AGENTOS.actions.dispatch"
	subjectResult   = "AGENTOS.actions.result" // .{taskID}.{actionID}
)

// actionRequest is the message published to NATS for worker consumption.
type actionRequest struct {
	TaskID   string         `json:"task_id"`
	ActionID string         `json:"action_id"`
	Kind     string         `json:"kind"`
	Payload  map[string]any `json:"payload"`
	Runtime  string         `json:"runtime"`
}

// actionResponse is the message workers publish back after execution.
type actionResponse struct {
	TaskID   string `json:"task_id"`
	ActionID string `json:"action_id"`
	ExitCode int    `json:"exit_code"`
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
	WorkerID string `json:"worker_id"`
	Error    string `json:"error,omitempty"`
}

// NATSScheduler dispatches actions via NATS queue groups so workers
// compete for tasks (competitive consumer pattern).
type NATSScheduler struct {
	js      nats.JetStreamContext
	results chan ActionResult
	mu      sync.Mutex
	subs    []*nats.Subscription
}

// NewNATSScheduler creates a scheduler using the given NATS JetStream context.
func NewNATSScheduler(js nats.JetStreamContext) *NATSScheduler {
	return &NATSScheduler{
		js:      js,
		results: make(chan ActionResult, 64),
	}
}

// Submit publishes an action request to the dispatch subject.
// Workers subscribe to this subject via a queue group and compete to consume it.
func (s *NATSScheduler) Submit(_ context.Context, taskID string, action *taskdsl.Action) error {
	req := actionRequest{
		TaskID:   taskID,
		ActionID: action.ID,
		Kind:     action.Kind,
		Payload:  action.Payload,
		Runtime:  action.RuntimeEnv,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal action request: %w", err)
	}

	// Subscribe to the result subject for this task+action if not already listening.
	resultSubject := fmt.Sprintf("%s.%s.%s", subjectResult, taskID, action.ID)
	if err := s.subscribeResult(resultSubject); err != nil {
		return fmt.Errorf("subscribe result: %w", err)
	}

	if _, err := s.js.Publish(subjectDispatch, data); err != nil {
		return fmt.Errorf("publish action: %w", err)
	}
	return nil
}

func (s *NATSScheduler) subscribeResult(subject string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, err := s.js.Subscribe(subject, func(msg *nats.Msg) {
		var resp actionResponse
		if err := json.Unmarshal(msg.Data, &resp); err != nil {
			return
		}
		ar := ActionResult{
			TaskID:   resp.TaskID,
			ActionID: resp.ActionID,
			ExitCode: resp.ExitCode,
			Stdout:   resp.Stdout,
			Stderr:   resp.Stderr,
			WorkerID: resp.WorkerID,
		}
		if resp.Error != "" {
			ar.Error = fmt.Errorf("%s", resp.Error)
		}
		s.results <- ar
	})
	if err != nil {
		return err
	}
	s.subs = append(s.subs, sub)
	return nil
}

// Results returns the channel receiving completed action results.
func (s *NATSScheduler) Results() <-chan ActionResult {
	return s.results
}

// Close unsubscribes all result listeners.
func (s *NATSScheduler) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		_ = sub.Unsubscribe()
	}
	s.subs = nil
	return nil
}

var _ Scheduler = (*NATSScheduler)(nil)
