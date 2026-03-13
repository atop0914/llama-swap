package upstream

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"llama-swap/internal/config"
)

// Upstream represents a single upstream server
type Upstream struct {
	Name     string
	URL      string
	APIKey   string
	Healthy  bool
	LastCheck time.Time
}

// Manager manages all upstream servers
type Manager struct {
	upstreams     map[string]*Upstream
	defaultUpstream string
	mu            sync.RWMutex
	client        *http.Client
}

// NewManager creates a new upstream manager
func NewManager(configs []config.UpstreamConfig, defaultName string) *Manager {
	m := &Manager{
		upstreams: make(map[string]*Upstream),
		defaultUpstream: defaultName,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	for _, c := range configs {
		m.upstreams[c.Name] = &Upstream{
			Name:     c.Name,
			URL:      c.URL,
			APIKey:   c.APIKey,
			Healthy:  false,
			LastCheck: time.Time{},
		}
	}

	return m
}

func (m *Manager) Get(name string) (*Upstream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if name == "" {
		name = m.defaultUpstream
	}

	upstream, ok := m.upstreams[name]
	if !ok {
		return nil, fmt.Errorf("upstream not found: %s", name)
	}

	return upstream, nil
}

func (m *Manager) GetDefault() (*Upstream, error) {
	return m.Get(m.defaultUpstream)
}

func (m *Manager) List() []*Upstream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Upstream, 0, len(m.upstreams))
	for _, u := range m.upstreams {
		result = append(result, u)
	}
	return result
}

func (m *Manager) GetDefaultName() string {
	return m.defaultUpstream
}

// HealthCheck checks the health of a single upstream by pinging its /health endpoint
func (u *Upstream) HealthCheck(client *http.Client) bool {
	url := u.URL + "/health"
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		u.Healthy = false
		return false
	}

	if u.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+u.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		u.Healthy = false
		return false
	}
	defer resp.Body.Close()

	u.LastCheck = time.Now()
	u.Healthy = resp.StatusCode >= 200 && resp.StatusCode < 300
	return u.Healthy
}

// HealthCheck checks the health of all upstreams
func (m *Manager) HealthCheckAll() map[string]bool {
	m.mu.RLock()
	results := make(map[string]bool)
	upstreamsCopy := make([]*Upstream, 0, len(m.upstreams))
	for _, u := range m.upstreams {
		upstreamsCopy = append(upstreamsCopy, u)
	}
	m.mu.RUnlock()

	for _, u := range upstreamsCopy {
		results[u.Name] = u.HealthCheck(m.client)
	}

	return results
}

// HealthCheckUpstream checks the health of a specific upstream by name
func (m *Manager) HealthCheckUpstream(name string) (bool, error) {
	m.mu.RLock()
	u, ok := m.upstreams[name]
	m.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("upstream not found: %s", name)
	}

	return u.HealthCheck(m.client), nil
}

// IsHealthy returns whether an upstream is healthy
func (m *Manager) IsHealthy(name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.upstreams[name]
	if !ok {
		return false, fmt.Errorf("upstream not found: %s", name)
	}

	return u.Healthy, nil
}
