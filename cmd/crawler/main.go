package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/crawler"
	"github.com/david/awesome-taiwan-mcp/internal/coordinator"
	"github.com/david/awesome-taiwan-mcp/internal/export"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/search"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
	"github.com/david/awesome-taiwan-mcp/internal/sources/github"
	"github.com/david/awesome-taiwan-mcp/internal/sources/githubrepo"
	"github.com/david/awesome-taiwan-mcp/internal/sources/mcpmarket"
	"github.com/david/awesome-taiwan-mcp/internal/sources/mcpserversorg"
	"github.com/david/awesome-taiwan-mcp/internal/sources/registry"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Flags
	configPath      string
	dbPath          string
	sourceFlag      string
	fullCrawl       bool
	incremental     bool
	workers         int
	jsonOutput      bool
	minScore        int
	levelFilter     string
	catFilter       string
	capabilityFlag  string
	markdownExport  bool
	maliciousReport bool
	maliciousDir    string
	maliciousThreshold string
	injectionReport bool
	injectionDir    string
	maxPerSource    int
	pipelineFlag    string
	history         bool
	batchSize       int
	dryRun          bool
	verbose         bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "crawler",
		Short: "Taiwan MCP Crawler",
		Long:  "Automated crawler for discovering, analyzing, and verifying Taiwan-related MCP Servers.",
	}

	// Version
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("crawler version: %s\n", version)
			fmt.Printf("commit: %s\n", commit)
			fmt.Printf("built: %s\n", date)
		},
	})

	// Crawl
	rootCmd.AddCommand(&cobra.Command{
		Use:   "crawl",
		Short: "Run the crawler pipeline",
		RunE:  runCrawl,
	})

	// Run (full pipeline - alias for crawl with pipeline coordinator)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the full discovery pipeline",
		Long:  "Run the complete pipeline: Discovery → Normalize → Classify → Verify → Scan → Score → Export",
		RunE:  runCrawl,
	})

	// Discover (discovery-only mode)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "discover",
		Short: "Run discovery only — output candidates",
		Long:  "Run only the discovery stage across all configured sources, outputting raw candidates.",
		RunE:  runDiscover,
	})

	// Classify (classify-only mode)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "classify",
		Short: "Run classification and scoring",
		Long:  "Load candidates from the database and execute Taiwan/AI relevance scoring and classification.",
		RunE:  runClassify,
	})

	// Verify (verify-only mode)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "verify",
		Short: "Run runtime verification on STATIC_VERIFIED servers",
		Long:  "Execute runtime verification (MCP protocol handshake) on servers with STATIC_VERIFIED status.",
		RunE:  runVerify,
	})

	// Scan (security scan)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "scan",
		Short: "Run security scanning",
		Long:  "Execute security scanning on entities to detect malicious patterns, obfuscation, and credential extraction.",
		RunE:  runScan,
	})

	// Score (quality scoring)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "score",
		Short: "Run quality scoring",
		Long:  "Compute quality scores for entities based on 10 components: data sources, maintenance, documentation, MCP compliance, tool schema, health, repository, license, security, community.",
		RunE:  runScore,
	})

	// Migrate (data migration)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Run database migration",
		Long:  "Run the V1→V2 database schema migration for the new Entity model.",
		RunE:  runMigrate,
	})

	// Export
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export registry JSON files",
		RunE:  runExport,
	}
	exportCmd.Flags().BoolVar(&maliciousReport, "malicious", true, "generate MALICIOUS_REPORT.md and blocklist.txt")
	exportCmd.Flags().BoolVar(&injectionReport, "injection", true, "generate INJECTION_REPORT.md and patterns.json")
	rootCmd.AddCommand(exportCmd)

	// Stats
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show registry statistics",
		RunE:  runStats,
	}
	statsCmd.Flags().BoolVar(&history, "history", false, "show crawl run history")
	rootCmd.AddCommand(statsCmd)

	// Search
	rootCmd.AddCommand(&cobra.Command{
		Use:   "search [query]",
		Short: "Search MCP servers",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSearch,
	})

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "config/sources.yaml", "config file path")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "./data/registry.db", "SQLite database path")
	rootCmd.PersistentFlags().StringVar(&sourceFlag, "source", "all", "source to crawl (github, registry, mcpserversorg, mcpmarket, all)")
	rootCmd.PersistentFlags().BoolVar(&incremental, "incremental", false, "run incremental crawl")
	rootCmd.PersistentFlags().StringVar(&pipelineFlag, "pipeline", "full", "pipeline mode: full|discovery-only|classify-only|verify-only")
	rootCmd.PersistentFlags().IntVar(&workers, "workers", 4, "number of workers per source")
	rootCmd.PersistentFlags().IntVar(&minScore, "min-score", 0, "minimum quality score filter")
	rootCmd.PersistentFlags().StringVar(&capabilityFlag, "capability", "", "search by capability keywords")
	rootCmd.PersistentFlags().StringVar(&catFilter, "category", "", "filter by category")
	rootCmd.PersistentFlags().BoolVar(&maliciousReport, "malicious-report", true, "generate MALICIOUS_REPORT.md and blocklist.txt")
	rootCmd.PersistentFlags().StringVar(&maliciousDir, "malicious-dir", "/data/registry/malicious", "directory for malicious report output")
	rootCmd.PersistentFlags().StringVar(&maliciousThreshold, "malicious-threshold", "MEDIUM", "minimum risk level for malicious report (LOW, MEDIUM, HIGH, CRITICAL)")
	rootCmd.PersistentFlags().StringVar(&injectionDir, "injection-dir", "/data/registry/security/injection", "directory for injection report output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output JSON format")
	rootCmd.PersistentFlags().IntVar(&batchSize, "batch-size", 100, "number of records to process per batch")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "validate without writing changes")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func openStore() (*storage.Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	store, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return store, nil
}

