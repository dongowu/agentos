package builtin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPGetTool_Name(t *testing.T) {
	tool := HTTPGetTool{}
	if tool.Name() != "http.get" {
		t.Fatalf("expected name http.get, got %q", tool.Name())
	}
}

func TestHTTPGetTool_Run_FetchesURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello from server"))
	}))
	defer srv.Close()

	tool := HTTPGetTool{}
	out, err := tool.Run(context.Background(), map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", out)
	}
	if result["status"] != http.StatusOK {
		t.Fatalf("expected status 200, got %v", result["status"])
	}
	if result["body"] != "hello from server" {
		t.Fatalf("expected body 'hello from server', got %q", result["body"])
	}
}

func TestHTTPGetTool_Run_MissingURL(t *testing.T) {
	tool := HTTPGetTool{}
	_, err := tool.Run(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestHTTPGetTool_Run_InvalidURLType(t *testing.T) {
	tool := HTTPGetTool{}
	_, err := tool.Run(context.Background(), map[string]any{"url": 123})
	if err == nil {
		t.Fatal("expected error for non-string url")
	}
}

func TestHTTPGetTool_Run_ReturnsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	tool := HTTPGetTool{}
	out, err := tool.Run(context.Background(), map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", out)
	}
	if result["status"] != http.StatusNotFound {
		t.Fatalf("expected status 404, got %v", result["status"])
	}
}

func TestHTTPPostTool_Name(t *testing.T) {
	tool := HTTPPostTool{}
	if tool.Name() != "http.post" {
		t.Fatalf("expected name http.post, got %q", tool.Name())
	}
}

func TestHTTPPostTool_Run_PostsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected content-type application/json, got %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		w.Write(body)
	}))
	defer srv.Close()

	tool := HTTPPostTool{}
	out, err := tool.Run(context.Background(), map[string]any{
		"url":          srv.URL,
		"body":         `{"key":"value"}`,
		"content_type": "application/json",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", out)
	}
	if result["status"] != http.StatusCreated {
		t.Fatalf("expected status 201, got %v", result["status"])
	}
	if result["body"] != `{"key":"value"}` {
		t.Fatalf("expected echoed body, got %q", result["body"])
	}
}

func TestHTTPPostTool_Run_DefaultsContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected default content-type application/json, got %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tool := HTTPPostTool{}
	_, err := tool.Run(context.Background(), map[string]any{
		"url":  srv.URL,
		"body": "test",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestHTTPPostTool_Run_MissingURL(t *testing.T) {
	tool := HTTPPostTool{}
	_, err := tool.Run(context.Background(), map[string]any{"body": "test"})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestHTTPPostTool_Run_MissingBody(t *testing.T) {
	tool := HTTPPostTool{}
	_, err := tool.Run(context.Background(), map[string]any{"url": "http://example.com"})
	if err == nil {
		t.Fatal("expected error for missing body")
	}
}
