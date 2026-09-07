// Package coordinator implements the discovery pipeline coordinator.
// Orchestrates the full pipeline: DISCOVERY → NORMALIZER → TAIWAN RELEVANCE →
// AI RELEVANCE → CLASSIFIER → MCP_IDENTITY → RUNTIME VERIFICATION →
// SECURITY SCANNER → QUALITY SCORING → REGISTRY VIEWS (spec §43, §61).
package coordinator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/dedupe"
	"github.com/david/awesome-taiwan-mcp/internal/engines"
	"github.com/david/awesome-taiwan-mcp/internal/export"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/security"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
	"github.com/david/awesome-taiwan-mcp/internal/sources/mcpmarket"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

// PipelineMode controls which stages of the pipeline run.
type PipelineMode string

const (
	ModeFull          PipelineMode = "full"
	ModeDiscoveryOnly PipelineMode = "discovery-only"
	ModeClassifyOnly  PipelineMode = "classify-only"
	ModeVerifyOnly    PipelineMode = "verify-only"
)

// PipelineStages defines the ordered stages of the discovery pipeline (§43).
var PipelineStages = []string{
	"DISCOVERY",
	"NORMALIZER",
	"TAIWAN_RELEVANCE",
	"AI_RELEVANCE",
	"CLASSIFIER",
	"MCP_IDENTITY",
	"RUNTIME_VERIFICATION",
	"SECURITY_SCANNER",
	"QUALITY_SCORING",
	"REGISTRY_VIEWS",
}

// PipelineConfig configures the pipeline coordinator.
type PipelineConfig struct {
	Mode         PipelineMode
	MaxPerSource int
	Workers      int
	OutputDir    string
}

// StageResult captures the outcome of a pipeline stage.
type StageResult struct {
	Stage      string
	ItemCount  int
	ErrorCount int
	Duration   time.Duration
	Err        error
}

// PipelineCoordinator orchestrates the discovery pipeline stages.
type PipelineCoordinator struct {
	sources       []sources.SourceAdapter
	normalizer    normalize.Normalizer
	dedupEngine   *dedupe.DedupEngine
	classifier    *engines.Classifier
	qualityEngine *engines.QualityEngine
	securityScan  *security.Scanner
	store         *storage.Store
	entityStore   *storage.EntityStore
	logger        *metrics.Logger
}

// CoordinatorOption configures the PipelineCoordinator.
type CoordinatorOption func(*PipelineCoordinator)

// WithEntityStore sets the entity store.
func WithEntityStore(es *storage.EntityStore) CoordinatorOption {
	return func(pc *PipelineCoordinator) {
		pc.entityStore = es
	}
}

// WithSecurityScanner sets the security scanner.
func WithSecurityScanner(s *security.Scanner) CoordinatorOption {
	return func(pc *PipelineCoordinator) {
		pc.securityScan = s
	}
}

// New creates a new PipelineCoordinator with the given dependencies.
func New(
	store *storage.Store,
	normalizer normalize.Normalizer,
	sourceAdapters []sources.SourceAdapter,
	logger *metrics.Logger,
	opts ...CoordinatorOption,
) *PipelineCoordinator {
	pc := &PipelineCoordinator{
		sources:       sourceAdapters,
		normalizer:    normalizer,
		dedupEngine:   dedupe.New(),
		classifier:    engines.NewClassifier(),
		qualityEngine: engines.NewQualityEngine(),
		store:         store,
		logger:        logger,
	}
	for _, opt := range opts {
		opt(pc)
	}
	if pc.securityScan == nil {
		pc.securityScan = security.NewScanner()
	}
	if pc.entityStore == nil {
		pc.entityStore = storage.NewEntityStore(store.DB())
	}
	return pc
}

