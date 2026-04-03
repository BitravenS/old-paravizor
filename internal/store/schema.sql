-- Schema for paravizor project database.
-- Table order matters: referenced tables must come before referencing ones.

CREATE TABLE IF NOT EXISTS scope_rules (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern    TEXT    NOT NULL,
    type       TEXT    NOT NULL CHECK(type IN ('exact','wildcard','path','regex','cidr')),
    in_scope   BOOLEAN NOT NULL DEFAULT 1, -- 1 for in-scope, 0 for out-of-scope
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS batches (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id      TEXT    NOT NULL,
    item_count   INTEGER NOT NULL DEFAULT 0,
    status       TEXT    NOT NULL DEFAULT 'pending'
                 CHECK(status IN ('pending','processing','completed','failed')),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_batches_node   ON batches(node_id);
CREATE INDEX IF NOT EXISTS idx_batches_status ON batches(status);

CREATE TABLE IF NOT EXISTS domains (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    source     TEXT    NOT NULL,
    is_live    BOOLEAN,
    ip         TEXT,
    batch_id   INTEGER REFERENCES batches(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_domains_name ON domains(name);
CREATE INDEX IF NOT EXISTS idx_domains_live ON domains(is_live);

CREATE TABLE IF NOT EXISTS techstack (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id  INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    technology TEXT    NOT NULL,
    version    TEXT,
    category   TEXT,
    source     TEXT    NOT NULL,
    UNIQUE(domain_id, technology)
);
CREATE INDEX IF NOT EXISTS idx_techstack_domain ON techstack(domain_id);
CREATE INDEX IF NOT EXISTS idx_techstack_tech   ON techstack(technology);

CREATE TABLE IF NOT EXISTS ips (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    address    TEXT    NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ports (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    ip_id    INTEGER NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    port     INTEGER NOT NULL,
    protocol TEXT    NOT NULL DEFAULT 'tcp',
    service  TEXT,
    banner   TEXT,
    source   TEXT    NOT NULL,
    UNIQUE(ip_id, port, protocol)
);
CREATE INDEX IF NOT EXISTS idx_ports_ip ON ports(ip_id);

CREATE TABLE IF NOT EXISTS urls (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    full_url     TEXT    NOT NULL UNIQUE,
    domain_id    INTEGER REFERENCES domains(id),
    path         TEXT,
    query_string TEXT,
    status_code  INTEGER,
    content_type TEXT,
    source       TEXT    NOT NULL,
    batch_id     INTEGER REFERENCES batches(id),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_urls_domain ON urls(domain_id);
CREATE INDEX IF NOT EXISTS idx_urls_status ON urls(status_code);

CREATE TABLE IF NOT EXISTS url_flags (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id     INTEGER NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    flag_type  TEXT    NOT NULL,
    flag_value TEXT,
    source     TEXT    NOT NULL,
    UNIQUE(url_id, flag_type, flag_value)
);
CREATE INDEX IF NOT EXISTS idx_url_flags_url  ON url_flags(url_id);
CREATE INDEX IF NOT EXISTS idx_url_flags_type ON url_flags(flag_type);

CREATE TABLE IF NOT EXISTS findings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id   INTEGER REFERENCES domains(id),
    url_id      INTEGER REFERENCES urls(id),
    scanner     TEXT    NOT NULL,
    severity    TEXT    CHECK(severity IN ('info','low','medium','high','critical')),
    title       TEXT    NOT NULL,
    description TEXT,
    evidence    TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_findings_severity ON findings(severity);
CREATE INDEX IF NOT EXISTS idx_findings_scanner  ON findings(scanner);
CREATE INDEX IF NOT EXISTS idx_findings_domain   ON findings(domain_id);

CREATE TABLE IF NOT EXISTS processes (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT    NOT NULL,
    command        TEXT    NOT NULL,
    pid            INTEGER,
    node_id        TEXT    NOT NULL,
    batch_id       INTEGER REFERENCES batches(id),
    status         TEXT    NOT NULL DEFAULT 'running'
                   CHECK(status IN ('running','completed','failed','killed','timeout')),
    exit_code      INTEGER,
    stdout_path    TEXT,
    stderr_path    TEXT,
    started_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at   DATETIME,
    last_heartbeat DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_processes_status ON processes(status);
CREATE INDEX IF NOT EXISTS idx_processes_node   ON processes(node_id);

CREATE TABLE IF NOT EXISTS pipeline_state (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    item_type    TEXT    NOT NULL,
    item_id      INTEGER NOT NULL,
    node_id      TEXT    NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'pending'
                 CHECK(status IN ('pending','processing','completed','failed','skipped')),
    started_at   DATETIME,
    completed_at DATETIME,
    error        TEXT,
    UNIQUE(item_id, item_type, node_id)
);
CREATE INDEX IF NOT EXISTS idx_pipeline_state_node   ON pipeline_state(node_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_state_status ON pipeline_state(status);

CREATE TABLE IF NOT EXISTS dns_records (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id   INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    record_type TEXT    NOT NULL,
    value       TEXT    NOT NULL,
    ttl         INTEGER,
    source      TEXT    NOT NULL,
    UNIQUE(domain_id, record_type, value)
);
CREATE INDEX IF NOT EXISTS idx_dns_records_domain ON dns_records(domain_id);

CREATE TABLE IF NOT EXISTS downloaded_files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id     INTEGER NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    file_path  TEXT    NOT NULL,
    file_type  TEXT    NOT NULL,
    size_bytes INTEGER,
    sha256     TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_downloaded_files_url  ON downloaded_files(url_id);
CREATE INDEX IF NOT EXISTS idx_downloaded_files_type ON downloaded_files(file_type);
