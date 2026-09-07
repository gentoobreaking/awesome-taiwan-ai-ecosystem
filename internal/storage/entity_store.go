package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// EntityFilter supports filtering entities by various dimensions.
type EntityFilter struct {
	EntityStatus          string
	PrimaryClassification string
	MCPIdentityStatus     string
	TaiwanLevel           models.TaiwanRelevanceLevel
	MinTaiwanLevel        models.TaiwanRelevanceLevel // T-series minimum (T0..T5)
	SecurityStatus        models.SecurityStatus
	QualityMin            int
	Limit                 int
	Offset                int
}

// EntityStore provides persistence for the canonical Entity model.
type EntityStore struct {
	db *sql.DB
}

// NewEntityStore creates a new EntityStore backed by an existing *sql.DB.
func NewEntityStore(db *sql.DB) *EntityStore {
	return &EntityStore{db: db}
}

// ApplySchemaV2 creates the entities table and related infrastructure.
// Safe to call multiple times (uses CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS).
func (s *EntityStore) ApplySchemaV2(ctx context.Context) error {
	schema, err := schemaV2Content()
	if err != nil {
		return fmt.Errorf("embed schema_v2: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply schema_v2: %w", err)
	}
	return nil
}

// Save inserts or updates an entity. Idempotent upsert.
func (s *EntityStore) Save(ctx context.Context, entity *models.Entity) error {
	return s.saveEntity(ctx, s.db, entity)
}

