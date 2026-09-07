// Command seed populates the v2 entities table from the legacy
// registry/registry.json file. Used to verify that the T099 export
// pipeline (EntityStore → ViewGenerator) produces non-empty views
// even before a real crawler run finishes.
//
// Usage:
//   go run ./cmd/seed --registry registry/registry.json --db data/registry.db --limit 20
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

type legacyFile struct {
	SchemaVersion  string             `json:"schema_version"`
	CrawlerVersion string             `json:"crawler_version"`
	Servers        []legacyServer     `json:"servers"`
	Statistics     map[string]any     `json:"statistics"`
}

type legacyServer struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Slug            string         `json:"slug"`
	Description     string         `json:"description"`
	Region          []string       `json:"region"`
	Category        []string       `json:"category"`
	TaiwanRelevance map[string]any `json:"taiwan_relevance"`
	Repository      map[string]any `json:"repository"`
	Endpoints       []map[string]any `json:"endpoints"`
	Transport       []string       `json:"transport"`
	Tools           []map[string]any `json:"tools"`
	Quality         map[string]any `json:"quality"`
	Status          string         `json:"status"`
	Health          string         `json:"health"`
	Sources         []map[string]any `json:"sources"`
	FirstSeenAt     string         `json:"first_seen_at"`
	LastSeenAt      string         `json:"last_seen_at"`
}

func main() {
	var (
		registryPath = flag.String("registry", "registry/registry.json", "path to legacy registry.json")
		dbPath       = flag.String("db", "data/registry.db", "path to v2 sqlite database")
		limit        = flag.Int("limit", 20, "max entities to seed (0 = all)")
	)
	flag.Parse()

	data, err := os.ReadFile(*registryPath)
	if err != nil {
		log.Fatalf("read registry: %v", err)
	}
	var lf legacyFile
	if err := json.Unmarshal(data, &lf); err != nil {
		log.Fatalf("parse registry: %v", err)
	}

	store, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	es := storage.NewEntityStore(store.DB())

	servers := lf.Servers
	if *limit > 0 && *limit < len(servers) {
		servers = servers[:*limit]
	}

	now := models.RFC3339Time(time.Now().UTC())
	for _, s := range servers {
		entity := toEntity(s, now)
		if err := es.Save(ctx, entity); err != nil {
			log.Printf("save %s: %v", s.ID, err)
			continue
		}
		fmt.Printf("seeded %s — %s\n", entity.ID, entity.Name)
	}
	fmt.Printf("Done: %d entities\n", len(servers))
}

// heuristicPrimary picks a PrimaryClassification from name +
// description signals when the legacy category field is empty.
// Conservative — defaults to MCP_SERVER when nothing matches, so
// genuine MCP servers are not silently re-classified.
func heuristicPrimary(name, description string) models.PrimaryClassification {
	nameLow := strings.ToLower(name)
	descLow := strings.ToLower(description)
	text := nameLow + " " + descLow

	// Pass 1: name patterns are the strongest signal — try them
	// first and return early.
	if primary := namePrimary(nameLow); primary != models.PrimaryClassification("") {
		return primary
	}
	// Pass 2: description keywords.
	if primary := descPrimary(text); primary != models.PrimaryClassification("") {
		return primary
	}
	return models.PrimaryClassificationMCPServer
}

// namePrimary matches high-confidence name patterns.
func namePrimary(name string) models.PrimaryClassification {
	switch {
	case containsAny(name, []string{"tutorial", "demo", "example", "getting-started", "how-to"}):
		return models.PrimaryClassificationAITutorial
	case containsAny(name, []string{"awesome-", "-awesome", "awesome_", "-collection", "curated-", "-awesome-list", "awesome-"}):
		return models.PrimaryClassificationMCPCollection
	case containsAny(name, []string{"-framework", "_framework", "framework-", "orchestrator", "crewai", "langgraph"}):
		return models.PrimaryClassificationAIFramework
	case containsAny(name, []string{"-agent", "_agent", "agent-", "agent_", "react-agent", "crew-", "chatbot"}):
		return models.PrimaryClassificationAIAgent
	case containsAny(name, []string{"-sdk", "_sdk", "sdk-", "client-py", "client-go", "client-ts"}):
		return models.PrimaryClassificationAISDK
	case containsAny(name, []string{"twmarket", "twstock", "twdataset", "taiwandata", "data-", "finmind", "fugle-data"}):
		return models.PrimaryClassificationAIDataset
	case containsAny(name, []string{"cli", "-cli", "_cli", "cli-", "cmd-"}):
		return models.PrimaryClassificationAITool
	}
	return models.PrimaryClassification("")
}

