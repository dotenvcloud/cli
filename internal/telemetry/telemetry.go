package telemetry

// Collector handles telemetry data collection
type Collector interface {
	Track(event string, properties map[string]interface{}) error
	SetEnabled(enabled bool)
	IsEnabled() bool
}

// TODO: This will be implemented if needed
