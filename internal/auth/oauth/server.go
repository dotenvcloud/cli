package oauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// CallbackServer handles OAuth2 callback
type CallbackServer struct {
	Port          string
	AuthCode      chan string
	AuthError     chan error
	AuthState     string
	ExpectedState string
}

// NewCallbackServer creates a new callback server
func NewCallbackServer(expectedState string) *CallbackServer {
	return &CallbackServer{
		AuthCode:      make(chan string, 1),
		AuthError:     make(chan error, 1),
		ExpectedState: expectedState,
	}
}

// FindAvailablePort finds an available port for the callback server
func (s *CallbackServer) FindAvailablePort() (string, error) {
	// Try ports 43893-43895 as configured in the OAuth client
	ports := []string{"43893", "43894", "43895"}

	for _, port := range ports {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
		if err == nil {
			listener.Close()
			s.Port = port
			return port, nil
		}
	}

	// If configured ports are not available, find a random port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", fmt.Errorf("failed to find available port: %w", err)
	}
	port := fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
	listener.Close()

	s.Port = port
	return port, nil
}

// Start starts the callback server
func (s *CallbackServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", s.Port),
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.AuthError <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Shutdown server when context is cancelled
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	return nil
}

// handleCallback handles the OAuth callback request
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	error := query.Get("error")
	errorDesc := query.Get("error_description")

	// Check for errors
	if error != "" {
		s.AuthError <- fmt.Errorf("authorization failed: %s - %s", error, errorDesc)
		s.showErrorPage(w, error, errorDesc)
		return
	}

	// Verify state
	if state != s.ExpectedState {
		s.AuthError <- fmt.Errorf("invalid state parameter")
		s.showErrorPage(w, "invalid_state", "State parameter mismatch")
		return
	}

	// Check for authorization code
	if code == "" {
		s.AuthError <- fmt.Errorf("no authorization code received")
		s.showErrorPage(w, "no_code", "No authorization code in response")
		return
	}

	// Send code through channel
	s.AuthCode <- code
	s.AuthState = state

	// Show success page
	s.showSuccessPage(w)
}

// GetCallbackURL returns the callback URL for this server
func (s *CallbackServer) GetCallbackURL() string {
	return fmt.Sprintf("http://localhost:%s/callback", s.Port)
}

// showSuccessPage displays a success page to the user
func (s *CallbackServer) showSuccessPage(w http.ResponseWriter) {
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
}

// showErrorPage displays an error page to the user
func (s *CallbackServer) showErrorPage(w http.ResponseWriter, error, description string) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authentication Failed</title>
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
            max-width: 400px;
        }
        .error {
            color: #ef4444;
            font-size: 3rem;
            margin-bottom: 1rem;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            color: #111827;
        }
        .error-code {
            color: #ef4444;
            font-weight: bold;
        }
        p {
            color: #6b7280;
            margin: 0.5rem 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="error">✗</div>
        <h1>Authentication Failed</h1>
        <p>Error: <span class="error-code">%s</span></p>
        <p>%s</p>
        <p>Please close this window and try again.</p>
    </div>
</body>
</html>`, url.QueryEscape(error), url.QueryEscape(description))
}
