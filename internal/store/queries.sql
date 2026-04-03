-- Queries for paravizor store.
-- All parameters use ? placeholders — never interpolated — making injection impossible.
-- Note: sqlc's SQLite parser does not support RETURNING or ON CONFLICT...DO UPDATE,
-- so upserts use INSERT OR IGNORE + UPDATE, and inserted IDs are retrieved via last_insert_rowid().

-- ============================================================
-- scope_rules
-- ============================================================

-- name: InsertScopeRule :exec
INSERT INTO scope_rules (pattern, type, in_scope) VALUES (?, ?, ?);

-- name: GetScopeRules :many
SELECT id, pattern, type, in_scope, created_at FROM scope_rules ORDER BY in_scope ASC, id ASC;

-- name: DeleteScopeRuleByPattern :exec
DELETE FROM scope_rules WHERE pattern = ?;

-- ============================================================
-- batches
-- ============================================================

-- name: InsertBatch :exec
INSERT INTO batches (node_id, item_count, status) VALUES (?, ?, 'processing');

-- name: GetBatchByID :one
SELECT id, node_id, item_count, status, created_at, completed_at FROM batches WHERE id = ?;

-- name: CompleteBatch :exec
UPDATE batches SET status = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?;

-- ============================================================
-- domains
-- ============================================================

-- name: InsertOrTouchDomain :exec
INSERT INTO domains (name, source, batch_id) VALUES (?, ?, ?)
ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP;

-- name: GetDomainByName :one
SELECT id, name, source, is_live, ip, batch_id, created_at, updated_at
FROM domains WHERE name = ?;

-- name: GetDomains :many
SELECT id, name, source, is_live, ip, batch_id, created_at, updated_at
FROM domains ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: GetLiveDomains :many
SELECT id, name, source, is_live, ip, batch_id, created_at, updated_at
FROM domains WHERE is_live = 1 ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: CountDomains :one
SELECT COUNT(*) FROM domains;

-- name: UpdateDomainLiveness :exec
UPDATE domains SET is_live = ?, ip = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- ============================================================
-- techstack
-- ============================================================

-- name: InsertOrIgnoreTechStack :exec
INSERT OR IGNORE INTO techstack (domain_id, technology, version, category, source)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateTechStack :exec
UPDATE techstack SET version = ?, category = ?, source = ?
WHERE domain_id = ? AND technology = ?;

-- name: GetTechStackByDomain :many
SELECT id, domain_id, technology, version, category, source FROM techstack WHERE domain_id = ?;

-- ============================================================
-- ips
-- ============================================================

-- name: InsertOrIgnoreIP :exec
INSERT OR IGNORE INTO ips (address) VALUES (?);

-- name: GetIPByAddress :one
SELECT id, address, created_at FROM ips WHERE address = ?;

-- ============================================================
-- ports
-- ============================================================

-- name: InsertOrIgnorePort :exec
INSERT OR IGNORE INTO ports (ip_id, port, protocol, service, banner, source)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdatePort :exec
UPDATE ports SET service = ?, banner = ?, source = ?
WHERE ip_id = ? AND port = ? AND protocol = ?;

-- name: GetPortsByIP :many
SELECT id, ip_id, port, protocol, service, banner, source FROM ports WHERE ip_id = ?;

-- ============================================================
-- urls
-- ============================================================

-- name: InsertOrTouchURL :exec
INSERT INTO urls (full_url, source, domain_id, batch_id) VALUES (?, ?, ?, ?)
ON CONFLICT(full_url) DO UPDATE SET updated_at = CURRENT_TIMESTAMP;

-- name: GetURLByFullURL :one
SELECT id, full_url, domain_id, path, query_string, status_code, content_type,
       source, batch_id, created_at, updated_at
FROM urls WHERE full_url = ?;

-- name: GetURLs :many
SELECT id, full_url, domain_id, path, query_string, status_code, content_type,
       source, batch_id, created_at, updated_at
FROM urls ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: CountURLs :one
SELECT COUNT(*) FROM urls;

-- name: UpdateURLDetails :exec
UPDATE urls
SET status_code = ?, content_type = ?, path = ?, query_string = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- ============================================================
-- url_flags
-- ============================================================

