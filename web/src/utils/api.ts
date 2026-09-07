import type {
  HealthResponse,
  ServersResponse,
  ServerResponse,
  SearchResponse,
  Statistics,
  RegistryResponse,
  EntitiesResponse,
  Entity,
  MCPServer,
  TaiwanLevel,
} from '../types/api';

export interface SearchParams {
  q?: string;
  level?: TaiwanLevel;
  category?: string;
  health?: string;
  'min-score'?: number;
  page?: number;
  limit?: number;
  status?: string;
  security?: string;
  transport?: string;
}

export interface ServerListParams {
  page?: number;
  limit?: number;
  level?: TaiwanLevel;
  category?: string;
  health?: string;
  'min-score'?: number;
  status?: string;
  security?: string;
}

const API_BASE = import.meta.env.DEV
  ? '/api/v1'
  : '/api/v1';

async function apiFetch<T>(path: string, params?: Record<string, string>): Promise<T> {
  const url = new URL(API_BASE + path, window.location.origin);
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== '') {
        url.searchParams.set(k, String(v));
      }
    }
  }

  const resp = await fetch(url.toString(), {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ message: resp.statusText }));
    throw new Error(err.message || `HTTP ${resp.status}`);
  }

  return resp.json();
}

export const api = {
  // Health check
  health(): Promise<HealthResponse> {
    return apiFetch<HealthResponse>('/health');
  },

  // List servers with pagination and filters
  servers(params?: ServerListParams): Promise<ServersResponse> {
    const p: Record<string, string> = {};
    if (params?.page) p.page = String(params.page);
    if (params?.limit) p.limit = String(params.limit);
    if (params?.level) p.level = params.level;
    if (params?.category) p.category = params.category;
    if (params?.health) p.health = params.health;
    if (params?.['min-score']) p['min-score'] = String(params['min-score']);
    if (params?.status) p.status = params.status;
    if (params?.security) p.security = params.security;
    return apiFetch<ServersResponse>('/servers', p);
  },

  // Get single server by ID
  server(id: string): Promise<ServerResponse> {
    return apiFetch<ServerResponse>(`/servers/${encodeURIComponent(id)}`);
  },

  // Search servers
  search(params: SearchParams): Promise<SearchResponse> {
    const p: Record<string, string> = {};
    if (params.q) p.q = params.q;
    if (params.level) p.level = params.level;
    if (params.category) p.category = params.category;
    if (params.health) p.health = params.health;
    if (params['min-score'] !== undefined) p['min-score'] = String(params['min-score']);
    if (params.page) p.page = String(params.page);
    if (params.limit) p.limit = String(params.limit);
    if (params.status) p.status = params.status;
    if (params.security) p.security = params.security;
    return apiFetch<SearchResponse>('/search', p);
  },

  // Get registry statistics
  statistics(): Promise<Statistics> {
    return apiFetch<Statistics>('/statistics');
  },

  // Get full registry
  registry(): Promise<RegistryResponse> {
    return apiFetch<RegistryResponse>('/registry');
  },

  // List all v2 entities (canonical per spec §43 / §60).
  entities(params?: ServerListParams): Promise<EntitiesResponse> {
    const p: Record<string, string> = {};
    if (params?.page) p.page = String(params.page);
    if (params?.limit) p.limit = String(params.limit);
    if (params?.level) p.level = params.level;
    if (params?.category) p.category = params.category;
    return apiFetch<EntitiesResponse>('/entities', p);
  },

  // Get single entity by ID.
  entity(id: string): Promise<{ entity: Entity }> {
    return apiFetch<{ entity: Entity }>(`/entities/${encodeURIComponent(id)}`);
  },

  // List available markdown view files (no file= param).
  registryMarkdownIndex(): Promise<{ directory: string; files: string[] }> {
    return apiFetch('/registry/markdown');
  },

  // Fetch a single markdown view file as raw text.
  registryMarkdown(file: string): Promise<string> {
    return apiFetchText(`/registry/markdown?file=${encodeURIComponent(file)}`);
  },
};

async function apiFetchText(path: string): Promise<string> {
  const url = new URL(API_BASE + path, window.location.origin);
  const resp = await fetch(url.toString(), {
    method: 'GET',
    headers: { 'Content-Type': 'text/markdown' },
  });
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status}`);
  }
  return resp.text();
}
