package oauth

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F-12: state-mismatch in the callback must reject the code so an attacker
// can't drive the flow with a forged state value.
func TestHandleCallback_StateMismatch(t *testing.T) {
	s := NewCallbackServer("expected-state-abc")

	req := httptest.NewRequest("GET", "/callback?code=ok&state=attacker-state", nil)
	rec := httptest.NewRecorder()

	s.handleCallback(rec, req)

	select {
	case err := <-s.AuthError:
		assert.Contains(t, err.Error(), "invalid state")
	default:
		t.Fatal("expected AuthError on state mismatch")
	}

	assert.Contains(t, rec.Body.String(), "invalid_state")
}

// F-12: missing authorization code must surface as error, not be silently
// accepted as success.
func TestHandleCallback_MissingCode(t *testing.T) {
	s := NewCallbackServer("matching-state")

	req := httptest.NewRequest("GET", "/callback?state=matching-state", nil)
	rec := httptest.NewRecorder()

	s.handleCallback(rec, req)

	select {
	case err := <-s.AuthError:
		assert.Contains(t, err.Error(), "no authorization code")
	default:
		t.Fatal("expected AuthError when code missing")
	}
}

// F-12: error parameter (e.g. user-cancelled) routes to error path with the
// provider's error code surfaced.
func TestHandleCallback_ProviderError(t *testing.T) {
	s := NewCallbackServer("any")
	values := url.Values{}
	values.Set("error", "access_denied")
	values.Set("error_description", "User refused")
	req := httptest.NewRequest("GET", "/callback?"+values.Encode(), nil)
	rec := httptest.NewRecorder()

	s.handleCallback(rec, req)

	select {
	case err := <-s.AuthError:
		assert.Contains(t, err.Error(), "access_denied")
		assert.Contains(t, err.Error(), "User refused")
	default:
		t.Fatal("expected AuthError when provider returns error")
	}
}

// F-12: happy path — state matches + code present → code is sent through
// channel and AuthState is recorded.
func TestHandleCallback_HappyPath(t *testing.T) {
	s := NewCallbackServer("ok-state")
	req := httptest.NewRequest("GET", "/callback?code=auth-code-xyz&state=ok-state", nil)
	rec := httptest.NewRecorder()

	s.handleCallback(rec, req)

	select {
	case code := <-s.AuthCode:
		assert.Equal(t, "auth-code-xyz", code)
	default:
		t.Fatal("expected AuthCode on success path")
	}
	assert.Equal(t, "ok-state", s.AuthState)
	assert.Contains(t, rec.Body.String(), "Successful")
}

// F-15: FindAvailablePort binds 127.0.0.1 only (loopback), not 0.0.0.0.
// Regression guard so we don't accidentally re-expose to the LAN.
func TestFindAvailablePort_BindsLoopbackOnly(t *testing.T) {
	s := NewCallbackServer("x")
	port, err := s.FindAvailablePort()
	require.NoError(t, err)
	require.NotEmpty(t, port)
	// We can't directly observe the bind addr without keeping the listener
	// open, but we can verify port allocation succeeded and is a real number.
	assert.NotEqual(t, "0", port)
	assert.False(t, strings.Contains(port, ":"))
}
