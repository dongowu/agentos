package agent

import "testing"

func TestConfigValidate_RequiresName(t *testing.T) {
    cfg := &Config{}
    if err := cfg.Validate(); err == nil {
        t.Fatal("expected validation error for empty name")
    }
}