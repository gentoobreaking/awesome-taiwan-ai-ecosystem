// Package api provides a REST API server for the Taiwan AI Ecosystem Registry.
// Based on spec §67 Phase 4 (T048).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

// Server is the REST API server (T048).
type Server struct {
	store   *storage.Store
	logger  *slog.Logger
	port    string
	rateLim *rateLimiter
}

// Config holds API server configuration.
type Config struct {
	Port            string
	DBPath          string
	RateLimitPerMin int
	CORSOrigins     []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DefaultConfig returns sensible defaults for the API server.
func DefaultConfig() Config {
	return Config{
		Port:            "8080",
		DBPath:          "./data/registry.db",
		RateLimitPerMin: 100,
		CORSOrigins:     []string{"*"},
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}

// New creates a new REST API server.
func New(cfg Config, store *storage.Store, logger *slog.Logger) *Server {
	return &Server{
		store:   store,
		logger:  logger,
		port:    cfg.Port,
		rateLim: newRateLimiter(cfg.RateLimitPerMin),
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	handler := s.rateLim.Middleware(mux)
	handler = s.corsMiddleware(handler)

	addr := ":" + s.port
	s.logger.Info("API server starting", "addr", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	return srv.ListenAndServe()
}

// registerRoutes registers all API routes.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/servers", s.handleServers)
	mux.HandleFunc("/api/v1/servers/", s.handleServerByID)
	mux.HandleFunc("/api/v1/search", s.handleSearch)
	mux.HandleFunc("/api/v1/registry", s.handleRegistry)
	mux.HandleFunc("/api/v1/statistics", s.handleStatistics)
	mux.HandleFunc("/api/v1/health", s.handleHealth)
}

// --- Handlers ---

// handleHealth returns server health status.
// GET /health, GET /api/v1/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	es := storage.NewEntityStore(s.store.DB())
	count, err := es.Count(ctx, storage.EntityFilter{})
	if err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, APIError{
			Error:   "database_unavailable",
			Message: fmt.Sprintf("Database connection failed: %v", err),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   "v0.1",
		DBCount:   count,
	})
}

// handleServers returns all servers with pagination and filtering.
// GET /api/v1/servers?page=1&limit=50&level=T3&category=AI_AGENT&min-score=70
func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	page, limit, err := parsePagination(r)
	if err != nil {
		s.writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}

	filter := parseEntityFilter(r)
	filter.Limit = limit
	filter.Offset = (page - 1) * limit

	es := storage.NewEntityStore(s.store.DB())
	entities, err := es.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list entities", "error", err)
		s.writeAPIError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	total, err := es.Count(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to count entities", "error", err)
		s.writeAPIError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	servers := entitiesToViews(entities)

	s.writeJSON(w, http.StatusOK, ServersResponse{
		Servers:    servers,
		Pagination: makePagination(page, limit, total),
	})
}

// handleServerByID returns a single server by ID.
// GET /api/v1/servers/{id}
func (s *Server) handleServerByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/servers/")
	if id == "" {
		s.writeAPIError(w, http.StatusBadRequest, "invalid_id", "server ID is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	es := storage.NewEntityStore(s.store.DB())
	entity, err := es.Get(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get entity", "id", id, "error", err)
		s.writeAPIError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	if entity == nil {
		s.writeAPIError(w, http.StatusNotFound, "not_found", fmt.Sprintf("server with ID %s not found", id))
		return
	}

	view := entity.ToMCPServerView()
	if view == nil {
		s.writeAPIError(w, http.StatusNotFound, "not_found", "entity is not an MCP server view")
		return
	}

	s.writeJSON(w, http.StatusOK, ServerResponse{Server: *view})
}

// handleSearch performs a keyword search.
// GET /api/v1/search?q=keyword&level=T3&category=AI_AGENT&min-score=70&limit=50&page=1
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	page, limit, err := parsePagination(r)
	if err != nil {
		s.writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}

	query := r.URL.Query().Get("q")
	filter := parseEntityFilter(r)
	filter.Limit = limit
	filter.Offset = (page - 1) * limit

	es := storage.NewEntityStore(s.store.DB())
	entities, err := es.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to search entities", "error", err)
		s.writeAPIError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	// Filter by text query
	var filtered []*models.Entity
	for _, e := range entities {
		if query != "" && !entityTextMatch(e, query) {
			continue
		}
		filtered = append(filtered, e)
	}

	servers := entitiesToViews(filtered)

	s.writeJSON(w, http.StatusOK, SearchResponse{
		Query:      query,
		Results:    servers,
		Pagination: makePagination(page, limit, len(entities)),
	})
}

// handleRegistry returns the full registry data.
// GET /api/v1/registry
func (s *Server) handleRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	es := storage.NewEntityStore(s.store.DB())
	entities, err := es.List(ctx, storage.EntityFilter{})
	if err != nil {
		s.logger.Error("Failed to fetch registry", "error", err)
		s.writeAPIError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	servers := entitiesToViews(entities)
	stats := entitiesToStats(entities)

	s.writeJSON(w, http.StatusOK, RegistryResponse{
		SchemaVersion:   "0.1",
		RegistryVersion: "1.0",
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		TotalServers:    len(entities),
		TaiwanRelevant:  stats["taiwan_relevant"].(int),
		Statistics:      stats,
		Servers:         servers,
	})
}

// handleStatistics returns registry statistics.
// GET /api/v1/statistics
func (s *Server) handleStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	es := storage.NewEntityStore(s.store.DB())
	entities, err := es.List(ctx, storage.EntityFilter{})
	if err != nil {
		s.logger.Error("Failed to fetch for statistics", "error", err)
		s.writeAPIError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, entitiesToStats(entities))
}

// --- Response types ---

// HealthResponse is the /health response.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
	DBCount   int    `json:"db_count"`
}