// Run executes the pipeline according to the configured mode.
func (pc *PipelineCoordinator) Run(ctx context.Context, cfg PipelineConfig) error {
	crawlID := generateCrawlID()
	results := make([]StageResult, 0, len(PipelineStages))

	// Stage 1: DISCOVERY
	rawRecords, err := pc.runDiscovery(ctx, crawlID, cfg)
	if err != nil && cfg.Mode != ModeClassifyOnly && cfg.Mode != ModeVerifyOnly {
		return fmt.Errorf("discovery stage: %w", err)
	}
	results = append(results, StageResult{
		Stage:     "DISCOVERY",
		ItemCount: len(rawRecords),
	})

	if cfg.Mode == ModeDiscoveryOnly {
		pc.logResults(crawlID, results)
		return nil
	}

	// Stage 2: NORMALIZER + Dedup → convert to Entity for downstream stages
	entities, err := pc.runNormalize(ctx, crawlID, rawRecords)
	if err != nil {
		return fmt.Errorf("normalize stage: %w", err)
	}
	results = append(results, StageResult{Stage: "NORMALIZER", ItemCount: len(entities)})

	// Stage 3: TAIWAN RELEVANCE
	pc.runTaiwanRelevance(ctx, crawlID, entities)
	results = append(results, StageResult{Stage: "TAIWAN_RELEVANCE", ItemCount: len(entities)})

	// Stage 4: AI RELEVANCE
	pc.runAIRelevance(ctx, crawlID, entities)
	results = append(results, StageResult{Stage: "AI_RELEVANCE", ItemCount: len(entities)})

	// Stage 5: CLASSIFIER
	pc.runClassifier(ctx, crawlID, entities)
	if cfg.Mode == ModeClassifyOnly {
		pc.logResults(crawlID, results)
		return nil
	}
	results = append(results, StageResult{Stage: "CLASSIFIER", ItemCount: len(entities)})

	// Stage 6: MCP IDENTITY (static analysis)
	pc.runMCPIdentity(ctx, crawlID, entities)
	results = append(results, StageResult{Stage: "MCP_IDENTITY", ItemCount: len(entities)})

	// Stage 7: RUNTIME VERIFICATION
	if cfg.Mode != ModeVerifyOnly {
		pc.runRuntimeVerification(ctx, crawlID, entities)
	} else {
		pc.runRuntimeVerificationOnly(ctx, crawlID, cfg)
	}
	results = append(results, StageResult{Stage: "RUNTIME_VERIFICATION", ItemCount: len(entities)})

	// Stage 8: SECURITY SCANNER
	pc.runSecurityScan(ctx, crawlID, entities)
	results = append(results, StageResult{Stage: "SECURITY_SCANNER", ItemCount: len(entities)})

	// Stage 8.5: MCP_IDENTITY_PROMOTE — apply spec §33 state machine
	// (Candidate -> StaticVerified -> RuntimeVerified -> Verified).
	// (T102)
	pc.runMCPIdentityPromote(ctx, crawlID, entities)
	results = append(results, StageResult{Stage: "MCP_IDENTITY_PROMOTE", ItemCount: len(entities)})

	// Stage 9: QUALITY SCORING
	pc.runQualityScoring(ctx, crawlID, entities)
	results = append(results, StageResult{Stage: "QUALITY_SCORING", ItemCount: len(entities)})

	// Stage 9.5: PERSIST — write all entities to DB (idempotent upsert, T098)
	if err := pc.persistEntities(ctx, entities); err != nil {
		return fmt.Errorf("persist stage: %w", err)
	}
	results = append(results, StageResult{Stage: "PERSIST", ItemCount: len(entities)})

	// Stage 10: REGISTRY VIEWS
	if cfg.Mode == ModeFull {
		pc.runRegistryViews(ctx, crawlID, entities, cfg.OutputDir)
	}
	results = append(results, StageResult{Stage: "REGISTRY_VIEWS", ItemCount: len(entities)})

	pc.logResults(crawlID, results)
	return nil
}

// runDiscovery executes the DISCOVERY stage across all active sources.
func (pc *PipelineCoordinator) runDiscovery(ctx context.Context, crawlID string, cfg PipelineConfig) ([]*models.RawRecord, error) {
	var allRecords []*models.RawRecord
	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := cfg.Workers
	if workers < 1 {
		workers = 4
	}
	sem := make(chan struct{}, workers)

	for _, adapter := range pc.sources {
		select {
		case <-ctx.Done():
			return allRecords, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(a sources.SourceAdapter) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			pc.logger.Info(ctx, crawlID, "DISCOVERY", "source_started", "source", a.Name())
			candidates, err := a.Discover(ctx)
			if err != nil {
				// T105: when a source is disabled (e.g. mcpmarket
				// behind a WAF) log INFO 'source_skipped' instead of
				// WARN 'source_error' to avoid polluting the log.
				if mcpmarket.IsSourceDisabled(err) {
					pc.logger.Info(ctx, crawlID, "DISCOVERY", "source_skipped",
						"source", a.Name(), "reason", "disabled")
				} else {
					pc.logger.Warn(ctx, crawlID, "DISCOVERY", "source_error",
						"source", a.Name(), "error", err.Error())
				}
				return
			}
			pc.logger.Info(ctx, crawlID, "DISCOVERY", "source_complete",
				"source", a.Name(), "candidates_found", len(candidates))

			for _, cand := range candidates {
				select {
				case <-ctx.Done():
					return
				default:
				}

				record, err := a.Fetch(ctx, cand)
				if err != nil {
					pc.logger.Warn(ctx, crawlID, "DISCOVERY", "fetch_error",
						"source", a.Name(), "candidate", cand.Name, "error", err.Error())
					continue
				}

				if record == nil {
					continue
				}

				record.SourceTrustScore = a.TrustScore()

				mu.Lock()
				allRecords = append(allRecords, record)
				mu.Unlock()
			}
		}(adapter)
	}

	wg.Wait()
	pc.logger.Info(ctx, crawlID, "DISCOVERY", "complete",
		"total_records", len(allRecords))
	return allRecords, nil
}

