package mocks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	dotenv "github.com/dotenv/sdk-go"
)

// MockAPIServer simulates the DotEnv API
type MockAPIServer struct {
	*httptest.Server
	mu             sync.Mutex
	organizations  []dotenv.Organization
	projects       map[string][]dotenv.Project
	secrets        map[string]map[string]string
	encryptionKeys map[string]string
	calls          []APICall
}

// APICall records an API call
type APICall struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

// NewMockAPIServer creates a new mock API server
func NewMockAPIServer() *MockAPIServer {
	m := &MockAPIServer{
		organizations:  makeDefaultOrganizations(),
		projects:       makeDefaultProjects(),
		secrets:        makeDefaultSecrets(),
		encryptionKeys: makeDefaultEncryptionKeys(),
		calls:          []APICall{},
	}

	m.Server = httptest.NewServer(http.HandlerFunc(m.handler))
	return m
}

func (m *MockAPIServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	call := APICall{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: make(map[string]string),
	}

	for k, v := range r.Header {
		call.Headers[k] = strings.Join(v, ", ")
	}

	if r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		call.Body = body
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	m.calls = append(m.calls, call)

	// Check authentication
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Route the request
	switch {
	case r.URL.Path == "/api/v1/organizations" && r.Method == "GET":
		m.handleListOrganizations(w, r)

	case strings.HasPrefix(r.URL.Path, "/api/v1/organizations/") && r.Method == "GET":
		m.handleGetOrganization(w, r)

	case strings.Contains(r.URL.Path, "/projects") && r.Method == "GET":
		m.handleListProjects(w, r)

	case strings.Contains(r.URL.Path, "/secrets") && r.Method == "GET":
		m.handleGetSecrets(w, r)

	case r.URL.Path == "/api/v1/secrets/retrieve" && r.Method == "POST":
		m.handleRetrieveSecrets(w, r)

	case strings.Contains(r.URL.Path, "/encryption-key") && r.Method == "GET":
		m.handleGetEncryptionKey(w, r)

	default:
		http.NotFound(w, r)
	}
}