// descPrimary matches description text signals.
func descPrimary(text string) models.PrimaryClassification {
	switch {
	case containsAny(text, []string{"orchestration framework", "workflow engine", "agent framework", "multi-agent orchestration"}):
		return models.PrimaryClassificationAIFramework
	case containsAny(text, []string{"ai agent", "agentic", "autonomous agent", "react agent", "multi-agent system", "chatbot powered by"}):
		return models.PrimaryClassificationAIAgent
	case containsAny(text, []string{"data library", "market data", "stock data", "data feed", "data pipeline", "data api", "etl pipeline", "postgres schema", "twse api", "tpex api", "taiwan stock data", "taiwan market data"}):
		return models.PrimaryClassificationAIDataset
	case containsAny(text, []string{"client library", "python sdk", "go sdk", "typescript sdk", "rust sdk"}):
		return models.PrimaryClassificationAISDK
	case containsAny(text, []string{"tutorial", "getting started", "step-by-step", "minimal example"}):
		return models.PrimaryClassificationAITutorial
	case containsAny(text, []string{"awesome list", "curated list", "awesome collection", "list of mcp"}):
		return models.PrimaryClassificationMCPCollection
	case containsAny(text, []string{"command-line", "command line tool"}):
		return models.PrimaryClassificationAITool
	}
	return models.PrimaryClassification("")
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func toEntity(s legacyServer, now models.RFC3339Time) *models.Entity {
	repo := models.RepositoryInfo{}
	if s.Repository != nil {
		if u, ok := s.Repository["url"].(string); ok {
			repo.URL = u
		}
		if name, ok := s.Repository["name"].(string); ok {
			repo.Name = name
		}
	}
	taiwan := models.TaiwanRelevance{}
	if s.TaiwanRelevance != nil {
		if score, ok := s.TaiwanRelevance["score"].(float64); ok {
			taiwan.Score = score
		}
		if level, ok := s.TaiwanRelevance["level"].(string); ok {
			taiwan.Level = models.TaiwanRelevanceLevel(level)
		}
	}
	quality := models.QualityScore{}
	if s.Quality != nil {
		if score, ok := s.Quality["score"].(float64); ok {
			quality.Score = int(score)
		}
		if grade, ok := s.Quality["grade"].(string); ok {
			quality.Grade = models.QualityGrade(grade)
		}
	}
	primary := models.PrimaryClassification("MCP_SERVER")
	// Use the legacy category as a crude primary classification signal
	// so seeded entities don't all end up in the same view. Falls
	// through to a heuristic pass over name + description if the
	// legacy category is empty (the 9/5 registry.json has no
	// categories at all).
	if len(s.Category) > 0 {
		switch s.Category[0] {
		case "AI_AGENT", "AGENT":
			primary = models.PrimaryClassificationAIAgent
		case "AI_TOOL", "TOOL":
			primary = models.PrimaryClassificationAITool
		case "AI_FRAMEWORK", "FRAMEWORK":
			primary = models.PrimaryClassificationAIFramework
		case "DATA_LIBRARY", "DATA":
			primary = models.PrimaryClassificationAIDataset
		case "TUTORIAL":
			primary = models.PrimaryClassificationAITutorial
		}
	} else {
		primary = heuristicPrimary(s.Name, s.Description)
	}

	return &models.Entity{
		ID:              s.ID,
		Name:            s.Name,
		Slug:            s.Slug,
		Description:     s.Description,
		Repository:      repo,
		Classification:  models.ClassificationResult{Primary: primary, Confidence: 0.8},
		TaiwanRelevance: taiwan,
		Quality:         quality,
		EntityStatus:    models.EntityStatusDiscovered,
		FirstSeen:       now,
		LastSeen:        now,
	}
}
