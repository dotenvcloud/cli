package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/browser"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
)

// LoginResponse from the web callback
type LoginResponse struct {
	Success  bool                    `json:"success"`
	Contexts map[string]LoginContext `json:"contexts"`
	Error    string                  `json:"error,omitempty"`
}

type LoginContext struct {
	APIKey           string   `json:"api_key"`
	Organization     string   `json:"organization"`
	OrganizationID   string   `json:"organization_id,omitempty"`
	UserEmail        string   `json:"user_email,omitempty"`
	OrganizationName string   `json:"organization_name,omitempty"`
	Permissions      []string `json:"permissions,omitempty"`
}

// BrowserLoginOptions contains options for browser-based login
type BrowserLoginOptions struct {
	APIUrl        string
	CallbackPort  string
	NoBrowser     bool
	IsInteractive bool // Set to false in CI environments
}

// DoBrowserLogin performs browser-based authentication flow
func DoBrowserLogin(ctx context.Context, cm *config.ContextManager, opts BrowserLoginOptions) error {
	// In CI environment, skip browser login
	if os.Getenv("CI") != "" || !opts.IsInteractive {
		return fmt.Errorf("browser login not available in non-interactive environment")
	}

	// Start local server for callback
	port := opts.CallbackPort
	if port == "" {
		// Find available port
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		port = fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
		listener.Close()
	}

	// Generate state for CSRF protection
	state := uuid.New().String()

	// Channel to receive auth result
	authResult := make(chan LoginResponse, 1)
	authError := make(chan error, 1)

	// Start callback server
	server := &http.Server{
		Addr: fmt.Sprintf(":%s", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}

			// Verify state
			if r.URL.Query().Get("state") != state {
				authError <- fmt.Errorf("invalid state parameter")
				http.Error(w, "Invalid state", http.StatusBadRequest)
				return
			}

			// Get auth data
			authData := r.URL.Query().Get("data")
			if authData == "" {
				authError <- fmt.Errorf("no auth data received")
				http.Error(w, "No auth data", http.StatusBadRequest)
				return
			}

			// Decode auth data
			var response LoginResponse
			if err := json.Unmarshal([]byte(authData), &response); err != nil {
				authError <- fmt.Errorf("failed to decode auth data: %w", err)
				http.Error(w, "Invalid auth data", http.StatusBadRequest)
				return
			}

			if !response.Success {
				authError <- fmt.Errorf("authentication failed: %s", response.Error)
				http.Error(w, response.Error, http.StatusBadRequest)
				return
			}

			// Send success response
			authResult <- response

			// Show success page
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authentication Successful</title>
    <style>
        body {
            font-family: system-ui, -apple-system, sans-serif;
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .container {
            text-align: center;
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .success {
            color: #10b981;
            font-size: 3rem;
            margin-bottom: 1rem;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            color: #111827;
        }
        p {
            color: #6b7280;
            margin: 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="success">✓</div>
        <h1>Authentication Successful!</h1>
        <p>You can now close this window and return to your terminal.</p>
    </div>
    <script>
        setTimeout(() => window.close(), 3000);
    </script>
</body>
</html>`)
		}),
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			authError <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Build auth URL
	callbackURL := fmt.Sprintf("http://localhost:%s/callback", port)
	baseURL := strings.TrimSuffix(opts.APIUrl, "/api/v1")
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://" + baseURL
	}
	authURL := fmt.Sprintf("%s/cli/authorize?callback=%s&state=%s",
		baseURL,
		callbackURL,
		state,
	)

	// Open browser or print URL
	if opts.NoBrowser {
		fmt.Println("Please open the following URL in your browser:")
		fmt.Println()
		fmt.Println(authURL)
		fmt.Println()
	} else {
		ui.PrintInfo("Opening browser for authentication...")
		if err := browser.OpenURL(authURL); err != nil {
			ui.PrintWarning("Failed to open browser: %v", err)
			fmt.Println("Please open the following URL manually:")
			fmt.Println()
			fmt.Println(authURL)
			fmt.Println()
		}
	}

	ui.PrintInfo("Waiting for authentication...")

	// Wait for auth result with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	select {
	case result := <-authResult:
		// Shutdown server
		server.Shutdown(context.Background())

		// Process contexts
		ui.PrintSuccess("Authentication successful!")

		for name, ctx := range result.Contexts {
			ui.PrintInfo("Adding context: %s", name)

			if err := cm.Create(name, opts.APIUrl, ctx.APIKey, ctx.Organization); err != nil {
				ui.PrintError("Failed to add context %s: %v", name, err)
				continue
			}

			// Update metadata
			if ctx.UserEmail != "" {
				updates := map[string]interface{}{
					"metadata": config.Metadata{
						UserEmail:        ctx.UserEmail,
						OrganizationID:   ctx.OrganizationID,
						OrganizationName: ctx.OrganizationName,
						Permissions:      ctx.Permissions,
					},
				}
				cm.Update(name, updates)
			}
		}

		// Set first context as current
		if len(result.Contexts) > 0 {
			for name := range result.Contexts {
				if err := cm.Use(name); err == nil {
					ui.PrintInfo("Set current context to: %s", name)
				}
				break
			}
		}

		ui.PrintSuccess("Login complete! You now have access to %d organization(s)",
			len(result.Contexts))

		return nil

	case err := <-authError:
		server.Shutdown(context.Background())
		return fmt.Errorf("authentication failed: %w", err)

	case <-timeoutCtx.Done():
		server.Shutdown(context.Background())
		return fmt.Errorf("authentication timed out")
	}
}
