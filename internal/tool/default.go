package tool

import (
	"os"
	"path/filepath"
)

// WriteDefaultTools writes missing default tool files into toolsDir.
// Each tool is stored in its own file named <toolname>.yaml.
func WriteDefaultTools(toolsDir string) error {
	subfinderPath := filepath.Join(toolsDir, "subfinder.yaml")
	if _, err := os.Stat(subfinderPath); err == nil {
		return nil
	}

	rawYAML := `---
tool:
  name: subfinder
  binary: subfinder
  description: Passive subdomain enumeration
  version_cmd: subfinder -version
  install: github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
  input:
    type: arg
    flag: -d
    bulk:
      type: stdin
      flag: ""
      separator: "\n"
  output:
    type: stdout
    format: line
    fields: {}
  flags:
    - -silent
  user_flags: []
  env: {}
  timeout:
    flag: -timeout
    default: 0
  consumes: domain
  produces: domain
`
	return os.WriteFile(subfinderPath, []byte(rawYAML), 0o644)
}
