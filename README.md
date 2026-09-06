<div align="center">

[English](README.md) | [繁體中文](README.zh-TW.md) | [简体中文](README.zh-CN.md)

</div>

# Taiwan AI Ecosystem Registry

Automated crawler and registry builder for discovering, analyzing, and verifying Taiwan-related AI tools, MCP servers, datasets, and infrastructure.

## Overview

**Taiwan AI Ecosystem Registry** crawls multiple sources (GitHub, official registries, community platforms) to discover AI-related entities — including MCP servers, AI tools, datasets, SDKs, and infrastructure — with Taiwan relevance. It normalizes, deduplicates, classifies, verifies, scans for security issues, scores quality, and exports a standardized registry.

The crawler identifies entities related to Taiwan through keyword matching, official domains (e.g. `.gov.tw`, `.org.tw`), government APIs, financial APIs (TWSE, TPEx), real estate data, and Traditional Chinese language detection.

**Pipeline:** Discovery → Normalize → Classify → Taiwan Relevance → AI Relevance → MCP Identity → Dedup → Runtime Verify → Security Scan → Quality Score → Persist → Export

### Core Principles

- **Discovery Broadly**: Cast wide nets across multiple sources to find candidate entities
- **Classify Explicitly**: Apply deterministic and LLM-based classification rules to categorize entities
- **Verify Objectively**: Runtime protocol verification and security scanning provide objective quality signals
- **Publish Conservatively**: Only well-verified, high-quality entities make it to the published registry


## Supported Entity Types

| Type | Description |
|---|---|
| **MCP Server** | Model Context Protocol server with tools/resources/prompts |
| **MCP Client** | Client application that connects to MCP servers |
| **MCP Host** | Host application managing multiple MCP client connections |
| **MCP SDK** | Software Development Kit for building MCP-compatible applications |
| **MCP Library** | General-purpose MCP-related library |
| **MCP Extension** | MCP protocol extension or plugin |
| **AI Agent** | Autonomous AI agent or assistant |
| **AI Tool** | Utility tool, CLI, or function |
| **AI SDK** | SDK for AI frameworks (langchain, llamaindex, etc.) |
| **AI Framework** | AI framework or platform |
| **AI Dataset** | Datasets, databases, or data APIs |
| **AI Application** | End-user AI application (web, mobile, desktop) |
| **AI Infrastructure** | Deployment, orchestration, monitoring tools |
| **AI Skill** | Reusable AI capability or function |
| **AI Knowledge Base** | RAG, embeddings, vector databases |
| **AI Collection** | Curated lists, registries, or collections |
| **AI Tutorial** | Tutorials, guides, and educational content |
| **AI Registry** | Registry or directory of AI resources |

## Registry Views

The pipeline generates multiple registry views for different consumers:

| View | Description |
|---|---|
| `registry.json` | Full registry with all entity data |
| `registry.min.json` | Compact version for web clients |
| `categories.json` | Entity distribution by category |
| `sources.json` | Distribution by discovery source |
| `statistics.json` | Aggregate statistics |
| `health.json` | Health status per entity |
| `REGISTRY.md` | Human-readable markdown registry |
| `awesome-taiwan-mcp.md` | Legacy MCP-only view (backward compatible) |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI (cmd/crawler, cmd/migrate, cmd/export) │
│  Commands: run, discover, classify, verify, scan, score, migrate, export │
└─────────────┬───────────────────────────────────────────────┘
              │
