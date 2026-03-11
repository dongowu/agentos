package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dongowu/agentos/pkg/taskdsl"
	"github.com/nats-io/nats.go"
)

type subscription interface {
	Unsubscribe() error
}

type jetStreamClient interface {
	Publish(subject string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error)
	Subscribe(subject string, cb nats.MsgHandler, opts ...nats.SubOpt) (subscription, error)
}

// actionRequest is the message published to NATS for worker consumption.
type jetStreamWrapper struct {
	inner nats.JetStreamContext
}

func (w jetStreamWrapper) Publish(subject string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	return w.inner.Publish(subject, data, opts...)
}

func (w jetStreamWrapper) Subscribe(subject string, cb nats.MsgHandler, opts ...nats.SubOpt) (subscription, error) {
	return w.inner.Subscribe(subject, cb, opts...)
}

type actionRequest struct {
	TaskID   string         `json:"task_id"`
	ActionID string         `json:"action_id"`
	Kind     string         `json:"kind"`
	Payload  map[string]any `json:"payload"`
	Runtime  string         `json:"runtime"`
}

// actionResponse is the message workers publish back after execution.
type actionResponse struct {
	TaskID    string `json:"task_id"`
	ActionID  string `json:"action_id"`
	ExitCode  int    `json:"exit_code"`
	Stdout    []byte `json:"stdout"`
	Stderr    []byte `json:"stderr"`
	WorkerID  string `json:"worker_id"`
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

func dispatchSubject(stream string) string {
	return normalizeStream(stream) + ".actions.dispatch"
}

func resultSubject(stream, taskID, actionID string) string {
	return fmt.Sprintf("%s.actions.result.%s.%s", normalizeStream(stream), taskID, actionID)
}

func normalizeStream(stream string) string {
	if stream == "" {
		return "AGENTOS"
	}
	return stream
}

// NATSScheduler dispatches actions via NATS so another process can consume and execute them.
type NATSScheduler struct {
	js      jetStreamClient
	stream  string
	results chan ActionResult
	mu      sync.Mutex
	subs    []subscription
}

// NewNATSScheduler creates a scheduler using the given JetStream client and subject prefix.
func NewNATSScheduler(js jetStreamClient, stream string) *NATSScheduler {
	return &NATSScheduler{
		js:      js,
		stream:  normalizeStream(stream),
		results: make(chan ActionResult, 64),
	}
}

// Submit publishes an action request to the dispatch subject.
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

	subject := resultSubject(s.stream, taskID, action.ID)
	if err := s.subscribeResult(subject); err != nil {
		return fmt.Errorf("subscribe result: %w", err)
	}

	if _, err := s.js.Publish(dispatchSubject(s.stream), data); err != nil {
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
		ar.Error = decodeActionError(resp.Error, resp.ErrorCode)
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

// NewNATSSchedulerFromJetStream adapts a real JetStream context into the scheduler interface.
func NewNATSSchedulerFromJetStream(js nats.JetStreamContext, stream string) *NATSScheduler {
	return NewNATSScheduler(jetStreamWrapper{inner: js}, stream)
}
