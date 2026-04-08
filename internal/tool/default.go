package tool

import (
	"os"
	"path/filepath"
)

var DefaultTools = []DefaultTool{
	// ── Subfinder ──────────────────────────────────────────────────────────────
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
`},

	// ── dnsx ──────────────────────────────────────────────────────────────────
	{
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

	// ── Chaos API ─────────────────────────────────────────────────────────────
	{
		Name: "chaos",
		RawYAML: `tool:
  name: chaos
  binary: chaos
  description: Passive subdomain enumeration via ProjectDiscovery Chaos API
  version_cmd: chaos -version
  install: github.com/projectdiscovery/chaos-client/cmd/chaos@latest
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
    flag: ""
    default: 0
  consumes: domain
  produces: domain
`,
	},

	// ── Amass ─────────────────────────────────────────────────────────────────
	{
		Name: "amass",
		RawYAML: `tool:
  name: amass-passive
  binary: amass
  description: Passive subdomain enumeration using OWASP Amass
  version_cmd: amass -version
  install: github.com/owasp-amass/amass/v4/cmd/amass@master
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
    pattern: '^(?P<name>[a-zA-Z0-9._-]+\.[a-zA-Z]{2,})\s*$'
    fields:
      name: name
  flags:
    - enum
    - -passive
    - -norecursive
    - -silent
  user_flags: []
  env: {}
  timeout:
    flag: -timeout
    default: 0
  consumes: domain
  produces: domain
`,
	},

	// ── puredns ───────────────────────────────────────────────────────────────
	{
		Name: "puredns",
		RawYAML: `tool:
  name: puredns-bruteforce
  binary: puredns
  description: Active DNS bruteforce subdomain enumeration using puredns
  install: github.com/d3mondev/puredns/v2@latest
  input:
    type: arg
    flag: -d
  output:
    type: stdout
    format: regex
    pattern: '^(?P<name>\S+)'
    fields:
      name: name
  flags:
    - bruteforce
    - /usr/share/seclists/Discovery/DNS/dns-Jhaddix.txt
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: domain

---
tool:
  name: puredns-resolve
  binary: puredns
  description: DNS resolution and validation using puredns
  install: github.com/d3mondev/puredns/v2@latest
  input:
    type: stdin
    flag: ""
    bulk:
      type: file
      flag: -l
      separator: "\n"
  output:
    type: stdout
    format: regex
    pattern: '^(?P<name>\S+)'
    fields:
      name: name
  flags:
    - resolve
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: domain
`,
	},

	// ── alterx ────────────────────────────────────────────────────────────────
	{
		Name: "alterx",
		RawYAML: `tool:
  name: alterx
  binary: alterx
  description: Subdomain permutation and wordlist generation
  version_cmd: alterx -version
  install: github.com/projectdiscovery/alterx/cmd/alterx@latest
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
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: domain
`,
	},

	// ── haylxon ───────────────────────────────────────────────────────────────
	{
		Name: "haylxon",
		RawYAML: `tool:
  name: haylxon
  binary: hlx
  description: Web screenshot capture tool (Rust-based Eyewitness alternative)
  input:
    type: stdin
    flag: ""
    bulk:
      type: stdin
      flag: ""
      separator: "\n"
  output:
    type: directory
    format: line
    flag: -out
    path: screenshots
  flags: []
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: file
`,
	},

	// ── rustscan ──────────────────────────────────────────────────────────────
	{
		Name: "rustscan",
		RawYAML: `tool:
  name: rustscan
  binary: rustscan
  description: Fast multi-threaded port scanner
  input:
    type: arg
    flag: -a
    bulk:
      type: file
      flag: --addresses
      separator: ","
  output:
    type: stdout
    format: regex
    pattern: 'Open (?P<address>[^:]+):(?P<port>\d+)'
    fields:
      address: address
      port: port
  flags:
    - --ulimit
    - "5000"
    - --
    - -sV
    - --open
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: ip
  produces: port
`,
	},

	// ── nmap ──────────────────────────────────────────────────────────────────
	{
		Name: "nmap",
		RawYAML: `tool:
  name: nmap
  binary: nmap
  description: Network port and service version scanner
  input:
    type: arg
    flag: ""
  output:
    type: stdout
    format: regex
    pattern: '(?P<port>\d+)/(?P<protocol>\w+)\s+open\s+(?P<title>\S+)'
    fields:
      port: port
      protocol: protocol
      title: title
  flags:
    - -sV
    - -open
    - --max-retries
    - "2"
    - -T4
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: port
  produces: finding
`,
	},

	// ── httpx ─────────────────────────────────────────────────────────────────
	{
		Name: "httpx",
		RawYAML: `tool:
  name: httpx-tech
  binary: httpx
  description: HTTP tech-stack fingerprinting
  version_cmd: httpx -version
  install: github.com/projectdiscovery/httpx/cmd/httpx@latest
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
    - -tech-detect
    - -no-color
  user_flags: []
  env: {}
  rate_limit:
    flag: -rl
    unit: second
  timeout:
    flag: -timeout
    default: 10
  consumes: domain
  produces: domain

---
tool:
  name: httpx-probe
  binary: httpx
  description: HTTP URL liveliness probing
  version_cmd: httpx -version
  install: github.com/projectdiscovery/httpx/cmd/httpx@latest
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
    pattern: '^(?P<full_url>https?://\S+)'
    fields:
      full_url: full_url
  flags:
    - -silent
    - -no-color
    - -mc
    - 200,201,204,206,301,302,401,403
  user_flags: []
  env: {}
  rate_limit:
    flag: -rl
    unit: second
  timeout:
    flag: -timeout
    default: 10
  consumes: url
  produces: url
`,
	},

	// ── nuclei ────────────────────────────────────────────────────────────────
	{
		Name: "nuclei",
		RawYAML: `tool:
  name: nuclei-tech
  binary: nuclei
  description: Tech-stack fingerprinting via Nuclei technology templates
  version_cmd: nuclei -version
  install: github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest
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
    pattern: '^\[(?P<name>[^\]]+)\]'
    fields:
      name: name
  flags:
    - -silent
    - -no-color
    - -t
    - http/technologies/
    - -tags
    - tech
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
  name: nuclei-host
  binary: nuclei
  description: Host-level vulnerability scanning via Nuclei
  version_cmd: nuclei -version
  install: github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest
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
    pattern: '^\[(?P<title>[^\]]+)\]\s+\[(?P<severity>\w+)\]'
    fields:
      title: title
      severity: severity
  flags:
    - -silent
    - -no-color
    - -tags
    - cve,exposure,panel,misconfig,tech
    - -severity
    - critical,high,medium
  user_flags: []
  env: {}
  rate_limit:
    flag: -rl
    unit: second
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: finding

---
tool:
  name: nuclei-dast
  binary: nuclei
  description: DAST and fuzzing via Nuclei on parameterised URLs
  version_cmd: nuclei -version
  install: github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest
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
    pattern: '^\[(?P<title>[^\]]+)\]\s+\[(?P<severity>\w+)\]'
    fields:
      title: title
      severity: severity
  flags:
    - -silent
    - -no-color
    - -tags
    - dast,fuzzing
    - -rl
    - "50"
  user_flags: []
  env: {}
  rate_limit:
    flag: -rl
    unit: second
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: finding
`,
	},

	// ── whatcms ───────────────────────────────────────────────────────────────
	{
		Name: "whatcms",
		RawYAML: `tool:
  name: whatcms
  binary: whatcms
  description: CMS detection using WhatCMS API
  input:
    type: arg
    flag: ""
  output:
    type: stdout
    format: regex
    pattern: '^(?P<name>\S+)'
    fields:
      name: name
  flags: []
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: domain
`,
	},

	// ── themedetect ───────────────────────────────────────────────────────────
	{
		Name: "themedetect",
		RawYAML: `tool:
  name: themedetect
  binary: themedetect
  description: WordPress theme and plugin detection
  input:
    type: arg
    flag: ""
  output:
    type: stdout
    format: regex
    pattern: '(?P<title>Theme:\s+\S+|Plugin:\s+\S+)'
    fields:
      title: title
  flags: []
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: finding
`,
	},

	// ── katana ────────────────────────────────────────────────────────────────
	{
		Name: "katana",
		RawYAML: `tool:
  name: katana
  binary: katana
  description: Active web crawler for URL discovery
  version_cmd: katana -version
  install: github.com/projectdiscovery/katana/cmd/katana@latest
  input:
    type: arg
    flag: -u
    bulk:
      type: stdin
      flag: ""
      separator: "\n"
  output:
    type: stdout
    format: regex
    pattern: '^(?P<full_url>https?://\S+)'
    fields:
      full_url: full_url
  flags:
    - -silent
    - -no-color
    - -d
    - "3"
  user_flags: []
  env: {}
  rate_limit:
    flag: -rl
    unit: second
  timeout:
    flag: -timeout
    default: 0
  scope_flags:
    include: -fs
    exclude: -frs
  consumes: domain
  produces: url
`,
	},

	// ── gau ───────────────────────────────────────────────────────────────────
	{
		Name: "gau",
		RawYAML: `tool:
  name: gau
  binary: gau
  description: Passive URL enumeration from multiple sources
  install: github.com/lc/gau/v2/cmd/gau@latest
  input:
    type: arg
    flag: ""
    bulk:
      type: stdin
      flag: ""
      separator: "\n"
  output:
    type: stdout
    format: regex
    pattern: '^(?P<full_url>https?://\S+)'
    fields:
      full_url: full_url
  flags:
    - --subs
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: url
`,
	},

	// ── waybackurls ───────────────────────────────────────────────────────────
	{
		Name: "waybackurls",
		RawYAML: `tool:
  name: waybackurls
  binary: waybackurls
  description: Fetch URLs from Wayback Machine for a domain
  install: github.com/tomnomnom/waybackurls@latest
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
    pattern: '^(?P<full_url>https?://\S+)'
    fields:
      full_url: full_url
  flags: []
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: url
`,
	},

	// ── uro ───────────────────────────────────────────────────────────────────
	{
		Name: "uro",
		RawYAML: `tool:
  name: uro
  binary: uro
  description: URL deduplication and normalization
  install: pip3 install uro
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
    pattern: '^(?P<full_url>https?://\S+)'
    fields:
      full_url: full_url
  flags: []
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url
`,
	},

	// ── grep tools (attack surface triage) ────────────────────────────────────
	{
		Name: "grep",
		RawYAML: `tool:
  name: grep-interesting-params
  binary: grep
  description: "Triage: interesting query parameter names (id, user, file, path, url, redirect, next, src, token, key, api_key, session)"
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
    - "-iE"
    - "(id|user|file|path|url|redirect|next|src|token|key|api_key|session)="
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: grep-interesting-endpoints
  binary: grep
  description: "Triage: interesting endpoint path segments (config, assets)"
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
    - "-iE"
    - "/(config|assets)"
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: grep-api-endpoints
  binary: grep
  description: "Triage: API endpoint path segments (api, v1-v9, graphql, rest, gql)"
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
    - "-iE"
    - "/(api|v[0-9]+|graphql|rest|gql)([/?#]|$)"
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: grep-file-upload
  binary: grep
  description: "Triage: file upload endpoint path segments"
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
    - "-iE"
    - "/(upload|file|attachment|document|image|avatar|photo|media)([/?#]|$)"
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: grep-admin-paths
  binary: grep
  description: "Triage: admin and internal path segments"
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
    - "-iE"
    - "/(admin|internal|debug|test|staging|dev|management|console)([/?#]|$)"
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: grep-auth-endpoints
  binary: grep
  description: "Triage: authentication and SSO endpoint path segments"
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
    - "-iE"
    - "/(oauth|login|auth|sso|oidc|saml|callback|token)([/?#]|$)"
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url
`,
	},

	// ── gf (grep framework) ───────────────────────────────────────────────────
	{
		Name: "gf",
		RawYAML: `tool:
  name: gf-xss
  binary: gf
  description: "GF pattern scan: Cross-Site Scripting (XSS) indicators"
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
    - xss
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: gf-ssrf
  binary: gf
  description: "GF pattern scan: Server-Side Request Forgery (SSRF) indicators"
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
    - ssrf
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: gf-rce
  binary: gf
  description: "GF pattern scan: Remote Code Execution (RCE) indicators"
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
    - rce
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: gf-idor
  binary: gf
  description: "GF pattern scan: Insecure Direct Object Reference (IDOR) indicators"
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
    - idor
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: gf-redirect
  binary: gf
  description: "GF pattern scan: Open Redirect indicators"
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
    - redirect
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: gf-lfi
  binary: gf
  description: "GF pattern scan: Local File Inclusion (LFI) indicators"
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
    - lfi
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url

---
tool:
  name: gf-sqli
  binary: gf
  description: "GF pattern scan: SQL Injection (SQLi) indicators"
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
    - sqli
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: url
`,
	},

	// ── wget (file download) ───────────────────────────────────────────────────
	{
		Name: "wget",
		RawYAML: `tool:
  name: wget-download
  binary: wget
  description: Download static files from URLs to the project downloads directory
  input:
    type: arg
    flag: ""
    bulk:
      type: file
      flag: -i
      separator: "\n"
  output:
    type: stdout
    format: line
    fields: {}
  flags:
    - "-q"
    - "--no-check-certificate"
    - "-P"
    - downloads
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: url
  produces: file
`,
	},

	// ── kanha (subdomain takeover) ────────────────────────────────────────────
	{
		Name: "kanha",
		RawYAML: `tool:
  name: kanha
  binary: kanha
  description: Subdomain takeover checker written in Rust
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
    pattern: '(?P<title>VULNERABLE[^\n]+)'
    fields:
      title: title
  flags: []
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: finding
`,
	},

	// ── wpscan ────────────────────────────────────────────────────────────────
	{
		Name: "wpscan",
		RawYAML: `tool:
  name: wpscan
  binary: wpscan
  description: WordPress vulnerability scanner
  input:
    type: arg
    flag: --url
  output:
    type: stdout
    format: regex
    pattern: '\[!\]\s+(?P<title>[^\n]+)'
    fields:
      title: title
  flags:
    - --no-update
    - --disable-tls-checks
    - --format
    - cli-no-color
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: domain
  produces: finding
`,
	},

	// ── linkfinder ────────────────────────────────────────────────────────────
	{
		Name: "linkfinder",
		RawYAML: `tool:
  name: linkfinder
  binary: python3
  description: JavaScript endpoint and URL extractor (LinkFinder)
  input:
    type: arg
    flag: -i
  output:
    type: stdout
    format: regex
    pattern: '^(?P<full_url>https?://\S+|/[^\s"'']+)'
    fields:
      full_url: full_url
  flags:
    - /usr/local/bin/linkfinder.py
    - -o
    - cli
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: file
  produces: url
`,
	},

	// ── assetfinder ───────────────────────────────────────────────────────────
	{
		Name: "assetfinder",
		RawYAML: `tool:
  name: assetfinder
  binary: assetfinder
  description: Asset and endpoint discovery within JavaScript files
  input:
    type: arg
    flag: ""
  output:
    type: stdout
    format: regex
    pattern: '^(?P<full_url>https?://\S+)'
    fields:
      full_url: full_url
  flags:
    - --subs-only
  user_flags: []
  env: {}
  timeout:
    flag: ""
    default: 0
  consumes: file
  produces: url
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
