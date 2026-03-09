package policy

import (
	"context"
	"testing"
)

func TestDefaultEngine_AllowByDefault(t *testing.T) {
	engine := NewDefaultEngine(Config{})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatalf("expected allowed, got denied: %s", dec.Reason)
	}
}

func TestDefaultEngine_DenyTakesPrecedence(t *testing.T) {
	engine := NewDefaultEngine(Config{
		Rules: []Rule{
			{
				Agent: "worker-*",
				Actions: Actions{
					Allow:            []string{"shell", "http"},
					Deny:             []string{"shell"},
					ApprovalRequired: []string{"shell"},
				},
			},
		},
	})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected denied because deny takes precedence over allow")
	}
}

func TestDefaultEngine_ApprovalRequiredBlocksMatchedTool(t *testing.T) {
	engine := NewDefaultEngine(Config{
		Rules: []Rule{
			{
				Agent: "worker-*",
				Actions: Actions{
					Allow:            []string{"shell", "http*"},
					ApprovalRequired: []string{"shell"},
				},
			},
		},
	})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected approval-required tool to be blocked")
	}
	if dec.Reason != ReasonApprovalRequired {
		t.Fatalf("expected reason %q, got %q", ReasonApprovalRequired, dec.Reason)
	}
}

func TestDefaultEngine_ApprovalRequiredPrecedesAllow(t *testing.T) {
	engine := NewDefaultEngine(Config{
		Rules: []Rule{
			{
				Agent: "worker-*",
				Actions: Actions{
					Allow:            []string{"git.*"},
					ApprovalRequired: []string{"git.clone"},
				},
			},
		},
	})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "git.clone",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected approval-required rule to win over allow")
	}
	if dec.Reason != ReasonApprovalRequired {
		t.Fatalf("expected reason %q, got %q", ReasonApprovalRequired, dec.Reason)
	}
}

func TestDefaultEngine_AllowPattern(t *testing.T) {
	engine := NewDefaultEngine(Config{
		Rules: []Rule{
			{
				Agent: "worker-*",
				Actions: Actions{
					Allow: []string{"http*"},
				},
			},
		},
	})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "http_get",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatalf("expected allowed by glob, got denied: %s", dec.Reason)
	}
}

func TestDefaultEngine_DenyUnmatchedAllow(t *testing.T) {
	engine := NewDefaultEngine(Config{
		Rules: []Rule{
			{
				Agent: "worker-*",
				Actions: Actions{
					Allow: []string{"http*"},
				},
			},
		},
	})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected denied because tool does not match any allow pattern")
	}
}

func TestDefaultEngine_AgentGlobMismatch(t *testing.T) {
	engine := NewDefaultEngine(Config{
		Rules: []Rule{
			{
				Agent: "admin-*",
				Actions: Actions{
					Deny: []string{"*"},
				},
			},
		},
	})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatal("expected allowed because agent glob does not match")
	}
}

func TestDefaultEngine_DangerousCommand(t *testing.T) {
	engine := NewDefaultEngine(Config{})
	cases := []string{
		"rm -rf /",
		"sudo rm -rf /*",
		"DD if=/dev/zero of=/dev/sda",
		"mkfs.ext4 /dev/sda",
	}
	for _, cmd := range cases {
		dec, err := engine.Evaluate(context.Background(), PolicyRequest{
			AgentName: "worker-1",
			ToolName:  "shell",
			Command:   cmd,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Allowed {
			t.Fatalf("expected command %q to be blocked", cmd)
		}
	}
}

func TestDefaultEngine_SafeCommand(t *testing.T) {
	engine := NewDefaultEngine(Config{})
	dec, err := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
		Command:   "ls -la /tmp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatalf("expected safe command allowed, got denied: %s", dec.Reason)
	}
}

func TestDefaultEngine_RateLimit(t *testing.T) {
	engine := NewDefaultEngine(Config{
		RateLimit: RateLimitConfig{MaxActionsPerHour: 3},
	})
	ctx := context.Background()
	req := PolicyRequest{AgentName: "worker-1", ToolName: "shell"}

	// First 3 should succeed
	for i := 0; i < 3; i++ {
		dec, err := engine.Evaluate(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if !dec.Allowed {
			t.Fatalf("request %d: expected allowed", i+1)
		}
	}

	// 4th should be rate limited
	dec, err := engine.Evaluate(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected rate limit to kick in on 4th request")
	}
	if dec.Reason == "" {
		t.Fatal("expected a reason for rate limit denial")
	}
}

func TestDefaultEngine_RateLimitPerAgent(t *testing.T) {
	engine := NewDefaultEngine(Config{
		RateLimit: RateLimitConfig{MaxActionsPerHour: 1},
	})
	ctx := context.Background()

	// Agent A uses its one allowed action
	dec, _ := engine.Evaluate(ctx, PolicyRequest{AgentName: "a", ToolName: "shell"})
	if !dec.Allowed {
		t.Fatal("agent a first request should be allowed")
	}

	// Agent B should still have its own allowance
	dec, _ = engine.Evaluate(ctx, PolicyRequest{AgentName: "b", ToolName: "shell"})
	if !dec.Allowed {
		t.Fatal("agent b first request should be allowed (independent rate limit)")
	}

	// Agent A second request should be denied
	dec, _ = engine.Evaluate(ctx, PolicyRequest{AgentName: "a", ToolName: "shell"})
	if dec.Allowed {
		t.Fatal("agent a second request should be rate limited")
	}
}

func TestDefaultEngine_DefaultAutonomy(t *testing.T) {
	engine := NewDefaultEngine(Config{
		DefaultAutonomy: Autonomous,
	})
	dec, _ := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if dec.Autonomy != Autonomous {
		t.Fatalf("expected autonomy %q, got %q", Autonomous, dec.Autonomy)
	}
}

func TestDefaultEngine_DefaultAutonomyFallback(t *testing.T) {
	engine := NewDefaultEngine(Config{})
	dec, _ := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if dec.Autonomy != Supervised {
		t.Fatalf("expected default autonomy %q, got %q", Supervised, dec.Autonomy)
	}
}

func TestDefaultEngine_MultipleRulesFirstMatch(t *testing.T) {
	engine := NewDefaultEngine(Config{
		Rules: []Rule{
			{
				Agent:   "worker-*",
				Actions: Actions{Allow: []string{"http*"}},
			},
			{
				Agent:   "*",
				Actions: Actions{Deny: []string{"*"}},
			},
		},
	})

	// worker-1 matches first rule, http_get is allowed
	dec, _ := engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "http_get",
	})
	if !dec.Allowed {
		t.Fatal("expected worker-1 http_get to match first rule and be allowed")
	}

	// worker-1 shell: first rule matches agent but tool not in allow list
	dec, _ = engine.Evaluate(context.Background(), PolicyRequest{
		AgentName: "worker-1",
		ToolName:  "shell",
	})
	if dec.Allowed {
		t.Fatal("expected worker-1 shell to be denied by first rule (not in allow list)")
	}
}
