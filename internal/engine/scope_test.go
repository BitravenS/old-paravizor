package engine

import (
	"testing"

	"github.com/bitravens/paravizor/v1/internal/items"
	"github.com/bitravens/paravizor/v1/internal/project"
)

func TestScopeEngine(t *testing.T) {
	cfg := project.ScopeConfig{
		Include: []string{"example.com", "*.target.com", "^192\\.168\\..*$"},
		Exclude: []string{"dev.target.com", "192.168.1.1"},
	}

	engine := NewScopeEngine(cfg)

	tests := []struct {
		name     string
		target   string
		itemType items.ItemType
		want     bool
	}{
		{"exact match include", "example.com", items.TypeDomain, true},
		{"wildcard match include", "api.target.com", items.TypeDomain, true},
		{"regex match include", "192.168.1.50", items.TypeIP, true},
		{"exact match exclude", "dev.target.com", items.TypeDomain, false},
		{"exact match exclude ip", "192.168.1.1", items.TypeIP, false},
		{"default deny (no rules matched)", "other.com", items.TypeDomain, false},
		{"url exact match include", "https://example.com/path", items.TypeURL, true},
		{"url wildcard match include", "http://app.target.com/api", items.TypeURL, true},
		{"url exclude domain", "http://dev.target.com/test", items.TypeURL, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item items.Item
			switch tt.itemType {
			case items.TypeDomain:
				item = &items.DomainItem{Name: tt.target}
			case items.TypeURL:
				item = &items.URLItem{FullURL: tt.target}
			case items.TypeIP:
				item = &items.IPItem{Address: tt.target}
			}

			if got := engine.IsInScope(item); got != tt.want {
				t.Errorf("IsInScope(%v) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestScopeEngineDefaultDenyAll(t *testing.T) {
	cfg := project.ScopeConfig{
		Include: []string{},
		Exclude: []string{},
	}

	engine := NewScopeEngine(cfg)
	item := &items.DomainItem{Name: "example.com"}

	if engine.IsInScope(item) {
		t.Error("Expected completely empty scope rules to deny all (default deny)")
	}
}
