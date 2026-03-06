// Package defaults imports all built-in adapter plugins so their init()
// functions register factories with the adapter registry.
package defaults

import (
	_ "github.com/dongowu/ai-orchestrator/internal/adapters/messaging/memory"
	_ "github.com/dongowu/ai-orchestrator/internal/adapters/messaging/nats"
	_ "github.com/dongowu/ai-orchestrator/internal/adapters/persistence/memory"
	_ "github.com/dongowu/ai-orchestrator/internal/adapters/persistence/postgres"
)
