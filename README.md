# Taiwan MCP Crawler

Automated crawler for discovering, analyzing, and verifying Taiwan-related MCP Servers.

## Overview

The Taiwan MCP Crawler continuously discovers MCP Servers from multiple sources (GitHub, Official Registry), normalizes them, deduplicates, classifies Taiwan relevance, verifies health, scores quality, and exports a standardized registry.

**Pipeline:** `Discovery → Candidate → Normalize → Deduplicate → Taiwan Relevance Detection → Repository/Endpoint Verification → Capability Extraction → Health Check → Quality Scoring → Registry`

## Quick Start

```bash
# Build
go build ./cmd/crawler

# Run a crawl
./crawler crawl --source github --workers 4

# Export registry
./crawler export

# Search servers
./crawler search "taiwan"

# View stats
./crawler stats
```

## Installation

### From source

```bash
git clone <repo>
cd awesome-taiwan-mcp
go build -o crawler ./cmd/crawler
```

### Docker

```bash
docker build -t awesome-taiwan-mcp .
docker run --rm awesome-taiwan-mcp version
```

### Docker Compose

```bash
docker compose up -d
```

## Commands

| Command | Description |
|---------|-------------|
| `crawl` | Run a crawl with options for source, workers, full/incremental |
| `export` | Export registry as JSON files |
| `search` | Search servers by text query |
| `stats` | Show crawl statistics |
| `version` | Print version information |

### Crawl flags

```
crawler crawl [--source <github\|registry\|all>] [--workers N] [--full] [--db <path>]
```

## Architecture

```
cmd/crawler/          CLI entry point (cobra)
internal/
  classify/     Taiwan relevance classification (T0–T5)
  dedupe/       Deduplication engine
  evidence/     Evidence collection
  health/       Endpoint health checks
  manifest/     Server manifest detection
  metrics/      Structured logging + metrics
  models/       Core data models
  normalize/    Raw→canonical normalization
  retry/        Exponential backoff retry
  scoring/      Quality scoring (10 components)
  search/       Search engine with ranking
  security/     Security scanning
  sources/      Source adapters (github, registry)
  storage/      SQLite persistence (modernc.org/sqlite)
  verify/       Repository + MCP protocol verification
config/         keywords.yaml, domains.yaml
Dockerfile      Multi-stage build (golang:1.26-alpine → alpine)
docker-compose.yaml  Production compose
tests/fixtures/ Test fixtures for all scenarios
```

## Data Model

Each MCP Server is represented as `models.MCPServer` with:

- **Repository**: URL, owner, name, language, stars, etc.
- **Taiwan Relevance**: T0–T5 classification with evidence
- **Quality Score**: 0–100 across 10 components
- **Health Status**: healthy, degraded, or unavailable
- **Tools**: Available tools extracted from the MCP server
- **Resources**: Available resources
- **Security**: Findings from security scanner

## Taiwan Relevance Levels

| Level | Meaning |
|-------|---------|
| T5 | Explicitly Taiwan-focused (gov API, .tw domain) |
| T4 | Strong Taiwan connection (TW entity, Taiwan org) |
| T3 | Moderate Taiwan connection |
| T2 | Some Taiwan connection |
| T1 | Possible Taiwan connection |
| T0 | No Taiwan connection |

## Quality Scoring

10 components (total 100 points):

1. Documentation (20 pts)
2. Description quality (10 pts)
3. Repository activity (10 pts)
4. Stars (10 pts)
5. Forks (5 pts)
6. License (5 pts)
7. Issues (5 pts)
8. Tests (10 pts)
9. Health status (15 pts)
10. Schema completeness (15 pts)

## Testing

```bash
go test ./...
```

### Coverage

- classify: 92%+
- dedupe: 90%+
- scoring: 94%+
- verify: 92%+
- health: 91%+
- security: 93%+

## License

MIT