func (m *MockAPIServer) handleListOrganizations(w http.ResponseWriter, r *http.Request) {
	resp := dotenv.JSONAPIResponse{
		Data: []interface{}{},
	}

	for _, org := range m.organizations {
		resp.Data = append(resp.Data, map[string]interface{}{
			"type": "organizations",
			"id":   org.ID,
			"attributes": map[string]interface{}{
				"ulid":       org.ULID,
				"name":       org.Name,
				"slug":       org.Slug,
				"status":     org.Status,
				"plan_name":  org.PlanName,
				"created_at": org.CreatedAt,
				"updated_at": org.UpdatedAt,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockAPIServer) handleGetOrganization(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	slug := parts[len(parts)-1]

	for _, org := range m.organizations {
		if org.Slug == slug {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(org)
			return
		}
	}

	http.NotFound(w, r)
}

func (m *MockAPIServer) handleListProjects(w http.ResponseWriter, r *http.Request) {
	// Extract organization from path
	parts := strings.Split(r.URL.Path, "/")
	var orgSlug string
	for i, p := range parts {
		if p == "organizations" && i+1 < len(parts) {
			orgSlug = parts[i+1]
			break
		}
	}

	projects, ok := m.projects[orgSlug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	resp := dotenv.JSONAPIResponse{
		Data: []interface{}{},
	}

	for _, proj := range projects {
		resp.Data = append(resp.Data, map[string]interface{}{
			"type": "projects",
			"id":   proj.ID,
			"attributes": map[string]interface{}{
				"ulid":              proj.ULID,
				"organization_id":   proj.OrganizationID,
				"name":              proj.Name,
				"slug":              proj.Slug,
				"description":       proj.Description,
				"has_secrets":       proj.HasSecrets,
				"secret_count":      proj.SecretCount,
				"environment_count": proj.EnvironmentCount,
				"target_count":      proj.TargetCount,
				"created_at":        proj.CreatedAt,
				"updated_at":        proj.UpdatedAt,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockAPIServer) handleGetSecrets(w http.ResponseWriter, r *http.Request) {
	// Extract project from path
	parts := strings.Split(r.URL.Path, "/")
	projectSlug := ""

	for i, p := range parts {
		if p == "api" && i+2 < len(parts) {
			projectSlug = parts[i+2]
			break
		}
	}

	secrets, ok := m.secrets[projectSlug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	resp := dotenv.JSONAPIResponse{
		Data: []interface{}{},
	}

	for k, v := range secrets {
		resp.Data = append(resp.Data, map[string]interface{}{
			"type": "secrets",
			"id":   fmt.Sprintf("secret-%s", k),
			"attributes": map[string]interface{}{
				"key":   k,
				"value": v,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockAPIServer) handleRetrieveSecrets(w http.ResponseWriter, r *http.Request) {
	var req dotenv.BulkSecretsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	secrets, ok := m.secrets[req.ProjectSlug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(secrets)
}

func (m *MockAPIServer) handleGetEncryptionKey(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	projectSlug := ""

	for i, p := range parts {
		if p == "api" && i+2 < len(parts) {
			projectSlug = parts[i+2]
			break
		}
	}

	key, ok := m.encryptionKeys[projectSlug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	resp := dotenv.JSONAPIResponse{
		Data: map[string]interface{}{
			"type": "encryption_keys",
			"id":   "key-123",
			"attributes": map[string]interface{}{
				"project_id": projectSlug,
				"key":        key,
				"is_active":  true,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Helper methods to set up test data
func (m *MockAPIServer) SetOrganizations(orgs []dotenv.Organization) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.organizations = orgs
}

func (m *MockAPIServer) SetProjects(orgSlug string, projects []dotenv.Project) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.projects == nil {
		m.projects = make(map[string][]dotenv.Project)
	}
	m.projects[orgSlug] = projects
}

func (m *MockAPIServer) SetSecrets(projectSlug string, secrets map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secrets == nil {
		m.secrets = make(map[string]map[string]string)
	}
	m.secrets[projectSlug] = secrets
}

func (m *MockAPIServer) GetCalls() []APICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]APICall{}, m.calls...)
}

func (m *MockAPIServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = []APICall{}
}

// Default test data
func makeDefaultOrganizations() []dotenv.Organization {
	return []dotenv.Organization{
		{
			ID:       "org-1",
			ULID:     "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Name:     "Test Organization",
			Slug:     "test-org",
			Status:   "active",
			PlanName: "pro",
		},
	}
}

func makeDefaultProjects() map[string][]dotenv.Project {
	return map[string][]dotenv.Project{
		"test-org": {
			{
				ID:               "proj-1",
				ULID:             "01ARZ3NDEKTSV4RRFFQ69G5FAV",
				OrganizationID:   "org-1",
				Name:             "Test Project",
				Slug:             "test-project",
				HasSecrets:       true,
				SecretCount:      3,
				EnvironmentCount: 2,
				TargetCount:      1,
			},
		},
	}
}

func makeDefaultSecrets() map[string]map[string]string {
	return map[string]map[string]string{
		"test-project": {
			"DATABASE_URL": "postgres://localhost/test",
			"API_KEY":      "test-api-key",
			"DEBUG":        "true",
		},
	}
}

func makeDefaultEncryptionKeys() map[string]string {
	return map[string]string{
		"test-project": "YTY0NzgyYjU4ZjU3MjM4YWQ3MjM0ZjgzYjM0ZmEzNGQ=", // Base64 32-byte key
	}
}

// SimulateError configures the server to return an error for specific requests
func (m *MockAPIServer) SimulateError(path string, status int, message string) {
	// This would require modifying the handler to check for error configurations
	// For now, we'll keep it simple with the default behavior
}
