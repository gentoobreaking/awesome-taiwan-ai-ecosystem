// Package storage provides SQLite persistence for the Taiwan MCP Crawler.
// All operations are idempotent (upsert-based) per §TST-037.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	_ "modernc.org/sqlite"
)

const schemaVersion = "0.1"

// Store provides SQLite persistence for the crawler.
type Store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path.
func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &Store{db: db}, nil
}

// Migrate applies all migrations up. Idempotent — running twice produces no error.
func (s *Store) Migrate(ctx context.Context) error {
	if err := s.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	migrations := []struct {
		name string
		up   string
	}{
		{"001_init_schema", migration001Up},
		{"002_server_snapshots", migration002Up},
	}

	for _, m := range migrations {
		var applied int
		err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM _crawler_migrations WHERE name = ?", m.name).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", m.name, err)
		}
		if applied > 0 {
			continue // already applied
		}

		_, err = s.db.ExecContext(ctx, m.up)
		if err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}

		_, err = s.db.ExecContext(ctx,
			"INSERT INTO _crawler_migrations (name, applied_at) VALUES (?, ?)",
			m.name, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
	}

	return nil
}

func (s *Store) createMigrationsTable(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _crawler_migrations (
			name TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`)
	return err
}

// UpsertServer inserts or updates an MCPServer. Idempotent.
func (s *Store) UpsertServer(ctx context.Context, server *models.MCPServer) error {
	repoJSON, _ := json.Marshal(server.Repository)
	endpointsJSON, _ := json.Marshal(server.Endpoints)
	taiwanJSON, _ := json.Marshal(server.TaiwanRelevance)
	qualityJSON, _ := json.Marshal(server.Quality)
	toolsJSON, _ := json.Marshal(server.Tools)
	resourcesJSON, _ := json.Marshal(server.Resources)
	promptsJSON, _ := json.Marshal(server.Prompts)
	dataSourcesJSON, _ := json.Marshal(server.DataSources)
	transportJSON, _ := json.Marshal(server.Transport)
	categoryJSON, _ := json.Marshal(server.Category)
	regionJSON, _ := json.Marshal(server.Region)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_servers (
			id, name, slug, description, category, region, taiwan_relevance,
			repository, endpoints, transport, tools, resources, prompts,
			data_sources, license, status, health, quality,
			first_seen_at, last_seen_at, last_verified_at, schema_version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			slug = excluded.slug,
			description = excluded.description,
			category = excluded.category,
			region = excluded.region,
			taiwan_relevance = excluded.taiwan_relevance,
			repository = excluded.repository,
			endpoints = excluded.endpoints,
			transport = excluded.transport,
			tools = excluded.tools,
			resources = excluded.resources,
			prompts = excluded.prompts,
			data_sources = excluded.data_sources,
			license = excluded.license,
			status = excluded.status,
			health = excluded.health,
			quality = excluded.quality,
			last_seen_at = excluded.last_seen_at,
			last_verified_at = excluded.last_verified_at,
			schema_version = excluded.schema_version
	`,
		server.ID, server.Name, server.Slug, server.Description,
		categoryJSON, regionJSON, taiwanJSON,
		repoJSON, endpointsJSON, transportJSON, toolsJSON, resourcesJSON, promptsJSON,
		dataSourcesJSON, server.License, string(server.Status), string(server.Health),
		qualityJSON,
		server.FirstSeen.Format(time.RFC3339), server.LastSeen.Format(time.RFC3339), server.LastVerified.Format(time.RFC3339),
		schemaVersion)

	if err != nil {
		return fmt.Errorf("upsert server: %w", err)
	}

	// Upsert repository details
	if err := s.upsertRepository(ctx, server.ID, &server.Repository); err != nil {
		return err
	}

	// Upsert endpoints
	for _, ep := range server.Endpoints {
		if err := s.upsertEndpoint(ctx, server.ID, &ep); err != nil {
			return err
		}
	}

	// Upsert tools
	for _, tool := range server.Tools {
		if err := s.upsertTool(ctx, server.ID, &tool); err != nil {
			return err
		}
	}

	// Upsert resources
	for _, res := range server.Resources {
		if err := s.upsertResource(ctx, server.ID, &res); err != nil {
			return err
		}
	}

	// Upsert prompts
	for _, prompt := range server.Prompts {
		if err := s.upsertPrompt(ctx, server.ID, &prompt); err != nil {
			return err
		}
	}

	// Upsert data sources
	for _, ds := range server.DataSources {
		if err := s.upsertDataSource(ctx, server.ID, &ds); err != nil {
			return err
		}
	}

	// Upsert sources
	for _, src := range server.Sources {
		if err := s.upsertSourceAndLink(ctx, server.ID, &src); err != nil {
			return err
		}
	}

	// Upsert health check
	if err := s.upsertHealthCheck(ctx, server.ID, server.Health); err != nil {
		return err
	}

	// Upsert quality score
	if err := s.upsertQualityScore(ctx, server.ID, &server.Quality); err != nil {
		return err
	}

	return nil
}

