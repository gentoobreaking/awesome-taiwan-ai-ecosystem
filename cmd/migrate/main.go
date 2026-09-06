// Package main implements the V1→V2 migration CLI for the Taiwan MCP registry.
// Re-processes old mcp_servers records through the full classification pipeline.
// Spec §52, §61 Phase 11.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/engines"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	_ "modernc.org/sqlite"
)

// MigrationConfig holds CLI configuration for the migration tool.
type MigrationConfig struct {
	InputDB       string
	OutputDB      string
	DryRun        bool
	BatchSize     int
	VerifyRuntime bool
}

// MigrationReport holds the final migration statistics.
type MigrationReport struct {
	Total          int            `json:"total"`
	Succeeded      int            `json:"succeeded"`
	Failed         int            `json:"failed"`
	FailedIDs      []string       `json:"failed_ids"`
	Classification map[string]int `json:"classification"`
	Status         map[string]int `json:"status_distribution"`
	TaiwanLevel    map[string]int `json:"taiwan_level"`
	StartTime      string         `json:"start_time"`
	EndTime        string         `json:"end_time"`
	Duration       string         `json:"duration"`
}

// v1Row represents a row from the legacy mcp_servers table.
type v1Row struct {
	id, name, slug, desc string
	category, region     sql.NullString
	taiwanRelJSON        sql.NullString
	repoJSON             sql.NullString
	endpointsJSON        sql.NullString
	transportJSON        sql.NullString
	toolsJSON            sql.NullString
	resourcesJSON        sql.NullString
	promptsJSON          sql.NullString
	dataSourcesJSON      sql.NullString
	license              sql.NullString
	status               sql.NullString
	health               sql.NullString
	qualityJSON          sql.NullString
	firstSeen            sql.NullString
	lastSeen             sql.NullString
	lastVerified         sql.NullString
	schemaVer            sql.NullString
}

// checkpointTable stores migrated IDs for resume support.
const checkpointTable = `
CREATE TABLE IF NOT EXISTS migration_checkpoint (
	id TEXT PRIMARY KEY,
	migrated_at TEXT
)
`

func main() {
	cfg := MigrationConfig{}
	flag.StringVar(&cfg.InputDB, "input-db", "", "path to the old v1 database (required)")
	flag.StringVar(&cfg.OutputDB, "output-db", "", "path to the new v2 database (required)")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "validate without writing to output DB")
	flag.IntVar(&cfg.BatchSize, "batch-size", 100, "number of records to process per batch")
	flag.BoolVar(&cfg.VerifyRuntime, "verify-runtime", false, "run runtime verification on static-verified servers")
	flag.Parse()

	if cfg.InputDB == "" || cfg.OutputDB == "" {
		fmt.Fprintln(os.Stderr, "Error: --input-db and --output-db are required")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runMigration(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		os.Exit(1)
	}
}

