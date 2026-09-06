package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/david/awesome-taiwan-mcp/internal/export"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	"github.com/spf13/cobra"
)

var (
	viewName  string
	outputDir string
	verbose   bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "export",
		Short: "Export registry views from the database",
		Long:  "Read entities from the database and generate registry views (Markdown + JSON).",
		RunE:  runExport,
	}

	rootCmd.Flags().StringVar(&viewName, "view", "all", "view to export: all|mcp|agents|tools|data|ecosystem")
	rootCmd.Flags().StringVar(&outputDir, "output-dir", "registry", "output directory for generated views")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "enable verbose logging")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runExport(cmd *cobra.Command, _ []string) error {
	db, err := storage.Open("data/registry.db")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	entityStore := storage.NewEntityStore(db.DB())
	entities, err := entityStore.List(ctx, storage.EntityFilter{})
	if err != nil {
		return fmt.Errorf("list entities: %w", err)
	}

	// Generate views using ViewGenerator
	vg := export.NewViewGenerator(export.ViewConfig{
		SchemaVersion:  "2.0",
		CrawlerVersion: "dev",
	})

	// Ensure output directory exists
	viewsDir := filepath.Join(outputDir, "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := vg.GenerateViews(entities, viewsDir); err != nil {
		return fmt.Errorf("generate views: %w", err)
	}

	if verbose {
		fmt.Printf("View filter: %s\n", viewName)
	}
	fmt.Printf("Exported %d entities to %s/\n", len(entities), viewsDir)

	// List generated view files
	entries, _ := os.ReadDir(viewsDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			fmt.Printf("  - %s\n", entry.Name())
		}
	}
	return nil
}

// viewFilePrefix maps CLI view names to view file prefixes.
func viewFilePrefix(name string) string {
	switch strings.ToLower(name) {
	case "all":
		return ""
	case "mcp":
		return "taiwan-mcp"
	case "agents":
		return "taiwan-ai-agents"
	case "tools":
		return "taiwan-ai-tools"
	case "data":
		return "taiwan-ai-data"
	case "ecosystem":
		return "taiwan-ai-ecosystem"
	default:
		return ""
	}
}
