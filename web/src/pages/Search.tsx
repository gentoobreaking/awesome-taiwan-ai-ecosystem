import { useEffect, useState } from 'react';
import type { TaiwanLevel } from '../types/api';
import { api, type SearchParams } from '../utils/api';
import type { MCPServer } from '../types/api';
import { LevelBadge } from '../components/LevelBadge';
import { GradeBadge } from '../components/GradeBadge';

export default function Search() {
  const [results, setResults] = useState<MCPServer[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [params, setParams] = useState<SearchParams>({ q: '', page: 1, limit: 50 });
  const [pagination, setPagination] = useState({ page: 1, total: 0, total_pages: 0 });

  const doSearch = (q: string, p: SearchParams) => {
    if (!q) {
      setResults([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    api
      .search(p)
      .then((data) => {
        setResults(data.results);
        setPagination(data.pagination);
      })
      .catch((err) => setError((err as Error).message))
      .finally(() => setLoading(false));
  };

  const handleSearch = () => {
    const newParams = { ...params, q: query, page: 1 };
    setParams(newParams);
    doSearch(query, newParams);
  };

  useEffect(() => {
    // Search on params change (for pagination)
    if (params.q) {
      doSearch(params.q, params);
    }
  }, [params]);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Search Registry</h1>

      {/* Search form */}
      <div className="bg-white p-4 rounded-lg shadow space-y-4">
        <div className="flex gap-2">
          <input
            type="text"
            placeholder="Enter keywords..."
            className="flex-1 px-3 py-2 border border-gray-300 rounded"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          />
          <button
            onClick={handleSearch}
            disabled={!query.trim()}
            className="px-4 py-2 bg-blue-600 text-white rounded disabled:opacity-50"
          >
            Search
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          <select
            className="px-3 py-2 border border-gray-300 rounded"
            value={params.level || ''}
            onChange={(e) => setParams((prev) => ({ ...prev, level: e.target.value as TaiwanLevel, page: 1 }))}
          >
            <option value="">All Levels</option>
            <option value="T5">T5</option>
            <option value="T4">T4</option>
            <option value="T3">T3</option>
            <option value="T2">T2</option>
            <option value="T1">T1</option>
            <option value="T0">T0</option>
          </select>
          <input
            type="text"
            placeholder="Category (e.g. MCP_SERVER)"
            className="px-3 py-2 border border-gray-300 rounded"
            value={params.category || ''}
            onChange={(e) => setParams((prev) => ({ ...prev, category: e.target.value || undefined, page: 1 }))}
          />
          <input
            type="number"
            placeholder="Min quality score"
            className="px-3 py-2 border border-gray-300 rounded"
            value={params['min-score'] || ''}
            onChange={(e) => setParams((prev) => ({ ...prev, 'min-score': e.target.value ? Number(e.target.value) : undefined, page: 1 }))}
          />
          <select
            className="px-3 py-2 border border-gray-300 rounded"
            value={params.status || ''}
            onChange={(e) => setParams((prev) => ({ ...prev, status: e.target.value || undefined, page: 1 }))}
          >
            <option value="">All Status</option>
            <option value="VERIFIED">Verified</option>
            <option value="CANDIDATE">Candidate</option>
            <option value="QUARANTINED">Quarantined</option>
            <option value="REJECTED">Rejected</option>
          </select>
          <select
            className="px-3 py-2 border border-gray-300 rounded"
            value={params.limit || 50}
            onChange={(e) => setParams((prev) => ({ ...prev, limit: Number(e.target.value), page: 1 }))}
          >
            <option value={20}>20 per page</option>
            <option value={50}>50 per page</option>
            <option value={100}>100 per page</option>
          </select>
        </div>
      </div>

      {/* Results */}
      {loading && <div className="text-center py-8">Searching...</div>}
      {error && <div className="text-red-600">Error: {error}</div>}
      {!loading && !error && results.length > 0 && (
        <div className="space-y-4">
          {results.map((s) => (
            <SearchResultCard key={s.id} server={s} />
          ))}
        </div>
      )}
      {!loading && !error && query && results.length === 0 && (
        <div className="text-center py-8 text-gray-500">No results found for "{query}"</div>
      )}

      {/* Pagination */}
      {!loading && !error && results.length > 0 && pagination.total_pages > 1 && (
        <div className="flex justify-center space-x-2">
          <button
            className="px-3 py-1 border border-gray-300 rounded disabled:opacity-50"
            disabled={pagination.page <= 1}
            onClick={() => setParams((prev) => ({ ...prev, page: prev.page! - 1 }))}
          >
            Previous
          </button>
          <span className="px-3 py-1">
            Page {pagination.page} of {pagination.total_pages}
          </span>
          <button
            className="px-3 py-1 border border-gray-300 rounded disabled:opacity-50"
            disabled={pagination.page >= pagination.total_pages}
            onClick={() => setParams((prev) => ({ ...prev, page: prev.page! + 1 }))}
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}

function SearchResultCard({ server }: { server: MCPServer }) {
  return (
    <div className="bg-white p-4 rounded-lg shadow hover:shadow-md transition-shadow">
      <div className="flex justify-between items-start">
        <div className="flex-1">
          <h3 className="text-lg font-semibold text-gray-900">{server.name}</h3>
          <p className="text-sm text-gray-600 mt-1 line-clamp-2">{server.description}</p>
          {server.repository?.url && (
            <a
              href={server.repository.url}
              className="text-xs text-blue-600 hover:underline block mt-1"
              target="_blank"
              rel="noopener noreferrer"
            >
              {server.repository.url}
            </a>
          )}
        </div>
        <div className="flex items-start gap-2 ml-4">
          <LevelBadge level={server.taiwan_relevance.level} />
          <GradeBadge grade={server.quality.grade} />
        </div>
      </div>
      <div className="flex gap-4 mt-3 text-xs text-gray-500">
        <span>⭐ {server.repository?.stars || 0}</span>
        <span>🛠️ {server.tools?.length || 0} tools</span>
        <span>📡 {server.endpoints?.length || 0} endpoints</span>
      </div>
    </div>
  );
}