// runMigration executes the full migration pipeline.
func runMigration(ctx context.Context, cfg MigrationConfig) error {
	start := time.Now()
	report := &MigrationReport{
		Classification: make(map[string]int),
		Status:         make(map[string]int),
		TaiwanLevel:    make(map[string]int),
		StartTime:      start.Format(time.RFC3339),
	}

	// Open input database (read-only)
	inDB, err := sql.Open("sqlite", cfg.InputDB)
	if err != nil {
		return fmt.Errorf("open input db: %w", err)
	}
	defer inDB.Close()

	// Open output database
	var outStore *storage.Store
	var entityStore *storage.EntityStore
	if !cfg.DryRun {
		outStore, err = storage.Open(cfg.OutputDB)
		if err != nil {
			return fmt.Errorf("open output db: %w", err)
		}
		defer outStore.Close()

		if err := outStore.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate output db: %w", err)
		}

		entityStore = storage.NewEntityStore(outStore.DB())

		// Create checkpoint table for resume support
		if _, err := outStore.DB().ExecContext(ctx, checkpointTable); err != nil {
			return fmt.Errorf("create checkpoint table: %w", err)
		}
	}

	// Initialize engines
	classifier := engines.NewClassifier()
	taiwanEng := engines.NewTaiwanRelevanceEngine()
	aiEng := engines.NewAIRelevanceEngine(nil)
	mcpIDEng := engines.NewMCPIdentityEngine()
	qualityEng := engines.NewQualityEngine()
	secScanner := engines.NewSecurityScanner()
	runtimeVerifier := engines.NewRuntimeVerifier()
	var endpointClassifier *engines.EndpointClassifier
	if cfg.VerifyRuntime {
		endpointClassifier = engines.NewEndpointClassifier()
	}

	// Load checkpoint for resume support
	var checkpointed map[string]bool
	if !cfg.DryRun {
		checkpointed = loadCheckpoint(outStore.DB(), ctx)
	}

	// Load all old server IDs from mcp_servers table
	rows, err := inDB.QueryContext(ctx, `
		SELECT id, name, slug, description, category, region, taiwan_relevance,
			repository, endpoints, transport, tools, resources, prompts,
			data_sources, license, status, health, quality, first_seen_at,
			last_seen_at, last_verified_at, schema_version
		FROM mcp_servers
	`)
	if err != nil {
		return fmt.Errorf("read mcp_servers: %w", err)
	}
	defer rows.Close()

	batch := make([]v1Row, 0, cfg.BatchSize)
	for {
		var r v1Row
		err := rows.Scan(
			&r.id, &r.name, &r.slug, &r.desc, &r.category, &r.region, &r.taiwanRelJSON,
			&r.repoJSON, &r.endpointsJSON, &r.transportJSON, &r.toolsJSON, &r.resourcesJSON,
			&r.promptsJSON, &r.dataSourcesJSON, &r.license, &r.status, &r.health,
			&r.qualityJSON, &r.firstSeen, &r.lastSeen, &r.lastVerified, &r.schemaVer,
		)
		if err != nil {
			break
		}

		// Skip already-migrated records (checkpoint/resume)
		if checkpointed != nil && checkpointed[r.id] {
			report.Succeeded++
			continue
		}

		batch = append(batch, r)
		if len(batch) >= cfg.BatchSize {
			report = processBatch(ctx, cfg, batch, report, classifier, taiwanEng, aiEng,
				mcpIDEng, qualityEng, secScanner, runtimeVerifier, endpointClassifier,
				entityStore, outStore)
			batch = batch[:0]
		}
	}
	// Process remaining
	if len(batch) > 0 {
		report = processBatch(ctx, cfg, batch, report, classifier, taiwanEng, aiEng,
			mcpIDEng, qualityEng, secScanner, runtimeVerifier, endpointClassifier,
			entityStore, outStore)
	}

	report.EndTime = time.Now().Format(time.RFC3339)
	report.Duration = time.Since(start).String()

	// Print progress and stats
	printReport(report)

	// Output JSON report
	if !cfg.DryRun {
		reportJSON, _ := json.MarshalIndent(report, "", "  ")
		reportPath := filepath.Join(filepath.Dir(cfg.OutputDB), "migration_report.json")
		if err := os.WriteFile(reportPath, reportJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not write report: %v\n", err)
		}
	}

	return nil
}

