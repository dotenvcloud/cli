package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	dotenv "github.com/dotenvcloud/sdk-go"
	"github.com/google/uuid"

	"github.com/dotenvcloud/cli/internal/build"
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
			Version:     build.GetInfo().Version,
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

// sendBatch flushes queued events to the telemetry endpoint. The endpoint
// accepts a single flat event per call (see the OpenAPI TelemetryRequest
// contract), so each queued event becomes its own request. Best-effort:
// failures are ignored, and we stop early if the shared deadline expires.
func (c *Client) sendBatch(events []Event) {
	if c.sdkClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := range events {
		if _, err := c.sdkClient.Telemetry.Send(ctx, eventToRequest(&events[i])); err != nil {
			// Telemetry is optional; ignore failures. Bail on the rest of the
			// batch once the context is done so we don't spin on a dead network.
			if ctx.Err() != nil {
				return
			}
		}
	}
}

// eventToRequest maps an internal Event onto the SDK's flat TelemetryRequest,
// pulling the typed fields out of the property bag the Track* helpers populate.
func eventToRequest(e *Event) dotenv.TelemetryRequest {
	req := dotenv.TelemetryRequest{
		Version:     e.Context.Version,
		OS:          e.Context.OS,
		Arch:        e.Context.Arch,
		AnonymousID: e.Context.AnalyticsID,
	}

	if v, ok := e.Properties["command"].(string); ok {
		req.Command = v
	}
	if v, ok := e.Properties["duration"].(int64); ok {
		req.Duration = v
	}
	if v, ok := e.Properties["success"].(bool); ok {
		req.Success = v
	}
	if v, ok := e.Properties["error_type"].(string); ok {
		req.ErrorType = v
	}
	if v, ok := e.Properties["feature"].(string); ok {
		req.Features = []string{v}
	}

	return req
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

	if err := c.Track("cli.error", map[string]interface{}{
		"command":    command,
		"error_type": errMsg,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}); err != nil {
		// Telemetry is best-effort; ignore submission failures.
		_ = err
	}
}

// TrackCommand tracks a command execution
func (c *Client) TrackCommand(command string, duration time.Duration, success bool) {
	if err := c.Track("cli.command", map[string]interface{}{
		"command":  command,
		"duration": duration.Milliseconds(),
		"success":  success,
	}); err != nil {
		// Telemetry is best-effort; ignore submission failures.
		_ = err
	}
}