// execer is an interface for both *sql.DB and *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *EntityStore) saveEntity(ctx context.Context, e execer, entity *models.Entity) error {
	repoJSON, err := json.Marshal(entity.Repository)
	if err != nil {
		return fmt.Errorf("marshal repository: %w", err)
	}
	endpointsJSON, err := json.Marshal(entity.Endpoints)
	if err != nil {
		return fmt.Errorf("marshal endpoints: %w", err)
	}
	toolsJSON, err := json.Marshal(entity.Tools)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}
	resourcesJSON, err := json.Marshal(entity.Resources)
	if err != nil {
		return fmt.Errorf("marshal resources: %w", err)
	}
	dataSourcesJSON, err := json.Marshal(entity.DataSources)
	if err != nil {
		return fmt.Errorf("marshal data_sources: %w", err)
	}
	sourcesJSON, err := json.Marshal(entity.Sources)
	if err != nil {
		return fmt.Errorf("marshal sources: %w", err)
	}
	classificationEvidenceJSON, err := json.Marshal(entity.Classification.Evidence)
	if err != nil {
		return fmt.Errorf("marshal classification evidence: %w", err)
	}
	mcpIdentityEvidenceJSON, err := json.Marshal(entity.MCPIdentity.Evidence)
	if err != nil {
		return fmt.Errorf("marshal mcp identity evidence: %w", err)
	}
	secondaryRolesJSON, err := json.Marshal(entity.MCPIdentity.SecondaryRoles)
	if err != nil {
		return fmt.Errorf("marshal secondary roles: %w", err)
	}
	taiwanEvidenceJSON, err := json.Marshal(entity.TaiwanRelevance.Evidence)
	if err != nil {
		return fmt.Errorf("marshal taiwan evidence: %w", err)
	}
	aiEvidenceJSON, err := json.Marshal(entity.AIRelevance.Evidence)
	if err != nil {
		return fmt.Errorf("marshal ai evidence: %w", err)
	}
	qualityComponentsJSON, err := json.Marshal(entity.Quality.Components)
	if err != nil {
		return fmt.Errorf("marshal quality components: %w", err)
	}
	qualityEvidenceJSON, err := json.Marshal(entity.Quality.Evidence)
	if err != nil {
		return fmt.Errorf("marshal quality evidence: %w", err)
	}
	securityFindingsJSON, err := json.Marshal(entity.SecurityStatus.Findings)
	if err != nil {
		return fmt.Errorf("marshal security findings: %w", err)
	}

	runtimeVerificationJSON := sql.NullString{}
	if entity.RuntimeVerification != nil {
		rvJSON, err := json.Marshal(entity.RuntimeVerification)
		if err != nil {
			return fmt.Errorf("marshal runtime verification: %w", err)
		}
		runtimeVerificationJSON = sql.NullString{String: string(rvJSON), Valid: true}
	}

	staticCheckedAtStr := sql.NullString{}
	if entity.MCPIdentity.StaticCheckedAt != nil {
		staticCheckedAtStr = sql.NullString{String: entity.MCPIdentity.StaticCheckedAt.String(), Valid: true}
	}
	runtimeVerifiedAtStr := sql.NullString{}
	if entity.MCPIdentity.RuntimeVerifiedAt != nil {
		runtimeVerifiedAtStr = sql.NullString{String: entity.MCPIdentity.RuntimeVerifiedAt.String(), Valid: true}
	}
	lastVerifiedStr := sql.NullString{}
	if entity.LastVerified != nil {
		lastVerifiedStr = sql.NullString{String: entity.LastVerified.String(), Valid: true}
	}

	qualityEvidenceStr := sql.NullString{}
	if len(qualityEvidenceJSON) > 2 {
		qualityEvidenceStr = sql.NullString{String: string(qualityEvidenceJSON), Valid: true}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	scannedAt := entity.SecurityStatus.ScannedAt.String()
	if entity.SecurityStatus.ScannedAt.IsZero() {
		scannedAt = ""
	}

	_, err = e.ExecContext(ctx, `
		INSERT INTO entities (
			id, name, slug, description, entity_status,
			primary_classification, classification_confidence, classification_evidence_json, mcp_role,
			mcp_identity_status, mcp_identity_evidence_json, mcp_identity_confidence,
			mcp_role_secondary, static_checked_at, runtime_verified_at,
			taiwan_score, taiwan_level, taiwan_evidence_json, taiwan_confidence,
			ai_score, ai_level, ai_evidence_json, ai_confidence,
			quality_score, quality_grade, quality_components_json, quality_evidence_json,
			security_status, security_findings_json, security_scanned_at,
			repository_json, endpoints_json, tools_json, resources_json,
			data_sources_json, sources_json, raw_content, runtime_verification_json,
			first_seen, last_seen, last_verified, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18, $19,
			$20, $21, $22, $23,
			$24, $25, $26, $27,
			$28, $29, $30,
			$31, $32, $33, $34,
			$35, $36, $37, $38,
			$39, $40, $41, $42, $43
		)
		ON CONFLICT(id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			description = EXCLUDED.description,
			entity_status = EXCLUDED.entity_status,
			primary_classification = EXCLUDED.primary_classification,
			classification_confidence = EXCLUDED.classification_confidence,
			classification_evidence_json = EXCLUDED.classification_evidence_json,
			mcp_role = EXCLUDED.mcp_role,
			mcp_identity_status = EXCLUDED.mcp_identity_status,
			mcp_identity_evidence_json = EXCLUDED.mcp_identity_evidence_json,
			mcp_identity_confidence = EXCLUDED.mcp_identity_confidence,
			mcp_role_secondary = EXCLUDED.mcp_role_secondary,
			static_checked_at = EXCLUDED.static_checked_at,
			runtime_verified_at = EXCLUDED.runtime_verified_at,
			taiwan_score = EXCLUDED.taiwan_score,
			taiwan_level = EXCLUDED.taiwan_level,
			taiwan_evidence_json = EXCLUDED.taiwan_evidence_json,
			taiwan_confidence = EXCLUDED.taiwan_confidence,
			ai_score = EXCLUDED.ai_score,
			ai_level = EXCLUDED.ai_level,
			ai_evidence_json = EXCLUDED.ai_evidence_json,
			ai_confidence = EXCLUDED.ai_confidence,
			quality_score = EXCLUDED.quality_score,
			quality_grade = EXCLUDED.quality_grade,
			quality_components_json = EXCLUDED.quality_components_json,
			quality_evidence_json = EXCLUDED.quality_evidence_json,
			security_status = EXCLUDED.security_status,
			security_findings_json = EXCLUDED.security_findings_json,
			security_scanned_at = EXCLUDED.security_scanned_at,
			repository_json = EXCLUDED.repository_json,
			endpoints_json = EXCLUDED.endpoints_json,
			tools_json = EXCLUDED.tools_json,
			resources_json = EXCLUDED.resources_json,
			data_sources_json = EXCLUDED.data_sources_json,
			sources_json = EXCLUDED.sources_json,
			raw_content = EXCLUDED.raw_content,
			runtime_verification_json = EXCLUDED.runtime_verification_json,
			last_seen = EXCLUDED.last_seen,
			last_verified = EXCLUDED.last_verified,
			updated_at = EXCLUDED.updated_at
		`,
		entity.ID, entity.Name, entity.Slug, entity.Description, entity.EntityStatus,
		entity.Classification.Primary, entity.Classification.Confidence, classificationEvidenceJSON, entity.Classification.MCPRole,
		entity.MCPIdentity.Status, mcpIdentityEvidenceJSON, entity.MCPIdentity.Confidence,
		secondaryRolesJSON, staticCheckedAtStr, runtimeVerifiedAtStr,
		entity.TaiwanRelevance.Score, entity.TaiwanRelevance.Level, taiwanEvidenceJSON, entity.TaiwanRelevance.Confidence,
		entity.AIRelevance.Score, entity.AIRelevance.Level, aiEvidenceJSON, entity.AIRelevance.Confidence,
		entity.Quality.Score, entity.Quality.Grade, qualityComponentsJSON, qualityEvidenceStr,
		entity.SecurityStatus.Status, securityFindingsJSON, scannedAt,
		repoJSON, endpointsJSON, toolsJSON, resourcesJSON,
		dataSourcesJSON, sourcesJSON, entity.RawContent, runtimeVerificationJSON,
		entity.FirstSeen.String(), entity.LastSeen.String(), lastVerifiedStr, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert entity %s: %w", entity.ID, err)
	}
	return nil
}

