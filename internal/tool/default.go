package tool

import (
	"os"
	"path/filepath"
)

var DefaultTools = []DefaultTool{
	{
		Name: "subfinder",
		RawYAML: `tool:
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
    format: regex
    pattern: '^(?P<name>\S+)'
    fields:
      name: name
  flags:
    - -silent
  user_flags: []
  env: {}
  timeout:
    flag: -timeout
    default: 0
  consumes: domain
  produces: domain
`}, {
		Name: "dnsx",
		RawYAML: `tool:
  name: dnsx-live
  binary: dnsx
  description: DNS resolution and live host filtering
  version_cmd: dnsx -version
  install: github.com/projectdiscovery/dnsx/cmd/dnsx@latest
  input:
    type: stdin
    flag: ""
    bulk:
      type: stdin
      flag: ""
      separator: "\n"
  output:
    type: stdout
    format: regex
    pattern: '^(?P<name>\S+)'
    fields:
      name: name
  flags:
    - -silent
    - -resp
    - -no-color
  user_flags: []
  env: {}
  rate_limit:
    flag: -rl
    unit: second
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: domain

---
tool:
  name: dnsx-resolve
  binary: dnsx
  description: DNS A record resolution for IP extraction
  version_cmd: dnsx -version
  install: github.com/projectdiscovery/dnsx/cmd/dnsx@latest
  input:
    type: stdin
    flag: ""
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
    - -a
    - -resp-only
  user_flags: []
  env: {}
  rate_limit:
    flag: -rl
    unit: second
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: ip
`,
	},
}

// WriteDefaultTools writes missing default tool files into toolsDir.
// Each tool is stored in its own file named <toolname>.yaml.
func WriteDefaultTools(toolsDir string) error {
	for _, tool := range DefaultTools {
		name := tool.Name
		rawYAML := tool.RawYAML
		toolPath := filepath.Join(toolsDir, name+".yaml")
		if _, err := os.Stat(toolPath); err == nil {
			continue
		}
		if err := os.WriteFile(toolPath, []byte(rawYAML), 0o644); err != nil {
			return err
		}
	}
	return nil
}
