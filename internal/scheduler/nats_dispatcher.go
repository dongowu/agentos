package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dongowu/agentos/pkg/taskdsl"
	"github.com/nats-io/nats.go"
)

// NATSDispatcher consumes queued actions from NATS and executes them through a worker pool.
type NATSDispatcher struct {
	js     jetStreamClient
	stream string
	pool   WorkerPool
	mu     sync.Mutex
	sub    subscription
}

func NewNATSDispatcher(js jetStreamClient, stream string, pool WorkerPool) *NATSDispatcher {
	return &NATSDispatcher{js: js, stream: normalizeStream(stream), pool: pool}
}

func (d *NATSDispatcher) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sub != nil {
		return nil
	}
	sub, err := d.js.Subscribe(dispatchSubject(d.stream), func(msg *nats.Msg) {
		d.handleMessage(ctx, msg)
	})
	if err != nil {
		return err
	}
	d.sub = sub
	go func() {
		<-ctx.Done()
		_ = d.Close()
	}()
	return nil
}

func (d *NATSDispatcher) handleMessage(ctx context.Context, msg *nats.Msg) {
	var req actionRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return
	}
	result := actionResponse{TaskID: req.TaskID, ActionID: req.ActionID}
	workerID, err := d.pool.SelectWorker(ctx)
	if err != nil {
		result.Error = err.Error()
		d.publishResult(result)
		return
	}
	result.WorkerID = workerID
	action := &taskdsl.Action{ID: req.ActionID, Kind: req.Kind, Payload: req.Payload, RuntimeEnv: req.Runtime}
	execResult, err := d.pool.Execute(ctx, workerID, req.TaskID, action)
	if err != nil {
		result.Error = err.Error()
		d.publishResult(result)
		return
	}
	if execResult != nil {
		result.ExitCode = execResult.ExitCode
		result.Stdout = execResult.Stdout
		result.Stderr = execResult.Stderr
	}
	d.publishResult(result)
}

func (d *NATSDispatcher) publishResult(result actionResponse) {
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	_, _ = d.js.Publish(resultSubject(d.stream, result.TaskID, result.ActionID), data)
}

func (d *NATSDispatcher) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sub == nil {
		return nil
	}
	err := d.sub.Unsubscribe()
	d.sub = nil
	return err
}

var _ interface{ Start(context.Context) error } = (*NATSDispatcher)(nil)
var _ interface{ Close() error } = (*NATSDispatcher)(nil)

func (d *NATSDispatcher) String() string {
	return fmt.Sprintf("NATSDispatcher(stream=%s)", d.stream)
}

// NewNATSDispatcherFromJetStream adapts a real JetStream context into the dispatcher interface.
func NewNATSDispatcherFromJetStream(js nats.JetStreamContext, stream string, pool WorkerPool) *NATSDispatcher {
	return NewNATSDispatcher(jetStreamWrapper{inner: js}, stream, pool)
}