// setupCrawler creates both the legacy and new pipeline coordinators.
func setupCrawler(store *storage.Store) (*crawler.CrawlCoordinator, *coordinator.PipelineCoordinator) {
	logger := metrics.New(false)
	var adapters []sources.SourceAdapter
	adapters = append(adapters, github.New(os.Getenv("GITHUB_TOKEN")))
	adapters = append(adapters, githubrepo.New("modelcontextprotocol/servers", os.Getenv("GITHUB_TOKEN")))
	adapters = append(adapters, githubrepo.New("modelcontextprotocol/servers-archived", os.Getenv("GITHUB_TOKEN")))
	adapters = append(adapters, registry.New())
	adapters = append(adapters, mcpserversorg.New())
	adapters = append(adapters, mcpmarket.New())
	norm := normalize.New()
	legacy := crawler.NewCrawlCoordinator(store, norm, adapters, logger)
	pipeline := coordinator.New(store, norm, adapters, logger)
	return legacy, pipeline
}

// runCrawl runs the crawler, using the new pipeline coordinator when --pipeline mode is set.
func runCrawl(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	_, pipeline := setupCrawler(store)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if incremental && !fullCrawl && pipelineFlag == "full" {
		incr := crawler.NewIncrementalCrawler(nil)
		_ = incr
		// Fall back to pipeline for incremental
	}

	var mode coordinator.PipelineMode
	switch pipelineFlag {
	case "discovery-only":
		mode = coordinator.ModeDiscoveryOnly
	case "classify-only":
		mode = coordinator.ModeClassifyOnly
	case "verify-only":
		mode = coordinator.ModeVerifyOnly
	default:
		mode = coordinator.ModeFull
	}

	cfg := coordinator.PipelineConfig{
		Mode:         mode,
		MaxPerSource: maxPerSource,
		Workers:      workers,
		OutputDir:    "/data/registry",
	}
	if err := pipeline.Run(ctx, cfg); err != nil {
		return err
	}

	// Auto-generate malicious and injection reports from the v2 entities
	// table (T099). Previously read from legacy store.GetServers(); the
	// EntityStore is the canonical source per spec §43.
	entityStore := storage.NewEntityStore(store.DB())
	entities, err := entityStore.List(ctx, storage.EntityFilter{})
	if err != nil {
		return fmt.Errorf("list entities for reports: %w", err)
	}
	if maliciousReport {
		if err := generateMaliciousReport(entities, maliciousDir, maliciousThreshold); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: malicious report generation failed: %v\n", err)
		}
	}
	if injectionReport {
		if err := generateInjectionReport(entities, injectionDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: injection report generation failed: %v\n", err)
		}
	}
	return nil
}

func runDiscover(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	_, pipeline := setupCrawler(store)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := coordinator.PipelineConfig{
		Mode:         coordinator.ModeDiscoveryOnly,
		MaxPerSource: maxPerSource,
		Workers:      workers,
		OutputDir:    "/data/registry",
	}

	if dryRun {
		fmt.Println("DRY RUN: discovery would run with config:", cfg)
		return nil
	}

	return pipeline.Run(ctx, cfg)
}

func runClassify(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	_, pipeline := setupCrawler(store)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := coordinator.PipelineConfig{
		Mode:         coordinator.ModeClassifyOnly,
		MaxPerSource: maxPerSource,
		Workers:      workers,
		OutputDir:    "/data/registry",
	}

	if dryRun {
		fmt.Println("DRY RUN: classify would run with config:", cfg)
		return nil
	}

	return pipeline.Run(ctx, cfg)
}

