package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/pkg/events"
)

type streamBus struct {
	mu         sync.Mutex
	handlers   map[string][]func(any)
	subscribed chan string
}

func newStreamBus() *streamBus {
	return &streamBus{handlers: make(map[string][]func(any)), subscribed: make(chan string, 16)}
}

func (b *streamBus) Publish(_ context.Context, topic string, payload any) error {
	b.mu.Lock()
	handlers := append([]func(any){}, b.handlers[topic]...)
	b.mu.Unlock()
	for _, handler := range handlers {
		handler(payload)
	}
	return nil
}

func (b *streamBus) Subscribe(topic string, handler func(any)) (func(), error) {
	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	b.mu.Unlock()
	b.subscribed <- topic
	return func() {}, nil
}

var _ messaging.EventBus = (*streamBus)(nil)

type flushBuffer struct {
	header http.Header
	buf    bytes.Buffer
	code   int
	mu     sync.Mutex
}

func newFlushBuffer() *flushBuffer {
	return &flushBuffer{header: make(http.Header)}
}

func (f *flushBuffer) Header() http.Header    { return f.header }
func (f *flushBuffer) WriteHeader(status int) { f.code = status }
func (f *flushBuffer) Write(p []byte) (int, error) {
	if f.code == 0 {
		f.code = http.StatusOK
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}
func (f *flushBuffer) Flush() {}
func (f *flushBuffer) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

func TestServer_TaskStream_EmitsActionCompletedTelemetry(t *testing.T) {
	bus := newStreamBus()
	srv := &Server{API: stubTaskAPI{}, Bus: bus}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/stream", nil).WithContext(ctx)
	rec := newFlushBuffer()
	done := make(chan struct{})
	go func() {
		srv.handler().ServeHTTP(rec, req)
		close(done)
	}()

	for i := 0; i < 5; i++ {
		select {
		case <-bus.subscribed:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for stream subscriptions")
		}
	}

	if err := bus.Publish(context.Background(), "task.action.completed", &events.ActionCompleted{TaskID: "task-123", ActionID: "act-1", ExitCode: 0, Stdout: "hello", Stderr: "warn"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	cancel()
	<-done

	if rec.code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.code)
	}
	if got := rec.header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", got)
	}
	body := rec.String()
	if !strings.Contains(body, "event: task.action.completed") {
		t.Fatalf("expected task.action.completed event, got %q", body)
	}
	if !strings.Contains(body, "\"stdout\":\"hello\"") {
		t.Fatalf("expected stdout in stream body, got %q", body)
	}
}

func TestServer_ActionStream_EmitsRuntimeOutputAndCompletion(t *testing.T) {
	bus := newStreamBus()
	srv := &Server{API: stubTaskAPI{}, Bus: bus}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-123/actions/act-1/stream", nil).WithContext(ctx)
	rec := newFlushBuffer()
	done := make(chan struct{})
	go func() {
		srv.handler().ServeHTTP(rec, req)
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-bus.subscribed:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for action stream subscriptions")
		}
	}

	if err := bus.Publish(context.Background(), "task.action.output", &events.ActionOutputChunk{TaskID: "task-123", ActionID: "act-1", Kind: "stdout", Text: "hello"}); err != nil {
		t.Fatalf("Publish output: %v", err)
	}
	if err := bus.Publish(context.Background(), "task.action.completed", &events.ActionCompleted{TaskID: "task-123", ActionID: "act-1", ExitCode: 0}); err != nil {
		t.Fatalf("Publish completed: %v", err)
	}
	cancel()
	<-done

	if rec.code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.code)
	}
	body := rec.String()
	if !strings.Contains(body, "event: task.action.output") {
		t.Fatalf("expected task.action.output event, got %q", body)
	}
	if !strings.Contains(body, "event: task.action.completed") {
		t.Fatalf("expected task.action.completed event, got %q", body)
	}
	if !strings.Contains(body, "\"action_id\":\"act-1\"") {
		t.Fatalf("expected action id in action stream body, got %q", body)
	}
	if !strings.Contains(body, "\"text\":\"hello\"") {
		t.Fatalf("expected output chunk text in action stream body, got %q", body)
	}
}
