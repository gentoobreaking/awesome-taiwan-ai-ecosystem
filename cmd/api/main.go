// Command api is the REST API server for the Taiwan AI Ecosystem Registry.
// Based on T048.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/api"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

func main() {
	var (
		port            = flag.String("port", "8080", "HTTP server port")
		dbPath          = flag.String("db", "./data/registry.db", "SQLite database path")
		rateLimit       = flag.Int("rate-limit", 100, "Requests per minute per IP")
		readTimeout     = flag.Duration("read-timeout", 30*time.Second, "HTTP read timeout")
		writeTimeout    = flag.Duration("write-timeout", 60*time.Second, "HTTP write timeout")
		shutdownTimeout = flag.Duration("shutdown-timeout", 10*time.Second, "Server shutdown timeout")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Open database
	store, err := storage.Open(*dbPath)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		logger.Error("Failed to migrate database", "error", err)
		os.Exit(1)
	}

	cfg := api.DefaultConfig()
	cfg.Port = *port
	cfg.DBPath = *dbPath
	cfg.RateLimitPerMin = *rateLimit
	cfg.ReadTimeout = *readTimeout
	cfg.WriteTimeout = *writeTimeout
	cfg.ShutdownTimeout = *shutdownTimeout

	srv := api.New(cfg, store, logger)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", "error", err)
			os.Exit(1)
		}
	}()

	logger.Info("API server started", "port", *port)

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down API server")
}
