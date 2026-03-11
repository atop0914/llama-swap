package handler

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"llama-swap/internal/proxy"
	"llama-swap/internal/upstream"
)

type Handler struct {
	manager *upstream.Manager
	proxy   *proxy.Proxy
	stats   *Stats
}

type Stats struct {
	Requests uint64
}

func NewHandler(manager *upstream.Manager, p *proxy.Proxy) *Handler {
	return &Handler{
		manager: manager,
		proxy:   p,
		stats:   &Stats{},
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	jsonResp := map[string]interface{}{
		"requests": atomic.LoadUint64(&h.stats.Requests),
	}
	json.NewEncoder(w).Encode(jsonResp)
}

func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	upstreams := h.manager.List()

	models := make([]map[string]interface{}, 0, len(upstreams))
	for _, u := range upstreams {
		models = append(models, map[string]interface{}{
			"id":      u.Name,
			"object":  "model",
			"created": 1677610602,
			"owned_by": "local",
		})
	}

	resp := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&h.stats.Requests, 1)

	modelName := r.Header.Get("X-Model")
	if modelName == "" {
		modelName = h.manager.GetDefaultName()
	}

	// Check for streaming
	if r.Header.Get("Accept") == "text/event-stream" || 
	   r.Header.Get("Content-Type") == "text/event-stream" {
		h.proxy.ServeHTTP(w, r, modelName)
		return
	}

	h.proxy.ProxyNonStreaming(w, r, modelName)
}

func (h *Handler) Completions(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&h.stats.Requests, 1)

	modelName := r.Header.Get("X-Model")
	if modelName == "" {
		modelName = h.manager.GetDefaultName()
	}

	// Check for streaming
	if r.Header.Get("Accept") == "text/event-stream" {
		h.proxy.ServeHTTP(w, r, modelName)
		return
	}

	h.proxy.ProxyNonStreaming(w, r, modelName)
}
