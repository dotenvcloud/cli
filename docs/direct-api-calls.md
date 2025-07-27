# Direct API Calls in DotEnv CLI

This document tracks direct API calls in the CLI that cannot be replaced with SDK methods.

## Background

As part of maintaining consistency between the CLI and SDK, we've replaced most direct API calls with SDK methods. However, some endpoints must remain as direct calls for specific reasons.

## Direct API Calls That Must Remain

### 1. OAuth Authorization URL Construction
**Location**: `internal/auth/oauth/client.go` - `GetAuthorizationURL()`  
**Endpoint**: `/oauth/authorize` (URL construction only)  
**Reason**: This is not an API call but URL construction for browser-based OAuth flow. The SDK doesn't need this functionality as it's specific to interactive CLI authentication.

### 2. GitHub Release API
**Location**: `cmd/update.go`  
**Endpoint**: `https://api.github.com/repos/dotenv/cli/releases/latest`  
**Reason**: External API for CLI self-update functionality. Not part of DotEnv API.

## Successfully Replaced API Calls

The following direct API calls have been successfully replaced with SDK methods:

### OAuth Token Operations
- **Token Exchange**: `/api/oauth/token` (authorization_code grant) → `client.OAuth.ExchangeToken()`
- **Token Refresh**: `/api/oauth/token` (refresh_token grant) → `client.OAuth.RefreshToken()`

### Telemetry
- **Telemetry Submission**: `/api/v1/cli/telemetry` → `client.Telemetry.SendBatch()`

### User Operations
- **Fetch User**: `/api/v1/user` → `client.User.GetAuthenticatedUser()`

## Best Practices

1. **Always prefer SDK methods** when available
2. **Document any new direct API calls** with clear justification
3. **Consider adding to SDK** if multiple direct calls to same endpoint are needed
4. **Keep this document updated** when adding or removing direct API calls