// Get retrieves a single entity by ID. Returns nil, nil if not found.
func (s *EntityStore) Get(ctx context.Context, id string) (*models.Entity, error) {
	query := `
		SELECT id, name, slug, description, entity_status,
			primary_classification, classification_confidence, classification_evidence_json, mcp_role,
			mcp_identity_status, mcp_identity_evidence_json, mcp_identity_confidence,
			mcp_role_secondary, static_checked_at, runtime_verified_at,
			taiwan_score, taiwan_level, taiwan_evidence_json, taiwan_confidence,
			ai_score, ai_level, ai_evidence_json, ai_confidence,
			quality_score, quality_grade, quality_components_json, quality_evidence_json,
			security_status, security_findings_json, security_scanned_at,
			repository_json, endpoints_json, tools_json, resources_json,
			data_sources_json, sources_json, raw_content, runtime_verification_json,
			first_seen, last_seen, last_verified
		FROM entities WHERE id = $1
	`

	var entity models.Entity
	var (
		classificationEvidenceJSON     []byte
		mcpIdentityEvidenceJSON        []byte
		secondaryRolesJSON             []byte
		taiwanEvidenceJSON             []byte
		aiEvidenceJSON                 []byte
		qualityComponentsJSON          []byte
		qualityEvidenceJSON            sql.NullString
		securityFindingsJSON           []byte
		repoJSON                       []byte
		endpointsJSON                  []byte
		toolsJSON                      []byte
		resourcesJSON                  []byte
		dataSourcesJSON                []byte
		sourcesJSON                    []byte
		runtimeVerificationJSON        sql.NullString
		staticCheckedAtStr             sql.NullString
		runtimeVerifiedAtStr           sql.NullString
		lastVerifiedStr                sql.NullString
		firstSeen, lastSeen            string
		securityScannedAt              string
	)
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&entity.ID, &entity.Name, &entity.Slug, &entity.Description, &entity.EntityStatus,
		&entity.Classification.Primary, &entity.Classification.Confidence, &classificationEvidenceJSON, &entity.Classification.MCPRole,
		&entity.MCPIdentity.Status, &mcpIdentityEvidenceJSON, &entity.MCPIdentity.Confidence,
		&secondaryRolesJSON, &staticCheckedAtStr, &runtimeVerifiedAtStr,
		&entity.TaiwanRelevance.Score, &entity.TaiwanRelevance.Level, &taiwanEvidenceJSON, &entity.TaiwanRelevance.Confidence,
		&entity.AIRelevance.Score, &entity.AIRelevance.Level, &aiEvidenceJSON, &entity.AIRelevance.Confidence,
		&entity.Quality.Score, &entity.Quality.Grade, &qualityComponentsJSON, &qualityEvidenceJSON,
		&entity.SecurityStatus.Status, &securityFindingsJSON, &securityScannedAt,
		&repoJSON, &endpointsJSON, &toolsJSON, &resourcesJSON,
		&dataSourcesJSON, &sourcesJSON, &entity.RawContent, &runtimeVerificationJSON,
		&firstSeen, &lastSeen, &lastVerifiedStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get entity %s: %w", id, err)
	}

	unmarshalEntityJSON(&entity, classificationEvidenceJSON, mcpIdentityEvidenceJSON,
		secondaryRolesJSON, taiwanEvidenceJSON, aiEvidenceJSON, qualityComponentsJSON,
		qualityEvidenceJSON, securityFindingsJSON, repoJSON, endpointsJSON, toolsJSON,
		resourcesJSON, dataSourcesJSON, sourcesJSON, runtimeVerificationJSON)

	parseEntityTimes(&entity, firstSeen, lastSeen, securityScannedAt,
		staticCheckedAtStr, runtimeVerifiedAtStr, lastVerifiedStr)

	return &entity, nil
}