-- name: InsertOrIgnoreURLFlag :exec
INSERT OR IGNORE INTO url_flags (url_id, flag_type, flag_value, source) VALUES (?, ?, ?, ?);

-- name: UpdateURLFlag :exec
UPDATE url_flags SET source = ? WHERE url_id = ? AND flag_type = ? AND flag_value = ?;

-- name: GetURLFlagsByURL :many
SELECT id, url_id, flag_type, flag_value, source FROM url_flags WHERE url_id = ?;

-- ============================================================
-- findings
-- ============================================================

-- name: InsertFinding :exec
INSERT INTO findings (domain_id, url_id, scanner, severity, title, description, evidence)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetFindings :many
SELECT id, domain_id, url_id, scanner, severity, title, description, evidence, created_at
FROM findings ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: CountFindings :one
SELECT COUNT(*) FROM findings;

-- ============================================================
-- processes
-- ============================================================

-- name: InsertProcess :exec
INSERT INTO processes (name, command, pid, node_id, batch_id, status, stdout_path, stderr_path)
VALUES (?, ?, ?, ?, ?, 'running', ?, ?);

-- name: GetProcessByID :one
SELECT id, name, command, pid, node_id, batch_id, status, exit_code,
       stdout_path, stderr_path, started_at, completed_at, last_heartbeat
FROM processes WHERE id = ?;

-- name: CompleteProcess :exec
UPDATE processes SET status = ?, exit_code = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?;
-- name: HeartbeatProcess :exec
UPDATE processes
SET last_heartbeat = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetRunningProcessPIDs :many
SELECT pid FROM processes WHERE status = 'running' AND pid IS NOT NULL;
-- ============================================================
-- pipeline_state
-- ============================================================

-- name: InsertOrIgnorePipelineState :exec
INSERT OR IGNORE INTO pipeline_state (item_type, item_id, node_id, status)
VALUES (?, ?, ?, 'pending');

-- name: UpdatePipelineState :exec
UPDATE pipeline_state
SET status = ?, started_at = ?, completed_at = ?, error = ?
WHERE item_id = ? AND item_type = ? AND node_id = ?;

-- name: GetPendingItems :many
SELECT id, item_type, item_id, node_id, status, started_at, completed_at, error
FROM pipeline_state
WHERE node_id = ? AND item_type = ? AND status = 'pending'
ORDER BY id ASC
LIMIT ?;

-- name: ResetProcessingItems :execresult
UPDATE pipeline_state SET status = 'pending', started_at = NULL WHERE status = 'processing';

-- name: HasInterruptedSession :one
SELECT COUNT(*) FROM pipeline_state WHERE status = 'processing';

-- ============================================================
-- dns_records
-- ============================================================

-- name: InsertOrIgnoreDNSRecord :exec
INSERT OR IGNORE INTO dns_records (domain_id, record_type, value, ttl, source) VALUES (?, ?, ?, ?, ?);

-- name: UpdateDNSRecord :exec
UPDATE dns_records SET ttl = ?, source = ?
WHERE domain_id = ? AND record_type = ? AND value = ?;

-- name: GetDNSRecordsByDomain :many
SELECT id, domain_id, record_type, value, ttl, source FROM dns_records WHERE domain_id = ?;

-- ============================================================
-- downloaded_files
-- ============================================================

-- name: InsertDownloadedFile :exec
INSERT INTO downloaded_files (url_id, file_path, file_type, size_bytes, sha256) VALUES (?, ?, ?, ?, ?);

-- name: GetDownloadedFilesByURL :many
SELECT id, url_id, file_path, file_type, size_bytes, sha256, created_at
FROM downloaded_files WHERE url_id = ?;

-- ============================================================
-- notes
-- ============================================================

-- name: InsertNote :exec
INSERT INTO notes (content) VALUES (?);

-- name: GetNotes :many
SELECT id, content, created_at, updated_at FROM notes ORDER BY created_at DESC;

-- name: UpdateNote :exec
UPDATE notes SET content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: DeleteNote :exec
DELETE FROM notes WHERE id = ?;