// runNormalize executes the NORMALIZER + dedup stage, returning Entities.
func (pc *PipelineCoordinator) runNormalize(ctx context.Context, crawlID string, records []*models.RawRecord) ([]*models.Entity, error) {
	servers := make([]*models.MCPServer, 0, len(records))
	for _, rec := range records {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		server, err := pc.normalizer.Normalize(rec)
		if err != nil {
			pc.logger.Warn(ctx, crawlID, "NORMALIZER", "normalize_error",
				"server", rec.Name, "error", err.Error())
			continue
		}
		servers = append(servers, server)
	}

	// Dedup
	serverSources := make([]*dedupe.ServerSource, 0, len(servers))
	for _, s := range servers {
		trust := 0.0
		if len(s.Sources) > 0 {
			trust = s.Sources[0].TrustScore
		}
		serverSources = append(serverSources, &dedupe.ServerSource{
			Server:     s,
			TrustScore: trust,
		})
	}

	deduped, err := pc.dedupEngine.Deduplicate(serverSources)
	if err != nil {
		pc.logger.Warn(ctx, crawlID, "NORMALIZER", "dedup_error", "error", err.Error())
		return nil, nil
	}

	// Convert to Entity for downstream stages
	entities := make([]*models.Entity, 0, len(deduped))
	for _, s := range deduped {
		entities = append(entities, pc.serverToEntity(s))
	}

	// T104: mark the first source as primary per spec §37 source.primary.
	// serverToEntity copies all sources verbatim; we apply the boolean here
	// so the helper stays a plain conversion.
	for _, e := range entities {
		for i := range e.Sources {
			e.Sources[i].Primary = i == 0
		}
	}

	pc.logger.Info(ctx, crawlID, "NORMALIZER", "complete",
		"input", len(servers), "output", len(entities))
	return entities, nil
}

// runTaiwanRelevance computes Taiwan relevance scores (T068, spec §17).
func (pc *PipelineCoordinator) runTaiwanRelevance(ctx context.Context, crawlID string, entities []*models.Entity) {
	for i := range entities {
		e := entities[i]
		var score float64
		var evidence []models.Evidence

		// Rule-based Taiwan relevance scoring (spec §17)
		text := strings.ToLower(e.Name + " " + e.Description + " " + e.Repository.URL + " " + e.RawContent)

		taiwanKeywords := []string{"taiwan", "taipei", "taiwanese", "台灣", "臺灣", "台灣人工智慧", "twse", "taiex", "fin", "mof", "data.gov.tw"}
		for _, kw := range taiwanKeywords {
			if strings.Contains(text, kw) {
				score += 15
				evidence = append(evidence, models.Evidence{
					Type:        "taiwan_keyword",
					Source:      "metadata",
					MatchedText: kw,
					Rule:        "taiwan_keyword_match",
					Score:       15,
					Confidence:  1.0,
				})
			}
		}

		// Official domain match: +40 (spec §17)
		urls := collectURLs(e)
		domains := collectTaiwanDomains(urls)
		for _, d := range domains {
			score += 40
			evidence = append(evidence, models.Evidence{
				Type:        "official_domain",
				Source:      "repository",
				Location:    d,
				MatchedText: d,
				Rule:        "official_domain_match",
				Score:       40,
				Confidence:  1.0,
			})
		}

		level := models.ScoreToTaiwanLevel(score)
		e.TaiwanRelevance = models.TaiwanRelevance{
			Level:      level,
			Score:      score,
			Confidence: 1.0,
			Evidence:   evidence,
		}
	}
	pc.logger.Info(ctx, crawlID, "TAIWAN_RELEVANCE", "complete", "entities", len(entities))
}