func runVerify(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	_, pipeline := setupCrawler(store)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := coordinator.PipelineConfig{
		Mode:         coordinator.ModeVerifyOnly,
		MaxPerSource: maxPerSource,
		Workers:      workers,
		OutputDir:    "/data/registry",
	}

	if dryRun {
		fmt.Println("DRY RUN: verify would run with config:", cfg)
		return nil
	}

	return pipeline.Run(ctx, cfg)
}

func runScan(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	_, pipeline := setupCrawler(store)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := coordinator.PipelineConfig{
		Mode:         coordinator.ModeFull,
		MaxPerSource: maxPerSource,
		Workers:      workers,
		OutputDir:    "/data/registry",
	}

	if dryRun {
		fmt.Println("DRY RUN: scan would run with config:", cfg)
		return nil
	}

	// Security scanning is embedded in the full pipeline; for standalone, run full pipeline
	return pipeline.Run(ctx, cfg)
}

func runScore(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	_, pipeline := setupCrawler(store)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := coordinator.PipelineConfig{
		Mode:         coordinator.ModeFull,
		MaxPerSource: maxPerSource,
		Workers:      workers,
		OutputDir:    "/data/registry",
	}

	if dryRun {
		fmt.Println("DRY RUN: score would run with config:", cfg)
		return nil
	}

	// Quality scoring is embedded in the full pipeline; for standalone, run full pipeline
	return pipeline.Run(ctx, cfg)
}

func runMigrate(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	if verbose {
		fmt.Println("Migration completed successfully")
	}
	return nil
}

func runExport(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	entityStore := storage.NewEntityStore(store.DB())
	entities, err := entityStore.List(ctx, storage.EntityFilter{})
	if err != nil {
		return fmt.Errorf("list entities: %w", err)
	}

	// Generate views into /data/registry (bind-mounted to host ./registry,
	// spec §60). The previous "registry" path was relative to the
	// container's WORKDIR (/app) and wasn't visible on the host.
	expDir := "/data/registry"
	if err := os.MkdirAll(expDir, 0755); err != nil {
		return fmt.Errorf("create export dir: %w", err)
	}
	vg := export.NewViewGenerator(export.ViewConfig{
		SchemaVersion:  "2.0",
		CrawlerVersion: "1.0.0",
	})
	if err := vg.GenerateViews(entities, expDir); err != nil {
		return fmt.Errorf("generate views: %w", err)
	}

	// Keep the legacy malicious/injection reports wired up too.
	if maliciousReport {
		if err := generateMaliciousReport(entities, maliciousDir, maliciousThreshold); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: malicious report generation failed: %v\n", err)
		}
	}
	if injectionReport {
		if err := generateInjectionReport(entities, injectionDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: injection report generation failed: %v\n", err)
		}
	}

	fmt.Printf("Exported %d entities to %s/\n", len(entities), expDir)
	entries, _ := os.ReadDir(expDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			fmt.Printf("  - %s\n", entry.Name())
		}
	}
	return nil
}

// serverToEntity converts MCPServer to Entity for export purposes.
func serverToEntity(s *models.MCPServer) *models.Entity {
	now := models.RFC3339Time(time.Now().UTC())
	return &models.Entity{
		ID:          s.ID,
		Name:        s.Name,
		Slug:        s.Slug,
		Description: s.Description,
		Repository:  s.Repository,
		Endpoints:   toEndpointWithTypes(s.Endpoints),
		Tools:       s.Tools,
		Resources:   s.Resources,
		Prompts:     s.Prompts,
		DataSources: s.DataSources,
		Sources:     s.Sources,
		FirstSeen:   now,
		LastSeen:    now,
		RawContent:  s.Readme,
	}
}

func toEndpointWithTypes(endpoints []models.Endpoint) []models.EndpointWithType {
	result := make([]models.EndpointWithType, 0, len(endpoints))
	for _, ep := range endpoints {
		result = append(result, models.EndpointWithType{
			Endpoint: ep,
		})
	}
	return result
}

func generateMaliciousReport(entities []*models.Entity, dir, threshold string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	exp := export.NewMaliciousExporter(entities, dir)
	return exp.Export()
}

func generateInjectionReport(entities []*models.Entity, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	exp := export.NewInjectionExporter(entities, dir)
	return exp.Export()
}

