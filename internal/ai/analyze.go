package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bitravens/paravizor/v1/internal/project"
	_ "modernc.org/sqlite"
)

const (
	defaultProvider = "ollama"
	defaultModel    = "llama3.1:8b-instruct-q4_K_M"
	defaultBaseURL  = "http://localhost:11434"

	domainSampleLimit  = 80
	urlSampleLimit     = 120
	findingSampleLimit = 60
	genericSampleLimit = 60
	logSampleLimit     = 8
	logTailBytes       = 1536
	chatHistoryLimit   = 8
	promptLineLimit    = 260
	logLineLimit       = 320
	ollamaContextSize  = 8192
	ollamaMaxTokens    = 1600
)

type ProjectSummary struct {
	ProjectName string
	ProjectDir  string
	Description string
	Pipeline    string
	Targets     []string
	Exclusions  []string
	Counts      map[string]int64

	Domains         []string
	LiveDomains     []string
	URLs            []string
	Findings        []string
	IPs             []string
	Ports           []string
	Technologies    []string
	DNSRecords      []string
	URLFlags        []string
	DownloadedFiles []string
	Notes           []string
	PipelineState   []string
	ProcessHistory  []string
	SourceCoverage  []string
	LogHighlights   []string
}

func DefaultConfig() AIConfig {
	return AIConfig{Enabled: true, Provider: defaultProvider, Model: defaultModel, BaseURL: defaultBaseURL, ConsentMode: "always_ask"}
}

func AnalyzeProject(ctx context.Context, cfg *AIConfig, projectDir string) (string, error) {
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return "", err
	}

	summary, err := BuildProjectSummary(ctx, projectDir)
	if err != nil {
		return "", err
	}
	return callOllama(ctx, resolved, BuildPrompt(summary))
}

func AskProjectQuestion(ctx context.Context, cfg *AIConfig, projectDir string, history []ChatMessage, question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", fmt.Errorf("question is required")
	}

	resolved, err := resolveConfig(cfg)
	if err != nil {
		return "", err
	}

	summary, err := BuildProjectSummary(ctx, projectDir)
	if err != nil {
		return "", err
	}
	return callOllama(ctx, resolved, BuildChatPrompt(summary, history, question))
}

func resolveConfig(cfg *AIConfig) (AIConfig, error) {
	resolved := DefaultConfig()
	if cfg != nil {
		if cfg.Provider != "" {
			resolved.Provider = cfg.Provider
		}
		if cfg.Model != "" {
			resolved.Model = cfg.Model
		}
		if cfg.BaseURL != "" {
			resolved.BaseURL = cfg.BaseURL
		}
		resolved.APIKey = cfg.APIKey
		resolved.Enabled = cfg.Enabled
		resolved.ConsentMode = cfg.ConsentMode
	}
	if !resolved.Enabled {
		return AIConfig{}, fmt.Errorf("AI assistant is disabled in config")
	}
	if strings.ToLower(resolved.Provider) != defaultProvider {
		return AIConfig{}, fmt.Errorf("unsupported AI provider %q; currently supported: ollama", resolved.Provider)
	}
	return resolved, nil
}

