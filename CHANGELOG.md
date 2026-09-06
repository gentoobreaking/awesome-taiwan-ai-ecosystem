# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Pipeline Stages** (`internal/coordinator/stages.go`): New `Stage` interface and `Pipeline` struct for orchestrating pipeline stages. Supports `Register`, `Run`, and `Stages` methods.
- **Migration CLI** (`cmd/migrate/main.go`): Standalone migration pipeline that runs the full classification pipeline (load → normalize → classify → Taiwan/AI score → MCP identity → runtime verify → security scan → quality score → save). Supports `--dry-run` and `--resume` (checkpoint/resume).
- **Export CLI** (`cmd/export/main.go`): Standalone export tool that reads from the database and calls `ViewGenerator` to produce all registry views.
- **Legacy markdown view** (`internal/export/view_generator.go`): Generates `awesome-taiwan-mcp.md` as a backward-compatible MCP-only view using `models.MCPServerView` and `legacyServerMarkdown()`.
- **Acceptance test suite** (`internal/engines/acceptance_test.go`): 12 test cases from spec §56 covering MCP keyword detection, SDK dependencies, server implementation, runtime verification, endpoint classification (GitHub/Documentation/Installer URLs), collections, tutorials, data SDKs, AI agents, and suspicious code detection.
- **False Positive Rate test** (`internal/engines/fp_rate_test.go`): KPI test with 50 positive + 100 negative ground truth samples. Computes FPR, Precision, Recall, and F1 score. Threshold: FPR < 5% (PASS), FPR < 2% (EXCELLENT).
- **Ground truth fixtures** (`tests/fixtures/ground_truth/`): 150 labeled samples for FP rate testing.
- **Test MCP server fixture** (`tests/fixtures/acceptance/mcp-test-server/server.py`): Minimal Python MCP stdio server for runtime verification testing.
- **Configuration file** (`config/pipeline.yaml`): Pipeline configuration for the new stage-based pipeline.

### Changed

- **MCP Identity Engine** (`internal/engines/mcp_identity.go`): `DetectMCPIdentity` now correctly returns `STATIC_VERIFIED` (not `RUNTIME_VERIFIED`) for server implementations detected via static analysis. Runtime verification status is set separately by the `RuntimeVerifier`.
- **Runtime Verifier** (`internal/engines/runtime_verifier.go`): Fixed `initializeResult.Capabilities` field type from `map[string]bool` to `map[string]json.RawMessage` to properly parse MCP protocol responses with nested capability objects.
- **Endpoint Classifier** (`internal/engines/endpoint_classifier.go`): Added `stdio:` URL prefix detection to `isPotentialMCPRuntimeEndpoint` for proper classification of stdio-based MCP server endpoints.

### Removed

- (No removals in this iteration.)

## [1.0.0] - 2026-09-06

### Added

- Initial release of the Taiwan AI Ecosystem Registry crawler.
- Multi-source discovery: GitHub, official registries, mcpservers.org.
- Entity classification: 18+ types including MCP Server, Client, Host, SDK, Library, AI Agent, Tool, Dataset, etc.
- Taiwan relevance scoring (T0-T5 levels).
- AI relevance scoring (A0-A4 levels).
- MCP identity detection (CANDIDATE → STATIC_VERIFIED → RUNTIME_VERIFIED).
- Runtime protocol verification (MCP initialize + tools/list handshake).
- Security scanning (credential extraction, shell injection, localhost exposure).
- Quality scoring (10-component, 100-point scale, A-F grades).
- SQLite persistence with modernc.org/sqlite (pure Go).
- JSON registry export (registry.json, registry.min.json, categories.json, sources.json, statistics.json, health.json).
- Markdown registry export (REGISTRY.md, awesome-taiwan-mcp.md).
- Text and capability-based search.
- Acceptance test suite (spec §56, 12 test cases).
- False Positive Rate test (spec §58, 150 ground truth samples).

### Security

- Non-root Docker container (uid 1000).
- Resource limits in Docker compose.
- Security scan as part of pipeline CI.