func runStats(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if history {
		return runStatsHistory(store)
	}

	servers, err := store.GetServers(context.Background())
	if err != nil {
		return err
	}

	stats := computeStats(servers)

	if jsonOutput {
		data, _ := json.MarshalIndent(stats, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STAT\tVALUE")
	fmt.Fprintf(w, "Total Servers\t%d\n", stats.TotalServers)
	fmt.Fprintf(w, "Taiwan Relevant\t%d\n", stats.TaiwanRelevant)
	fmt.Fprintf(w, "Verified\t%d\n", stats.Verified)
	fmt.Fprintf(w, "Failed\t%d\n", stats.Failed)
	for _, level := range []string{"T0", "T1", "T2", "T3", "T4", "T5"} {
		fmt.Fprintf(w, "Level %s\t%d\n", level, stats.ByLevel[level])
	}
	for _, health := range []string{"HEALTHY", "DEGRADED", "UNAVAILABLE", "UNKNOWN"} {
		fmt.Fprintf(w, "Health %s\t%d\n", health, stats.ByHealth[health])
	}
	for _, grade := range []string{"A", "B", "C", "D", "F"} {
		fmt.Fprintf(w, "Quality %s\t%d\n", grade, stats.QualityDist[grade])
	}
	w.Flush()
	return nil
}

func runSearch(cmd *cobra.Command, args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	servers, err := store.GetServers(context.Background())
	if err != nil {
		return err
	}

	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	// Use SearchEngine for ranking and filtering (§22, T036, T037)
	se := search.New(servers)

	// If capability flag is set, use capability search (T037)
	if capabilityFlag != "" {
		results, err := se.SearchByCapability(capabilityFlag)
		if err != nil {
			return err
		}
		return printSearchResults(results, jsonOutput)
	}

	searchQuery := search.SearchQuery{
		Text:     query,
		Level:    levelFilter,
		Category: nil,
		MinScore: minScore,
		Limit:    100,
	}
	if catFilter != "" {
		searchQuery.Category = []string{catFilter}
	}

	results, err := se.Search(searchQuery)
	if err != nil {
		return err
	}

	return printSearchResults(results, jsonOutput)
}

func printSearchResults(results []search.SearchResult, jsonOut bool) error {
	// Convert to plain servers for display
	displayServers := make([]models.MCPServer, len(results))
	for i, r := range results {
		displayServers[i] = r.Server
	}

	if jsonOut {
		data, _ := json.MarshalIndent(displayServers, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLEVEL\tHEALTH\tQUALITY\tSOURCES\tRELEVANCE")
	for _, r := range results {
		s := r.Server
		srcs := make([]string, len(s.Sources))
		for i, src := range s.Sources {
			srcs[i] = src.Source
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%.1f\n",
			s.Name, s.TaiwanRelevance.Level, s.Health, s.Quality.Score, strings.Join(srcs, ", "), r.Score)
	}
	w.Flush()
	return nil
}


type crawlStats struct {
	TotalServers   int
	TaiwanRelevant int
	Verified       int
	Failed         int
	ByLevel        map[string]int
	ByHealth       map[string]int
	QualityDist    map[string]int
}

func computeStats(servers []models.MCPServer) crawlStats {
	s := crawlStats{
		TotalServers: len(servers),
		Verified:     len(servers),
		ByLevel:      make(map[string]int),
		ByHealth:     make(map[string]int),
		QualityDist:  make(map[string]int),
	}

	for _, srv := range servers {
		level := string(srv.TaiwanRelevance.Level)
		if level == "" {
			level = "T0"
		}
		s.ByLevel[level]++

		s.ByHealth[string(srv.Health)]++

		grade := string(srv.Quality.Grade)
		if grade == "" {
			grade = "F"
		}
		s.QualityDist[grade]++

		if level != "" && level != "T0" {
			s.TaiwanRelevant++
		}
	}

	return s
}
func runStatsHistory(store *storage.Store) error {
	ctx := context.Background()

	runs, err := store.GetCrawlRuns(ctx)
	if err != nil {
		return fmt.Errorf("get crawl runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("No crawl runs found")
		return nil
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(runs, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CRAWL ID\tSTARTED AT\tSOURCES SCANNED\tCANDIDATES FOUND\tNORMALIZED\tDEDUPED\tTAIWAN CANDIDATES\tVERIFIED\tFAILED")
	for _, run := range runs {
		started := run.StartedAt.Time().Format("2006-01-02 15:04:05")
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			run.CrawlID,
			started,
			run.SourcesScanned,
			run.CandidatesFound,
			run.CandidatesNorm,
			run.DuplicatesRemoved,
			run.TaiwanCandidates,
			run.Verified,
			run.Failed,
		)
	}
	w.Flush()
	return nil
}
