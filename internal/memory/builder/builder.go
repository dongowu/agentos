// Package builder provides a factory for creating memory.Provider instances by type name.
package builder

import (
	"fmt"
	"time"

	"github.com/dongowu/agentos/internal/adapters/memory/inmemory"
	"github.com/dongowu/agentos/internal/adapters/memory/redis"
	"github.com/dongowu/agentos/internal/memory"
)

// NewProvider creates a memory.Provider by type name.
// Supported types: "inmemory", "redis".
func NewProvider(providerType string, opts map[string]any) (memory.Provider, error) {
	switch providerType {
	case "inmemory":
		var iopts []inmemory.Option
		if v, ok := opts["ttl"]; ok {
			switch d := v.(type) {
			case time.Duration:
				iopts = append(iopts, inmemory.WithTTL(d))
			case string:
				dur, err := time.ParseDuration(d)
				if err != nil {
					return nil, fmt.Errorf("invalid ttl %q: %w", d, err)
				}
				iopts = append(iopts, inmemory.WithTTL(dur))
			}
		}
		return inmemory.New(iopts...), nil

	case "redis":
		addr, _ := opts["addr"].(string)
		if addr == "" {
			addr = "127.0.0.1:6379"
		}
		var ropts []redis.Option
		if p, ok := opts["prefix"].(string); ok {
			ropts = append(ropts, redis.WithPrefix(p))
		}
		return redis.New(addr, ropts...)

	default:
		return nil, fmt.Errorf("unknown memory provider type: %q", providerType)
	}
}
