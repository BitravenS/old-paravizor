package tool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/bitravens/paravizor/v1/internal/utils"
)

// Registry manages tool ToolConfigs from embedded, global, and project sources.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolConfig
}

// NewRegistry creates a new, empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*ToolConfig),
	}
}

// LoadEmbedded loads tool ToolConfigs from embedded YAML bytes.
// The map key is a name used for error messages only; each value may contain
// multiple documents separated by "---".
func (r *Registry) LoadEmbedded(files map[string][]byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, data := range files {
		defs, err := parseDefs(data)
		if err != nil {
			return fmt.Errorf("parse embedded tool %s: %w", name, err)
		}
		for _, def := range defs {
			r.tools[def.Name] = def
		}
	}
	return nil
}

// LoadDir loads tool ToolConfigs from a directory of YAML files.
// Files may contain multiple documents separated by "---".
// ToolConfigs override any existing tool with the same name.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // directory not present is acceptable
		}
		return fmt.Errorf("read tool dir %s: %w", dir, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read tool file %s: %w", entry.Name(), err)
		}

		defs, err := parseDefs(data)
		if err != nil {
			return fmt.Errorf("parse tool file %s: %w", entry.Name(), err)
		}
		for _, def := range defs {
			r.tools[def.Name] = def
		}
	}
	return nil
}

// CheckAvailability resolves binary paths for all registered tools.
// toolPaths overrides key=toolName → absolute path from the user config.
func (r *Registry) CheckAvailability(toolPaths map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, def := range r.tools {
		// Honour explicit path override from config.
		if customPath, ok := toolPaths[name]; ok {
			if _, err := os.Stat(customPath); err == nil {
				def.Available = true
				def.BinaryPath = customPath
				continue
			}
		}

		// Fall back to PATH lookup.
		path, err := exec.LookPath(def.Binary)
		if err == nil {
			def.Available = true
			def.BinaryPath = path
		} else {
			def.Available = false
			def.BinaryPath = ""
		}
	}
}

// Get returns a tool ToolConfig by name.
func (r *Registry) Get(name string) (*ToolConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.tools[name]
	return def, ok
}

// All returns a copy of all registered tool ToolConfigs.
func (r *Registry) All() map[string]*ToolConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*ToolConfig, len(r.tools))
	for k, v := range r.tools {
		result[k] = v
	}
	return result
}

// Available returns only tools whose binary was found.
func (r *Registry) Available() map[string]*ToolConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*ToolConfig)
	for k, v := range r.tools {
		if v.Available {
			result[k] = v
		}
	}
	return result
}

// Missing returns tools whose binary was not found.
func (r *Registry) Missing() map[string]*ToolConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*ToolConfig)
	for k, v := range r.tools {
		if !v.Available {
			result[k] = v
		}
	}
	return result
}

// parseDefs decodes one or more ToolWrapper documents from YAML bytes.
// Documents are separated by "---" as per the YAML multi-doc spec.
// Empty documents (no tool.name) are skipped. Each non-empty document is validated.
func parseDefs(data []byte) ([]*ToolConfig, error) {
	wrappers, err := utils.ParseYAMLBytesMultiDoc[ToolWrapper](data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	var defs []*ToolConfig
	for _, w := range wrappers {
		if w.Tool.Name == "" {
			continue // skip blank separator documents
		}
		if err := utils.Validator.Struct(w.Tool); err != nil {
			return nil, fmt.Errorf("invalid tool %q: %w", w.Tool.Name, err)
		}
		def := w.Tool
		if err := ValidateTool(&def); err != nil {
			return nil, fmt.Errorf("tool %q failed validation: %w", def.Name, err)
		}
		defs = append(defs, &def)
	}

	if len(defs) == 0 {
		return nil, fmt.Errorf("no tool definitions found in YAML")
	}
	return defs, nil
}
