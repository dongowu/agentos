package skills

import (
	"fmt"
	"strings"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

// Resolver validates actions against a registry and resolves runtime profiles.
type Resolver struct {
	registry *Registry
}

// NewResolver returns a registry-backed action resolver.
func NewResolver(registry *Registry) *Resolver {
	if registry == nil {
		registry = DefaultRegistry()
	}
	return &Resolver{registry: registry}
}

// Resolve validates action payload shape and returns the runtime profile.
func (r *Resolver) Resolve(action *taskdsl.Action) (string, error) {
	if action == nil {
		return "", fmt.Errorf("skill resolve: nil action")
	}
	kind := strings.TrimSpace(action.Kind)
	if kind == "" {
		return "", fmt.Errorf("skill resolve: action kind required")
	}
	def, ok := r.registry.Get(kind)
	if !ok {
		return "", fmt.Errorf("skill resolve: unsupported action kind %q", kind)
	}
	if err := validatePayload(action.Payload, def); err != nil {
		return "", fmt.Errorf("skill resolve %s: %w", kind, err)
	}
	if profile := strings.TrimSpace(action.RuntimeEnv); profile != "" {
		return profile, nil
	}
	if profile := strings.TrimSpace(def.RuntimeProfile); profile != "" {
		return profile, nil
	}
	return "default", nil
}

func validatePayload(payload map[string]any, def Definition) error {
	for _, field := range def.Fields {
		value, ok := lookupField(payload, field.Names)
		if field.Required && !ok {
			return fmt.Errorf("missing required field %q", primaryFieldName(field.Names))
		}
		if !ok {
			continue
		}
		switch field.Type {
		case "", "any":
			continue
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("field %q must be a string", primaryFieldName(field.Names))
			}
		default:
			return fmt.Errorf("unsupported field type %q", field.Type)
		}
	}
	return nil
}

func lookupField(payload map[string]any, names []string) (any, bool) {
	for _, name := range names {
		if payload == nil {
			return nil, false
		}
		if value, ok := payload[name]; ok {
			return value, true
		}
	}
	return nil, false
}

func primaryFieldName(names []string) string {
	if len(names) == 0 {
		return "field"
	}
	return names[0]
}
