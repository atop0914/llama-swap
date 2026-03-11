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

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, modelName string) {
	// Get upstream
	upstream, err := p.manager.Get(modelName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Build upstream URL
	upstreamURL := fmt.Sprintf("%s%s", upstream.URL, r.URL.Path)

	// Log request
	log.Printf("[%s] %s %s -> %s", r.Method, modelName, r.URL.Path, upstreamURL)

	// Create proxy request
	req, err := http.NewRequest(r.Method, upstreamURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			if key == "Host" {
				continue
			}
			req.Header.Add(key, value)
		}
	}

	// Add API key if set
	if upstream.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	}

	// Remove X-Model header for upstream
	req.Header.Del("X-Model")

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("Proxy error: %v", err)
		http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status
	w.WriteHeader(resp.StatusCode)

	// Copy response body (streaming)
	if flusher, ok := w.(http.Flusher); ok {
		buffer := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				w.Write(buffer[:n])
				flusher.Flush()
			}
			if err != nil {
				break
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}
}

// ProxyNonStreaming handles non-streaming requests
func (p *Proxy) ProxyNonStreaming(w http.ResponseWriter, r *http.Request, modelName string) error {
	upstream, err := p.manager.Get(modelName)
	if err != nil {
		return err
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	upstreamURL := fmt.Sprintf("%s%s", upstream.URL, r.URL.Path)

	log.Printf("[%s] %s %s -> %s", r.Method, modelName, r.URL.Path, upstreamURL)

	req, err := http.NewRequest(r.Method, upstreamURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			if key == "Host" {
				continue
			}
			req.Header.Add(key, value)
		}
	}

	if upstream.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	}
	req.Header.Del("X-Model")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	return nil
}