// SaveServer persists a server and its snapshot for a crawl run. Idempotent.
func (s *Store) SaveServer(ctx context.Context, server *models.MCPServer, crawlID string) error {
	if err := s.UpsertServer(ctx, server); err != nil {
		return fmt.Errorf("save server: %w", err)
	}
	if err := s.InsertServerSnapshot(ctx, server.ID, crawlID, server); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	return nil
}

func (s *Store) upsertRepository(ctx context.Context, serverID string, repo *models.RepositoryInfo) error {
	topics, _ := json.Marshal(repo.Topics)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO repositories (
			server_id, url, host, owner, name, stars, forks, watchers,
			open_issues, language, topics, license, default_branch, archived,
			fork, homepage, created_at, updated_at, pushed_at, last_commit_at, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET
			url = excluded.url, host = excluded.host, owner = excluded.owner,
			name = excluded.name, stars = excluded.stars, forks = excluded.forks,
			watchers = excluded.watchers, open_issues = excluded.open_issues,
			language = excluded.language, topics = excluded.topics,
			license = excluded.license, default_branch = excluded.default_branch,
			archived = excluded.archived, fork = excluded.fork,
			homepage = excluded.homepage, created_at = excluded.created_at,
			updated_at = excluded.updated_at, pushed_at = excluded.pushed_at,
			last_commit_at = excluded.last_commit_at, status = excluded.status
	`,
		serverID, repo.URL, repo.Host, repo.Owner, repo.Name, repo.Stars, repo.Forks, repo.Watchers,
		repo.OpenIssues, repo.Language, topics, repo.License, repo.DefaultBranch,
		repo.Archived, repo.Fork, repo.Homepage,
		repo.CreatedAt, repo.UpdatedAt, repo.PushedAt, repo.LastCommitAt, string(models.StatusUnknown))
	return err
}

func (s *Store) upsertEndpoint(ctx context.Context, serverID string, ep *models.Endpoint) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO endpoints (server_id, url, transport, protocol_version, tls, status)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, url) DO UPDATE SET
			transport = excluded.transport,
			protocol_version = excluded.protocol_version,
			tls = excluded.tls,
			status = excluded.status
	`, serverID, ep.URL, ep.Transport, ep.ProtocolVersion, ep.TLS, ep.Status)
	return err
}

func (s *Store) upsertTool(ctx context.Context, serverID string, tool *models.Tool) error {
	inputSchema, _ := json.Marshal(tool.InputSchema)
	annotations, _ := json.Marshal(tool.Annotations)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tools (server_id, name, description, input_schema, annotations)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(server_id, name) DO UPDATE SET
			description = excluded.description,
			input_schema = excluded.input_schema,
			annotations = excluded.annotations
	`, serverID, tool.Name, tool.Description, inputSchema, annotations)
	return err
}

func (s *Store) upsertResource(ctx context.Context, serverID string, res *models.Resource) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO resources (server_id, uri, name, description, mime_type)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(server_id, uri) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			mime_type = excluded.mime_type
	`, serverID, res.URI, res.Name, res.Description, res.MIMEType)
	return err
}

