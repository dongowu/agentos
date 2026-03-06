package agent

import (
	"testing"
)

func TestNewRuntime_ValidConfig(t *testing.T) {
	cfg := &Config{
		Name:  "test-agent",
		Model: "gpt-4",
		Tools: []string{"shell.exec", "browser.search"},
		Policy: PolicyConfig{
			Allow: []string{"shell.*", "browser.*"},
			Deny:  []string{"shell.rm"},
		},
		Memory: MemoryConfig{Type: "redis", TTL: 3600},
	}
	rt, err := NewRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.Config.Name != "test-agent" {
		t.Fatalf("expected name test-agent, got %s", rt.Config.Name)
	}
}

func TestNewRuntime_NilConfig(t *testing.T) {
	_, err := NewRuntime(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewRuntime_InvalidConfig(t *testing.T) {
	cfg := &Config{} // missing name and model
	_, err := NewRuntime(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCheckPolicy_AllowMatch(t *testing.T) {
	cfg := &Config{
		Name:  "policy-agent",
		Model: "gpt-4",
		Policy: PolicyConfig{
			Allow: []string{"browser.*"},
		},
	}
	rt, err := NewRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rt.CheckPolicy("browser.search"); err != nil {
		t.Fatalf("expected browser.search to be allowed: %v", err)
	}
}

func TestCheckPolicy_DenyMatch(t *testing.T) {
	cfg := &Config{
		Name:  "policy-agent",
		Model: "gpt-4",
		Policy: PolicyConfig{
			Allow: []string{"shell.*"},
			Deny:  []string{"shell.rm"},
		},
	}
	rt, err := NewRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rt.CheckPolicy("shell.rm"); err == nil {
		t.Fatal("expected shell.rm to be denied")
	}
}

func TestCheckPolicy_DenyTakesPrecedence(t *testing.T) {
	cfg := &Config{
		Name:  "policy-agent",
		Model: "gpt-4",
		Policy: PolicyConfig{
			Allow: []string{"*"},
			Deny:  []string{"shell.exec"},
		},
	}
	rt, err := NewRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// shell.exec matches both allow and deny; deny wins
	if err := rt.CheckPolicy("shell.exec"); err == nil {
		t.Fatal("expected shell.exec to be denied even though allow has *")
	}
	// browser.search matches allow, not deny
	if err := rt.CheckPolicy("browser.search"); err != nil {
		t.Fatalf("expected browser.search to be allowed: %v", err)
	}
}

func TestCheckPolicy_NotInAllow(t *testing.T) {
	cfg := &Config{
		Name:  "policy-agent",
		Model: "gpt-4",
		Policy: PolicyConfig{
			Allow: []string{"browser.*"},
		},
	}
	rt, err := NewRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rt.CheckPolicy("shell.exec"); err == nil {
		t.Fatal("expected shell.exec to be denied when not in allow list")
	}
}

func TestCheckPolicy_EmptyPolicy(t *testing.T) {
	cfg := &Config{
		Name:  "open-agent",
		Model: "gpt-4",
	}
	rt, err := NewRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No policy = everything allowed
	if err := rt.CheckPolicy("anything.goes"); err != nil {
		t.Fatalf("expected all tools allowed with empty policy: %v", err)
	}
}

func TestAvailableTools(t *testing.T) {
	cfg := &Config{
		Name:  "tool-agent",
		Model: "gpt-4",
		Tools: []string{"browser.search", "shell.exec", "git.commit"},
	}
	rt, err := NewRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tools := rt.AvailableTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}
