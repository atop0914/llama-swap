package upstream

import (
	"fmt"
	"sync"

	"llama-swap/internal/config"
)

type Manager struct {
	upstreams map[string]*Upstream
	defaultUpstream string
	mu         sync.RWMutex
}

type Upstream struct {
	Name   string
	URL    string
	APIKey string
}

func NewManager(configs []config.UpstreamConfig, defaultName string) *Manager {
	m := &Manager{
		upstreams: make(map[string]*Upstream),
		defaultUpstream: defaultName,
	}

	for _, c := range configs {
		m.upstreams[c.Name] = &Upstream{
			Name:   c.Name,
			URL:    c.URL,
			APIKey: c.APIKey,
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
