package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	dotenv "github.com/dotenv/sdk-go"
	"github.com/google/uuid"
)

// Event represents a telemetry event
type Event struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
	Context    Context                `json:"context"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Context contains telemetry context information
type Context struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Version     string `json:"version"`
	CI          bool   `json:"ci"`
	SessionID   string `json:"session_id"`
	AnalyticsID string `json:"analytics_id,omitempty"`
}

// Client handles telemetry data collection and submission
type Client struct {
	enabled     bool
	analyticsID string
	sessionID   string
	sdkClient   *dotenv.Client
	queue       chan Event
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// NewClient creates a new telemetry client
func NewClient(sdkClient *dotenv.Client, analyticsID string) *Client {
	client := &Client{
		enabled:     false, // Opt-in by default
		analyticsID: analyticsID,
		sessionID:   uuid.New().String(),
		sdkClient:   sdkClient,
		queue:       make(chan Event, 100),
	}

	// Start background worker
	client.wg.Add(1)
	go client.worker()

	return client
}

// Track records a telemetry event
func (c *Client) Track(name string, properties map[string]interface{}) error {
	c.mu.RLock()
	enabled := c.enabled
	c.mu.RUnlock()

	if !enabled {
		return nil
	}

	// Don't track in CI environments unless explicitly enabled
	if isCI() && !isCITelemetryEnabled() {
		return nil
	}

	event := Event{
		ID:         uuid.New().String(),
		Name:       name,
		Properties: properties,
		Context: Context{
			OS:          runtime.GOOS,
			Arch:        runtime.GOARCH,
			Version:     "1.0.0", // TODO: Get from version package
			CI:          isCI(),
			SessionID:   c.sessionID,
			AnalyticsID: c.analyticsID,
		},
		Timestamp: time.Now(),
	}

	// Non-blocking send
	select {
	case c.queue <- event:
		return nil
	default:
		// Queue is full, drop event
		return nil
	}
}

// SetEnabled enables or disables telemetry collection
func (c *Client) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
}

// IsEnabled returns whether telemetry is enabled
func (c *Client) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// Close shuts down the telemetry client
func (c *Client) Close() error {
	close(c.queue)
	c.wg.Wait()
	return nil
}

// worker processes events in the background
func (c *Client) worker() {
	defer c.wg.Done()

	batch := make([]Event, 0, 10)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-c.queue:
			if !ok {
				// Channel closed, flush remaining events
				if len(batch) > 0 {
					c.sendBatch(batch)
				}
				return
			}
			batch = append(batch, event)

			// Send batch if it's full
			if len(batch) >= 10 {
				c.sendBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Send batch periodically
			if len(batch) > 0 {
				c.sendBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// sendBatch sends a batch of events to the telemetry endpoint
func (c *Client) sendBatch(events []Event) {
	if c.sdkClient == nil {
		return
	}

	// Convert our Event type to SDK's TelemetryEvent type
	sdkEvents := make([]dotenv.TelemetryEvent, len(events))
	for i, event := range events {
		sdkEvents[i] = dotenv.TelemetryEvent{
			ID:         event.ID,
			Name:       event.Name,
			Properties: event.Properties,
			Context: dotenv.TelemetryContext{
				OS:          event.Context.OS,
				Arch:        event.Context.Arch,
				Version:     event.Context.Version,
				CI:          event.Context.CI,
				SessionID:   event.Context.SessionID,
				AnalyticsID: event.Context.AnalyticsID,
			},
			Timestamp: event.Timestamp,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add CLI version to context for the SDK to use
	ctx = context.WithValue(ctx, "cli-version", "1.0.0") // TODO: Get from version package

	// Use SDK to send batch
	_, err := c.sdkClient.Telemetry.SendBatch(ctx, sdkEvents)
	if err != nil {
		// Silently ignore errors as telemetry is optional
		return
	}
}

// isCI detects if running in a CI environment
func isCI() bool {
	ciVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"JENKINS_URL",
		"BUILDKITE",
	}

	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}

	return false
}

// isCITelemetryEnabled checks if telemetry is explicitly enabled in CI
func isCITelemetryEnabled() bool {
	return os.Getenv("DOTENV_CLI_TELEMETRY_CI") == "1"
}

// TrackError tracks an error event with sanitized information
func (c *Client) TrackError(command string, err error) {
	if err == nil {
		return
	}

	// Sanitize error message to remove sensitive data
	errMsg := fmt.Sprintf("%T", err)

	c.Track("cli.error", map[string]interface{}{
		"command":    command,
		"error_type": errMsg,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	})
}

// TrackCommand tracks a command execution
func (c *Client) TrackCommand(command string, duration time.Duration, success bool) {
	c.Track("cli.command", map[string]interface{}{
		"command":  command,
		"duration": duration.Milliseconds(),
		"success":  success,
	})
}