// List returns entities matching the given filter.
func (s *EntityStore) List(ctx context.Context, filter EntityFilter) ([]*models.Entity, error) {
	query := strings.Builder{}
	query.WriteString(`SELECT id, name, slug, description, entity_status,
		primary_classification, classification_confidence, classification_evidence_json, mcp_role,
		mcp_identity_status, mcp_identity_evidence_json, mcp_identity_confidence,
		mcp_role_secondary, static_checked_at, runtime_verified_at,
		taiwan_score, taiwan_level, taiwan_evidence_json, taiwan_confidence,
		ai_score, ai_level, ai_evidence_json, ai_confidence,
		quality_score, quality_grade, quality_components_json, quality_evidence_json,
		security_status, security_findings_json, security_scanned_at,
		repository_json, endpoints_json, tools_json, resources_json,
		data_sources_json, sources_json, raw_content, runtime_verification_json,
		first_seen, last_seen, last_verified
	FROM entities WHERE 1=1`)
	var args []any
	idx := 1

	addClause := func(cond string, val any) {
		query.WriteString(fmt.Sprintf(" AND %s = $%d", cond, idx))
		args = append(args, val)
		idx++
	}

	if filter.EntityStatus != "" {
		addClause("entity_status", filter.EntityStatus)
	}
	if filter.PrimaryClassification != "" {
		addClause("primary_classification", filter.PrimaryClassification)
	}
	if filter.MCPIdentityStatus != "" {
		addClause("mcp_identity_status", filter.MCPIdentityStatus)
	}
	if filter.TaiwanLevel != "" {
		addClause("taiwan_level", string(filter.TaiwanLevel))
	}
	if filter.MinTaiwanLevel != "" {
		// Spec §60 views filter to T1+ Taiwan relevant. We OR over
		// the allowed T levels because the JSON column is a TEXT
		// and we don't want to depend on schema-level CHECK
		// constraints or per-level columns.
		or := []string{}
		for _, lvl := range []models.TaiwanRelevanceLevel{
			models.TaiwanRelevanceLevelT1, models.TaiwanRelevanceLevelT2,
			models.TaiwanRelevanceLevelT3, models.TaiwanRelevanceLevelT4,
			models.TaiwanRelevanceLevelT5,
		} {
			if string(lvl) >= string(filter.MinTaiwanLevel) {
				or = append(or, fmt.Sprintf("taiwan_level = $%d", idx))
				args = append(args, string(lvl))
				idx++
			}
		}
		if len(or) > 0 {
			query.WriteString(" AND (")
			query.WriteString(strings.Join(or, " OR "))
			query.WriteString(")")
		}
	}
	if filter.SecurityStatus != "" {
		addClause("security_status", string(filter.SecurityStatus))
	}
	if filter.QualityMin > 0 {
		query.WriteString(fmt.Sprintf(" AND quality_score >= $%d", idx))
		args = append(args, filter.QualityMin)
		idx++
	}

	query.WriteString(" ORDER BY quality_score DESC, id ASC")
	if filter.Limit > 0 {
		query.WriteString(fmt.Sprintf(" LIMIT $%d", idx))
		args = append(args, filter.Limit)
		idx++
	}
	if filter.Offset > 0 {
		if filter.Limit == 0 {
			// SQLite needs LIMIT when using OFFSET
			query.WriteString(fmt.Sprintf(" LIMIT -1 OFFSET $%d", idx))
		} else {
			query.WriteString(fmt.Sprintf(" OFFSET $%d", idx))
		}
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	var entities []*models.Entity
	for rows.Next() {
		entity, err := scanEntityRows(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return entities, nil
}

// Count returns the number of entities matching the filter.
func (s *EntityStore) Count(ctx context.Context, filter EntityFilter) (int, error) {
	query := "SELECT COUNT(*) FROM entities WHERE 1=1"
	var args []any
	idx := 1

	addClause := func(cond string, val any) {
		query += fmt.Sprintf(" AND %s = $%d", cond, idx)
		args = append(args, val)
		idx++
	}

	if filter.EntityStatus != "" {
		addClause("entity_status", filter.EntityStatus)
	}
	if filter.PrimaryClassification != "" {
		addClause("primary_classification", filter.PrimaryClassification)
	}
	if filter.MCPIdentityStatus != "" {
		addClause("mcp_identity_status", filter.MCPIdentityStatus)
	}
	if filter.TaiwanLevel != "" {
		addClause("taiwan_level", string(filter.TaiwanLevel))
	}
	if filter.SecurityStatus != "" {
		addClause("security_status", string(filter.SecurityStatus))
	}
	if filter.QualityMin > 0 {
		query += fmt.Sprintf(" AND quality_score >= $%d", idx)
		args = append(args, filter.QualityMin)
		idx++
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count entities: %w", err)
	}
	return count, nil
}

// Delete removes an entity by ID.
func (s *EntityStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM entities WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete entity %s: %w", id, err)
	}
	return nil
}

// Update modifies an existing entity.
func (s *EntityStore) Update(ctx context.Context, entity *models.Entity) error {
	return s.Save(ctx, entity)
}

// scanEntityRows scans a single row into an Entity.
func scanEntityRows(rows *sql.Rows) (*models.Entity, error) {
	var entity models.Entity
	var (
		classificationEvidenceJSON     []byte
		mcpIdentityEvidenceJSON        []byte
		secondaryRolesJSON             []byte
		taiwanEvidenceJSON             []byte
		aiEvidenceJSON                 []byte
		qualityComponentsJSON          []byte
		qualityEvidenceJSON            sql.NullString
		securityFindingsJSON           []byte
		repoJSON                       []byte
		endpointsJSON                  []byte
		toolsJSON                      []byte
		resourcesJSON                  []byte
		dataSourcesJSON                []byte
		sourcesJSON                    []byte
		runtimeVerificationJSON        sql.NullString
		staticCheckedAtStr             sql.NullString
		runtimeVerifiedAtStr           sql.NullString
		lastVerifiedStr                sql.NullString
		firstSeen, lastSeen            string
		securityScannedAt              string
	)
	err := rows.Scan(
		&entity.ID, &entity.Name, &entity.Slug, &entity.Description, &entity.EntityStatus,
		&entity.Classification.Primary, &entity.Classification.Confidence, &classificationEvidenceJSON, &entity.Classification.MCPRole,
		&entity.MCPIdentity.Status, &mcpIdentityEvidenceJSON, &entity.MCPIdentity.Confidence,
		&secondaryRolesJSON, &staticCheckedAtStr, &runtimeVerifiedAtStr,
		&entity.TaiwanRelevance.Score, &entity.TaiwanRelevance.Level, &taiwanEvidenceJSON, &entity.TaiwanRelevance.Confidence,
		&entity.AIRelevance.Score, &entity.AIRelevance.Level, &aiEvidenceJSON, &entity.AIRelevance.Confidence,
		&entity.Quality.Score, &entity.Quality.Grade, &qualityComponentsJSON, &qualityEvidenceJSON,
		&entity.SecurityStatus.Status, &securityFindingsJSON, &securityScannedAt,
		&repoJSON, &endpointsJSON, &toolsJSON, &resourcesJSON,
		&dataSourcesJSON, &sourcesJSON, &entity.RawContent, &runtimeVerificationJSON,
		&firstSeen, &lastSeen, &lastVerifiedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scan entity: %w", err)
	}

	unmarshalEntityJSON(&entity, classificationEvidenceJSON, mcpIdentityEvidenceJSON,
		secondaryRolesJSON, taiwanEvidenceJSON, aiEvidenceJSON, qualityComponentsJSON,
		qualityEvidenceJSON, securityFindingsJSON, repoJSON, endpointsJSON, toolsJSON,
		resourcesJSON, dataSourcesJSON, sourcesJSON, runtimeVerificationJSON)

	parseEntityTimes(&entity, firstSeen, lastSeen, securityScannedAt,
		staticCheckedAtStr, runtimeVerifiedAtStr, lastVerifiedStr)

	return &entity, nil
}

// unmarshalEntityJSON populates entity fields from JSON byte slices.
func unmarshalEntityJSON(
	entity *models.Entity,
	classificationEvidenceJSON, mcpIdentityEvidenceJSON,
	secondaryRolesJSON, taiwanEvidenceJSON, aiEvidenceJSON, qualityComponentsJSON []byte,
	qualityEvidenceJSON sql.NullString,
	securityFindingsJSON, repoJSON, endpointsJSON, toolsJSON,
	resourcesJSON, dataSourcesJSON, sourcesJSON []byte,
	runtimeVerificationJSON sql.NullString,
) {
	_ = json.Unmarshal(classificationEvidenceJSON, &entity.Classification.Evidence)
	_ = json.Unmarshal(mcpIdentityEvidenceJSON, &entity.MCPIdentity.Evidence)
	_ = json.Unmarshal(secondaryRolesJSON, &entity.MCPIdentity.SecondaryRoles)
	_ = json.Unmarshal(taiwanEvidenceJSON, &entity.TaiwanRelevance.Evidence)
	_ = json.Unmarshal(aiEvidenceJSON, &entity.AIRelevance.Evidence)
	_ = json.Unmarshal(qualityComponentsJSON, &entity.Quality.Components)
	if qualityEvidenceJSON.Valid {
		_ = json.Unmarshal([]byte(qualityEvidenceJSON.String), &entity.Quality.Evidence)
	}
	_ = json.Unmarshal(securityFindingsJSON, &entity.SecurityStatus.Findings)
	_ = json.Unmarshal(repoJSON, &entity.Repository)
	_ = json.Unmarshal(endpointsJSON, &entity.Endpoints)
	_ = json.Unmarshal(toolsJSON, &entity.Tools)
	_ = json.Unmarshal(resourcesJSON, &entity.Resources)
	_ = json.Unmarshal(dataSourcesJSON, &entity.DataSources)
	_ = json.Unmarshal(sourcesJSON, &entity.Sources)
	if runtimeVerificationJSON.Valid {
		var rv models.RuntimeVerification
		_ = json.Unmarshal([]byte(runtimeVerificationJSON.String), &rv)
		entity.RuntimeVerification = &rv
	}
}

// parseEntityTimes converts time string fields into model types.
func parseEntityTimes(
	entity *models.Entity,
	firstSeen, lastSeen, securityScannedAt string,
	staticCheckedAt, runtimeVerifiedAt, lastVerified sql.NullString,
) {
	if firstSeen != "" {
		if t, err := time.Parse(time.RFC3339, firstSeen); err == nil {
			entity.FirstSeen = models.RFC3339Time(t)
		}
	}
	if lastSeen != "" {
		if t, err := time.Parse(time.RFC3339, lastSeen); err == nil {
			entity.LastSeen = models.RFC3339Time(t)
		}
	}
	if securityScannedAt != "" {
		if t, err := time.Parse(time.RFC3339, securityScannedAt); err == nil {
			entity.SecurityStatus.ScannedAt = models.RFC3339Time(t)
		}
	}
	if staticCheckedAt.Valid {
		if t, err := time.Parse(time.RFC3339, staticCheckedAt.String); err == nil {
			st := models.RFC3339Time(t)
			entity.MCPIdentity.StaticCheckedAt = &st
		}
	}
	if runtimeVerifiedAt.Valid {
		if t, err := time.Parse(time.RFC3339, runtimeVerifiedAt.String); err == nil {
			rt := models.RFC3339Time(t)
			entity.MCPIdentity.RuntimeVerifiedAt = &rt
		}
	}
	if lastVerified.Valid {
		if t, err := time.Parse(time.RFC3339, lastVerified.String); err == nil {
			lv := models.RFC3339Time(t)
			entity.LastVerified = &lv
		}
	}
}
