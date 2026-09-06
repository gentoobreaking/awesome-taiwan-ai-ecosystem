import { useEffect, useState } from 'react';
import { api, type ServerListParams } from '../utils/api';
import type { MCPServer } from '../types/api';
import { LevelBadge } from '../components/LevelBadge';
import { GradeBadge } from '../components/GradeBadge';
import { HealthBadge } from '../components/HealthBadge';

export default function ServerList() {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [pagination, setPagination] = useState({ page: 1, limit: 50, total: 0, total_pages: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<ServerListParams>({ page: 1, limit: 50 });

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      setError(null);
      try {
        const data = await api.servers(filters);
        setServers(data.servers);
        setPagination(data.pagination);
      } catch (err) {
        setError((err as Error).message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [filters]);

  const updateFilter = (key: keyof ServerListParams, value: string | number | undefined) => {
    setFilters((prev) => ({ ...prev, [key]: value, page: 1 }));
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Registry Servers</h1>

      {/* Filters */}
      <div className="bg-white p-4 rounded-lg shadow space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          <input
            type="text"
            placeholder="Search by name..."
            className="px-3 py-2 border border-gray-300 rounded"
            onChange={(e) => updateFilter('q', e.target.value || undefined)}
          />
          <select
            className="px-3 py-2 border border-gray-300 rounded"
            onChange={(e) => updateFilter('level', e.target.value || undefined)}
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
            type="number"
            placeholder="Min quality score"
            className="px-3 py-2 border border-gray-300 rounded"
            onChange={(e) => updateFilter('min-score', e.target.value ? Number(e.target.value) : undefined)}
          />
          <select
            className="px-3 py-2 border border-gray-300 rounded"
            onChange={(e) => updateFilter('health', e.target.value || undefined)}
          >
            <option value="">All Health</option>
            <option value="HEALTHY">Healthy</option>
            <option value="DEGRADED">Degraded</option>
            <option value="UNAVAILABLE">Unavailable</option>
          </select>
          <select
            className="px-3 py-2 border border-gray-300 rounded"
            onChange={(e) => updateFilter('limit', Number(e.target.value))}
          >
            <option value={50}>50 per page</option>
            <option value={100}>100 per page</option>
            <option value={200}>200 per page</option>
          </select>
        </div>
      </div>

      {/* Loading / Error */}
      {loading && <div className="text-center py-8">Loading...</div>}
      {error && <div className="text-red-600">Error: {error}</div>}

      {/* Server List */}
      {!loading && !error && (
        <>
          <div className="space-y-4">
            {servers.map((s) => (
              <ServerCard key={s.id} server={s} />
            ))}
            {servers.length === 0 && <div className="text-center py-8 text-gray-500">No servers found</div>}
          </div>

          {/* Pagination */}
          {pagination.total_pages > 1 && (
            <div className="flex justify-center space-x-2">
              <button
                className="px-3 py-1 border border-gray-300 rounded disabled:opacity-50"
                disabled={pagination.page <= 1}
                onClick={() => setFilters((prev) => ({ ...prev, page: prev.page! - 1 }))}
              >
                Previous
              </button>
              <span className="px-3 py-1">
                Page {pagination.page} of {pagination.total_pages}
              </span>
              <button
                className="px-3 py-1 border border-gray-300 rounded disabled:opacity-50"
                disabled={pagination.page >= pagination.total_pages}
                onClick={() => setFilters((prev) => ({ ...prev, page: prev.page! + 1 }))}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function ServerCard({ server }: { server: MCPServer }) {
  return (
    <div className="bg-white p-4 rounded-lg shadow hover:shadow-md transition-shadow">
      <div className="flex justify-between items-start">
        <div className="flex-1">
          <h3 className="text-lg font-semibold text-gray-900">{server.name}</h3>
          <p className="text-sm text-gray-600 mt-1">{server.description}</p>
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
        <div className="flex gap-2 ml-4">
          <LevelBadge level={server.taiwan_relevance.level} />
          <GradeBadge grade={server.quality.grade} />
          <HealthBadge health={server.health} />
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
