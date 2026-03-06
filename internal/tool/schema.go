package tool

// SchemaAware is an optional interface that tools can implement
// to provide a JSON Schema description of their input parameters.
type SchemaAware interface {
	Schema() map[string]any
}