// Pagination metadata for paginated responses.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ServersResponse is the /api/v1/servers response.
type ServersResponse struct {
	Servers    []models.MCPServerView `json:"servers"`
	Pagination Pagination             `json:"pagination"`
}

// ServerResponse is the /api/v1/servers/{id} response.
type ServerResponse struct {
	Server models.MCPServerView `json:"server"`
}

// SearchResponse is the /api/v1/search response.
type SearchResponse struct {
	Query      string                 `json:"query"`
	Results    []models.MCPServerView `json:"results"`
	Pagination Pagination             `json:"pagination"`
}

// RegistryResponse is the /api/v1/registry response.
type RegistryResponse struct {
	SchemaVersion   string                 `json:"schema_version"`
	RegistryVersion string                 `json:"registry_version"`
	GeneratedAt     string                 `json:"generated_at"`
	TotalServers    int                    `json:"total_servers"`
	TaiwanRelevant  int                    `json:"taiwan_relevant"`
	Statistics      map[string]interface{} `json:"statistics"`
	Servers         []models.MCPServerView `json:"servers"`
}

// APIError is a standard error response.
type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// --- Helpers ---

// makePagination computes pagination metadata.
func makePagination(page, limit, total int) Pagination {
	totalPages := 1
	if limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
		if totalPages < 1 {
			totalPages = 1
		}
	}
	return Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}

// entitiesToViews converts entities to MCPServerView slices, skipping
// entities that don't produce a valid view (e.g. non-MCP_SERVER).
func entitiesToViews(entities []*models.Entity) []models.MCPServerView {
	servers := make([]models.MCPServerView, 0, len(entities))
	for _, e := range entities {
		if e == nil {
			continue
		}
		view := e.ToMCPServerView()
		if view == nil {
			continue
		}
		servers = append(servers, *view)
	}
	return servers
}

// entitiesToStats computes statistics from entity slice.
func entitiesToStats(entities []*models.Entity) map[string]interface{} {
	byLevel := make(map[string]int)
	byHealth := make(map[string]int)
	byQuality := make(map[string]int)
	byStatus := make(map[string]int)
	taiwanCount := 0
	total := len(entities)

	for _, e := range entities {
		if e == nil {
			continue
		}

		level := string(e.TaiwanRelevance.Level)
		if level >= "T1" && level <= "T5" {
			taiwanCount++
		}
		byLevel[level]++

		// Health from Quality.Components.Health (0-10 → HEALTHY/DEGRADED/UNHEALTHY)
		healthScore := e.Quality.Components.Health
		var healthStr string
		switch {
		case healthScore >= 8:
			healthStr = "HEALTHY"
		case healthScore > 0:
			healthStr = "DEGRADED"
		default:
			healthStr = "UNHEALTHY"
		}
		byHealth[healthStr]++

		byQuality[string(e.Quality.Grade)]++
		byStatus[string(e.EntityStatus)]++

		for _, c := range e.Classification.Evidence {
			_ = c
		}
	}

	return map[string]interface{}{
		"total_servers":       total,
		"taiwan_relevant":     taiwanCount,
		"by_level":            byLevel,
		"by_health":           byHealth,
		"quality_distribution": byQuality,
		"by_status":           byStatus,
	}
}

// parsePagination extracts page and limit from query parameters.
func parsePagination(r *http.Request) (page, limit int, err error) {
	page = 1
	limit = 50

	if p := r.URL.Query().Get("page"); p != "" {
		page, err = strconv.Atoi(p)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid page: %w", err)
		}
		if page < 1 {
			page = 1
		}
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		limit, err = strconv.Atoi(l)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid limit: %w", err)
		}
		if limit < 1 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
	}

	return page, limit, nil
}

// parseEntityFilter builds an EntityFilter from query parameters.
func parseEntityFilter(r *http.Request) storage.EntityFilter {
	filter := storage.EntityFilter{}

	if level := r.URL.Query().Get("level"); level != "" {
		filter.TaiwanLevel = models.TaiwanRelevanceLevel(level)
	}
	if cat := r.URL.Query().Get("category"); cat != "" {
		filter.PrimaryClassification = cat
	}
	if sec := r.URL.Query().Get("security"); sec != "" {
		filter.SecurityStatus = models.SecurityStatus(sec)
	}
	if minScore := r.URL.Query().Get("min-score"); minScore != "" {
		if score, err := strconv.Atoi(minScore); err == nil {
			filter.QualityMin = score
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.EntityStatus = status
	}

	return filter
}

// entityTextMatch performs a simple text search across entity fields.
func entityTextMatch(e *models.Entity, query string) bool {
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(e.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Slug), q) {
		return true
	}
	if e.Repository.URL != "" && strings.Contains(strings.ToLower(e.Repository.URL), q) {
		return true
	}
	for _, t := range e.Tools {
		if strings.Contains(strings.ToLower(t.Name), q) {
			return true
		}
	}
	return false
}

// writeJSON writes a JSON response with proper headers.
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// writeAPIError writes a standard API error response.
func (s *Server) writeAPIError(w http.ResponseWriter, status int, errCode, message string) {
	s.writeJSON(w, status, APIError{Error: errCode, Message: message})
}

// corsMiddleware adds CORS headers.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
