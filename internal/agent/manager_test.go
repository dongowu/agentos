package agent

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestManagerLoadFromDir(t *testing.T) {
	dir := t.TempDir()

	// Write two agent YAML files
	agent1 := `
name: agent-one
model: gpt-4
memory:
  type: redis
  ttl: 3600
tools:
  - browser.search
workflow:
  - plan
  - execute
`
	agent2 := `
name: agent-two
model: gemini-pro
tools:
  - shell.exec
policy:
  allow: ["shell.*"]
  deny: ["shell.rm"]
workflow:
  - execute
`
	if err := os.WriteFile(filepath.Join(dir, "agent-one.yaml"), []byte(agent1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent-two.yaml"), []byte(agent2), 0644); err != nil {
		t.Fatal(err)
	}
	// Write a non-yaml file that should be ignored
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager()
	if err := mgr.LoadFromDir(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := mgr.List()
	sort.Strings(names)
	if len(names) != 2 {
		t.Fatalf("expected 2 agents, got %d: %v", len(names), names)
	}
	if names[0] != "agent-one" || names[1] != "agent-two" {
		t.Fatalf("unexpected agent names: %v", names)
	}

	rt := mgr.Get("agent-one")
	if rt == nil {
		t.Fatal("expected agent-one runtime, got nil")
	}
	if rt.Config.Memory.TTL != 3600 {
		t.Fatalf("expected TTL 3600, got %d", rt.Config.Memory.TTL)
	}

	rt2 := mgr.Get("agent-two")
	if rt2 == nil {
		t.Fatal("expected agent-two runtime, got nil")
	}
	if err := rt2.CheckPolicy("shell.exec"); err != nil {
		t.Fatalf("expected shell.exec allowed for agent-two: %v", err)
	}
	if err := rt2.CheckPolicy("shell.rm"); err == nil {
		t.Fatal("expected shell.rm denied for agent-two")
	}
}

func TestManagerGet_NotFound(t *testing.T) {
	mgr := NewManager()
	if rt := mgr.Get("nonexistent"); rt != nil {
		t.Fatal("expected nil for nonexistent agent")
	}
}

func TestManagerLoadFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()
	if err := mgr.LoadFromDir(dir); err != nil {
		t.Fatalf("unexpected error for empty dir: %v", err)
	}
	if len(mgr.List()) != 0 {
		t.Fatal("expected 0 agents for empty dir")
	}
}

func TestManagerLoadFromDir_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	bad := `name: ""`
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager()
	if err := mgr.LoadFromDir(dir); err == nil {
		t.Fatal("expected error for invalid agent yaml")
	}
}

func TestManagerLoadFromDir_NonexistentDir(t *testing.T) {
	mgr := NewManager()
	if err := mgr.LoadFromDir("/tmp/nonexistent-agent-dir-xyz"); err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
