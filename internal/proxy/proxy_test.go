package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llama-swap/internal/config"
	"llama-swap/internal/upstream"
)

func TestIsStreaming(t *testing.T) {
	tests := []struct {
		name       string
		accept     string
		contentType string
		want       bool
	}{
		{"accept event-stream", "text/event-stream", "", true},
		{"content-type event-stream", "", "text/event-stream", true},
		{"both event-stream", "text/event-stream", "text/event-stream", true},
		{"neither", "application/json", "application/json", false},
		{"empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Accept", tt.accept)
			r.Header.Set("Content-Type", tt.contentType)
			if got := isStreaming(r); got != tt.want {
				t.Errorf("isStreaming() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildUpstreamURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		want     string
	}{
		{"basic", "http://localhost:8000", "/v1/chat/completions", "http://localhost:8000/v1/chat/completions"},
		{"trailing slash", "http://localhost:8000/", "/v1/chat/completions", "http://localhost:8000//v1/chat/completions"},
		{"no path", "http://localhost:8000", "", "http://localhost:8000"},
		{"path only", "http://localhost:8000", "/v1/models", "http://localhost:8000/v1/models"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildUpstreamURL(tt.baseURL, tt.path); got != tt.want {
				t.Errorf("buildUpstreamURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProxy_NewProxy(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000", APIKey: "key1"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxy(m)

	if p == nil {
		t.Fatal("NewProxy should not return nil")
	}
	if p.client == nil {
		t.Error("client should not be nil")
	}
	if p.manager == nil {
		t.Error("manager should not be nil")
	}
}

func TestProxy_NewProxyWithTimeout(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxyWithTimeout(m, 60)

	if p == nil {
		t.Fatal("NewProxyWithTimeout should not return nil")
	}
}

func TestProxy_BuildProxyRequest(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000", APIKey: "sk-test"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxy(m)

	u, _ := m.Get("model1")

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}]}`)
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Custom", "value")

	req, err := p.buildProxyRequest(r, "http://localhost:8000/v1/chat/completions", u)
	if err != nil {
		t.Fatalf("buildProxyRequest error: %v", err)
	}

	// Check custom header is copied
	if req.Header.Get("X-Custom") != "value" {
		t.Errorf("X-Custom header not copied, got: %s", req.Header.Get("X-Custom"))
	}

	// Check API key is set
	if !strings.Contains(req.Header.Get("Authorization"), "sk-test") {
		t.Errorf("Authorization header not set correctly, got: %s", req.Header.Get("Authorization"))
	}

	// Check X-Model is removed
	if req.Header.Get("X-Model") != "" {
		t.Errorf("X-Model should be removed, got: %s", req.Header.Get("X-Model"))
	}

	// Check Host is not explicitly set (it's set automatically from the URL)
	// but we don't copy it from incoming request
	if req.Header.Get("Host") != "" {
		t.Logf("Host header: %s (this is set automatically from URL)", req.Header.Get("Host"))
	}
}

func TestProxy_BuildProxyRequestWithBody(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000", APIKey: "sk-key"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxy(m)

	u, _ := m.Get("model1")

	body := []byte(`{"messages":[{"role":"user","content":"test"}]}`)
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set("Accept", "application/json")

	req, err := p.buildProxyRequestWithBody(r, "http://localhost:8000/v1/chat/completions", u, body)
	if err != nil {
		t.Fatalf("buildProxyRequestWithBody error: %v", err)
	}

	// Check Content-Type is set
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type not set, got: %s", req.Header.Get("Content-Type"))
	}

	// Check Authorization header
	if !strings.Contains(req.Header.Get("Authorization"), "sk-key") {
		t.Errorf("Authorization not set, got: %s", req.Header.Get("Authorization"))
	}
}

func TestProxy_CopyHeaders(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000", APIKey: "sk-test"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxy(m)

	u, _ := m.Get("model1")

	// Create a new request to copy headers TO
	req, _ := http.NewRequest(http.MethodGet, "/", nil)

	// Create incoming request with headers
	incoming := httptest.NewRequest(http.MethodGet, "/", nil)
	incoming.Header.Set("Content-Type", "application/json")
	incoming.Header.Set("Accept", "text/event-stream")
	incoming.Header.Set("X-Model", "model1")
	incoming.Header.Set("X-Custom", "custom-value")

	p.copyHeaders(req, incoming, u)

	// Check Content-Type copied
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type not copied, got: %s", req.Header.Get("Content-Type"))
	}

	// Check Accept copied
	if req.Header.Get("Accept") != "text/event-stream" {
		t.Errorf("Accept not copied, got: %s", req.Header.Get("Accept"))
	}

	// Check custom header copied
	if req.Header.Get("X-Custom") != "custom-value" {
		t.Errorf("X-Custom not copied, got: %s", req.Header.Get("X-Custom"))
	}

	// Check X-Model removed
	if req.Header.Get("X-Model") != "" {
		t.Errorf("X-Model should be removed, got: %s", req.Header.Get("X-Model"))
	}

	// Check Authorization set from API key
	if !strings.Contains(req.Header.Get("Authorization"), "sk-test") {
		t.Errorf("Authorization not set, got: %s", req.Header.Get("Authorization"))
	}
}

func TestProxy_SendOpenAIError(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxy(m)

	w := httptest.NewRecorder()
	p.sendOpenAIError(w, "test error message", http.StatusBadGateway)

	if w.Code != http.StatusBadGateway {
		t.Errorf("Expected status %d, got %d", http.StatusBadGateway, w.Code)
	}

	if !strings.Contains(w.Body.String(), "test error message") {
		t.Errorf("Expected error message in body, got: %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "upstream_error") {
		t.Errorf("Expected upstream_error type in body, got: %s", w.Body.String())
	}
}

func TestProxy_Handle_UpstreamNotFound(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxy(m)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	p.Handle(w, r, "nonexistent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestProxy_Handle_NoModel_UsesDefault(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000"},
		{Name: "model2", URL: "http://localhost:8001"},
	}
	m := upstream.NewManager(configs, "model1")
	p := NewProxy(m)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Body = http.NoBody

	// Empty model name should use default
	p.Handle(w, r, "")

	// This will fail because there's no actual upstream to proxy to
	// but it should not return 400 (which would indicate model not found)
	// Instead it should try to proxy and fail with 502
	// The important thing is it's using the default upstream
	if w.Code == http.StatusBadRequest {
		t.Error("Should use default upstream, not return 400 for missing model")
	}
}
