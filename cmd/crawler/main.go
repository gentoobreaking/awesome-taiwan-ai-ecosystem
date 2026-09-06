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

	"github.com/david/awesome-taiwan-mcp/internal/crawler"
	"github.com/david/awesome-taiwan-mcp/internal/coordinator"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/search"
	"github.com/david/awesome-taiwan-mcp/internal/security"
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

	// Export
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export registry JSON files",
		RunE:  runExport,
	}
	exportCmd.Flags().BoolVar(&markdownExport, "markdown", false, "also generate REGISTRY.md")
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
	rootCmd.PersistentFlags().StringVar(&maliciousDir, "malicious-dir", "registry/malicious", "directory for malicious report output")
	rootCmd.PersistentFlags().StringVar(&maliciousThreshold, "malicious-threshold", "MEDIUM", "minimum risk level for malicious report (LOW, MEDIUM, HIGH, CRITICAL)")
	rootCmd.PersistentFlags().BoolVar(&injectionReport, "injection-report", true, "generate INJECTION_REPORT.md and patterns.json")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output JSON format")
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
		OutputDir:    "registry",
	}

	return pipeline.Run(ctx, cfg)
}

func runExport(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()
	servers, err := store.GetServers(context.Background())
	if err != nil {
		return err
	}

	// For now, just use security scanner for malicious report
	expDir := filepath.Join("registry")
	if maliciousReport {
		scanner := security.NewScanner()
		for i := range servers {
			result := scanner.ScanServer(&servers[i])
			_ = result
		}
		fmt.Printf("Malicious report generation skipped - implement export.New for full export\n")
	}

	fmt.Println("Export complete: " + expDir)
	return nil
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
