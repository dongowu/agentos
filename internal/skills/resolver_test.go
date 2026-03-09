package skills

import (
	"testing"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

func TestResolver_UsesRegisteredRuntimeProfile(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Definition{
		Name:           "command.exec",
		RuntimeProfile: "sandbox",
		Fields:         []FieldDefinition{{Names: []string{"cmd", "command"}, Type: "string", Required: true}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolver := NewResolver(registry)

	profile, err := resolver.Resolve(&taskdsl.Action{Kind: "command.exec", Payload: map[string]any{"cmd": "echo ok"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if profile != "sandbox" {
		t.Fatalf("expected sandbox, got %q", profile)
	}
}

func TestResolver_PreservesExplicitRuntimeEnv(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Definition{
		Name:           "command.exec",
		RuntimeProfile: "sandbox",
		Fields:         []FieldDefinition{{Names: []string{"cmd", "command"}, Type: "string", Required: true}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolver := NewResolver(registry)

	profile, err := resolver.Resolve(&taskdsl.Action{Kind: "command.exec", RuntimeEnv: "docker", Payload: map[string]any{"cmd": "echo ok"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if profile != "docker" {
		t.Fatalf("expected docker, got %q", profile)
	}
}

func TestResolver_RejectsMissingRequiredField(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Definition{
		Name:           "file.write",
		RuntimeProfile: "default",
		Fields: []FieldDefinition{
			{Names: []string{"path"}, Type: "string", Required: true},
			{Names: []string{"content"}, Type: "string", Required: true},
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolver := NewResolver(registry)

	_, err := resolver.Resolve(&taskdsl.Action{Kind: "file.write", Payload: map[string]any{"path": "/tmp/out.txt"}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