┌─────────────▼───────────────────────────────────────────────┐
│                   PipelineCoordinator                        │
│  Stage interface + Pipeline struct (internal/coordinator/stages.go) │
│  Orchestrates the full pipeline (12 stages):               │
│  1. Discover + Fetch  2. Normalize  3. Classify             │
│  4. Taiwan Relevance  5. AI Relevance  6. MCP Identity     │
│  7. Endpoint Classification  8. Runtime Verify              │
│  9. Security Scan  10. Quality Score  11. Persist  12. Export│
└───────────────────────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┬──────────────────┬──────────────────┐
    │                   │                  │                  │
    ▼                   ▼                  ▼                  ▼
  Sources           Storage            Engines           Export
  (adapters)    (SQLite store)      (verify, security,   (JSON + MD)
    │                   │                  │
    │                   │                  │
    ▼                   ▼                  ▼
  GitHub          SQLite DB        MCP Protocol
  Registry                        Health Check
  mcpservers.org                  Security Scan
  (Sitemap-based)                 Quality Scoring
  modelcontextprotocol/servers    Taiwan Relevance
  (archived)                      AI Relevance

┌─────────────────────────────────────────────────────────────┐
│  Migration Pipeline (cmd/migrate)                          │
│  Load → Normalize → Classify → Taiwan/AI Score →           │
│  MCP Identity → Runtime Verify → Security Scan →           │
│  Quality Score → Save (with dry-run + checkpoint/resume)  │
└─────────────────────────────────────────────────────────────┘
```

## Project Structure

```
├── cmd/
│   ├── crawler/              # Main CLI entry point (cobra)
│   │   └── main.go           # Commands: run, discover, classify, verify, scan, score, migrate, export
│   ├── migrate/              # Migration CLI (cmd/migrate/main.go)
│   │   └── main.go           # Full pipeline: load→normalize→classify→score→verify→scan→save
│   └── export/               # Standalone export tool (cmd/export/main.go)
│       └── main.go           # Reads from DB, calls ViewGenerator
├── internal/
│   ├── classify/             # Taiwan relevance classification
│   │   ├── keywords.go       # Keyword matching (embedded config)
│   │   ├── llm.go            # LLM classifier (OpenAI-compatible API)
│   │   └── rules.go          # Scoring rules (official domain, gov API, etc.)
│   ├── coordinator/          # Pipeline orchestration
│   │   ├── coordinator.go    # CrawlCoordinator
│   │   └── stages.go         # Stage interface + Pipeline struct
│   ├── crawler/              # Crawler pipeline
│   │   ├── coordinator.go    # Pipeline orchestration
│   │   ├── incremental.go    # IncrementalCrawler
│   │   └── run/              # Crawl run management
│   ├── dedupe/               # Deduplication engine
│   ├── engines/              # Classification and verification engines
│   │   ├── classifier.go     # Entity classification (12+ types)
│   │   ├── mcp_identity.go   # MCP identity detection engine
│   │   ├── runtime_verifier.go # MCP protocol runtime verification
│   │   ├── security_scanner.go # Security scanning engine
│   │   ├── quality_engine.go  # Quality scoring engine
│   │   ├── taiwan_relevance.go # Taiwan relevance engine
│   │   ├── ai_relevance.go    # AI relevance engine
│   │   ├── endpoint_classifier.go # Endpoint URL classification
│   │   └── acceptance_test.go  # Acceptance test suite (spec §56)
│   ├── evidence/             # Evidence collection
│   ├── export/               # Export and view generation
│   │   ├── exporter.go       # Legacy markdown export
│   │   └── view_generator.go # Registry view generation
│   ├── health/               # Endpoint health checking
│   ├── manifest/             # MCP manifest detection
│   ├── metrics/              # Structured logging + crawl metrics
│   ├── models/               # Data models (Entity, MCPIdentity, etc.)
│   ├── normalize/            # Normalizer (RawRecord → Entity)
│   ├── retry/                # Retry client with exponential backoff
│   ├── scoring/             # Quality scoring engine (10 components)
│   ├── search/              # Search engine (text + capability)
│   ├── security/            # Security scanner
│   ├── sources/             # Source adapters
│   │   ├── github/          # GitHub repo discovery
│   │   ├── githubrepo/      # GitHub directory-based
│   │   ├── mcpmarket/       # mcpmarket.com
│   │   ├── mcpserversorg/   # mcpservers.org via Sitemap
│   │   └── registry/        # Official registry adapter
│   ├── storage/             # SQLite persistence layer
│   └── verify/              # Repository + MCP protocol verification
├── config/
│   └── pipeline.yaml        # Pipeline configuration
├── tests/
│   ├── fixtures/            # JSON test fixtures
│   │   ├── acceptance/      # Acceptance test fixtures
│   │   ├── golden/          # Golden regression test data
│   │   ├── ground_truth/    # FP rate test ground truth (50 pos, 100 neg)
│   │   └── ...              # Other test fixtures
│   ├── integration/          # E2E pipeline tests
│   ├── unit/                 # Unit + golden regression tests
│   └── benchmarks/           # Performance benchmarks
├── migrations/               # Database schema migrations
├── Dockerfile                # Multi-stage: golang:1.26-alpine → alpine:latest
├── docker-compose.yaml       # crawler service
└── .golangci.yml             # Linter config
```

## Requirements

- **Go** 1.25+
- **GITHUB_TOKEN** — GitHub API token for repository discovery
- **OPENAI_API_KEY** — (optional) For LLM classification of ambiguous candidates
- **OPENAI_BASE_URL** — (optional) OpenAI-compatible API endpoint, defaults to `https://api.openai.com/v1`
- **Docker** — For container builds

## Installation

### Build from source

```bash
go build -o crawler ./cmd/crawler
```

### Docker

```bash
docker build -t awesome-taiwan-ai-ecosystem .
```

## Configuration

| Environment Variable | Required | Default | Description |
|---|---|---|---|
| `GITHUB_TOKEN` | Yes | — | GitHub API token for repository search and fetch |
| `OPENAI_API_KEY` | No | — | OpenAI-compatible API key for LLM classification |
| `OPENAI_BASE_URL` | No | `https://opencode.ai/zen/v1` | OpenAI-compatible API base URL |
| `OPENAI_MODEL` | No | — | Override model chain for **this crawler instance only** (does not affect other instances). Default fallback chain: `muse-spark-1.2-contributor-free` → `nemotron-3-ultra-free`. opencode.ai/zen/v1 requires bare IDs without `opencode/` prefix. When set, the crawler uses only this single model (no fallback).
CLI flags:

| Flag | Default | Description |
|---|---|---|
| `--source` | `all` | Source to crawl: `github`, `registry`, `mcpserversorg`, `mcpmarket`, or `all` |
| `--full` | `false` | Force full crawl |
| `--incremental` | `false` | Run incremental crawl (checks last crawl time) |
| `--db` | `./data/registry.db` | SQLite database path |
| `--config` | `config/sources.yaml` | Config file path |
| `--markdown` | `false` | (export subcommand) Also generate REGISTRY.md |
| `--capability` | — | (search subcommand) Search by capability keywords |
| `--min-score` | `0` | Minimum quality score filter |
| `--level` | — | Filter by Taiwan relevance level (T0-T5) |
| `--category` | — | Filter by category |

## Quick Start

```bash
# 1. Build the CLIs
go build -o crawler ./cmd/crawler
go build -o migrator ./cmd/migrate
go build -o exporter ./cmd/export

# 2. Crawl Taiwan AI entities (GitHub source)
export GITHUB_TOKEN=your_github_token_here
./crawler run --source github --workers 4 --max-per-source 10

# Also crawl mcpservers.org (10k+ servers via Sitemap)
./crawler run --source mcpserversorg --workers 2 --max-per-source 100

# 3. Run migration pipeline (reclassify existing entities)
./migrator --db ./data/registry.db --dry-run

# 4. Export registry views
./exporter --db ./data/registry.db --markdown

# 5. Search entities
./crawler search "taiwan"
./crawler search --capability "filesystem"

# 6. View stats
./crawler stats
```

### CLI Commands

```bash
# Main CLI (cmd/crawler)
crawler run        # Full pipeline: discover → classify → verify → score → persist
crawler discover   # Only discovery stage (fetch from sources)
crawler classify   # Only classification stage (classify existing entities)
crawler verify     # Only runtime verification stage
crawler scan       # Only security scanning stage
crawler score      # Only quality scoring stage
crawler migrate    # Migration pipeline (reclassify all entities)
crawler export     # Export registry views (JSON + Markdown)
crawler search     # Search the registry
crawler stats      # View aggregate statistics

# Migration CLI (cmd/migrate) - standalone full pipeline
migrator --db ./data/registry.db --dry-run    # Dry run (no writes)
migrator --db ./data/registry.db --resume     # Resume from checkpoint
migrator --db ./data/registry.db              # Full migration with writes

# Export CLI (cmd/export) - standalone export tool
exporter --db ./data/registry.db --markdown   # Generate all views + markdown
```

## Usage

### Run (Full Pipeline)

```bash
# Full pipeline run (force refresh all)
./crawler run --full --source all

# Incremental run (only check for updates)
./crawler run --incremental --source github

# Limit candidates per source
./crawler run --source github --max-per-source 20
```

### Migration

```bash
# Migrate existing database records (recompute all scores)
./migrator --db ./data/registry.db --dry-run

# Resume interrupted migration
./migrator --db ./data/registry.db --resume
```

### Export

```bash
# Export JSON registry (6 files) via standalone export CLI
./exporter --db ./data/registry.db --markdown

# Or using the main CLI
./crawler export --markdown
```

Output files in `registry/`:
- `registry.json` — Full registry with all entity data
- `registry.min.json` — Compact version for web clients
- `categories.json` — Category distribution
- `sources.json` — Source distribution
- `statistics.json` — Aggregate statistics
- `health.json` — Health status per server
- `REGISTRY.md` — Human-readable markdown registry (with `--markdown`)
- `awesome-taiwan-mcp.md` — Legacy MCP-only view (backward compatible)

### Markdown Registry

The `--markdown` flag generates a human-readable `REGISTRY.md` file organized by functional category:

- **Statistics**: Total servers, Taiwan relevance distribution, health, quality grades
- **🇹🇼 Taiwan-relevant Servers**: Servers with T1-T5 relevance, grouped by functional category (Finance, Government, Real Estate, etc.)
- **🌍 International Servers**: T0 servers not Taiwan-specific but MCP-compatible
- **Per-server details**: Repository link (with star count), language (linked to GitHub search), Taiwan relevance level + score, classification evidence, health, quality, tools, endpoints

Category mapping is defined in `config/categories.yaml` (single source of truth). Sub-categories like `stock`, `etf`, `banking` are normalized to parent `finance`, `land`/`housing` → `real-estate`, etc. The `Other` category only collects servers with no matching category.
Taiwan relevance levels:
- **T5**: Definitively Taiwan-focused — official government or financial APIs with Taiwan-specific data
- **T4**: Very strong Taiwan relevance — Taiwan data sources with clear local focus
- **T3**: Strong Taiwan relevance — Taiwan-specific data or services (real estate, finance, etc.)
- **T2**: Moderate Taiwan relevance — some Taiwan content or keywords detected
- **T1**: Weak Taiwan relevance — minimal Taiwan connection
- **T0**: No Taiwan relevance — international or general-purpose server

```bash
# Text search
./crawler search "financial"
./crawler search --level T3

# Capability search
./crawler search --capability "filesystem"
./crawler search --capability "database"

# Filter by quality
./crawler search --min-score 70

# JSON output
./crawler search "taiwan" --json
```

### Stats

```bash
./crawler stats
```

## Data Model

### Entity (canonical model)

| Field | Type | Description |
|---|---|---|
| `id` | `string` | Canonical ID (SHA256 of normalized repo URL) |
| `name` | `string` | Display name |
| `slug` | `string` | URL-safe slug |
| `description` | `string` | Short description |
| `classification` | `ClassificationResult` | Primary + secondary classification |
| `taiwan_relevance` | `TaiwanRelevance` | Taiwan classification (level T0-T5, score, confidence, evidence) |
| `ai_relevance` | `AIRelevance` | AI relevance (level A0-A4, score, confidence) |
| `mcp_identity` | `MCPIdentity` | MCP identity status (CANDIDATE, STATIC_VERIFIED, RUNTIME_VERIFIED, NOT_MCP) |
| `repository` | `RepositoryInfo` | Repository metadata (URL, owner, topics, package files) |
| `endpoints` | `[]EndpointWithType` | Classified endpoints |
| `tools` | `[]Tool` | Extracted tools |
| `resources` | `[]Resource` | Extracted resources |
| `prompts` | `[]Prompt` | Extracted prompts |
| `quality` | `QualityScore` | 100-point quality assessment (score, grade A-F) |
| `security_status` | `SecurityStatusDetail` | Security scan results |
| `runtime_verification` | `*RuntimeVerification` | MCP protocol runtime verification result |
| `first_seen` / `last_seen` | `RFC3339Time` | Discovery timestamps |
| `sources` | `[]SourceReference` | Discovery source references with trust scores |

### Entity Status Lifecycle

```
CANDIDATE → STATIC_VERIFIED → RUNTIME_VERIFIED → PUBLISHED
        ↘ NOT_MCP
```

### AI Relevance Levels

| Level | Score Range | Meaning |
|---|---|---|
| A0 | 0 | No AI relevance |
| A1 | 1-25 | Weak AI relevance |
| A2 | 26-50 | Moderate AI relevance |
| A3 | 51-75 | Strong AI relevance |
| A4 | 76-100 | Definitively AI-related |

## Scoring

### Taiwan Relevance (§17)

| Rule | Score | Evidence Type |
|---|---|---|
| Official Taiwan domain (.gov.tw, .org.tw) | +40 | `official_domain` |
| Taiwan government API detected | +40 | `official_gov_api` |
| Taiwan financial API (TWSE, TPEx, FinMind) | +35 | `taiwan_financial_api` |
| Taiwan-specific dataset detected | +30 | `taiwan_dataset` |
| Taiwan keyword in repo name/description | +20 | `repository_keyword` |
| Taiwan language (zh-TW, Traditional Chinese) | +15 | `taiwan_language` |
| Taiwan company/service detected | +15 | `taiwan_company` |
| README mentions Taiwan | +5 | `readme_mention` |

Levels: T0 (0-19), T1 (20-35), T2 (36-55), T3 (56-70), T4 (71-85), T5 (86-100)

### Quality Score (§31)

10 components scored 0-10 each:
- Repository stars
- Repository activity (updated within 90 days)
- README completeness
- Documentation (CONTRIBUTING, LICENSE, etc.)
- Manifest file present (claude.json, config.json, etc.)
- MCP protocol compliance
- Transport support (stdio + HTTP)
- Tool count (>0)
- Resource count (>0)
- Prompt count (>0)

Grades: A (90-100), B (80-89), C (70-79), D (60-69), F (0-59)

## Error Handling

- **Rate limiting**: Exponential backoff (1s → 2s → 4s → 8s, capped at 30s, max 3 retries)
- **Source degradation**: Failed sources are logged and skipped, pipeline continues
- **LLM failures**: Fallback to deterministic T0 classification, server metadata unchanged
- **Network timeouts**: Context-cancellable throughout pipeline
- **SQL errors**: Individual server save failures logged, pipeline continues

## Testing

```bash
# All tests
go test ./... -count=1 -timeout=120s

# Acceptance tests (spec §56)
go test ./internal/engines/... -run TestAcceptance -v -count=1

# False Positive Rate test (spec §58)
go test ./internal/engines/... -run TestFPRate -v -count=1

# With race detector
go test -race ./internal/... -count=1 -timeout=120s

# Coverage (by package)
go test ./... -count=1 -cover

# Golden regression tests
go test ./tests/unit/ -v -run Golden

# Benchmarks
go test ./tests/benchmarks/ -bench=. -benchmem

# Integration tests
go test ./tests/integration/ -v

# CI pipeline (equivalent)
go build ./... && go vet ./... && go test ./... -cover -timeout 120s
```

Coverage by package:

| Package | Coverage |
|---|---|
| `internal/classify` | 87.6% |
| `internal/coordinator` | 90.0% |
| `internal/dedupe` | 90.7% |
| `internal/engines` | 85.0% |
| `internal/evidence` | 100.0% |
| `internal/export` | 87.0% |
| `internal/health` | 91.7% |
| `internal/manifest` | 92.9% |
| `internal/metrics` | 100.0% |
| `internal/normalize` | 90.0% |
| `internal/scoring` | 94.0% |
| `internal/security` | 93.2% |
| `internal/storage` | 85.9% |
| `internal/verify` | 94.7% |

## Build

```bash
# Standard build
go build ./...

# Vet
go vet ./...

# Module verification
go mod verify

# Docker
docker build -t awesome-taiwan-ai-ecosystem .
docker compose up
```

### Docker

The Dockerfile uses multi-stage build:
1. **Builder**: `golang:1.26-alpine3.24` — compiles the binary
2. **Runtime**: `alpine:latest` — runs the binary as non-root user

Security: non-root user (uid 1000), no privileged, resource limits.

```bash
docker build -t awesome-taiwan-ai-ecosystem .
docker run --rm \
  -e GITHUB_TOKEN=your_token \
  -v $(pwd)/data:/data \
  awesome-taiwan-ai-ecosystem crawl --db /data/registry.db
```

## Development

```bash
# Install dependencies
go mod download

# Run linter
golangci-lint run

# Format
gofmt -s -w .

# Run all tests
go test ./... -count=1 -timeout=120s

# Run acceptance tests (spec §56)
go test ./internal/engines/... -run TestAcceptance -v -count=1

# Run FP rate test (spec §58)
go test ./internal/engines/... -run TestFPRate -v -count=1

# Run tests with verbose output for a specific package
go test ./internal/classify/ -v
```

## Known Limitations

- **Official registry source**: `api.mcp-servers.dev` may be unavailable in sandboxed environments (DNS resolution failure)
- **GitHub rate limits**: 5000 requests/hour per token; 404 keywords × 2s = ~88s for full discovery
- **LLM classifier**: Requires `OPENAI_API_KEY` env var; gracefully degrades to deterministic classification
- **Incremental crawl**: Uses last crawl timestamp from SQLite; requires prior crawl data
- **Docker compose**: Runs `--help` by default; must override command for actual crawling
- **MCP protocol verification**: Requires publicly accessible HTTP endpoints; local endpoints (localhost) may fail

## License

See `LICENSE` file.
