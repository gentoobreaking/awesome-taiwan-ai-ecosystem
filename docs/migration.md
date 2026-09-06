# Migration Guide: v0.x → v1.0

This guide covers migrating from the legacy MCP-centric schema to the new
canonical **Entity Model** (spec §61 Phase 1, §62).

## Overview

The old registry stored data in a single `mcp_servers` table with a flat
`MCPServer` struct. The new architecture introduces a unified `Entity` model
that supports 25+ classification types (MCP_SERVER, MCP_CLIENT, AI_AGENT,
AI_DATASET, etc.) with independent dimensions for Taiwan relevance, AI
relevance, MCP identity, security status, and quality score.

## Breaking Changes

### 1. Entity Model

| Old Type | New Type |
|---|---|
| `MCPServer` | `Entity` (with `Classification.Primary == MCP_SERVER`) |
| `server.Tools` | `entity.Tools` |
| `server.Endpoints` (flat) | `entity.Endpoints` (`[]EndpointWithType` with type classification) |
| `server.TaiwanRelevance.Level` (bool `taiwan_relevant`) | `entity.TaiwanRelevance.Level` (T0–T5) |

### 2. Status System

The old `Status` field (boolean flags like `verified`, `deleted`) is replaced
by a multi-dimensional status system:

- **`EntityStatus`**: `DISCOVERED`, `CANDIDATE`, `VERIFIED`, `QUARANTINED`, `REJECTED`
- **`MCPIdentityStatus`**: `CANDIDATE`, `STATIC_VERIFIED`, `RUNTIME_VERIFIED`, `NOT_MCP`
- **`SecurityStatus`**: `CLEAN`, `SUSPICIOUS`, `QUARANTINED`, `BLOCKED`

### 3. Database Schema

The old `mcp_servers`, `sources`, `crawl_runs`, `security_findings` tables are
replaced by the V2 schema:

```sql
-- Entities table (canonical)
entities
entity_evidence
entity_endpoints
migration_log
```

The old tables are backed up as `mcp_servers_v1_backup` during migration.

### 4. CLI Changes

Old command | New command
--- | ---
`crawler crawl` (old) | `crawler run` (full pipeline) or `crawler crawl` (legacy)
`crawler export` | `crawler export` (with `--markdown`, `--malicious`, `--injection` flags)
N/A | `crawler migrate` (V1→V2 migration)
N/A | `crawler discover`, `classify`, `verify`, `scan`, `score`

### 5. Registry Views

The old single `awesome-taiwan-mcp.md` is now one of several generated views:

```
taiwan-ai-ecosystem.{md,json}      — All Taiwan AI entities
taiwan-mcp.{md,json}               — Verified MCP Servers only (RUNTIME_VERIFIED)
taiwan-mcp-candidates.{md,json}    — MCP Candidates (CANDIDATE/STATIC_VERIFIED)
taiwan-ai-agents.{md,json}         — AI Agents
taiwan-ai-tools.{md,json}          — AI Tools/SDKs/Frameworks/Plugins
taiwan-ai-data.{md,json}           — Datasets/Knowledge bases/APIs
taiwan-ai-skills.{md,json}         — AI/MCP Skills
taiwan-ai-infrastructure.{md,json} — AI Infrastructure
taiwan-ai-tutorials.{md,json}      — Tutorials/Examples
awesome-taiwan-mcp.md              — Backward-compatible MCP-only view (RUNTIME_VERIFIED only)
```

## How to Run the Migration

### Step 1: Build the migration CLI

```bash
go build -o bin/migrate ./cmd/migrate
```

### Step 2: Run the migration

```bash
# Full migration (reclassifies all entities)
./bin/migrate --input-db=./data/registry_v1.db --output-db=./data/registry.db

# Dry-run (no write, just stats)
./bin/migrate --input-db=./data/registry_v1.db --output-db=./data/registry.db --dry-run

# Resume from checkpoint (if interrupted)
./bin/migrate --input-db=./data/registry_v1.db --output-db=./data/registry.db --resume
```

### Step 3: Export the new registry

```bash
go build -o bin/crawler ./cmd/crawler
./bin/crawler export
```

## Migration Report

The migration CLI outputs a JSON report with statistics:

```json
{
  "total": 561,
  "classification": {"MCP_SERVER": 23, "AI_AGENT": 45, ...},
  "status": {"VERIFIED": 100, "QUARANTINED": 5, ...},
  "errors": [...]
}
```

## Expected Impact

- **MCP_SERVER count will decrease significantly** — the old registry included
  tutorials, clients, collections, and SDK repos as "MCP servers" by name.
  The new classifier correctly categorizes these (spec §59).
- **Taiwan relevance is recomputed** — `taiwan_relevant: true` is NOT copied;
  the Taiwan relevance engine recalculates scores (spec §52).
- **All entities get runtime verification** — only `RUNTIME_VERIFIED` MCP
  servers appear in the `taiwan-mcp.md` view (spec §54).

## Rollback

The migration is designed to be **idempotent** and **re-runnable**. The old
data is preserved in `mcp_servers_v1_backup`. To rollback:

```bash
sqlite3 registry.db "DROP TABLE entities, entity_evidence, entity_endpoints, migration_log;"
# Old data is still in mcp_servers_v1_backup
```

## Frequently Asked Questions

### Q: Why did my MCP server count drop after migration?

The old registry counted any project mentioning "MCP" as a server. The new
classifier applies strict rules: an entity must have actual MCP server
implementation code (McpServer struct, transport, tool definitions, entrypoint)
to be classified as `MCP_SERVER` (spec §56 Test 3). Tutorials, clients, SDK
packages, collections, and AI agents using MCP are classified differently.

### Q: Can I still get the old `awesome-taiwan-mcp.md`?

Yes — the `awesome-taiwan-mcp.md` file is still generated as a backward-
compatible view. However, it now only includes `RUNTIME_VERIFIED` MCP servers
(rather than all servers with `taiwan_relevant: true`).

### Q: How do I migrate my CI/CD pipeline?

The `crawler export` command now generates all registry views automatically.
Update your pipeline to read from `registry/taiwan-*.json` instead of
`registry.json`/servers.json` directly.