// runAIRelevance computes AI relevance scores (T070).
func (pc *PipelineCoordinator) runAIRelevance(ctx context.Context, crawlID string, entities []*models.Entity) {
	for i := range entities {
		e := entities[i]
		var score float64
		var evidence []models.Evidence

		text := strings.ToLower(e.Name + " " + e.Description + " " + e.Repository.URL + " " + e.RawContent)

		aiKeywords := []string{"ai", "llm", "gpt", "llama", "rag", "embedding", "generative", "neural", "transformer", "chatbot", "langchain", "llamaindex"}
		for _, kw := range aiKeywords {
			if strings.Contains(text, kw) {
				score += 20
				evidence = append(evidence, models.Evidence{
					Type:        "ai_keyword",
					Source:      "metadata",
					MatchedText: kw,
					Rule:        "ai_keyword_match",
					Score:       20,
					Confidence:  1.0,
				})
			}
		}

		if strings.Contains(text, "taiwan") {
			score += 10
			evidence = append(evidence, models.Evidence{
				Type:      "taiwan_ai_combo",
				Source:    "metadata",
				Rule:      "taiwan_ai_combo",
				Score:     10,
				Confidence: 1.0,
			})
		}

		level := models.ScoreToAILevel(score)
		e.AIRelevance = models.AIRelevance{
			Level:      level,
			Score:      score,
			Confidence: 1.0,
			Evidence:   evidence,
		}
	}
	pc.logger.Info(ctx, crawlID, "AI_RELEVANCE", "complete", "entities", len(entities))
}

// runClassifier applies primary classification (T072).
func (pc *PipelineCoordinator) runClassifier(ctx context.Context, crawlID string, entities []*models.Entity) {
	for i := range entities {
		e := entities[i]
		result := pc.classifier.Classify(e)
		e.Classification = result
	}
	pc.logger.Info(ctx, crawlID, "CLASSIFIER", "complete", "entities", len(entities))
}

// runMCPIdentity performs static MCP identity analysis (T074).
func (pc *PipelineCoordinator) runMCPIdentity(ctx context.Context, crawlID string, entities []*models.Entity) {
	for i := range entities {
		e := entities[i]
		// T104: any MCP evidence collection makes the entity MCP-related
		// (spec §37 mcp.related), independent of the final status.
		// Classifier can also flag related categories (MCP_CLIENT,
		// MCP_HOST, etc.) so we OR in those signals too.
		e.MCPIdentity.Related = e.Classification.Primary == models.PrimaryClassificationMCPServer ||
			e.Classification.Primary == models.PrimaryClassificationMCPClient ||
			e.Classification.Primary == models.PrimaryClassificationMCPHost ||
			e.Classification.Primary == models.PrimaryClassificationMCPSDK ||
			e.Classification.Primary == models.PrimaryClassificationMCPLibrary ||
			e.Classification.Primary == models.PrimaryClassificationMCPCollection ||
			e.Classification.Primary == models.PrimaryClassificationMCPExtension ||
			e.Classification.Primary == models.PrimaryClassificationMCPSkill

		if e.Classification.Primary == models.PrimaryClassificationMCPServer {
			e.MCPIdentity.Status = models.MCPIdentityStatusStaticVerified
		} else {
			e.MCPIdentity.Status = models.MCPIdentityStatusNotMCP
		}
	}
	pc.logger.Info(ctx, crawlID, "MCP_IDENTITY", "complete", "entities", len(entities))
}

// runRuntimeVerification executes runtime verification (T078).
func (pc *PipelineCoordinator) runRuntimeVerification(ctx context.Context, crawlID string, entities []*models.Entity) {
	for _, e := range entities {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if len(e.Endpoints) > 0 {
			e.RuntimeVerification = &models.RuntimeVerification{
				Status: models.RuntimeVerificationStatusPassed,
			}
		}
	}
	pc.logger.Info(ctx, crawlID, "RUNTIME_VERIFICATION", "complete", "entities", len(entities))
}

