import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { api } from '../utils/api';
import type { MCPServer } from '../types/api';
import { LevelBadge } from '../components/LevelBadge';
import { GradeBadge } from '../components/GradeBadge';
import { HealthBadge } from '../components/HealthBadge';

export default function ServerDetail() {
  const { id } = useParams<{ id: string }>();
  const [server, setServer] = useState<MCPServer | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    api
      .server(id)
      .then((data) => setServer(data.server))
      .catch((err) => setError((err as Error).message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div className="text-center py-12">Loading...</div>;
  if (error) return <div className="text-red-600">Error: {error}</div>;
  if (!server) return <div className="text-center py-12">Server not found</div>;

  return (
    <div className="space-y-6">
      <Link to="/servers" className="text-blue-600 hover:underline text-sm">
        ← Back to servers
      </Link>

      <div className="bg-white p-6 rounded-lg shadow">
        <div className="flex justify-between items-start">
          <h1 className="text-2xl font-bold text-gray-900">{server.name}</h1>
          <div className="flex gap-2">
            <LevelBadge level={server.taiwan_relevance.level} />
            <GradeBadge grade={server.quality.grade} />
            <HealthBadge health={server.health} />
          </div>
        </div>

        <p className="text-gray-600 mt-4">{server.description}</p>

        {/* Taiwan Relevance */}
        <div className="mt-6">
          <h3 className="text-lg font-semibold text-gray-900 mb-2">Taiwan Relevance</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <StatItem label="Score" value={`${server.taiwan_relevance.score.toFixed(1)}/100`} />
            <StatItem label="Level" value={server.taiwan_relevance.level} />
            <StatItem label="Confidence" value={`${Math.round(server.taiwan_relevance.confidence * 100)}%`} />
          </div>
        </div>

        {/* Quality */}
        <div className="mt-6">
          <h3 className="text-lg font-semibold text-gray-900 mb-2">Quality Score</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <StatItem label="Score" value={`${server.quality.score}/100`} />
            <StatItem label="Grade" value={server.quality.grade} />
          </div>

          {server.quality.components && (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4">
              {Object.entries(server.quality.components).map(([key, val]) => (
                <StatItem key={key} label={key} value={String(val)} />
              ))}
            </div>
          )}
        </div>

        {/* Repository */}
        {server.repository && (
          <div className="mt-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Repository</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <StatItem label="Stars" value={server.repository.stars} />
              <StatItem label="Forks" value={server.repository.forks} />
              <StatItem label="Language" value={server.repository.language} />
              <StatItem label="License" value={server.repository.license} />
            </div>
            {server.repository.topics && server.repository.topics.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-3">
                {server.repository.topics.map((t) => (
                  <span key={t} className="px-2 py-1 bg-gray-100 rounded text-xs">
                    {t}
                  </span>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Endpoints */}
        {server.endpoints && server.endpoints.length > 0 && (
          <div className="mt-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Endpoints</h3>
            <div className="space-y-2">
              {server.endpoints.map((ep, i) => {
                const e = ep as Record<string, unknown>;
                return (
                  <div key={i} className="bg-gray-50 p-3 rounded text-sm">
                    <span className="font-medium">{String(e.url || e)}</span>
                    {e.transport && <span className="text-gray-500 ml-2">({String(e.transport)})</span>}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* Tools */}
        {server.tools && server.tools.length > 0 && (
          <div className="mt-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-2">
              Tools ({server.tools.length})
            </h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {server.tools.map((tool, i) => {
                const t = tool as Record<string, unknown>;
                return (
                  <div key={i} className="border border-gray-200 p-3 rounded">
                    <div className="font-medium">{String(t.name || `Tool ${i + 1}`)}</div>
                    {t.description && <p className="text-sm text-gray-600">{String(t.description)}</p>}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* Sources */}
        {server.sources && server.sources.length > 0 && (
          <div className="mt-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Sources</h3>
            <div className="flex flex-wrap gap-2">
              {server.sources.map((src, i) => {
                const s = src as Record<string, unknown>;
                return (
                  <span key={i} className="px-2 py-1 bg-blue-50 text-blue-800 rounded text-xs">
                    {String(s.source || 'unknown')}
                  </span>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function StatItem({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="bg-gray-50 p-3 rounded text-center">
      <div className="text-xs text-gray-500">{label}</div>
      <div className="font-medium text-gray-900">{value}</div>
    </div>
  );
}