func (s *Store) upsertPrompt(ctx context.Context, serverID string, p *models.Prompt) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO prompts (server_id, name, description)
		VALUES (?, ?, ?)
		ON CONFLICT(server_id, name) DO UPDATE SET
			description = excluded.description
	`, serverID, p.Name, p.Description)
	return err
}

func (s *Store) upsertDataSource(ctx context.Context, serverID string, ds *models.DataSource) error {
	official := 0
	if ds.Official {
		official = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO server_data_sources
			(server_id, name, type, url, country, official, access_method)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, serverID, ds.Name, string(ds.Type), ds.URL, ds.Country, official, ds.AccessMethod)
	return err
}

func (s *Store) upsertSourceAndLink(ctx context.Context, serverID string, src *models.SourceReference) error {
	var sourceID int
	// Upsert into sources table
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sources (source, url, discovered_at, last_seen_at, trust_score)
		VALUES (?, ?, ?, ?, ?)
	`, src.Source, src.URL, src.DiscoveredAt, src.LastSeen, src.TrustScore)
	if err != nil {
		return fmt.Errorf("insert source: %w", err)
	}

	// Find the source ID (by source + url)
	err = s.db.QueryRowContext(ctx,
		"SELECT id FROM sources WHERE source = ? AND url = ? ORDER BY id DESC LIMIT 1",
		src.Source, src.URL).Scan(&sourceID)
	if err != nil {
		return fmt.Errorf("find source id: %w", err)
	}

	// Link server to source
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO server_sources (server_id, source_id)
		VALUES (?, ?)
	`, serverID, sourceID)
	if err != nil {
		return fmt.Errorf("link source: %w", err)
	}
	return nil
}

func (s *Store) upsertHealthCheck(ctx context.Context, serverID string, health models.HealthStatus) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO health_checks (server_id, status, checked_at)
		VALUES (?, ?, ?)
	`, serverID, string(health), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) upsertQualityScore(ctx context.Context, serverID string, q *models.QualityScore) error {
	components, _ := json.Marshal(q.Components)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quality_scores (server_id, score, grade, components, calculated_at)
		VALUES (?, ?, ?, ?, ?)
	`, serverID, q.Score, q.Grade, components, time.Now().UTC().Format(time.RFC3339))
	return err
}

// GetServer retrieves a single MCPServer by ID.
func (s *Store) GetServer(ctx context.Context, id string) (*models.MCPServer, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, name, slug, description, category, region, taiwan_relevance, "+
		"repository, endpoints, transport, tools, resources, prompts, data_sources, "+
		"license, status, health, quality, first_seen_at, last_seen_at, last_verified_at "+
		"FROM mcp_servers WHERE id = ?", id)

	var server models.MCPServer
	var categoryJSON, regionJSON, taiwanJSON, repoJSON, endpointsJSON, transportJSON,
		toolsJSON, resourcesJSON, promptsJSON, dataSourcesJSON, qualityJSON []byte

	var firstSeenStr, lastSeenStr, lastVerifiedStr string
	if err := row.Scan(
		&server.ID, &server.Name, &server.Slug, &server.Description,
		&categoryJSON, &regionJSON, &taiwanJSON,
		&repoJSON, &endpointsJSON, &transportJSON, &toolsJSON, &resourcesJSON, &promptsJSON,
		&dataSourcesJSON, &server.License, &server.Status, &server.Health, &qualityJSON,
		&firstSeenStr, &lastSeenStr, &lastVerifiedStr); err != nil {
		return nil, err
	}

	server.FirstSeen, _ = time.Parse(time.RFC3339, firstSeenStr)
	server.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr)
	server.LastVerified, _ = time.Parse(time.RFC3339, lastVerifiedStr)

	json.Unmarshal(categoryJSON, &server.Category)
	json.Unmarshal(regionJSON, &server.Region)
	json.Unmarshal(taiwanJSON, &server.TaiwanRelevance)
	json.Unmarshal(repoJSON, &server.Repository)
	json.Unmarshal(endpointsJSON, &server.Endpoints)
	json.Unmarshal(transportJSON, &server.Transport)
	json.Unmarshal(toolsJSON, &server.Tools)
	json.Unmarshal(resourcesJSON, &server.Resources)
	json.Unmarshal(promptsJSON, &server.Prompts)
	json.Unmarshal(dataSourcesJSON, &server.DataSources)
	json.Unmarshal(qualityJSON, &server.Quality)

	// Load sources
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.source, s.url, s.discovered_at, s.last_seen_at, s.trust_score
		FROM server_sources ss JOIN sources s ON ss.source_id = s.id WHERE ss.server_id = ?
	`, id)
	if err == nil {
		for rows.Next() {
			var sr models.SourceReference
			rows.Scan(&sr.Source, &sr.URL, &sr.DiscoveredAt, &sr.LastSeen, &sr.TrustScore)
			server.Sources = append(server.Sources, sr)
		}
		rows.Close()
	}

	return &server, nil
}