// processBatch processes a batch of V1 rows through the full pipeline.
func processBatch(
	ctx context.Context,
	cfg MigrationConfig,
	batch []v1Row,
	report *MigrationReport,
	classifier *engines.Classifier,
	taiwanEng *engines.TaiwanRelevanceEngine,
	aiEng *engines.AIRelevanceEngine,
	mcpIDEng *engines.MCPIdentityEngine,
	qualityEng *engines.QualityEngine,
	secScanner *engines.SecurityScanner,
	runtimeVerifier *engines.RuntimeVerifier,
	endpointClassifier *engines.EndpointClassifier,
	entityStore *storage.EntityStore,
	outStore *storage.Store,
) *MigrationReport {
	var mu sync.Mutex
	processed := 0

	for _, r := range batch {
		mu.Lock()
		report.Total++
		mu.Unlock()

		entity, err := convertV1ToEntity(r)
		if err != nil {
			mu.Lock()
			report.Failed++
			report.FailedIDs = append(report.FailedIDs, r.id)
			mu.Unlock()
			continue
		}

		// Clear old scores — must recompute (spec §52: not directly copy taiwan_relevant)
		entity.TaiwanRelevance = models.TaiwanRelevance{}
		entity.AIRelevance = models.AIRelevance{}
		entity.Classification = models.ClassificationResult{}
		entity.MCPIdentity = models.MCPIdentity{}
		entity.SecurityStatus = models.SecurityStatusDetail{}
		entity.Quality = models.QualityScore{}
		entity.RuntimeVerification = nil

		// Step 3: Classify (T072)
		classResult := classifier.Classify(entity)
		entity.Classification = classResult

		// Step 4: Taiwan Score (T068) — RECOMPUTED, not copied
		entity.TaiwanRelevance = taiwanEng.Score(entity)

		// Step 5: AI Score (T070) — RECOMPUTED
		entity.AIRelevance = aiEng.Score(entity)

	// Step 6: MCP Identity (T074)
	mcpResult := mcpIDEng.DetectMCPIdentity(entity)
	entity.MCPIdentity = models.MCPIdentity{
		Status:         mcpResult.Status,
		Evidence:       mcpResult.Evidence,
		Confidence:     mcpResult.Confidence,
		Role:           mcpResult.MCPRole,
		SecondaryRoles: mcpResult.SecondaryRoles,
	}

	// Step 7: Runtime Verification (T078) — optional

		if cfg.VerifyRuntime {
		if endpointClassifier != nil {
			entity.Endpoints = endpointClassifier.ClassifyEndpoints(entity)
		}
		rvResult := runtimeVerifier.Verify(ctx, entity)
		if rvResult != nil {
			entity.RuntimeVerification = runtimeResultToModel(rvResult)
		}
	}

		// Step 8: Security Scan (T080)
		scanResult := secScanner.Scan(entity)
		entity.SecurityStatus = scanResult.ToSecurityStatusDetail()

		// Step 9: Quality Score (T082)
		entity.Quality = qualityEng.Score(entity)

		// Step 10: Save to output DB
		if !cfg.DryRun && entityStore != nil {
			if err := entityStore.Save(ctx, entity); err != nil {
				mu.Lock()
				report.Failed++
				report.FailedIDs = append(report.FailedIDs, r.id)
				mu.Unlock()
				continue
			}
		}

		// Update classification stats
		mu.Lock()
		report.Succeeded++
		report.Classification[string(entity.Classification.Primary)]++
		report.Status[string(entity.EntityStatus)]++
		report.TaiwanLevel[string(entity.TaiwanRelevance.Level)]++

		// Checkpoint
		if !cfg.DryRun && outStore != nil {
			_, _ = outStore.DB().ExecContext(ctx,
				"INSERT OR REPLACE INTO migration_checkpoint (id, migrated_at) VALUES (?, ?)",
				r.id, time.Now().Format(time.RFC3339))
		}

		processed++
		if processed%100 == 0 {
			fmt.Fprintf(os.Stderr, "[migration] Processed %d records (total so far: %d)\n",
				processed, report.Total)
		}
		mu.Unlock()
	}

	return report
}

