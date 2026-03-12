package upstream

import (
	"testing"

	"llama-swap/internal/config"
)

func TestNewManager(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000", APIKey: "key1"},
		{Name: "model2", URL: "http://localhost:8001", APIKey: "key2"},
	}

	m := NewManager(configs, "model1")

	if m == nil {
		t.Fatal("Manager should not be nil")
	}

	if m.defaultUpstream != "model1" {
		t.Errorf("Expected default model1, got %s", m.defaultUpstream)
	}
}

func TestManagerGet(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000", APIKey: "key1"},
		{Name: "model2", URL: "http://localhost:8001", APIKey: "key2"},
	}

	m := NewManager(configs, "model1")

	// Test getting existing upstream
	u, err := m.Get("model1")
	if err != nil {
		t.Errorf("Failed to get model1: %v", err)
	}
	if u.Name != "model1" {
		t.Errorf("Expected name model1, got %s", u.Name)
	}
	if u.URL != "http://localhost:8000" {
		t.Errorf("Expected URL http://localhost:8000, got %s", u.URL)
	}

	// Test getting non-existing upstream
	_, err = m.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent upstream")
	}

	// Test getting default when empty
	u, err = m.Get("")
	if err != nil {
		t.Errorf("Failed to get default: %v", err)
	}
	if u.Name != "model1" {
		t.Errorf("Expected default model1, got %s", u.Name)
	}
}

func TestManagerList(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000"},
		{Name: "model2", URL: "http://localhost:8001"},
	}

	m := NewManager(configs, "model1")

	list := m.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 upstreams, got %d", len(list))
	}
}

func TestManagerGetDefaultName(t *testing.T) {
	configs := []config.UpstreamConfig{
		{Name: "model1", URL: "http://localhost:8000"},
	}

	m := NewManager(configs, "model1")

	if m.GetDefaultName() != "model1" {
		t.Errorf("Expected default name model1, got %s", m.GetDefaultName())
	}
}