func BuildProjectSummary(ctx context.Context, projectDir string) (ProjectSummary, error) {
	cfg, err := project.LoadProject(projectDir)
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("load project: %w", err)
	}
	database, err := sql.Open("sqlite", project.DBPath(projectDir))
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	summary := ProjectSummary{
		ProjectName: cfg.Name,
		ProjectDir:  projectDir,
		Description: cfg.Description,
		Pipeline:    cfg.Pipeline,
		Targets:     append([]string(nil), cfg.Scope.Include...),
		Exclusions:  append([]string(nil), cfg.Scope.Exclude...),
		Counts:      make(map[string]int64),
	}
	countQueries := map[string]string{
		"domains":             `SELECT COUNT(*) FROM domains`,
		"live_domains":        `SELECT COUNT(*) FROM domains WHERE is_live = 1`,
		"urls":                `SELECT COUNT(*) FROM urls`,
		"urls_with_status":    `SELECT COUNT(*) FROM urls WHERE status_code IS NOT NULL`,
		"ips":                 `SELECT COUNT(*) FROM ips`,
		"ports":               `SELECT COUNT(*) FROM ports`,
		"techstack":           `SELECT COUNT(*) FROM techstack`,
		"dns_records":         `SELECT COUNT(*) FROM dns_records`,
		"url_flags":           `SELECT COUNT(*) FROM url_flags`,
		"downloaded_files":    `SELECT COUNT(*) FROM downloaded_files`,
		"findings":            `SELECT COUNT(*) FROM findings`,
		"notes":               `SELECT COUNT(*) FROM notes`,
		"batches":             `SELECT COUNT(*) FROM batches`,
		"processes":           `SELECT COUNT(*) FROM processes`,
		"pipeline_items":      `SELECT COUNT(*) FROM pipeline_state`,
		"pipeline_pending":    `SELECT COUNT(*) FROM pipeline_state WHERE status = 'pending'`,
		"pipeline_processing": `SELECT COUNT(*) FROM pipeline_state WHERE status = 'processing'`,
		"pipeline_completed":  `SELECT COUNT(*) FROM pipeline_state WHERE status = 'completed'`,
		"pipeline_failed":     `SELECT COUNT(*) FROM pipeline_state WHERE status = 'failed'`,
		"pipeline_skipped":    `SELECT COUNT(*) FROM pipeline_state WHERE status = 'skipped'`,
	}
	for key, query := range countQueries {
		summary.Counts[key] = queryCount(ctx, database, query)
	}

	summary.Domains = queryStrings(ctx, database, fmt.Sprintf(`
SELECT name || ' | source=' || source ||
       CASE WHEN is_live = 1 THEN ' | live=true' ELSE '' END ||
       CASE WHEN ip IS NOT NULL AND ip <> '' THEN ' | ip=' || ip ELSE '' END
FROM domains
ORDER BY updated_at DESC, created_at DESC
LIMIT %d`, domainSampleLimit))
	summary.LiveDomains = queryStrings(ctx, database, fmt.Sprintf(`
SELECT name ||
       CASE WHEN ip IS NOT NULL AND ip <> '' THEN ' | ip=' || ip ELSE '' END ||
       ' | source=' || source
FROM domains
WHERE is_live = 1
ORDER BY updated_at DESC, created_at DESC
LIMIT %d`, domainSampleLimit))
	summary.URLs = queryStrings(ctx, database, fmt.Sprintf(`
SELECT full_url ||
       CASE WHEN status_code IS NOT NULL THEN ' | status=' || status_code ELSE '' END ||
       CASE WHEN content_type IS NOT NULL AND content_type <> '' THEN ' | type=' || content_type ELSE '' END ||
       CASE WHEN path IS NOT NULL AND path <> '' THEN ' | path=' || path ELSE '' END ||
       ' | source=' || source
FROM urls
ORDER BY updated_at DESC, created_at DESC
LIMIT %d`, urlSampleLimit))
	summary.Findings = queryStrings(ctx, database, fmt.Sprintf(`
SELECT COALESCE(findings.severity, 'info') || ' | ' || findings.scanner || ' | ' || findings.title ||
       CASE WHEN domains.name IS NOT NULL THEN ' | domain=' || domains.name ELSE '' END ||
       CASE WHEN urls.full_url IS NOT NULL THEN ' | url=' || urls.full_url ELSE '' END ||
       CASE WHEN findings.evidence IS NOT NULL AND findings.evidence <> '' THEN ' | evidence=' || substr(replace(replace(findings.evidence, char(10), ' '), char(13), ' '), 1, 240) ELSE '' END
FROM findings
LEFT JOIN domains ON domains.id = findings.domain_id
LEFT JOIN urls ON urls.id = findings.url_id
ORDER BY CASE COALESCE(findings.severity, 'info')
    WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 ELSE 5 END,
    findings.created_at DESC
LIMIT %d`, findingSampleLimit))
	summary.IPs = queryStrings(ctx, database, fmt.Sprintf(`
SELECT address
FROM ips
ORDER BY created_at DESC
LIMIT %d`, genericSampleLimit))
	summary.Ports = queryStrings(ctx, database, fmt.Sprintf(`
SELECT ips.address || ':' || ports.port || '/' || ports.protocol ||
       CASE WHEN ports.service IS NOT NULL AND ports.service <> '' THEN ' | service=' || ports.service ELSE '' END ||
       CASE WHEN ports.banner IS NOT NULL AND ports.banner <> '' THEN ' | banner=' || substr(replace(replace(ports.banner, char(10), ' '), char(13), ' '), 1, 180) ELSE '' END ||
       ' | source=' || ports.source
FROM ports
JOIN ips ON ips.id = ports.ip_id
ORDER BY ips.address ASC, ports.port ASC
LIMIT %d`, genericSampleLimit))
	summary.Technologies = queryStrings(ctx, database, fmt.Sprintf(`
SELECT domains.name || ' | ' || techstack.technology ||
       CASE WHEN techstack.version IS NOT NULL AND techstack.version <> '' THEN '@' || techstack.version ELSE '' END ||
       CASE WHEN techstack.category IS NOT NULL AND techstack.category <> '' THEN ' | category=' || techstack.category ELSE '' END ||
       ' | source=' || techstack.source
FROM techstack
JOIN domains ON domains.id = techstack.domain_id
ORDER BY domains.name ASC, techstack.technology ASC
LIMIT %d`, genericSampleLimit))
	summary.DNSRecords = queryStrings(ctx, database, fmt.Sprintf(`
SELECT domains.name || ' | ' || dns_records.record_type || '=' || dns_records.value ||
       CASE WHEN dns_records.ttl IS NOT NULL THEN ' | ttl=' || dns_records.ttl ELSE '' END ||
       ' | source=' || dns_records.source
FROM dns_records
JOIN domains ON domains.id = dns_records.domain_id
ORDER BY domains.name ASC, dns_records.record_type ASC
LIMIT %d`, genericSampleLimit))
	summary.URLFlags = queryStrings(ctx, database, fmt.Sprintf(`
SELECT urls.full_url || ' | ' || url_flags.flag_type ||
       CASE WHEN url_flags.flag_value IS NOT NULL AND url_flags.flag_value <> '' THEN '=' || url_flags.flag_value ELSE '' END ||
       ' | source=' || url_flags.source
FROM url_flags
JOIN urls ON urls.id = url_flags.url_id
ORDER BY url_flags.flag_type ASC, urls.full_url ASC
LIMIT %d`, genericSampleLimit))
	summary.DownloadedFiles = queryStrings(ctx, database, fmt.Sprintf(`
SELECT urls.full_url || ' | ' || downloaded_files.file_type || ' | ' || downloaded_files.file_path ||
       CASE WHEN downloaded_files.size_bytes IS NOT NULL THEN ' | bytes=' || downloaded_files.size_bytes ELSE '' END ||
       CASE WHEN downloaded_files.sha256 IS NOT NULL AND downloaded_files.sha256 <> '' THEN ' | sha256=' || downloaded_files.sha256 ELSE '' END
FROM downloaded_files
JOIN urls ON urls.id = downloaded_files.url_id
ORDER BY downloaded_files.created_at DESC
LIMIT %d`, genericSampleLimit))
	summary.Notes = queryStrings(ctx, database, fmt.Sprintf(`
SELECT substr(replace(replace(content, char(10), ' '), char(13), ' '), 1, 320)
FROM notes
ORDER BY updated_at DESC, created_at DESC
LIMIT %d`, genericSampleLimit))
	summary.PipelineState = queryStrings(ctx, database, fmt.Sprintf(`
SELECT node_id || ' | ' || item_type || ' | ' || status || '=' || COUNT(*) ||
       CASE WHEN SUM(CASE WHEN error IS NOT NULL AND error <> '' THEN 1 ELSE 0 END) > 0 THEN ' | errors=' || SUM(CASE WHEN error IS NOT NULL AND error <> '' THEN 1 ELSE 0 END) ELSE '' END
FROM pipeline_state
GROUP BY node_id, item_type, status
ORDER BY node_id ASC, status ASC, item_type ASC
LIMIT %d`, genericSampleLimit))
	summary.ProcessHistory = queryStrings(ctx, database, fmt.Sprintf(`
SELECT node_id || ' | ' || name || ' | status=' || status ||
       CASE WHEN exit_code IS NOT NULL THEN ' | exit=' || exit_code ELSE '' END ||
       CASE WHEN pid IS NOT NULL THEN ' | pid=' || pid ELSE '' END ||
       ' | command=' || substr(command, 1, 180)
FROM processes
ORDER BY started_at DESC
LIMIT %d`, genericSampleLimit))
	summary.SourceCoverage = queryStrings(ctx, database, fmt.Sprintf(`
SELECT kind || ' | ' || source || '=' || total
FROM (
    SELECT 'domains' AS kind, source, COUNT(*) AS total FROM domains WHERE source <> '' GROUP BY source
    UNION ALL SELECT 'urls', source, COUNT(*) FROM urls WHERE source <> '' GROUP BY source
    UNION ALL SELECT 'ports', source, COUNT(*) FROM ports WHERE source <> '' GROUP BY source
    UNION ALL SELECT 'techstack', source, COUNT(*) FROM techstack WHERE source <> '' GROUP BY source
    UNION ALL SELECT 'dns_records', source, COUNT(*) FROM dns_records WHERE source <> '' GROUP BY source
    UNION ALL SELECT 'url_flags', source, COUNT(*) FROM url_flags WHERE source <> '' GROUP BY source
    UNION ALL SELECT 'findings', scanner, COUNT(*) FROM findings WHERE scanner <> '' GROUP BY scanner
)
ORDER BY kind ASC, total DESC
LIMIT %d`, genericSampleLimit))
	summary.LogHighlights = collectLogHighlights(projectDir)

	return summary, nil
}

