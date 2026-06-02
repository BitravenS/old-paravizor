package ai

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitravens/paravizor/v1/internal/project"
	_ "modernc.org/sqlite"
)

func TestBuildProjectSummaryUsesAllReconEvidence(t *testing.T) {
	cfg, err := project.CreateProject("demo", "Demo recon", "", "default", "", nil, project.ScopeConfig{
		Include: []string{"example.com"},
		Exclude: []string{"out.example.com"},
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	projectDir, err := project.InitProject(t.TempDir(), *cfg)
	if err != nil {
		t.Fatalf("InitProject returned error: %v", err)
	}

	database, err := sql.Open("sqlite", project.DBPath(projectDir))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	execSQL(t, database, `
INSERT INTO domains (id, name, source, is_live, ip) VALUES
    (1, 'app.example.com', 'subfinder', 1, '192.0.2.10'),
    (2, 'dev.example.com', 'amass', NULL, NULL);
INSERT INTO ips (id, address) VALUES (1, '192.0.2.10');
INSERT INTO ports (ip_id, port, protocol, service, banner, source)
    VALUES (1, 443, 'tcp', 'https', 'nginx test banner', 'naabu');
INSERT INTO urls (id, full_url, domain_id, path, status_code, content_type, source)
    VALUES (1, 'https://app.example.com/login?next=/admin', 1, '/login', 200, 'text/html', 'katana');
INSERT INTO techstack (domain_id, technology, version, category, source)
    VALUES (1, 'nginx', '1.24', 'web-server', 'httpx');
INSERT INTO dns_records (domain_id, record_type, value, ttl, source)
    VALUES (1, 'A', '192.0.2.10', 300, 'dnsx');
INSERT INTO url_flags (url_id, flag_type, flag_value, source)
    VALUES (1, 'has-params', 'next', 'parser');
INSERT INTO downloaded_files (url_id, file_path, file_type, size_bytes, sha256)
    VALUES (1, '/tmp/app.js', 'javascript', 42, 'abc123');
INSERT INTO findings (domain_id, url_id, scanner, severity, title, description, evidence)
    VALUES (1, 1, 'nuclei', 'high', 'Exposed admin panel', 'admin surface found', 'matched /admin marker');
INSERT INTO notes (content) VALUES ('Manual note: login redirects toward admin area.');
INSERT INTO batches (id, node_id, item_count, status) VALUES (1, 'node-a', 1, 'completed');
INSERT INTO pipeline_state (item_type, item_id, node_id, status, error)
    VALUES ('domain', 1, 'node-a', 'completed', NULL), ('url', 1, 'node-b', 'failed', 'timeout');
INSERT INTO processes (name, command, pid, node_id, batch_id, status, exit_code, stdout_path, stderr_path)
    VALUES ('subfinder', 'subfinder -d example.com', 123, 'node-a', 1, 'completed', 0, 'stdout', 'stderr');
`)

	logDir := filepath.Join(projectDir, "logs", "node-a")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "subfinder-20260602-120000.stderr"), []byte("rate-limit warning\nfinished cleanly\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	summary, err := BuildProjectSummary(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("BuildProjectSummary returned error: %v", err)
	}

	if summary.Counts["domains"] != 2 || summary.Counts["ports"] != 1 || summary.Counts["pipeline_failed"] != 1 {
		t.Fatalf("unexpected counts: %#v", summary.Counts)
	}
	assertAnyContains(t, summary.Domains, "app.example.com")
	assertAnyContains(t, summary.URLs, "status=200")
	assertAnyContains(t, summary.Ports, "192.0.2.10:443/tcp")
	assertAnyContains(t, summary.Technologies, "nginx@1.24")
	assertAnyContains(t, summary.DNSRecords, "A=192.0.2.10")
	assertAnyContains(t, summary.URLFlags, "has-params=next")
	assertAnyContains(t, summary.DownloadedFiles, "javascript")
	assertAnyContains(t, summary.Findings, "Exposed admin panel")
	assertAnyContains(t, summary.Notes, "Manual note")
	assertAnyContains(t, summary.PipelineState, "failed=1")
	assertAnyContains(t, summary.ProcessHistory, "subfinder -d example.com")
	assertAnyContains(t, summary.SourceCoverage, "domains | subfinder=1")
	assertAnyContains(t, summary.LogHighlights, "rate-limit warning")
}

func TestBuildChatPromptIncludesHistoryAndQuestion(t *testing.T) {
	summary := ProjectSummary{
		ProjectName: "demo",
		ProjectDir:  "/tmp/demo",
		Pipeline:    "default",
		Targets:     []string{"example.com"},
		Counts:      map[string]int64{"domains": 1, "live_domains": 1, "urls": 1, "findings": 0},
		LiveDomains: []string{"app.example.com | ip=192.0.2.10 | source=httpx"},
		URLs:        []string{"https://app.example.com/login | status=200"},
	}
	history := []ChatMessage{
		{Role: "user", Content: "What looks alive?"},
		{Role: "assistant", Content: "app.example.com is live."},
	}

	prompt := BuildChatPrompt(summary, history, "What should I inspect next?")
	for _, expected := range []string{
		"local recon chat assistant",
		"app.example.com",
		"## Chat History",
		"app.example.com is live",
		"## Operator Question",
		"What should I inspect next?",
		"usually under 500 words",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("chat prompt missing %q:\n%s", expected, prompt)
		}
	}
}

