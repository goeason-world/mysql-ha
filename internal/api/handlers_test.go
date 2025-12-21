package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// MockAgent implements AgentInterface for testing
type MockAgent struct{}

func (m *MockAgent) GetState() interface{} {
	return map[string]string{"node_id": "test", "role": "leader"}
}

func (m *MockAgent) IsLeader() bool {
	return true
}

// **Feature: mysql-ha-solution, Property 10: API Authentication Enforcement**
// For any API request without valid authentication credentials, the API should return HTTP 401 status code.
func TestAPIAuthenticationEnforcement(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("missing API key returns 401", prop.ForAll(
		func(path string) bool {
			validPaths := []string{"/api/v1/cluster", "/api/v1/nodes", "/api/v1/history"}
			if !contains(validPaths, path) {
				return true
			}

			server := NewServer(Config{
				Listen: "localhost",
				Port:   8080,
				APIKey: "secret-key",
			}, &MockAgent{}, nil)

			req := httptest.NewRequest("GET", path, nil)
			// No API key provided
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			return w.Code == http.StatusUnauthorized
		},
		gen.OneConstOf("/api/v1/cluster", "/api/v1/nodes", "/api/v1/history"),
	))

	properties.Property("wrong API key returns 401", prop.ForAll(
		func(wrongKey string) bool {
			server := NewServer(Config{
				Listen: "localhost",
				Port:   8080,
				APIKey: "correct-key",
			}, &MockAgent{}, nil)

			req := httptest.NewRequest("GET", "/api/v1/cluster", nil)
			req.Header.Set("X-API-Key", wrongKey)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			return w.Code == http.StatusUnauthorized
		},
		gen.Identifier().SuchThat(func(s string) bool { return s != "correct-key" }),
	))

	properties.Property("correct API key returns 200", prop.ForAll(
		func(_ int) bool {
			server := NewServer(Config{
				Listen: "localhost",
				Port:   8080,
				APIKey: "correct-key",
			}, &MockAgent{}, nil)

			req := httptest.NewRequest("GET", "/api/v1/cluster", nil)
			req.Header.Set("X-API-Key", "correct-key")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			return w.Code == http.StatusOK
		},
		gen.Int(),
	))

	properties.TestingRun(t)
}

// **Feature: mysql-ha-solution, Property 11: API Input Validation**
// For any API request with invalid parameters, the API should return HTTP 400 status code.
func TestAPIInputValidation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("empty switchover target returns 400", prop.ForAll(
		func(_ int) bool {
			server := NewServer(Config{
				Listen: "localhost",
				Port:   8080,
				APIKey: "",
			}, &MockAgent{}, nil)

			// Empty target_node_id
			body := `{"target_node_id": ""}`
			req := httptest.NewRequest("POST", "/api/v1/switchover", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.Int(),
	))

	properties.Property("invalid JSON returns 400", prop.ForAll(
		func(invalidJSON string) bool {
			server := NewServer(Config{
				Listen: "localhost",
				Port:   8080,
				APIKey: "",
			}, &MockAgent{}, nil)

			req := httptest.NewRequest("POST", "/api/v1/switchover", strings.NewReader(invalidJSON))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.OneConstOf("{invalid}", "not json", "{", ""),
	))

	properties.Property("missing node ID in path returns 400", prop.ForAll(
		func(_ int) bool {
			server := NewServer(Config{
				Listen: "localhost",
				Port:   8080,
				APIKey: "",
			}, &MockAgent{}, nil)

			req := httptest.NewRequest("GET", "/api/v1/nodes/", nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Either 400 or 404 is acceptable for missing path param
			return w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound
		},
		gen.Int(),
	))

	properties.TestingRun(t)
}

func TestCheckAuth(t *testing.T) {
	tests := []struct {
		apiKey      string
		providedKey string
		expected    bool
	}{
		{"", "", true},    // No auth required
		{"", "any", true}, // No auth required
		{"secret", "secret", true},
		{"secret", "wrong", false},
		{"secret", "", false},
	}

	for _, tt := range tests {
		result := CheckAuth(tt.apiKey, tt.providedKey)
		if result != tt.expected {
			t.Errorf("CheckAuth(%q, %q) = %v, want %v", tt.apiKey, tt.providedKey, result, tt.expected)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
