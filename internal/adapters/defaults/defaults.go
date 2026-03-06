// Package defaults imports all built-in adapter plugins so their init()
// functions register factories with the adapter registry.
package defaults

import (
	_ "github.com/dongowu/agentos/internal/adapters/messaging/memory"
	_ "github.com/dongowu/agentos/internal/adapters/messaging/nats"
	_ "github.com/dongowu/agentos/internal/adapters/persistence/memory"
	_ "github.com/dongowu/agentos/internal/adapters/persistence/postgres"
)