func TestBuildPromptRequiresStructuredReconAnalysis(t *testing.T) {
	summary := ProjectSummary{
		ProjectName:     "demo",
		ProjectDir:      "/tmp/demo",
		Description:     "Demo recon",
		Pipeline:        "default",
		Targets:         []string{"example.com"},
		Exclusions:      []string{"out.example.com"},
		Counts:          map[string]int64{"domains": 1, "live_domains": 1, "urls": 1, "ports": 1, "findings": 1},
		Domains:         []string{"app.example.com | source=subfinder"},
		LiveDomains:     []string{"app.example.com | ip=192.0.2.10 | source=httpx"},
		URLs:            []string{"https://app.example.com/login | status=200"},
		Ports:           []string{"192.0.2.10:443/tcp | service=https"},
		Findings:        []string{"high | nuclei | Exposed admin panel"},
		SourceCoverage:  []string{"domains | subfinder=1"},
		LogHighlights:   []string{"node-a | stderr | rate-limit warning"},
		ProcessHistory:  []string{"node-a | subfinder | status=completed"},
		PipelineState:   []string{"node-a | domain | completed=1"},
		Technologies:    []string{"app.example.com | nginx@1.24"},
		DNSRecords:      []string{"app.example.com | A=192.0.2.10"},
		URLFlags:        []string{"https://app.example.com/login | has-params=next"},
		DownloadedFiles: []string{"https://app.example.com/app.js | javascript"},
		Notes:           []string{"Manual note"},
	}

	prompt := BuildPrompt(summary)
	for _, expected := range []string{
		"Use every populated evidence category at least once",
		"## Source Coverage",
		"## Open Ports",
		"## Tool Output Highlights",
		"Keep the report under 1,200 words",
		"cap tables at the strongest 5 rows",
		"# Recon Command Brief",
		"## Signal Board",
		"## Priority Queue",
		"## Next Recon Plan",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q:\n%s", expected, prompt)
		}
	}
}

func execSQL(t *testing.T, database *sql.DB, query string) {
	t.Helper()
	if _, err := database.Exec(query); err != nil {
		t.Fatalf("exec sql: %v\n%s", err, query)
	}
}

func assertAnyContains(t *testing.T, values []string, needle string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(value, needle) {
			return
		}
	}
	t.Fatalf("%q not found in %#v", needle, values)
}
