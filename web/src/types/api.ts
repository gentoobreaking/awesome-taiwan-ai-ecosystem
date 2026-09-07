// API response types

export type HealthStatus = 'HEALTHY' | 'DEGRADED' | 'UNHEALTHY' | string;
export type SecurityStatus = 'CLEAN' | 'SUSPICIOUS' | 'QUARANTINED' | 'BLOCKED' | string;
export type QualityGrade = 'A' | 'B' | 'C' | 'D' | 'F' | string;
export type TaiwanLevel = 'T0' | 'T1' | 'T2' | 'T3' | 'T4' | 'T5' | string;

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export interface TaiwanRelevance {
  score: number;
  level: TaiwanLevel;
  evidence: unknown[];
  confidence: number;
}

export interface QualityScore {
  score: number;
  grade: QualityGrade;
  components: Record<string, number>;
  evidence: unknown[];
}

export interface RepositoryInfo {
  url: string;
  host: string;
  owner: string;
  name: string;
  stars: number;
  forks: number;
  open_issues: number;
  language: string;
  license: string;
  topics: string[];
  created_at: string;
  updated_at: string;
  last_commit_at: string;
}

export interface MCPServer {
  id: string;
  name: string;
  slug: string;
  description: string;
  category: string[];
  region: string[];
  taiwan_relevance: TaiwanRelevance;
  repository: RepositoryInfo;
  endpoints: unknown[];
  transport: string[];
  tools: unknown[];
  resources: unknown[];
  prompts: unknown[];
  data_sources: unknown[];
  license: string;
  status: string;
  quality: QualityScore;
  sources: unknown[];
  first_seen: string;
  last_seen: string;
  last_verified: string | null;
}

export interface HealthResponse {
  status: string;
  timestamp: string;
  version: string;
  db_count: number;
}

export interface ServersResponse {
  servers: MCPServer[];
  pagination: Pagination;
}

export interface ServerResponse {
  server: MCPServer;
}

export interface SearchResponse {
  query: string;
  results: MCPServer[];
  pagination: Pagination;
}

export interface Statistics {
  total_servers: number;
  taiwan_relevant: number;
  by_level: Record<string, number>;
  by_health: Record<string, number>;
  quality_distribution: Record<string, number>;
  by_status: Record<string, number>;
  by_classification?: Record<string, number>;
}

export interface RegistryResponse {
  schema_version: string;
  registry_version: string;
  generated_at: string;
  total_servers: number;
  taiwan_relevant: number;
  statistics: Statistics;
  servers: MCPServer[];
}

// V2 canonical entity model (spec §37). Returned by /api/v1/entities.
export type MCPIdentityStatus =
  | 'CANDIDATE'
  | 'STATIC_VERIFIED'
  | 'RUNTIME_VERIFIED'
  | 'VERIFIED'
  | 'NOT_MCP'
  | string;

export interface Classification {
  primary: string;
  secondary?: string[];
  confidence: number;
  evidence?: unknown[];
  reasoning?: string;
}

export interface MCPIdentity {
  related: boolean;
  status: MCPIdentityStatus;
  confidence: number;
  role: string;
}

export interface Endpoint {
  url: string;
  transport: string;
  type: string;
  verified: boolean;
}

export interface Entity {
  id: string;
  name: string;
  slug: string;
  description: string;
  classification: Classification;
  taiwan_relevance: TaiwanRelevance;
  ai_relevance: { score: number; level: string; evidence: unknown[] };
  mcp_identity: MCPIdentity;
  endpoints: Endpoint[];
  tools: unknown[];
  resources: unknown[];
  prompts: unknown[];
  data_sources: unknown[];
  sources: unknown[];
  repository: RepositoryInfo;
  quality: QualityScore;
  entity_status: string;
  first_seen: string;
  last_seen: string;
  last_verified: string | null;
}

export interface EntitiesResponse {
  entities: Entity[];
  pagination: Pagination;
}
