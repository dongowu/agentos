package agent

import (
	"path/filepath"
	"testing"
)

func TestExampleAgentConfigs_LoadCleanly(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "examples", "agents", "*.yaml"))
	if err != nil {
		t.Fatalf("glob example agents: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("expected example agent configs")
	}

	required := map[string]bool{
		"coding-agent":      false,
		"review-agent":      false,
		"release-agent":     false,
		"ops-runbook-agent": false,
	}

	for _, path := range paths {
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		if _, err := NewRuntime(cfg); err != nil {
			t.Fatalf("runtime %s: %v", path, err)
		}
		if _, ok := required[cfg.Name]; ok {
			required[cfg.Name] = true
		}
	}

	for name, found := range required {
		if !found {
			t.Fatalf("expected example agent %q", name)
		}
	}
}
