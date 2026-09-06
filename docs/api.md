# Registry API Documentation

> **Status**: Planned (Phase 4) — Not yet implemented. See [T048](../tasks/T048-rest-api.md).

The Registry API provides programmatic access to the Taiwan AI Ecosystem
Registry. It will allow searching, browsing, and retrieving entity metadata,
Taiwan/AI relevance scores, MCP identity status, quality scores, and security
status.

Base URL: `http://localhost:8080`

## Endpoints

### `GET /api/v1/health`

Health check endpoint.

**Response** `200 OK`
```json
{
  "status": "ok"
}
```

### `GET /api/v1/servers`

List all servers with pagination, filtering, and sorting.

**Query Parameters**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `limit` | int | 20 | Items per page |
| `level` | string | (all) | Filter by Taiwan relevance level (T0–T5) |
| `category` | string | (all) | Filter by primary classification |
| `min_score` | int | 0 | Minimum quality score |
| `sort` | string | `last_seen` | Sort field (`name`, `score`, `quality`, `last_seen`) |
| `order` | string | `desc` | Sort order (`asc`, `desc`) |
| `mcp_status` | string | (all) | Filter by MCP identity status (`RUNTIME_VERIFIED`, `STATIC_VERIFIED`, `CANDIDATE`, `NOT_MCP`) |
| `security_status` | string | (all) | Filter by security status (`CLEAN`, `SUSPICIOUS`, `QUARANTINED`, `BLOCKED`) |

**Response** `200 OK`
```json
{
  "data": [
    {
      "id": "sha256-abc123...",
      "name": "MCP Server Name",
      "description": "Description",
      "primary_classification": "MCP_SERVER",
      "mcp_role": "SERVER",
      "mcp_identity_status": "RUNTIME_VERIFIED",
      "taiwan_relevance": {
        "score": 85.0,
        "level": "T5",
        "confidence": 1.0
      },
      "ai_relevance": {
        "score": 75.0,
        "level": "A4",
        "confidence": 1.0
      },
      "quality": {
        "score": 92,
        "grade": "A"
      },
      "security_status": "CLEAN",
      "entity_status": "VERIFIED",
      "repository": {
        "url": "https://github.com/owner/repo",
        "stars": 100
      },
      "endpoints": [
        {
          "url": "https://api.example.com/mcp",
          "type": "MCP_RUNTIME_ENDPOINT"
        }
      ]
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 561,
    "total_pages": 29
  }
}
```

### `GET /api/v1/servers/{id}`

Get a single entity by ID.

**Path Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Entity ID (sha256 hex) |

**Response** `200 OK`
```json
{
  "id": "sha256-abc123...",
  "name": "MCP Server Name",
  "slug": "mcp-server-name",
  "description": "...",
  "classification": {
    "primary": "MCP_SERVER",
    "confidence": 0.95,
    "mcp_role": "SERVER",
    "evidence": [...]
  },
  "taiwan_relevance": {
    "score": 85.0,
    "level": "T5",
    "evidence": [...],
    "confidence": 1.0
  },
  "ai_relevance": {
    "score": 75.0,
    "level": "A4",
    "evidence": [...],
    "confidence": 1.0
  },
  "mcp_identity": {
    "status": "RUNTIME_VERIFIED",
    "confidence": 1.0,
    "role": "SERVER",
    "secondary_roles": [],
    "static_checked_at": "2026-01-01T00:00:00Z",
    "runtime_verified_at": "2026-01-02T00:00:00Z"
  },
  "runtime_verification": {
    "status": "PASSED",
    "initialize_result": {
      "success": true,
      "latency_ms": 50
    },
    "tools_list_result": {
      "success": true,
      "tool_count": 5,
      "latency_ms": 30
    }
  },
  "security_status": {
    "status": "CLEAN",
    "findings": [],
    "scanned_at": "2026-01-01T00:00:00Z"
  },
  "quality": {
    "score": 92,
    "grade": "A",
    "components": {...}
  },
  "repository": {...},
  "endpoints": [...],
  "tools": [...],
  "sources": [...],
  "entity_status": "VERIFIED",
  "first_seen": "2026-01-01T00:00:00Z",
  "last_seen": "2026-01-01T00:00:00Z"
}
```

**Response** `404 Not Found`
```json
{
  "error": "entity not found"
}
```

### `GET /api/v1/search`

Search entities by keyword.

**Query Parameters**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | (required) | Search keyword |
| `level` | string | (all) | Filter by Taiwan relevance level |
| `category` | string | (all) | Filter by primary classification |
| `min_score` | int | 0 | Minimum quality score |
| `limit` | int | 20 | Maximum results |

**Response** `200 OK`
```json
{
  "data": [...],
  "query": "mcp",
  "count": 42
}
```

### `GET /api/v1/registry`

Returns the full registry in JSON format (same as `registry.json`).

### `GET /api/v1/statistics`

Returns registry statistics.

**Response** `200 OK`
```json
{
  "total_entities": 561,
  "by_classification": {
    "MCP_SERVER": 23,
    "AI_AGENT": 45,
    ...
  },
  "by_taiwan_level": {
    "T5": 100,
    "T4": 50,
    ...
  },
  "by_quality_grade": {
    "A": 30,
    "B": 40,
    ...
  },
  "by_mcp_identity": {
    "RUNTIME_VERIFIED": 15,
    "STATIC_VERIFIED": 20,
    "CANDIDATE": 50,
    "NOT_MCP": 476
  }
}
```

## Rate Limiting

- Default: 100 requests per minute per IP
- `429 Too Many Requests` returned when limit exceeded
- Response includes `Retry-After` header

## Authentication

No authentication required for public registry data. Write operations
(not yet available) will require API keys.

## Errors

Common error responses:

```json
{
  "error": "description",
  "code": 400,
  "details": "..."
}
```

| HTTP Status | Meaning |
|-------------|---------|
| 200 | Success |
| 400 | Bad request (invalid parameters) |
| 404 | Not found |
| 429 | Rate limited |
| 500 | Internal server error |

## Implementation Notes

The API server will be implemented in Go using the standard library
`net/http` package. It will read from the same SQLite database as the
crawler CLI and will reuse the `internal/search` package for search
functionality.
