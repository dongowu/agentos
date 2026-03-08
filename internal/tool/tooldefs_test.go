package tool

import (
	"context"
	"testing"
)

type mockToolWithSchema struct {
	name   string
	desc   string
	schema map[string]any
}

func (m *mockToolWithSchema) Name() string                                         { return m.name }
func (m *mockToolWithSchema) Description() string                                  { return m.desc }
func (m *mockToolWithSchema) Run(_ context.Context, _ map[string]any) (any, error) { return nil, nil }
func (m *mockToolWithSchema) Schema() map[string]any                               { return m.schema }

type mockToolWithoutSchema struct {
	name string
	desc string
}

func (m *mockToolWithoutSchema) Name() string        { return m.name }
func (m *mockToolWithoutSchema) Description() string { return m.desc }
func (m *mockToolWithoutSchema) Run(_ context.Context, _ map[string]any) (any, error) {
	return nil, nil
}

func TestBuildToolDefs_WithSchema(t *testing.T) {
	tools := []Tool{
		&mockToolWithSchema{
			name:   "shell",
			desc:   "Execute shell commands",
			schema: map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}},
		},
	}
	defs := BuildToolDefs(tools)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Name != "shell" {
		t.Errorf("expected name 'shell', got %q", defs[0].Name)
	}
	if defs[0].Description != "Execute shell commands" {
		t.Errorf("expected description, got %q", defs[0].Description)
	}
	if defs[0].Parameters["type"] != "object" {
		t.Errorf("expected schema with type=object, got %v", defs[0].Parameters)
	}
}

func TestBuildToolDefs_WithoutSchema(t *testing.T) {
	tools := []Tool{
		&mockToolWithoutSchema{name: "basic", desc: "A basic tool"},
	}
	defs := BuildToolDefs(tools)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Parameters["type"] != "object" {
		t.Errorf("expected fallback schema with type=object, got %v", defs[0].Parameters)
	}
}

func TestBuildToolDefs_NilSchema(t *testing.T) {
	tools := []Tool{
		&mockToolWithSchema{name: "nil-schema", desc: "Tool with nil schema", schema: nil},
	}
	defs := BuildToolDefs(tools)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Parameters["type"] != "object" {
		t.Errorf("expected fallback schema when Schema() returns nil, got %v", defs[0].Parameters)
	}
}

func TestBuildToolDefs_Empty(t *testing.T) {
	defs := BuildToolDefs(nil)
	if len(defs) != 0 {
		t.Errorf("expected 0 defs for nil input, got %d", len(defs))
	}
}