// CountServers returns the total number of servers.
func (s *Store) CountServers(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mcp_servers").Scan(&count)
	return count, err
}

// GetServerIDs returns all server IDs.
func (s *Store) GetServerIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM mcp_servers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

// GetTaiwanServers returns servers at a specific Taiwan relevance level.
func (s *Store) GetTaiwanServers(ctx context.Context, level string) ([]models.MCPServer, error) {
	ids, err := s.GetServerIDs(ctx)
	if err != nil {
		return nil, err
	}

	var servers []models.MCPServer
	for _, id := range ids {
		server, err := s.GetServer(ctx, id)
		if err != nil {
			continue
		}
		if server.TaiwanRelevance.Level == level {
			servers = append(servers, *server)
		}
	}
	return servers, nil
}

// GetServers returns all servers.
func (s *Store) GetServers(ctx context.Context) ([]models.MCPServer, error) {
	ids, err := s.GetServerIDs(ctx)
	if err != nil {
		return nil, err
	}
	var servers []models.MCPServer
	for _, id := range ids {
		server, err := s.GetServer(ctx, id)
		if err == nil {
			servers = append(servers, *server)
		}
	}
	return servers, nil
}

// UpsertCrawlRun inserts or updates a crawl run record.
func (s *Store) UpsertCrawlRun(ctx context.Context, run *models.CrawlRun) error {
	errorsJSON, _ := json.Marshal(run.Errors)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO crawl_runs (
			crawl_id, started_at, finished_at, sources_scanned,
			candidates_found, candidates_normalized, duplicates_removed,
			taiwan_candidates, verified, failed, errors
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(crawl_id) DO UPDATE SET
			finished_at = excluded.finished_at,
			sources_scanned = excluded.sources_scanned,
			candidates_found = excluded.candidates_found,
			candidates_normalized = excluded.candidates_normalized,
			duplicates_removed = excluded.duplicates_removed,
			taiwan_candidates = excluded.taiwan_candidates,
			verified = excluded.verified,
			failed = excluded.failed,
			errors = excluded.errors
	`,
		run.CrawlID, run.StartedAt, run.FinishedAt, run.SourcesScanned,
		run.CandidatesFound, run.CandidatesNorm, run.DuplicatesRemoved,
		run.TaiwanCandidates, run.Verified, run.Failed, errorsJSON)
	return err
}

// CreateCrawlRun starts a new crawl run record.
func (s *Store) CreateCrawlRun(ctx context.Context, crawlID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO crawl_runs (crawl_id, started_at) VALUES (?, ?)
	`, crawlID, time.Now().UTC().Format(time.RFC3339))
	return err
}

// InsertEvidence stores evidence for a server.
func (s *Store) InsertEvidence(ctx context.Context, serverID string, ev *models.Evidence) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO evidence (
			server_id, type, source, location, content_hash,
			matched_text, rule, score, confidence, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		serverID, ev.Type, ev.Source, ev.Location, ev.ContentHash,
		ev.MatchedText, ev.Rule, ev.Score, ev.Confidence, ev.Timestamp)
	return err
}

// InsertSecurityFinding stores a security finding.
func (s *Store) InsertSecurityFinding(ctx context.Context, serverID string, f *models.SecurityFinding) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO security_findings (
			server_id, finding_type, severity, source, location, evidence, detected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		serverID, f.Type, string(f.Severity), f.Source, f.Location, f.Evidence,
		time.Now().UTC().Format(time.RFC3339))
	return err
}

// InsertServerSnapshot stores an append-only snapshot.
func (s *Store) InsertServerSnapshot(ctx context.Context, serverID, crawlID string, server *models.MCPServer) error {
	snapshotJSON, _ := json.Marshal(server)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO server_snapshots (server_id, crawl_id, snapshot)
		VALUES (?, ?, ?)
	`, serverID, crawlID, snapshotJSON)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
