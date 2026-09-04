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
	"github.com/david/awesome-taiwan-mcp/internal/export"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
	"github.com/david/awesome-taiwan-mcp/internal/sources/github"
	"github.com/david/awesome-taiwan-mcp/internal/sources/registry"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Flags
	configPath  string
	dbPath      string
	sourceFlag  string
	fullCrawl   bool
	workers     int
	jsonOutput  bool
	minScore    int
	levelFilter string
	catFilter   string
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
	rootCmd.AddCommand(&cobra.Command{
		Use:   "export",
		Short: "Export registry JSON files",
		RunE:  runExport,
	})

	// Stats
	rootCmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Show registry statistics",
		RunE:  runStats,
	})

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
	rootCmd.PersistentFlags().StringVar(&sourceFlag, "source", "all", "source to crawl (github, registry, all)")
	rootCmd.PersistentFlags().BoolVar(&fullCrawl, "full", false, "force full crawl")
	rootCmd.PersistentFlags().IntVar(&workers, "workers", 4, "number of workers per source")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().IntVar(&minScore, "min-score", 0, "minimum quality score filter")
	rootCmd.PersistentFlags().StringVar(&levelFilter, "level", "", "filter by Taiwan relevance level (T0-T5)")
	rootCmd.PersistentFlags().StringVar(&catFilter, "category", "", "filter by category")

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

func setupCrawler(store *storage.Store) *crawler.CrawlCoordinator {
	logger := metrics.New(false)
	var adapters []sources.SourceAdapter
	adapters = append(adapters, github.New(os.Getenv("GITHUB_TOKEN")))
	adapters = append(adapters, registry.New())

	norm := normalize.New()
	return crawler.NewCrawlCoordinator(store, norm, adapters, logger)
}

func runCrawl(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	coord := setupCrawler(store)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return coord.Run(ctx, crawler.CrawlOptions{
		Source:    sourceFlag,
		FullCrawl: fullCrawl,
		Workers:   workers,
	})
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

	expDir := filepath.Join("registry")
	re := export.New()
	return re.Export(expDir, servers)
}

func runStats(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

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

	var results []models.MCPServer
	for _, s := range servers {
		if matchesSearch(s, query) &&
			(levelFilter == "" || s.TaiwanRelevance.Level == levelFilter) &&
			(catFilter == "" || hasCategory(s.Category, catFilter)) &&
			(minScore == 0 || s.Quality.Score >= minScore) {
			results = append(results, s)
		}
	}

	// Sort: Taiwan relevance + health + quality
	sortResults(results)

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLEVEL\tHEALTH\tQUALITY\tSOURCES")
	for _, s := range results {
		srcs := make([]string, len(s.Sources))
		for i, src := range s.Sources {
			srcs[i] = src.Source
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			s.Name, s.TaiwanRelevance.Level, s.Health, s.Quality.Score, strings.Join(srcs, ", "))
	}
	w.Flush()
	return nil
}

func matchesSearch(s models.MCPServer, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(s.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Description), q) {
		return true
	}
	for _, c := range s.Category {
		if strings.Contains(strings.ToLower(c), q) {
			return true
		}
	}
	for _, t := range s.Tools {
		if strings.Contains(strings.ToLower(t.Name), q) {
			return true
		}
	}
	for _, ds := range s.DataSources {
		if strings.Contains(strings.ToLower(ds.Name), q) {
			return true
		}
	}
	return false
}

func hasCategory(cats []string, cat string) bool {
	for _, c := range cats {
		if strings.EqualFold(c, cat) {
			return true
		}
	}
	return false
}

func sortResults(results []models.MCPServer) {
	// Sort by Taiwan relevance level (T5 first), then health, then quality, then name
	levelOrder := map[string]int{"T5": 0, "T4": 1, "T3": 2, "T2": 3, "T1": 4, "T0": 5}
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			si, sj := results[i], results[j]
			li, lj := levelOrder[si.TaiwanRelevance.Level], levelOrder[sj.TaiwanRelevance.Level]
			if li > lj || (li == lj && si.Health < sj.Health) ||
				(li == lj && si.Health == sj.Health && si.Quality.Score < sj.Quality.Score) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
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
		level := srv.TaiwanRelevance.Level
		if level == "" {
			level = "T0"
		}
		s.ByLevel[level]++

		s.ByHealth[string(srv.Health)]++

		grade := srv.Quality.Grade
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