func BuildPrompt(summary ProjectSummary) string {
	var b strings.Builder
	b.WriteString("You are Paravizor's local recon analysis assistant.\n\n")
	b.WriteString("Operating contract:\n")
	b.WriteString("- Analyze only the authorized recon evidence below and do not invent assets, statuses, vulnerabilities, or tool results.\n")
	b.WriteString("- Use every populated evidence category at least once, even if the conclusion is that it has no high-signal findings.\n")
	b.WriteString("- Prioritize synthesis, safe validation, next recon actions, and report organization.\n")
	b.WriteString("- Do not provide exploit payloads, destructive instructions, credential abuse, or steps outside the stated scope.\n")
	b.WriteString("- When evidence is weak or sampled, explicitly say what is missing and how to validate safely.\n\n")

	appendProjectEvidence(&b, summary)

	b.WriteString("## Required Response Design\n")
	b.WriteString("Return polished Markdown only. Make it easy to scan in a terminal: short paragraphs, compact bullets, concrete evidence references, and tables for comparisons. Keep the report under 1,200 words, cap tables at the strongest 5 rows unless more are necessary, and avoid filler, oversized prose blocks, emojis, and decorative characters.\n\n")
	b.WriteString("Use this exact structure:\n\n")
	b.WriteString("# Recon Command Brief\n")
	b.WriteString("## At A Glance\n")
	b.WriteString("- 4-6 compact bullets with bold labels: Scope, Coverage, Strongest Signal, Highest Priority, Biggest Gap, and Immediate Next Move when supported by evidence.\n")
	b.WriteString("## Signal Board\n")
	b.WriteString("- Markdown table: Area | What Stands Out | Evidence | Confidence.\n")
	b.WriteString("- Cover live hosts, endpoints, network exposure, technologies, DNS, files, findings, notes, and pipeline/process/log health when populated.\n")
	b.WriteString("## Priority Queue\n")
	b.WriteString("- Markdown table: Rank | Asset Or Cluster | Signal | Evidence Used | Safe Next Move.\n")
	b.WriteString("- Keep ranks operator-focused and explain why each item deserves attention now.\n")
	b.WriteString("## Evidence Walkthrough\n")
	b.WriteString("- Use concise subsections for Live Surface, Endpoints, Network And DNS, Technology And Files, Findings And Notes, and Pipeline Health.\n")
	b.WriteString("- In each subsection, separate confirmed evidence from weak or sampled signals.\n")
	b.WriteString("## Report Candidates\n")
	b.WriteString("- Markdown table: Candidate | Status | Supporting Evidence | Validation Needed | Report Value.\n")
	b.WriteString("- Clearly label each candidate as Confirmed, Needs Validation, Weak Signal, or Not Supported.\n")
	b.WriteString("## Next Recon Plan\n")
	b.WriteString("- Markdown table: Order | Action | Why | Expected Output | Depends On.\n")
	b.WriteString("- Prioritize safe validation steps, reruns for failed tools, and data gaps before speculative testing.\n")
	b.WriteString("## Operator Notes\n")
	b.WriteString("- Briefly call out sampled data, missing categories, failed or noisy tools, confidence limits, and manual checks worth doing.\n")
	return b.String()
}

