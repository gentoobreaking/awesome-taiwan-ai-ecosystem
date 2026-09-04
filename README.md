# Taiwan MCP Crawler

Automated crawler for discovering, analyzing, and verifying Taiwan-related MCP Servers.

## Overview

The Taiwan MCP Crawler continuously discovers MCP Servers from multiple sources (GitHub, Glama, PulseMCP, MCP.so, Official Registry), normalizes them, deduplicates, classifies Taiwan relevance, verifies health, scores quality, and exports a standardized registry.

## Pipeline

```text
Discovery → Candidate → Normalize → Deduplicate → Taiwan Relevance Detection
→ Repository/Endpoint Verification → Capability Extraction → Health Check
→ Quality Scoring → Registry
```

## MVP Scope (Phase 1)

- GitHub + Official MCP Registry discovery
- Deterministic Taiwan relevance classification (T0–T5)
- Repository verification + MCP protocol verification
- Quality scoring (100-point, 10 components)
- JSON registry export (6 files)
- SQLite persistence

## Deferred (Phase 2+)

- Glama / PulseMCP / MCP.so adapters
- LLM ambiguous classification
- Historical snapshots
- REST API + Web UI

(See `spec.md` for full scope and task list.)
