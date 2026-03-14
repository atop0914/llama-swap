package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"llama-swap/internal/upstream"
)

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
	logRequest(r.Method, u.Name, r.URL.Path, upstreamURL)

	req, err := p.buildProxyRequest(r, upstreamURL, u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := p.doRequest(req)
	if err != nil {
		log.Printf("Proxy error: %v", err)
		http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	p.copyResponse(w, resp, true)
}

// proxyNonStream handles non-streaming requests
func (p *Proxy) proxyNonStream(w http.ResponseWriter, r *http.Request, u *upstream.Upstream) {
	upstreamURL := buildUpstreamURL(u.URL, r.URL.Path)
	logRequest(r.Method, u.Name, r.URL.Path, upstreamURL)

	// Read body for reuse
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	req, err := p.buildProxyRequestWithBody(r, upstreamURL, u, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := p.doRequest(req)
	if err != nil {
		log.Printf("Proxy error: %v", err)
		http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

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
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status
	w.WriteHeader(resp.StatusCode)

	// Copy body
	if streaming {
		p.copyBodyStreaming(w, resp.Body)
	} else {
		io.Copy(w, resp.Body)
	}
}

// copyBodyStreaming copies body with streaming support
func (p *Proxy) copyBodyStreaming(w http.ResponseWriter, body io.Reader) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	buffer := make([]byte, 4096)
	for {
		n, err := body.Read(buffer)
		if n > 0 {
			w.Write(buffer[:n])
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

// logRequest logs the proxy request
func logRequest(method, model, path, upstream string) {
	log.Printf("[%s] %s %s -> %s", method, model, path, upstream)
}

// ProxyNonStreaming handles non-streaming requests (legacy compatibility)
func (p *Proxy) ProxyNonStreaming(w http.ResponseWriter, r *http.Request, modelName string) error {
	u, err := p.manager.Get(modelName)
	if err != nil {
		return err
	}

	upstreamURL := buildUpstreamURL(u.URL, r.URL.Path)
	logRequest(r.Method, u.Name, r.URL.Path, upstreamURL)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	req, err := p.buildProxyRequestWithBody(r, upstreamURL, u, body)
	if err != nil {
		return err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
