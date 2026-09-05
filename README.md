<div align="center">

[English](README.md) | [繁體中文](README.zh-TW.md) | [简体中文](README.zh-CN.md)

</div>

# Awesome Taiwan MCP

Automated crawler for discovering, analyzing, and verifying Taiwan-related MCP Servers.

## Overview

**Awesome Taiwan MCP** crawls multiple sources (GitHub, official registries) to discover MCP (Model Context Protocol) servers with Taiwan relevance, normalizes them, deduplicates, classifies Taiwan relevance, verifies health and protocols, scores quality, and exports a standardized registry.

The crawler identifies servers related to Taiwan through keyword matching, official domains (e.g. `.gov.tw`, `.org.tw`), government APIs, financial APIs (TWSE, TPEx), real estate data, and Traditional Chinese language detection.

**Pipeline:** Discovery → Normalize → Taiwan Scoring → LLM Classification → Dedup → Verify → Health Check → Quality Score → Persist → Export

## Features

- **Multi-source discovery**: GitHub repositories, official MCP registries, mcpservers.org (via Sitemap)
- **Quality scoring**: 10-component quality assessment (A-F grade)
- **Security scanning**: Injection patterns, unsafe transport, fork detection
- **Protocol verification**: Full MCP protocol (initialize, tools/list, resources/list, prompts/list)
- **Health checking**: Endpoint latency and availability monitoring
- **Incremental crawling**: `--incremental` flag only re-crawls changed candidates
- **LLM classification**: Ambiguous candidates (score 20-55) are classified via OpenAI-compatible LLM API
- **JSON registry export**: registry.json, registry.min.json, categories.json, sources.json, statistics.json, health.json
- **Markdown export**: Human-readable REGISTRY.md with full server details
- **Search**: Text search and capability-based search
- **SQLite persistence**: All data stored in SQLite (modernc.org/sqlite, pure Go)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI (cmd/crawler)                      │
│              Commands: crawl, export, search, stats         │
└─────────────┬───────────────────────────────────────────────┘
              │
┌─────────────▼───────────────────────────────────────────────┐
│                   CrawlCoordinator                           │
│  Orchestrates the full pipeline (8 stages):                   │
│  1. Discover + Fetch  2. Normalize  3. Taiwan Score           │
│  3b. LLM Classification  4. Quality Score  5. Identity        │
│  6. Dedup  6.5. Verify  7. Persist  8. Finish                │
└───────────────────────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┬──────────────────┬──────────────────┐
    │                   │                  │                  │
    ▼                   ▼                  ▼                  ▼
  Sources           Storage            Verify           Export
  (adapters)    (SQLite store)      (repo/proto)     (JSON + MD)
    │                   │                  │
    ▼                   ▼                  ▼
  GitHub          SQLite DB           Health
  Registry                           Security
    mcpservers.org                   Scoring
  (Sitemap-based)
  modelcontextprotocol/servers-archived
                                          │
                                          ▼
                                       Scoring
```

## Project Structure

```
├── cmd/crawler/
│   └── main.go               # CLI entry point (cobra)
├── internal/
│   ├── classify/             # Taiwan relevance classification
│   │   ├── keywords.go       # Keyword matching (embedded config)
│   │   ├── llm.go            # LLM classifier (OpenAI-compatible API)
│   │   └── rules.go          # Scoring rules (official domain, gov API, etc.)
│   ├── crawler/              # Pipeline orchestration
│   │   ├── coordinator.go    # CrawlCoordinator (8-stage pipeline)
│   │   ├── incremental.go    # IncrementalCrawler
│   │   └── run/              # Crawl run management
│   ├── dedupe/               # Deduplication engine
│   ├── evidence/             # Evidence collection
│   ├── health/               # Endpoint health checking
│   ├── manifest/             # MCP manifest detection
│   ├── metrics/              # Structured logging + crawl metrics
│   ├── models/               # Data models (MCPServer, etc.)
│   ├── normalize/            # Normalizer (RawRecord → MCPServer)
│   ├── retry/                # Retry client with exponential backoff
│   ├── scoring/              # Quality scoring engine (10 components)
│   ├── search/               # Search engine (text + capability)
│   ├── security/             # Security scanner
│   ├── sources/              # Source adapters
│   │   ├── github/           # GitHub repo discovery
│   │   ├── githubrepo/       # GitHub directory-based (modelcontextprotocol/servers, servers-archived)
│   │   ├── mcpmarket/        # mcpmarket.com (skeleton, blocked by Vercel WAF)
│   │   ├── mcpserversorg/    # mcpservers.org via Sitemap + goquery
│   │   └── registry/         # Official registry adapter
│   └── verify/               # Repository + MCP protocol verification
├── config/
│   ├── keywords.yaml         # Taiwan keyword matrix
│   └── domains.yaml          # Official Taiwan domains
├── tests/
│   ├── fixtures/             # JSON test fixtures
│   ├── integration/          # E2E pipeline tests
│   ├── unit/                 # Unit + golden regression tests
│   └── benchmarks/           # Performance benchmarks
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
# 1. Crawl Taiwan MCP servers
export GITHUB_TOKEN=your_github_token_here
./crawler crawl --source github --workers 4 --max-per-source 10

# Also crawl mcpservers.org (10k+ servers via Sitemap)
./crawler crawl --source mcpserversorg --workers 2 --max-per-source 100

# 2. Export registry
./crawler export --markdown
```

# 3. Search servers
./crawler search "taiwan"
./crawler search --capability "filesystem"

# 4. View stats
./crawler stats
```

## Usage

### Crawl

```bash
# Full crawl (force refresh all)
./crawler crawl --full --source all

# Incremental crawl (only check for updates)
./crawler crawl --incremental --source github

# Limit candidates per source
./crawler crawl --source github --max-per-source 20
```

### Export

```bash
# Export JSON registry (6 files)
./crawler export

# Also generate human-readable markdown
./crawler export --markdown
```

Output files in `registry/`:
- `registry.json` — Full registry with all server data
- `registry.min.json` — Compact version for web clients
- `categories.json` — Category distribution
- `sources.json` — Source distribution
- `statistics.json` — Aggregate statistics
- `health.json` — Health status per server
- `REGISTRY.md` — Human-readable markdown (with `--markdown`)

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

### MCPServer

| Field | Type | Description |
|---|---|---|
| `id` | `string` | SHA256 of normalized repo URL (CanonicalID) |
| `name` | `string` | Display name |
| `slug` | `string` | URL-safe slug |
| `description` | `string` | Short description |
| `taiwan_relevance` | `TaiwanRelevance` | Taiwan classification (level T0-T5, score, confidence, evidence) |
| `repository` | `RepositoryInfo` | GitHub repository metadata |
| `endpoints` | `[]Endpoint` | MCP endpoints (URL, transport, TLS) |
| `tools` | `[]Tool` | Extracted tools |
| `resources` | `[]Resource` | Extracted resources |
| `prompts` | `[]Prompt` | Extracted prompts |
| `quality` | `QualityScore` | 100-point quality assessment (score, grade A-F) |
| `security` | `[]SecurityFinding` | Security findings |
| `health` | `HealthStatus` | HEALTHY, DEGRADED, UNAVAILABLE, UNKNOWN |

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
```

Coverage by package:

| Package | Coverage |
|---|---|
| `internal/classify` | 87.6% |
| `internal/dedupe` | 90.7% |
| `internal/evidence` | 100.0% |
| `internal/export` | 87.0% |
| `internal/health` | 91.7% |
| `internal/manifest` | 92.9% |
| `internal/metrics` | 100.0% |
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

# Run tests with verbose output
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
