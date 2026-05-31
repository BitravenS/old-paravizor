package tool

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bitravens/paravizor/v1/internal/assets"
)

// WriteDefaultTools writes missing default tool files into toolsDir.
// It extracts the bundled YAML configurations.
func WriteDefaultTools(toolsDir string) error {
	entries, err := assets.ReadDir("tools")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		toolPath := filepath.Join(toolsDir, entry.Name())
		if _, err := os.Stat(toolPath); err == nil {
			continue // File exists
		}

		data, err := assets.ReadFile("tools/" + entry.Name())
		if err != nil {
			return err
		}

		if err := os.WriteFile(toolPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