func BuildChatPrompt(summary ProjectSummary, history []ChatMessage, question string) string {
	var b strings.Builder
	b.WriteString("You are Paravizor's local recon chat assistant.\n\n")
	b.WriteString("Operating contract:\n")
	b.WriteString("- Answer only from the authorized recon evidence below and the chat history.\n")
	b.WriteString("- Do not invent assets, statuses, vulnerabilities, tool output, or scan results.\n")
	b.WriteString("- If evidence is missing or sampled, say exactly what is missing and how to validate safely.\n")
	b.WriteString("- Do not provide exploit payloads, destructive instructions, credential abuse, or steps outside the stated scope.\n")
	b.WriteString("- Keep answers focused on the operator's question.\n\n")

	appendProjectEvidence(&b, summary)

	b.WriteString("## Chat History\n")
	start := 0
	if len(history) > chatHistoryLimit {
		start = len(history) - chatHistoryLimit
	}
	written := 0
	for _, msg := range history[start:] {
		content := trimPromptContent(msg.Content, 2400)
		if content == "" {
			continue
		}
		role := normalizeChatRole(msg.Role)
		b.WriteString("### ")
		b.WriteString(role)
		b.WriteString("\n")
		b.WriteString(content)
		b.WriteString("\n\n")
		written++
	}
	if written == 0 {
		b.WriteString("_No previous chat messages._\n\n")
	}

	b.WriteString("## Operator Question\n")
	b.WriteString(trimPromptContent(question, 2400))
	b.WriteString("\n\n")
	b.WriteString("Answer in concise Markdown, usually under 500 words. Cite concrete recon evidence from the context when possible. If the data does not support the answer, say what is missing and suggest safe recon validation steps.\n")
	return b.String()
}

