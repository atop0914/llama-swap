package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llama-swap/internal/logger"
	"llama-swap/internal/upstream"
)

// OpenAIError represents an OpenAI-compatible error response
type OpenAIError struct {
	Error OpenAIErrorDetail `json:"error"`
}

type OpenAIErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// ErrorResponse represents an error from upstream
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

type Proxy struct {
	client  *http.Client
	manager *upstream.Manager
}

func NewProxy(manager *upstream.Manager) *Proxy {
	return &Proxy{
		client: &http.Client{
			Timeout: 300 * time.Second,
		},
		manager: manager,
	}
}

// Handle handles proxy requests with automatic streaming detection
func (p *Proxy) Handle(w http.ResponseWriter, r *http.Request, modelName string) {
	// Get upstream
	u, err := p.manager.Get(modelName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if streaming
	if isStreaming(r) {
		p.proxyStream(w, r, u)
	} else {
		p.proxyNonStream(w, r, u)
	}
}

// proxyStream handles streaming requests
func (p *Proxy) proxyStream(w http.ResponseWriter, r *http.Request, u *upstream.Upstream) {
	upstreamURL := buildUpstreamURL(u.URL, r.URL.Path)
	start := time.Now()

	req, err := p.buildProxyRequest(r, upstreamURL, u)
	if err != nil {
		logger.Error("Failed to build proxy request",
			logger.String("error", err.Error()),
			logger.String("model", u.Name),
			logger.String("path", r.URL.Path),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := p.doRequest(req)
	if err != nil {
		duration := time.Since(start)
		logger.ProxyRequest(r.Method, u.Name, r.URL.Path, upstreamURL, duration, 0, err)
		http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	logger.ProxyRequest(r.Method, u.Name, r.URL.Path, upstreamURL, duration, resp.StatusCode, nil)
	p.copyResponse(w, resp, true)
}

// proxyNonStream handles non-streaming requests
func (p *Proxy) proxyNonStream(w http.ResponseWriter, r *http.Request, u *upstream.Upstream) {
	upstreamURL := buildUpstreamURL(u.URL, r.URL.Path)
	start := time.Now()

	// Read body for reuse
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read request body",
			logger.String("error", err.Error()),
			logger.String("model", u.Name),
		)
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	req, err := p.buildProxyRequestWithBody(r, upstreamURL, u, body)
	if err != nil {
		logger.Error("Failed to build proxy request",
			logger.String("error", err.Error()),
			logger.String("model", u.Name),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := p.doRequest(req)
	if err != nil {
		duration := time.Since(start)
		logger.ProxyRequest(r.Method, u.Name, r.URL.Path, upstreamURL, duration, 0, err)
		http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	logger.ProxyRequest(r.Method, u.Name, r.URL.Path, upstreamURL, duration, resp.StatusCode, nil)
	p.copyResponse(w, resp, false)
}

// buildProxyRequest builds a proxy request from incoming request (for streaming)
func (p *Proxy) buildProxyRequest(r *http.Request, upstreamURL string, u *upstream.Upstream) (*http.Request, error) {
	req, err := http.NewRequest(r.Method, upstreamURL, r.Body)
	if err != nil {
		return nil, err
	}

	p.copyHeaders(req, r, u)
	return req, nil
}

// buildProxyRequestWithBody builds a proxy request with given body (for non-streaming)
func (p *Proxy) buildProxyRequestWithBody(r *http.Request, upstreamURL string, u *upstream.Upstream, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(r.Method, upstreamURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	p.copyHeaders(req, r, u)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// copyHeaders copies headers from incoming request to proxy request
func (p *Proxy) copyHeaders(req *http.Request, r *http.Request, u *upstream.Upstream) {
	for key, values := range r.Header {
		if key == "Host" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Add API key if set
	if u.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+u.APIKey)
	}

	// Remove X-Model header for upstream
	req.Header.Del("X-Model")
}

// doRequest sends the proxy request
func (p *Proxy) doRequest(req *http.Request) (*http.Response, error) {
	return p.client.Do(req)
}

// copyResponse copies response headers and body to the client
func (p *Proxy) copyResponse(w http.ResponseWriter, resp *http.Response, streaming bool) {
	// Copy response headers (except Transfer-Encoding for streaming)
	for key, values := range resp.Header {
		if streaming && strings.ToLower(key) == "transfer-encoding" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Handle error responses
	if resp.StatusCode >= 400 {
		p.handleErrorResponse(w, resp, streaming)
		return
	}

	// Set status
	w.WriteHeader(resp.StatusCode)

	// Copy body based on content type
	contentType := resp.Header.Get("Content-Type")
	if streaming || strings.Contains(contentType, "text/event-stream") {
		p.copyBodyStreaming(w, resp.Body)
	} else if strings.Contains(contentType, "application/json") {
		p.copyJSONResponse(w, resp.Body)
	} else {
		io.Copy(w, resp.Body)
	}
}

// handleErrorResponse transforms upstream errors to OpenAI-compatible format
func (p *Proxy) handleErrorResponse(w http.ResponseWriter, resp *http.Response, streaming bool) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.sendOpenAIError(w, fmt.Sprintf("upstream error: %v", err), resp.StatusCode)
		return
	}

	// Try to parse upstream error response
	var upstreamErr ErrorResponse
	if json.Unmarshal(body, &upstreamErr) == nil && upstreamErr.Error.Message != "" {
		// Transform to OpenAI format
		p.sendOpenAIError(w, upstreamErr.Error.Message, resp.StatusCode)
		return
	}

	// Try to parse other JSON error formats
	var genericErr map[string]interface{}
	if json.Unmarshal(body, &genericErr) == nil {
		if msg, ok := genericErr["message"].(string); ok {
			p.sendOpenAIError(w, msg, resp.StatusCode)
			return
		}
		if msg, ok := genericErr["error"].(string); ok {
			p.sendOpenAIError(w, msg, resp.StatusCode)
			return
		}
	}

	// Fallback: use body as message
	message := string(body)
	if message == "" {
		message = fmt.Sprintf("upstream error: %d", resp.StatusCode)
	}
	p.sendOpenAIError(w, message, resp.StatusCode)
}

// sendOpenAIError sends an OpenAI-compatible error response
func (p *Proxy) sendOpenAIError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errResp := OpenAIError{
		Error: OpenAIErrorDetail{
			Message: message,
			Type:    "upstream_error",
			Code:    fmt.Sprintf("%d", statusCode),
		},
	}

	json.NewEncoder(w).Encode(errResp)
}

// copyJSONResponse handles JSON response transformation
func (p *Proxy) copyJSONResponse(w http.ResponseWriter, body io.Reader) {
	data, err := io.ReadAll(body)
	if err != nil {
		p.sendOpenAIError(w, fmt.Sprintf("failed to read response: %v", err), http.StatusInternalServerError)
		return
	}

	// Try to parse and re-encode to ensure valid JSON
	var jsonBody interface{}
	if err := json.Unmarshal(data, &jsonBody); err == nil {
		// Valid JSON, write it
		w.Write(data)
	} else {
		// Not JSON, write as-is
		w.Write(data)
	}
}

// copyBodyStreaming copies body with streaming support (SSE)
func (p *Proxy) copyBodyStreaming(w http.ResponseWriter, body io.Reader) {
	// Check if ResponseWriter supports flushing (for streaming)
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	// Set SSE headers if not already set
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Use a larger buffer for better streaming performance
	buffer := make([]byte, 8192)
	for {
		n, err := body.Read(buffer)
		if n > 0 {
			// Write data directly, preserving SSE format
			_, writeErr := w.Write(buffer[:n])
			if writeErr != nil {
				logger.Debug("Streaming write error",
					logger.String("error", writeErr.Error()),
				)
				break
			}
			flusher.Flush()
		}
		if err != nil {
			if err != io.EOF {
				logger.Debug("Streaming read error",
					logger.String("error", err.Error()),
				)
			}
			break
		}
	}
}

// copySSEStream properly handles Server-Sent Events stream
func (p *Proxy) copySSEStream(w http.ResponseWriter, body io.Reader) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	buffer := make([]byte, 4096)
	for {
		n, err := body.Read(buffer)
		if n > 0 {
			_, writeErr := w.Write(buffer[:n])
			if writeErr != nil {
				break
			}
			flusher.Flush()
		}
		if err != nil {
			break
		}
	}
}

// isStreaming checks if the request wants streaming response
func isStreaming(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	contentType := r.Header.Get("Content-Type")
	return accept == "text/event-stream" || contentType == "text/event-stream"
}

// buildUpstreamURL builds the full upstream URL
func buildUpstreamURL(baseURL, path string) string {
	return fmt.Sprintf("%s%s", baseURL, path)
}

// ProxyNonStreaming handles non-streaming requests (legacy compatibility)
func (p *Proxy) ProxyNonStreaming(w http.ResponseWriter, r *http.Request, modelName string) error {
	u, err := p.manager.Get(modelName)
	if err != nil {
		return err
	}

	upstreamURL := buildUpstreamURL(u.URL, r.URL.Path)
	start := time.Now()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read request body",
			logger.String("error", err.Error()),
		)
		return err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	req, err := p.buildProxyRequestWithBody(r, upstreamURL, u, body)
	if err != nil {
		logger.Error("Failed to build proxy request",
			logger.String("error", err.Error()),
		)
		return err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		duration := time.Since(start)
		logger.ProxyRequest(r.Method, u.Name, r.URL.Path, upstreamURL, duration, 0, err)
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	logger.ProxyRequest(r.Method, u.Name, r.URL.Path, upstreamURL, duration, resp.StatusCode, nil)
	p.copyResponse(w, resp, false)
	return nil
}

// NewProxyWithTimeout creates a proxy with custom timeout
func NewProxyWithTimeout(manager *upstream.Manager, timeout time.Duration) *Proxy {
	return &Proxy{
		client: &http.Client{
			Timeout: timeout,
		},
		manager: manager,
	}
}