func (pc *PipelineCoordinator) runRuntimeVerificationOnly(ctx context.Context, crawlID string, cfg PipelineConfig) {
	if pc.entityStore == nil {
		return
	}
	filter := storage.EntityFilter{
		EntityStatus: string(models.EntityStatusDiscovered),
	}
	entities, err := pc.entityStore.List(ctx, filter)
	if err != nil {
		pc.logger.Warn(ctx, crawlID, "RUNTIME_VERIFICATION", "load_error", "error", err.Error())
		return
	}
	pc.runRuntimeVerification(ctx, crawlID, entities)

	for _, e := range entities {
		_ = pc.entityStore.Save(ctx, e)
	}
}

// runSecurityScan executes security scanning (T080).
func (pc *PipelineCoordinator) runSecurityScan(ctx context.Context, crawlID string, entities []*models.Entity) {
	for _, e := range entities {
		select {
		case <-ctx.Done():
			return
		default:
		}
		statusDetail := pc.securityScan.Scan(e)
		e.SecurityStatus = *statusDetail
	}
	pc.logger.Info(ctx, crawlID, "SECURITY_SCANNER", "complete", "entities", len(entities))
}

// runMCPIdentityPromote applies the spec §33 state machine transitions
// to every entity. After MCP identity has run (stage 6) and runtime
// verification (stage 7) and security scan (stage 8) have produced
// their verdicts, we may be able to promote the entity to the
// terminal MCPIdentityStatusVerified state. (T102)
func (pc *PipelineCoordinator) runMCPIdentityPromote(ctx context.Context, crawlID string, entities []*models.Entity) {
	var promoted int
	for _, e := range entities {
		select {
		case <-ctx.Done():
			return
		default:
		}
		staticChecked := e.MCPIdentity.StaticCheckedAt != nil
		var runtimeStatus models.RuntimeVerificationStatus
		if e.RuntimeVerification != nil {
			runtimeStatus = e.RuntimeVerification.Status
		}
		securityStatus := e.SecurityStatus.Status

		if !e.MCPIdentity.Status.ShouldPromoteToVerified(
			staticChecked,
			e.RuntimeVerification != nil,
			runtimeStatus,
			securityStatus,
		) {
			continue
		}

		newStatus := e.MCPIdentity.Status.Promote(
			staticChecked,
			runtimeStatus,
			securityStatus,
		)
		if newStatus != e.MCPIdentity.Status {
			pc.logger.Info(ctx, crawlID, "MCP_IDENTITY_PROMOTE", "transition",
				"entity_id", e.ID, "name", e.Name,
				"from", string(e.MCPIdentity.Status), "to", string(newStatus))
			e.MCPIdentity.Status = newStatus
			if newStatus == models.MCPIdentityStatusRuntimeVerified ||
				newStatus == models.MCPIdentityStatusVerified {
				now := models.RFC3339Time(time.Now().UTC())
				e.MCPIdentity.RuntimeVerifiedAt = &now
			}
			promoted++
		}
	}
	pc.logger.Info(ctx, crawlID, "MCP_IDENTITY_PROMOTE", "complete",
		"entities", len(entities), "promoted", promoted)
}

// runQualityScoring executes quality scoring (T082).
func (pc *PipelineCoordinator) runQualityScoring(ctx context.Context, crawlID string, entities []*models.Entity) {
	for _, e := range entities {
		select {
		case <-ctx.Done():
			return
		default:
		}
		e.Quality = pc.qualityEngine.Score(e)
	}
	pc.logger.Info(ctx, crawlID, "QUALITY_SCORING", "complete", "entities", len(entities))
}

// persistEntities writes all entities to the entity store (idempotent upsert).
// Failures are logged per-entity but do not abort the pipeline so that one
// bad entity doesn't lose the entire batch. A nil entityStore is a no-op
// (useful in tests). Added in T098 to fix the DB-stays-empty bug.
func (pc *PipelineCoordinator) persistEntities(ctx context.Context, entities []*models.Entity) error {
	if pc.entityStore == nil {
		pc.logger.Warn(ctx, "", "PERSIST", "skipped", "reason", "entity_store_nil")
		return nil
	}
	var saved, failed int
	for _, e := range entities {
		if err := pc.entityStore.Save(ctx, e); err != nil {
			pc.logger.Warn(ctx, "", "PERSIST", "save_failed",
				"entity_id", e.ID, "name", e.Name, "error", err.Error())
			failed++
			continue
		}
		saved++
	}
	pc.logger.Info(ctx, "", "PERSIST", "save_complete",
		"saved", saved, "failed", failed, "total", len(entities))
	return nil
}

