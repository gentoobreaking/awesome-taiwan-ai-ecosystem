-- schema_v2.sql — New SQLite schema supporting the canonical Entity model (spec §61 Phase 10).
-- This schema co-locates core dimensions (classification, MCP identity,
-- Taiwan/AI relevance, quality, security) as top-level columns for efficient
-- filtering while storing complex structures as JSON.

-- ── entities table (core entity model) ────────────────────────────────────
CREATE TABLE IF NOT EXISTS entities (
    -- Core identity
    id                   TEXT PRIMARY KEY,
    name                 TEXT,
    slug                 TEXT UNIQUE,
    description          TEXT,
    entity_status        TEXT NOT NULL DEFAULT 'DISCOVERED',

    -- Classification (spec §8, §55)
    primary_classification           TEXT,
    classification_confidence        REAL,
    classification_evidence_json     JSON,
    mcp_role                         TEXT,

    -- MCP Identity (spec §59, §61 Phase 7)
    mcp_identity_status              TEXT,
    mcp_identity_evidence_json       JSON,
    mcp_identity_confidence          REAL,
    mcp_role_secondary               JSON,            -- []MCPRole
    static_checked_at                TEXT,
    runtime_verified_at              TEXT,

    -- Taiwan Relevance (spec §14, §17)
    taiwan_score            REAL,
    taiwan_level            TEXT,
    taiwan_evidence_json    JSON,
    taiwan_confidence       REAL,

    -- AI Relevance
    ai_score            REAL,
    ai_level            TEXT,
    ai_evidence_json    JSON,
    ai_confidence       REAL,

    -- Quality (spec §15, §31)
    quality_score            REAL,
    quality_grade            TEXT,
    quality_components_json  JSON,
    quality_evidence_json    JSON,

    -- Security (spec §12, §56 Test 12)
    security_status           TEXT,
    security_findings_json    JSON,
    security_scanned_at       TEXT,

    -- Nested structures (JSON blobs)
    repository_json           JSON,
    endpoints_json            JSON,
    tools_json                JSON,
    resources_json            JSON,
    data_sources_json         JSON,
    sources_json              JSON,
    raw_content               TEXT,
    runtime_verification_json JSON,

    -- Timestamps
    first_seen       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen        TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_verified    TEXT,
    created_at       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient filtering
CREATE INDEX IF NOT EXISTS idx_entities_entity_status        ON entities(entity_status);
CREATE INDEX IF NOT EXISTS idx_entities_primary_classification ON entities(primary_classification);
CREATE INDEX IF NOT EXISTS idx_entities_mcp_identity_status  ON entities(mcp_identity_status);
CREATE INDEX IF NOT EXISTS idx_entities_taiwan_level         ON entities(taiwan_level);
CREATE INDEX IF NOT EXISTS idx_entities_security_status      ON entities(security_status);
CREATE INDEX IF NOT EXISTS idx_entities_slug                 ON entities(slug);
CREATE INDEX IF NOT EXISTS idx_entities_quality_score        ON entities(quality_score);
CREATE INDEX IF NOT EXISTS idx_entities_ai_level             ON entities(ai_level);

-- ── entity_evidence table (evidence records for all dimensions) ────────────
CREATE TABLE IF NOT EXISTS entity_evidence (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id     TEXT NOT NULL,
    dimension     TEXT NOT NULL,  -- classification | taiwan | ai | mcp_identity | security | quality
    rule          TEXT NOT NULL,
    source        TEXT,
    location      TEXT,
    matched_text  TEXT,
    content_hash  TEXT,
    score         REAL,
    confidence    REAL,
    timestamp     TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_evidence_entity  ON entity_evidence(entity_id);
CREATE INDEX IF NOT EXISTS idx_evidence_dimension ON entity_evidence(dimension);
CREATE INDEX IF NOT EXISTS idx_evidence_rule    ON entity_evidence(rule);

-- ── entity_endpoints table (first-class endpoint storage) ──────────────────
CREATE TABLE IF NOT EXISTS entity_endpoints (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id       TEXT NOT NULL,
    url             TEXT NOT NULL,
    transport       TEXT,
    type            TEXT,
    protocol_version TEXT,
    auth_json       JSON,
    tls             INTEGER DEFAULT 0,  -- boolean as integer
    status          TEXT,
    evidence_json   JSON,
    confidence      REAL,
    FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_endpoints_entity ON entity_endpoints(entity_id);

-- ── migration_log table (V1→V2 migration tracking) ────────────────────────
CREATE TABLE IF NOT EXISTS migration_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    old_id          TEXT,           -- legacy mcp_servers.id
    new_id          TEXT,           -- entities.id (same value since ID is sha256)
    entity_name     TEXT,
    status          TEXT,           -- SUCCESS | FAILED | SKIPPED
    error_message   TEXT,
    migrated_at     TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_migration_old_id ON migration_log(old_id);
CREATE INDEX IF NOT EXISTS idx_migration_new_id ON migration_log(new_id);
CREATE INDEX IF NOT EXISTS idx_migration_status ON migration_log(status);