// convertV1ToEntity converts a V1 mcp_servers row into a new Entity,
// stripping old scores that must be recomputed.
func convertV1ToEntity(r v1Row) (*models.Entity, error) {
	entity := &models.Entity{
		ID:          r.id,
		Name:        r.name,
		Slug:        r.slug,
		Description: r.desc,
	}

	// Parse entity_status from legacy status
	switch r.status.String {
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

	// Parse repository JSON
	var repo models.RepositoryInfo
	if err := json.Unmarshal([]byte(nullString(r.repoJSON)), &repo); err != nil {
		repo = models.RepositoryInfo{}
	}
	if repo.License == "" && r.license.Valid {
		repo.License = r.license.String
	}
	entity.Repository = repo

	// Parse endpoints JSON
	var endpoints []models.EndpointWithType
	_ = json.Unmarshal([]byte(nullString(r.endpointsJSON)), &endpoints)
	entity.Endpoints = endpoints

	// Parse tools JSON
	var tools []models.Tool
	_ = json.Unmarshal([]byte(nullString(r.toolsJSON)), &tools)
	entity.Tools = tools

	// Parse resources JSON
	var resources []models.Resource
	_ = json.Unmarshal([]byte(nullString(r.resourcesJSON)), &resources)
	entity.Resources = resources

	// Parse prompts JSON
	var prompts []models.Prompt
	_ = json.Unmarshal([]byte(nullString(r.promptsJSON)), &prompts)
	entity.Prompts = prompts

	// Parse data sources JSON
	var dataSources []models.DataSource
	_ = json.Unmarshal([]byte(nullString(r.dataSourcesJSON)), &dataSources)
	entity.DataSources = dataSources

	// Parse timestamps
	if r.firstSeen.Valid {
		if t, err := time.Parse(time.RFC3339, r.firstSeen.String); err == nil {
			entity.FirstSeen = models.RFC3339Time(t)
		}
	}
	if r.lastSeen.Valid {
		if t, err := time.Parse(time.RFC3339, r.lastSeen.String); err == nil {
			entity.LastSeen = models.RFC3339Time(t)
		}
	}
	if r.lastVerified.Valid {
		if t, err := time.Parse(time.RFC3339, r.lastVerified.String); err == nil {
			rv := models.RFC3339Time(t)
			entity.LastVerified = &rv
		}
	}

	return entity, nil
}

// runtimeResultToModel converts engines.RuntimeVerificationResult to models.RuntimeVerification.
func runtimeResultToModel(r *engines.RuntimeVerificationResult) *models.RuntimeVerification {
	if r == nil {
		return nil
	}
	modelRV := &models.RuntimeVerification{
		Status:       models.RuntimeVerificationStatus(r.Status),
		Evidence:     r.Evidence,
		Timestamp:    r.Timestamp,
	}
	// JSON round-trip for InitializeResult and ToolsListResult (different structs)
	if r.InitializeResult != nil {
		data, _ := json.Marshal(r.InitializeResult)
		_ = json.Unmarshal(data, &modelRV.InitializeResult)
	}
	if r.ToolsListResult != nil {
		data, _ := json.Marshal(r.ToolsListResult)
		_ = json.Unmarshal(data, &modelRV.ToolsListResult)
	}
	return modelRV
}

// loadCheckpoint reads already-migrated IDs from the output database.
func loadCheckpoint(db *sql.DB, ctx context.Context) map[string]bool {
	checkpointed := make(map[string]bool)
	rows, err := db.QueryContext(ctx, "SELECT id FROM migration_checkpoint")
	if err != nil {
		return checkpointed
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		checkpointed[id] = true
	}
	return checkpointed
}

// nullString converts sql.NullString to string.
func nullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// printReport prints human-readable migration statistics.
func printReport(r *MigrationReport) {
	fmt.Println("\n=== Migration Report ===")
	fmt.Printf("Total: %s\n", strconv.Itoa(r.Total))
	fmt.Printf("Succeeded: %s\n", strconv.Itoa(r.Succeeded))
	fmt.Printf("Failed: %s\n", strconv.Itoa(r.Failed))
	fmt.Printf("Duration: %s\n", r.Duration)
	fmt.Println("\nClassification distribution:")
	for k, v := range r.Classification {
		fmt.Printf("  %s: %d\n", k, v)
	}
	fmt.Println("\nStatus distribution:")
	for k, v := range r.Status {
		fmt.Printf("  %s: %d\n", k, v)
	}
	fmt.Println("\nTaiwan relevance levels:")
	for k, v := range r.TaiwanLevel {
		fmt.Printf("  %s: %d\n", k, v)
	}
	if len(r.FailedIDs) > 0 {
		fmt.Printf("\nFailed IDs (%d):\n", len(r.FailedIDs))
		for _, id := range r.FailedIDs {
			fmt.Printf("  - %s\n", id)
		}
	}
}