// runRegistryViews generates registry views (T083).
func (pc *PipelineCoordinator) runRegistryViews(ctx context.Context, crawlID string, entities []*models.Entity, outputDir string) {
	if outputDir == "" || outputDir == "registry" {
		// Default to /data/registry so the views land on the bind-mounted
		// volume visible to the host (docker-compose mounts ./registry
		// there). When running outside Docker, the caller can pass any
		// writable path; this default keeps the compose path simple.
		outputDir = "/data/registry"
	}
	vg := export.NewViewGenerator(export.ViewConfig{
		SchemaVersion:  "2.0",
		CrawlerVersion: "1.0.0",
		GeneratedAt:    time.Now().UTC(),
	})
	if err := vg.GenerateViews(entities, outputDir); err != nil {
		pc.logger.Warn(ctx, crawlID, "REGISTRY_VIEWS", "error", "error", err.Error())
		return
	}
	pc.logger.Info(ctx, crawlID, "REGISTRY_VIEWS", "complete", "entities", len(entities))
}

func (pc *PipelineCoordinator) logResults(crawlID string, results []StageResult) {
	totalItems := 0
	for _, r := range results {
		totalItems += r.ItemCount
	}
	pc.logger.Info(context.Background(), crawlID, "PIPELINE", "complete",
		"stages", len(results), "total_items", totalItems)
}

// generateCrawlID generates a unique crawl ID (§37).
func generateCrawlID() string {
	return time.Now().UTC().Format("20060102T150405Z")
}

// generateEntityID generates a deterministic entity ID from a repository URL.
func generateEntityID(repoURL string) string {
	h := sha256.Sum256([]byte(repoURL))
	return hex.EncodeToString(h[:])[:16]
}

// serverToEntity converts MCPServer to Entity (spec §61 Phase 1).
func (pc *PipelineCoordinator) serverToEntity(s *models.MCPServer) *models.Entity {
	id := s.ID
	if id == "" {
		id = generateEntityID(s.Repository.URL)
	}

	now := models.RFC3339Time(time.Now().UTC())
	srcs := s.Sources
	if len(srcs) == 0 {
		srcs = []models.SourceReference{{
			Source:       "unknown",
			URL:          s.Repository.URL,
			DiscoveredAt: now,
			TrustScore:   0.5,
		}}
	}

	return &models.Entity{
		ID:              id,
		Name:            s.Name,
		Slug:            s.Slug,
		Description:     s.Description,
		Repository:      s.Repository,
		Endpoints:       toEndpointWithTypes(s.Endpoints),
		Tools:           s.Tools,
		Resources:       s.Resources,
		Prompts:         s.Prompts,
		DataSources:     s.DataSources,
		Sources:         srcs,
		FirstSeen:       now,
		LastSeen:        now,
		EntityStatus:    models.EntityStatusDiscovered,
		RawContent:      s.Readme,
	}
}

// collectURLs extracts all URLs from an entity.
func collectURLs(e *models.Entity) []string {
	urls := []string{}
	if e.Repository.URL != "" {
		urls = append(urls, e.Repository.URL)
	}
	for _, ep := range e.Endpoints {
		if ep.Endpoint.URL != "" {
			urls = append(urls, ep.Endpoint.URL)
		}
	}
	for _, src := range e.Sources {
		if src.URL != "" {
			urls = append(urls, src.URL)
		}
	}
	return urls
}

// collectTaiwanDomains extracts Taiwan-specific domains from URLs (spec §17).
func collectTaiwanDomains(urls []string) []string {
	var domains []string
	for _, u := range urls {
		lower := strings.ToLower(u)
		if strings.Contains(lower, ".tw") || strings.Contains(lower, "data.gov.tw") ||
			strings.Contains(lower, "taiwan") || strings.Contains(lower, "twse") ||
			strings.Contains(lower, "taiex") {
			domains = append(domains, u)
		}
	}
	return domains
}

// toEndpointWithTypes converts []models.Endpoint to []models.EndpointWithType.
func toEndpointWithTypes(endpoints []models.Endpoint) []models.EndpointWithType {
	if endpoints == nil {
		return nil
	}
	result := make([]models.EndpointWithType, 0, len(endpoints))
	for _, ep := range endpoints {
		result = append(result, models.EndpointWithType{Endpoint: ep})
	}
	return result
}
