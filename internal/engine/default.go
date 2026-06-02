package engine

import (
	"os"

	"github.com/bitravens/paravizor/v1/internal/utils"
)

// WriteDefaultPipeline writes the default recon pipeline to path if it does not
// already exist. The pipeline follows the Paravizor spec:
//
//   - Stage 1: Subdomain Enumeration (passive → bruteforce → permutation → live filter)
//   - Stage 2: Fingerprinting (screenshots, port scanning, tech stack)
//   - Stage 3: URL Extraction (crawl → dedup → liveliness)
//   - Stage 4: Attack Surface Triage (grep/gf patterns, all terminal)
//   - Stage 5: Static File Extraction (download by type)
//   - Stage 6: Vulnerability Scanning (takeover, nuclei, wpscan, dast, JS analysis)
//
// Architectural constraints: one tool per node, fanout via multi-target Routes,
// conditional routing via EvalCondition expressions.
func WriteDefaultPipeline(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	cfg := PipelineConfig{
		Name:        "default",
		Description: "Default Paravizor bug-bounty recon pipeline",

		// Init maps scope item types to their pipeline entry points.
		// Wildcard and exact domains fan-out to passive enumeration tools so a
		// normal target like example.com still discovers subdomains without the
		// user having to write *.example.com. Exact domains also go directly to
		// live-host filtering. Path-scoped targets enter at URL deduplication.
		Init: []InitConfig{
			{Scope: "wildcard", Node: "subdomain-passive-subfinder", ItemType: "domain"},
			{Scope: "wildcard", Node: "subdomain-passive-chaos", ItemType: "domain"},
			{Scope: "wildcard", Node: "subdomain-passive-amass", ItemType: "domain"},
			{Scope: "exact", Node: "subdomain-passive-subfinder", ItemType: "domain"},
			{Scope: "exact", Node: "subdomain-passive-chaos", ItemType: "domain"},
			{Scope: "exact", Node: "subdomain-passive-amass", ItemType: "domain"},
			{Scope: "exact", Node: "dnsx-live", ItemType: "domain"},
			{Scope: "path", Node: "url-dedup", ItemType: "url"},
		},

		Stages: []StageConfig{
			{ID: 1, Name: "Subdomain Enumeration"},
			{ID: 2, Name: "Fingerprinting"},
			{ID: 3, Name: "URL Extraction"},
			{ID: 4, Name: "Attack Surface Triage"},
			{ID: 5, Name: "Static File Extraction"},
			{ID: 6, Name: "Vulnerability Scanning"},
		},

		Nodes: []NodeConfig{

			// ── Stage 1: Subdomain Enumeration ────────────────────────────────────

			// 1.1a  Passive enumeration — subfinder
			// Output goes to: bruteforce (further expand), live filter, takeover scan.
			{
				ID:       "subdomain-passive-subfinder",
				Name:     "Passive Subdomain Enum (subfinder)",
				Stage:    1,
				Tool:     "subfinder",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 1, Timeout: "10s"},
				Routes: []RouteConfig{
					{To: "subdomain-bruteforce"},
					{To: "dnsx-live"},
					{To: "vuln-takeover"},
				},
			},

			// 1.1b  Passive enumeration — Chaos API
			{
				ID:       "subdomain-passive-chaos",
				Name:     "Passive Subdomain Enum (Chaos API)",
				Stage:    1,
				Tool:     "chaos",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 1, Timeout: "10s"},
				Routes: []RouteConfig{
					{To: "subdomain-bruteforce"},
					{To: "dnsx-live"},
					{To: "vuln-takeover"},
				},
			},

			// 1.1c  Passive enumeration — Amass passive
			{
				ID:       "subdomain-passive-amass",
				Name:     "Passive Subdomain Enum (Amass)",
				Stage:    1,
				Tool:     "amass-passive",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 1, Timeout: "2m"},
				Routes: []RouteConfig{
					{To: "subdomain-bruteforce"},
					{To: "dnsx-live"},
					{To: "vuln-takeover"},
				},
			},

			// 1.2  Active DNS bruteforce — puredns bruteforce
			// Newly discovered subdomains feed permutation expansion, live filter,
			// and takeover checks in parallel.
			{
				ID:       "subdomain-bruteforce",
				Name:     "Active DNS Bruteforce (puredns)",
				Stage:    1,
				Tool:     "puredns-bruteforce",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 1, Timeout: "30m"},
				Routes: []RouteConfig{
					{To: "subdomain-permutation-expand"},
					{To: "dnsx-live"},
					{To: "vuln-takeover"},
				},
			},

			// 1.3a  Permutation expansion — alterx generates candidates
			{
				ID:       "subdomain-permutation-expand",
				Name:     "Subdomain Permutation Expand (alterx)",
				Stage:    1,
				Tool:     "alterx",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 50, Timeout: "5m", WaitForPeers: true},
				Routes: []RouteConfig{
					{To: "subdomain-permutation-resolve"},
				},
			},

			// 1.3b  Permutation resolution — puredns resolves generated candidates
			{
				ID:       "subdomain-permutation-resolve",
				Name:     "Permutation DNS Resolve (puredns)",
				Stage:    1,
				Tool:     "puredns-resolve",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 100, Timeout: "10m", WaitForPeers: true},
				Routes: []RouteConfig{
					{To: "dnsx-live"},
					{To: "vuln-takeover"},
				},
			},

			// 1.4  Live host filter — dnsx
			// The central hub of stage 1: all confirmed-live subdomains fan out to
			// fingerprinting (stage 2) and URL crawling (stage 3) simultaneously.
			{
				ID:       "dnsx-live",
				Name:     "Live Host Filter (dnsx)",
				Stage:    1,
				Tool:     "dnsx-live",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 500, Timeout: "5m", WaitForPeers: true},
				Routes: []RouteConfig{
					{To: "screenshot"},
					{To: "port-scan-resolve"},
					{To: "tech-httpx"},
					{To: "tech-nuclei"},
					{To: "tech-whatcms"},
					{To: "vuln-nuclei-host"},
					{To: "url-crawl-katana"},
					{To: "url-crawl-gau"},
					{To: "url-crawl-wayback"},
				},
			},

			// ── Stage 2: Fingerprinting ────────────────────────────────────────────

			// 2.1  Screenshots — haylxon (terminal)
			{
				ID:       "screenshot",
				Name:     "Web Screenshot Capture (haylxon)",
				Stage:    2,
				Tool:     "haylxon",
				Consumes: "domain",
				Produces: "file",
				Batch:    BatchConfig{Size: 50, Timeout: "5m"},
			},

			// 2.2a  Port scan: resolve domain → IP addresses
			{
				ID:       "port-scan-resolve",
				Name:     "DNS A Record Resolution (dnsx)",
				Stage:    2,
				Tool:     "dnsx-resolve",
				Consumes: "domain",
				Produces: "ip",
				Batch:    BatchConfig{Size: 100, Timeout: "5m"},
				Routes: []RouteConfig{
					{To: "port-scan-rustscan"},
				},
			},

			// 2.2b  Port scan: fast scanner
			{
				ID:       "port-scan-rustscan",
				Name:     "Fast Port Scan (rustscan)",
				Stage:    2,
				Tool:     "rustscan",
				Consumes: "ip",
				Produces: "port",
				Batch:    BatchConfig{Size: 50, Timeout: "10m"},
				Routes: []RouteConfig{
					{To: "port-scan-nmap"},
				},
			},

			// 2.2c  Port scan: service/version detection — terminal
			{
				ID:       "port-scan-nmap",
				Name:     "Service Version Scan (nmap)",
				Stage:    2,
				Tool:     "nmap",
				Consumes: "port",
				Produces: "finding",
				Batch:    BatchConfig{Size: 20, Timeout: "20m"},
			},

			// 2.3a  Tech fingerprinting via httpx
			// Conditional routing: items whose source contains "wordpress" are sent
			// to WordPress-specific nodes.  Tools must set the source name accordingly.
			{
				ID:       "tech-httpx",
				Name:     "HTTP Tech Fingerprint (httpx)",
				Stage:    2,
				Tool:     "httpx-tech",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 100, Timeout: "10m"},
				Routes: []RouteConfig{
					{To: "wordpress-theme-detect", Condition: `'wordpress' in source`},
					{To: "vuln-wpscan", Condition: `'wordpress' in source`},
				},
			},

			// 2.3b  Tech fingerprinting via Nuclei technology templates
			{
				ID:       "tech-nuclei",
				Name:     "Tech Fingerprint via Nuclei",
				Stage:    2,
				Tool:     "nuclei-tech",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 100, Timeout: "10m"},
				Routes: []RouteConfig{
					{To: "wordpress-theme-detect", Condition: `'wordpress' in source`},
					{To: "vuln-wpscan", Condition: `'wordpress' in source`},
				},
			},

			// 2.3c  Tech fingerprinting via whatcms
			{
				ID:       "tech-whatcms",
				Name:     "CMS Detection (whatcms)",
				Stage:    2,
				Tool:     "whatcms",
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{Size: 50, Timeout: "10m"},
				Routes: []RouteConfig{
					{To: "wordpress-theme-detect", Condition: `'wordpress' in source`},
					{To: "vuln-wpscan", Condition: `'wordpress' in source`},
				},
			},

			// 2.4  WordPress theme detection — terminal
			{
				ID:       "wordpress-theme-detect",
				Name:     "WordPress Theme Detection (themedetect)",
				Stage:    2,
				Tool:     "themedetect",
				Consumes: "domain",
				Produces: "finding",
				Batch:    BatchConfig{Size: 20, Timeout: "5m"},
			},

			// ── Stage 3: URL Extraction ────────────────────────────────────────────

			// 3.1  Active crawl — katana
			{
				ID:       "url-crawl-katana",
				Name:     "Active Web Crawl (katana)",
				Stage:    3,
				Tool:     "katana",
				Consumes: "domain",
				Produces: "url",
				Batch:    BatchConfig{Size: 20, Timeout: "10m"},
				Routes:   []RouteConfig{{To: "url-dedup"}},
			},

			// 3.2a  Passive URL collection — gau
			{
				ID:       "url-crawl-gau",
				Name:     "Passive URL Enum (gau)",
				Stage:    3,
				Tool:     "gau",
				Consumes: "domain",
				Produces: "url",
				Batch:    BatchConfig{Size: 20, Timeout: "5m"},
				Routes:   []RouteConfig{{To: "url-dedup"}},
			},

			// 3.2b  Passive URL collection — waybackurls
			{
				ID:       "url-crawl-wayback",
				Name:     "Wayback Machine URLs (waybackurls)",
				Stage:    3,
				Tool:     "waybackurls",
				Consumes: "domain",
				Produces: "url",
				Batch:    BatchConfig{Size: 20, Timeout: "5m"},
				Routes:   []RouteConfig{{To: "url-dedup"}},
			},

			// 3.3  URL deduplication — uro
			// This is the stage-3 hub: deduplicated URLs fan out to:
			//   • liveliness check (→ static file download)
			//   • DAST scan (only parameterised URLs: '=' in full_url)
			//   • all 13 attack-surface triage nodes (stage 4)
			{
				ID:       "url-dedup",
				Name:     "URL Deduplication (uro)",
				Stage:    3,
				Tool:     "uro",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 1000, Timeout: "2m", WaitForPeers: true},
				Routes: []RouteConfig{
					{To: "url-liveliness"},
					// 3.7: route parameterised URLs to DAST (grep '=')
					{To: "vuln-nuclei-dast", Condition: `'=' in full_url`},
					// Stage 4: attack surface triage — all nodes receive the full URL set
					{To: "attack-surface-params"},
					{To: "attack-surface-endpoints"},
					{To: "attack-surface-api"},
					{To: "attack-surface-upload"},
					{To: "attack-surface-admin"},
					{To: "attack-surface-auth"},
					{To: "attack-surface-gf-xss"},
					{To: "attack-surface-gf-ssrf"},
					{To: "attack-surface-gf-rce"},
					{To: "attack-surface-gf-idor"},
					{To: "attack-surface-gf-redirect"},
					{To: "attack-surface-gf-lfi"},
					{To: "attack-surface-gf-sqli"},
				},
			},

			// 3.6  URL liveliness check — httpx
			// Extension-based conditional routing to stage-5 file download nodes.
			// Status-code-based routing (200 vs 301 vs 401…) requires engine
			// enhancement; currently all live URLs are forwarded.
			{
				ID:       "url-liveliness",
				Name:     "URL Liveliness Check (httpx)",
				Stage:    3,
				Tool:     "httpx-probe",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 500, Timeout: "10m"},
				Routes: []RouteConfig{
					{To: "static-file-js", Condition: `full_url matches '\.js([?#]|$)'`},
					{To: "static-file-docs", Condition: `full_url matches '\.(pdf|doc|docx|xls|xlsx)([?#]|$)'`},
					{To: "static-file-text", Condition: `full_url matches '\.(txt|csv|xml|conf|cnf|reg|inf|rdp|cfg|ora|ini|log|json)([?#]|$)'`},
					{To: "static-file-backup", Condition: `full_url matches '\.(bak|backup|bkp|old|bkf)([?#]|$)'`},
				},
			},

			// ── Stage 4: Attack Surface Triage (all terminal) ─────────────────────
			// Each node runs a grep/gf pattern over the full URL corpus and saves
			// the matching URLs as "interesting" items for manual review.

			// 4.1  Interesting query parameters
			{
				ID:       "attack-surface-params",
				Name:     "Interesting Parameters Triage",
				Stage:    4,
				Tool:     "grep-interesting-params",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},

			// 4.2  Interesting endpoint paths
			{
				ID:       "attack-surface-endpoints",
				Name:     "Interesting Endpoints Triage",
				Stage:    4,
				Tool:     "grep-interesting-endpoints",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},

			// 4.3  API endpoint paths
			{
				ID:       "attack-surface-api",
				Name:     "API Endpoint Triage",
				Stage:    4,
				Tool:     "grep-api-endpoints",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},

			// 4.4  File upload endpoints
			{
				ID:       "attack-surface-upload",
				Name:     "File Upload Endpoint Triage",
				Stage:    4,
				Tool:     "grep-file-upload",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},

			// 4.5  Admin / internal paths
			{
				ID:       "attack-surface-admin",
				Name:     "Admin and Internal Path Triage",
				Stage:    4,
				Tool:     "grep-admin-paths",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},

			// 4.6  Authentication endpoints
			{
				ID:       "attack-surface-auth",
				Name:     "Auth Endpoint Triage",
				Stage:    4,
				Tool:     "grep-auth-endpoints",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},

			// 4.7  GF patterns
			{
				ID:       "attack-surface-gf-xss",
				Name:     "GF XSS Pattern Scan",
				Stage:    4,
				Tool:     "gf-xss",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},
			{
				ID:       "attack-surface-gf-ssrf",
				Name:     "GF SSRF Pattern Scan",
				Stage:    4,
				Tool:     "gf-ssrf",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},
			{
				ID:       "attack-surface-gf-rce",
				Name:     "GF RCE Pattern Scan",
				Stage:    4,
				Tool:     "gf-rce",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},
			{
				ID:       "attack-surface-gf-idor",
				Name:     "GF IDOR Pattern Scan",
				Stage:    4,
				Tool:     "gf-idor",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},
			{
				ID:       "attack-surface-gf-redirect",
				Name:     "GF Open Redirect Pattern Scan",
				Stage:    4,
				Tool:     "gf-redirect",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},
			{
				ID:       "attack-surface-gf-lfi",
				Name:     "GF LFI Pattern Scan",
				Stage:    4,
				Tool:     "gf-lfi",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},
			{
				ID:       "attack-surface-gf-sqli",
				Name:     "GF SQLi Pattern Scan",
				Stage:    4,
				Tool:     "gf-sqli",
				Consumes: "url",
				Produces: "url",
				Batch:    BatchConfig{Size: 5000, Timeout: "5m", WaitForPeers: true},
			},

			// ── Stage 5: Static File Extraction ───────────────────────────────────
			// Items arrive pre-filtered by extension via conditions on url-liveliness
			// routes.  wget downloads them to the project downloads directory.

			// 5.1  JavaScript files → JS analysis (stage 6.5)
			{
				ID:       "static-file-js",
				Name:     "JavaScript File Download",
				Stage:    5,
				Tool:     "wget-download",
				Consumes: "url",
				Produces: "file",
				Batch:    BatchConfig{Size: 100, Timeout: "10m"},
				Routes: []RouteConfig{
					{To: "vuln-js-linkfinder"},
					{To: "vuln-js-assetfinder"},
				},
			},

			// 5.2  Document files — terminal
			{
				ID:       "static-file-docs",
				Name:     "Document File Download",
				Stage:    5,
				Tool:     "wget-download",
				Consumes: "url",
				Produces: "file",
				Batch:    BatchConfig{Size: 50, Timeout: "10m"},
			},

			// 5.3  Text / config files — terminal
			{
				ID:       "static-file-text",
				Name:     "Text and Config File Download",
				Stage:    5,
				Tool:     "wget-download",
				Consumes: "url",
				Produces: "file",
				Batch:    BatchConfig{Size: 50, Timeout: "10m"},
			},

			// 5.4  Backup files — terminal
			{
				ID:       "static-file-backup",
				Name:     "Backup File Download",
				Stage:    5,
				Tool:     "wget-download",
				Consumes: "url",
				Produces: "file",
				Batch:    BatchConfig{Size: 50, Timeout: "10m"},
			},

			// ── Stage 6: Vulnerability Scanning ───────────────────────────────────

			// 6.1  Subdomain takeover — kanha (terminal)
			{
				ID:       "vuln-takeover",
				Name:     "Subdomain Takeover Check (kanha)",
				Stage:    6,
				Tool:     "kanha",
				Consumes: "domain",
				Produces: "finding",
				Batch:    BatchConfig{Size: 100, Timeout: "10m"},
			},

			// 6.2  Nuclei host-level scan (terminal)
			{
				ID:       "vuln-nuclei-host",
				Name:     "Nuclei Host Scan",
				Stage:    6,
				Tool:     "nuclei-host",
				Consumes: "domain",
				Produces: "finding",
				Batch:    BatchConfig{Size: 50, Timeout: "30m"},
			},

			// 6.3  WordPress vulnerability scan (terminal)
			{
				ID:       "vuln-wpscan",
				Name:     "WordPress Vulnerability Scan (wpscan)",
				Stage:    6,
				Tool:     "wpscan",
				Consumes: "domain",
				Produces: "finding",
				Batch:    BatchConfig{Size: 10, Timeout: "10m"},
			},

			// 6.4  Nuclei DAST / fuzzing on parameterised URLs (terminal)
			{
				ID:       "vuln-nuclei-dast",
				Name:     "Nuclei DAST and Fuzzing",
				Stage:    6,
				Tool:     "nuclei-dast",
				Consumes: "url",
				Produces: "finding",
				Batch:    BatchConfig{Size: 200, Timeout: "60m"},
			},

			// 6.5a  JS link extraction — LinkFinder (terminal)
			// Produces url items (endpoints found in JS) saved to DB.
			// Requires engine support for file-type item resolution.
			{
				ID:       "vuln-js-linkfinder",
				Name:     "JS Link Extraction (linkfinder)",
				Stage:    6,
				Tool:     "linkfinder",
				Consumes: "file",
				Produces: "url",
				Batch:    BatchConfig{Size: 20, Timeout: "5m"},
			},

			// 6.5b  JS asset discovery (terminal)
			{
				ID:       "vuln-js-assetfinder",
				Name:     "JS Asset Discovery (assetfinder)",
				Stage:    6,
				Tool:     "assetfinder",
				Consumes: "file",
				Produces: "url",
				Batch:    BatchConfig{Size: 20, Timeout: "5m"},
			},
		},
	}

	return utils.WriteYAML(path, PipelineWrapper{Pipeline: cfg})
}