func appendProjectEvidence(b *strings.Builder, summary ProjectSummary) {
	b.WriteString("## Project Context\n")
	appendKV(b, "Name", summary.ProjectName)
	appendKV(b, "Directory", summary.ProjectDir)
	appendKV(b, "Description", summary.Description)
	appendKV(b, "Pipeline", summary.Pipeline)
	appendKV(b, "In scope", inlineList(summary.Targets))
	appendKV(b, "Out of scope", inlineList(summary.Exclusions))
	b.WriteString("\n")

	appendListSection(b, "Counts", strings.Split(formatCounts(summary.Counts), "\n"), 0)
	appendListSection(b, "Source Coverage", summary.SourceCoverage, 0)
	appendListSection(b, "Domains", summary.Domains, summary.Counts["domains"])
	appendListSection(b, "Live Domains", summary.LiveDomains, summary.Counts["live_domains"])
	appendListSection(b, "URLs", summary.URLs, summary.Counts["urls"])
	appendListSection(b, "IPs", summary.IPs, summary.Counts["ips"])
	appendListSection(b, "Open Ports", summary.Ports, summary.Counts["ports"])
	appendListSection(b, "Technologies", summary.Technologies, summary.Counts["techstack"])
	appendListSection(b, "DNS Records", summary.DNSRecords, summary.Counts["dns_records"])
	appendListSection(b, "URL Flags", summary.URLFlags, summary.Counts["url_flags"])
	appendListSection(b, "Downloaded Files", summary.DownloadedFiles, summary.Counts["downloaded_files"])
	appendListSection(b, "Findings", summary.Findings, summary.Counts["findings"])
	appendListSection(b, "Operator Notes", summary.Notes, summary.Counts["notes"])
	appendListSection(b, "Pipeline State", summary.PipelineState, summary.Counts["pipeline_items"])
	appendListSection(b, "Process History", summary.ProcessHistory, summary.Counts["processes"])
	appendListSection(b, "Tool Output Highlights", summary.LogHighlights, int64(len(summary.LogHighlights)))
}

func normalizeChatRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "Assistant"
	default:
		return "Operator"
	}
}

func trimPromptContent(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "\n[truncated]"
}

func callOllama(ctx context.Context, cfg AIConfig, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	payload := map[string]any{
		"model":  cfg.Model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]any{
			"temperature": 0.2,
			"num_ctx":     ollamaContextSize,
			"num_predict": ollamaMaxTokens,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ollama at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	var decoded struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	if resp.StatusCode >= 400 || decoded.Error != "" {
		if decoded.Error == "" {
			decoded.Error = resp.Status
		}
		return "", fmt.Errorf("ollama error: %s", decoded.Error)
	}
	return strings.TrimSpace(decoded.Response), nil
}

func queryCount(ctx context.Context, db *sql.DB, query string) int64 {
	var value int64
	_ = db.QueryRowContext(ctx, query).Scan(&value)
	return value
}

func queryStrings(ctx context.Context, db *sql.DB, query string) []string {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err == nil && strings.TrimSpace(value) != "" {
			result = append(result, compactLine(value, promptLineLimit))
		}
	}
	return result
}

func collectLogHighlights(projectDir string) []string {
	logsDir := filepath.Join(projectDir, "logs")
	entries, err := collectLogFiles(logsDir)
	if err != nil || len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].stream != entries[j].stream {
			return entries[i].stream == "stderr"
		}
		return entries[i].modTime.After(entries[j].modTime)
	})

	var highlights []string
	for _, entry := range entries {
		if len(highlights) >= logSampleLimit {
			break
		}
		tail := readTail(entry.path, logTailBytes)
		if tail == "" {
			continue
		}
		highlights = append(highlights, compactLine(fmt.Sprintf("%s | %s | %s", entry.nodeID, entry.stream, tail), logLineLimit))
	}
	return highlights
}

type logFileEntry struct {
	path    string
	nodeID  string
	stream  string
	modTime time.Time
}

func collectLogFiles(logsDir string) ([]logFileEntry, error) {
	var entries []logFileEntry
	err := filepath.WalkDir(logsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		stream := ""
		switch {
		case strings.HasSuffix(entry.Name(), ".stderr"):
			stream = "stderr"
		case strings.HasSuffix(entry.Name(), ".stdout"):
			stream = "stdout"
		default:
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		nodeID := filepath.Base(filepath.Dir(path))
		entries = append(entries, logFileEntry{path: path, nodeID: nodeID, stream: stream, modTime: info.ModTime()})
		return nil
	})
	return entries, err
}

func readTail(path string, maxBytes int64) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ""
	}
	start := info.Size() - maxBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func appendKV(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "none"
	}
	b.WriteString("- ")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\n")
}

func appendListSection(b *strings.Builder, title string, values []string, total int64) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n")
	if total > int64(len(values)) && len(values) > 0 {
		b.WriteString(fmt.Sprintf("_Showing %d of %d captured rows._\n", len(values), total))
	}
	if len(values) == 0 {
		b.WriteString("_None captured._\n\n")
		return
	}
	for _, value := range values {
		b.WriteString("- ")
		b.WriteString(value)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func inlineList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func joinList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return "- " + strings.Join(values, "\n- ")
}

func formatCounts(counts map[string]int64) string {
	keys := []string{
		"domains",
		"live_domains",
		"urls",
		"urls_with_status",
		"ips",
		"ports",
		"techstack",
		"dns_records",
		"url_flags",
		"downloaded_files",
		"findings",
		"notes",
		"batches",
		"processes",
		"pipeline_items",
		"pipeline_pending",
		"pipeline_processing",
		"pipeline_completed",
		"pipeline_failed",
		"pipeline_skipped",
	}
	var lines []string
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d", key, counts[key]))
	}
	return strings.Join(lines, "\n")
}

func compactLine(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-1]) + "…"
}

func ReportPath(projectDir string) string {
	return filepath.Join(projectDir, "ai-analysis.md")
}
