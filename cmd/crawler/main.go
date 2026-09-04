package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "crawler",
		Short: "Taiwan MCP Crawler",
		Long:  "Automated crawler for discovering, analyzing, and verifying Taiwan-related MCP Servers.",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("crawler version: %s\n", version)
			fmt.Printf("commit: %s\n", commit)
			fmt.Printf("built: %s\n", date)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "crawl",
		Short: "Run the crawler pipeline",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("crawl command not yet implemented")
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
