package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// MigrateV1ToV2 migrates from the legacy mcp_servers schema to the new
// entities schema. It reads all rows from mcp_servers, transforms them,
// backs up the old table, and writes to the new entities table.
// Idempotent: running twice produces no error and skips already-migrated rows.
func (s *EntityStore) MigrateV1ToV2(ctx context.Context) error {
	// First, ensure schema_v2 tables exist
	if err := s.ApplySchemaV2(ctx); err != nil {
		return fmt.Errorf("apply schema_v2: %w", err)
	}

	// Check if mcp_servers table exists
	var tableExists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type='table' AND name='mcp_servers'
		)
	`).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("check mcp_servers existence: %w", err)
	}

	if !tableExists {
		// No legacy table — nothing to migrate
		return nil
	}

	// Check if entities table already has data
	var entityCount int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities").Scan(&entityCount)
	if err != nil {
		return fmt.Errorf("count entities: %w", err)
	}
	if entityCount > 0 {
		// Already migrated
		return nil
	}

	// Read all old servers — collect into memory first to avoid connection
	// pool deadlock when SetMaxOpenConns(1) is used.
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, slug, description, category, region, taiwan_relevance,
			repository, endpoints, transport, tools, resources, prompts,
			data_sources, license, status, health, quality, first_seen_at,
			last_seen_at, last_verified_at, schema_version
		FROM mcp_servers
	`)
	if err != nil {
		return fmt.Errorf("read mcp_servers: %w", err)
	}

	type v1Row struct {
		id, name, slug, desc string
		category, region sql.NullString
		taiwanRelJSON sql.NullString
		repoJSON, endpointsJSON, transportJSON, toolsJSON, resourcesJSON,
			promptsJSON, dataSourcesJSON, license, status, health, qualityJSON sql.NullString
		firstSeen, lastSeen, lastVerified, schemaVer sql.NullString
	}
	var v1Rows []v1Row
	for rows.Next() {
		var r v1Row
		if err := rows.Scan(
			&r.id, &r.name, &r.slug, &r.desc, &r.category, &r.region, &r.taiwanRelJSON,
			&r.repoJSON, &r.endpointsJSON, &r.transportJSON, &r.toolsJSON, &r.resourcesJSON,
			&r.promptsJSON, &r.dataSourcesJSON, &r.license, &r.status, &r.health, &r.qualityJSON,
			&r.firstSeen, &r.lastSeen, &r.lastVerified, &r.schemaVer,
		); err != nil {
			rows.Close()
			return fmt.Errorf("scan v1 row: %w", err)
		}
		v1Rows = append(v1Rows, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}
	rows.Close()

	// Backup the old table
	_, _ = s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp_servers_v1_backup AS
		SELECT * FROM mcp_servers
	`)

	// Process each row in a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	migrated := 0
	failed := 0
	for _, r := range v1Rows {
		entity, err := transformV1ToEntity(r.id, r.name, r.slug, r.desc,
			r.category, r.region,
			nullStringToString(r.taiwanRelJSON), nullStringToString(r.repoJSON),
			nullStringToString(r.endpointsJSON),
			nullStringToString(r.transportJSON), nullStringToString(r.toolsJSON),
			nullStringToString(r.resourcesJSON), nullStringToString(r.promptsJSON),
			nullStringToString(r.dataSourcesJSON), nullStringToString(r.license),
			nullStringToString(r.status), nullStringToString(r.health),
			nullStringToString(r.qualityJSON),
			r.firstSeen, r.lastSeen, r.lastVerified)
		if err != nil {
			failed++
			_ = recordMigration(tx, r.id, "", r.name, "FAILED", err.Error())
			continue
		}

		if err := s.saveEntity(ctx, tx, entity); err != nil {
			failed++
			_ = recordMigration(tx, r.id, "", r.name, "FAILED", err.Error())
			continue
		}

		migrated++
		_ = recordMigration(tx, r.id, entity.ID, r.name, "SUCCESS", "")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}

	fmt.Printf("Migration complete: %d migrated, %d failed\n", migrated, failed)
	return nil
}

// nullStringToString converts sql.NullString to string (empty if not valid).
func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// recordMigration logs a migration entry.
func recordMigration(tx *sql.Tx, oldID, newID, name, status, errMsg string) error {
	_, err := tx.ExecContext(context.Background(), `
		INSERT INTO migration_log (old_id, new_id, entity_name, status, error_message)
		VALUES (?, ?, ?, ?, ?)
	`, oldID, newID, name, status, errMsg)
	return err
}

// transformV1ToEntity converts a legacy mcp_servers row into a new Entity.
func transformV1ToEntity(
	id, name, slug, desc string,
	category, region sql.NullString,
	taiwanRelJSON, repoJSON, endpointsJSON, transportJSON, toolsJSON,
	resourcesJSON, promptsJSON, dataSourcesJSON, license, status, health,
	qualityJSON string,
	firstSeen, lastSeen, lastVerified sql.NullString,
) (*models.Entity, error) {
	entity := &models.Entity{
		ID:          id,
		Name:        name,
		Slug:        slug,
		Description: desc,
	}

	// Parse entity_status from legacy status
	switch status {
	case "verified", "VERIFIED":
		entity.EntityStatus = models.EntityStatusVerified
	case "candidate", "CANDIDATE":
		entity.EntityStatus = models.EntityStatusCandidate
	case "quarantined", "QUARANTINED":
		entity.EntityStatus = models.EntityStatusQuarantined
	case "rejected", "REJECTED":
		entity.EntityStatus = models.EntityStatusRejected
	default:
		entity.EntityStatus = models.EntityStatusDiscovered
	}

	// Parse repository JSON (extract license if not set)
	var repo models.RepositoryInfo
	if err := json.Unmarshal([]byte(repoJSON), &repo); err != nil {
		repo = models.RepositoryInfo{}
	}
	if repo.License == "" {
		repo.License = license
	}
	entity.Repository = repo

	// Parse endpoints JSON
	var endpoints []models.EndpointWithType
	_ = json.Unmarshal([]byte(endpointsJSON), &endpoints)
	entity.Endpoints = endpoints

	// Parse tools JSON
	var tools []models.Tool
	_ = json.Unmarshal([]byte(toolsJSON), &tools)
	entity.Tools = tools

	// Parse resources JSON
	var resources []models.Resource
	_ = json.Unmarshal([]byte(resourcesJSON), &resources)
	entity.Resources = resources

	// Parse data sources JSON
	var dataSources []models.DataSource
	_ = json.Unmarshal([]byte(dataSourcesJSON), &dataSources)
	entity.DataSources = dataSources

	// Parse quality JSON
	var quality models.QualityScore
	if err := json.Unmarshal([]byte(qualityJSON), &quality); err == nil {
		entity.Quality = quality
	}

	// Parse taiwan_relevance JSON
	var taiwanRel models.TaiwanRelevance
	if err := json.Unmarshal([]byte(taiwanRelJSON), &taiwanRel); err == nil {
		entity.TaiwanRelevance = taiwanRel
	}

	// Parse timestamps
	if firstSeen.Valid {
		if t, err := time.Parse(time.RFC3339, firstSeen.String); err == nil {
			entity.FirstSeen = models.RFC3339Time(t)
		}
	}
	if lastSeen.Valid {
		if t, err := time.Parse(time.RFC3339, lastSeen.String); err == nil {
			entity.LastSeen = models.RFC3339Time(t)
		}
	}
	if lastVerified.Valid {
		if t, err := time.Parse(time.RFC3339, lastVerified.String); err == nil {
			rv := models.RFC3339Time(t)
			entity.LastVerified = &rv
		}
	}

	return entity, nil
}
