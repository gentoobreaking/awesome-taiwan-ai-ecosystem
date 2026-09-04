package storage

const migration001Up = `
-- 001_init_schema.up.sql
CREATE TABLE mcp_servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    category JSON,
    region JSON,
    taiwan_relevance JSON,
    repository JSON,
    endpoints JSON,
    transport JSON,
    tools JSON,
    resources JSON,
    prompts JSON,
    data_sources JSON,
    license TEXT,
    status TEXT,
    health TEXT,
    quality JSON,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    last_verified_at TEXT,
    schema_version TEXT
);

CREATE TABLE sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    url TEXT,
    discovered_at TEXT,
    last_seen_at TEXT,
    trust_score REAL
);

CREATE TABLE server_sources (
    server_id TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    PRIMARY KEY (server_id, source_id),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
);

CREATE TABLE server_data_sources (
    server_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    url TEXT,
    country TEXT,
    official INTEGER,
    access_method TEXT,
    PRIMARY KEY (server_id, name),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE crawl_runs (
    crawl_id TEXT PRIMARY KEY,
    started_at TEXT,
    finished_at TEXT,
    sources_scanned INTEGER DEFAULT 0,
    candidates_found INTEGER DEFAULT 0,
    candidates_normalized INTEGER DEFAULT 0,
    duplicates_removed INTEGER DEFAULT 0,
    taiwan_candidates INTEGER DEFAULT 0,
    verified INTEGER DEFAULT 0,
    failed INTEGER DEFAULT 0,
    errors JSON
);

CREATE TABLE health_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    crawl_id TEXT,
    status TEXT,
    latency_ms INTEGER,
    checks JSON,
    checked_at TEXT,
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE quality_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    crawl_id TEXT,
    score INTEGER,
    grade TEXT,
    components JSON,
    calculated_at TEXT,
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE security_findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    finding_type TEXT NOT NULL,
    severity TEXT,
    source TEXT,
    location TEXT,
    evidence TEXT,
    detected_at TEXT,
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE evidence (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    type TEXT NOT NULL,
    source TEXT,
    location TEXT,
    content_hash TEXT,
    matched_text TEXT,
    rule TEXT,
    score REAL,
    confidence REAL,
    timestamp TEXT,
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema JSON,
    annotations JSON,
    UNIQUE(server_id, name),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE resources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    uri TEXT NOT NULL,
    name TEXT,
    description TEXT,
    mime_type TEXT,
    UNIQUE(server_id, uri),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE prompts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    UNIQUE(server_id, name),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE data_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    url TEXT,
    country TEXT,
    official INTEGER,
    access_method TEXT,
    UNIQUE(server_id, name),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    url TEXT,
    host TEXT,
    owner TEXT,
    name TEXT,
    stars INTEGER,
    forks INTEGER,
    watchers INTEGER,
    open_issues INTEGER,
    language TEXT,
    topics JSON,
    license TEXT,
    default_branch TEXT,
    archived INTEGER,
    fork INTEGER,
    homepage TEXT,
    created_at TEXT,
    updated_at TEXT,
    pushed_at TEXT,
    last_commit_at TEXT,
    status TEXT,
    UNIQUE(server_id),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE TABLE endpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    url TEXT NOT NULL,
    transport TEXT,
    protocol_version TEXT,
    tls INTEGER,
    status TEXT,
    UNIQUE(server_id, url),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE INDEX idx_mcp_servers_status ON mcp_servers(status);
CREATE INDEX idx_mcp_servers_last_seen ON mcp_servers(last_seen_at);
CREATE INDEX idx_health_checks_server ON health_checks(server_id);
CREATE INDEX idx_quality_scores_server ON quality_scores(server_id);
CREATE INDEX idx_security_findings_server ON security_findings(server_id);
CREATE INDEX idx_evidence_server ON evidence(server_id);
`

const migration002Up = `
CREATE TABLE server_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    crawl_id TEXT NOT NULL,
    snapshot JSON NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
    FOREIGN KEY (crawl_id) REFERENCES crawl_runs(crawl_id) ON DELETE CASCADE
);

CREATE INDEX idx_snapshots_server ON server_snapshots(server_id);
CREATE INDEX idx_snapshots_crawl ON server_snapshots(crawl_id);
`
