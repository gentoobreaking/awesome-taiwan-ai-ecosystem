-- 002_server_snapshots.up.sql
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
