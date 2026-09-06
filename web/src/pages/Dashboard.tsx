import { useEffect, useState } from 'react';
import { api } from '../utils/api';
import type { Statistics, HealthResponse } from '../types/api';
import { LevelBadge } from '../components/LevelBadge';
import { GradeBadge } from '../components/GradeBadge';
import { HealthBadge } from '../components/HealthBadge';

export default function Dashboard() {
  const [stats, setStats] = useState<Statistics | null>(null);
  const [health, setHealth] = useState<HealthResponse | null>(null);
import type { TaiwanLevel, HealthStatus, QualityGrade } from '../types/api';
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      api.statistics().then(setStats).catch(setError),
      api.health().then(setHealth).catch(setError),
    ]).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="text-center py-12">Loading dashboard...</div>;
  if (error) return <div className="text-red-600">Error: {error.message}</div>;

  return (
    <div className="space-y-8">
      <h1 className="text-3xl font-bold text-gray-900">Taiwan AI Ecosystem Registry</h1>

      {health && (
        <div className="bg-white p-4 rounded-lg shadow">
          Status: <span className="font-medium">{health.status}</span> · Version {health.version}
        </div>
      )}

      {stats && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <StatCard label="Total Servers" value={stats.total_servers} />
            <StatCard label="Taiwan Relevant" value={stats.taiwan_relevant} />
            <StatCard label="Total Categories" value={Object.keys(stats.by_level || {}).length} />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <StatBreakdown
              title="Taiwan Relevance Levels"
              data={stats.by_level || {}}
              renderKey={(k) => <LevelBadge level={k as TaiwanLevel} />}
            />
            <StatBreakdown
              title="Health Status"
              data={stats.by_health || {}}
              renderKey={(k) => <HealthBadge health={k as HealthStatus} />}
            />
            <StatBreakdown
              title="Quality Grades"
              data={stats.quality_distribution || {}}
              renderKey={(k) => <GradeBadge grade={k as QualityGrade} />}
            />
          </div>

          {Object.keys(stats.by_status || {}).length > 0 && (
            <StatBreakdown
              title="Entity Status"
              data={stats.by_status || {}}
              renderKey={(k) => <span className="px-2 py-1 bg-gray-100 rounded text-sm">{k}</span>}
            />
          )}
        </>
      )}
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="bg-white p-6 rounded-lg shadow text-center">
      <div className="text-3xl font-bold text-blue-600">{value}</div>
      <div className="text-sm text-gray-600">{label}</div>
    </div>
  );
}

function StatBreakdown({
  title,
  data,
  renderKey,
}: {
  title: string;
  data: Record<string, number>;
  renderKey: (key: string) => React.ReactNode;
}) {
  const entries = Object.entries(data).sort((a, b) => b[1] - a[1]);
  const max = entries.length > 0 ? entries[0][1] : 1;

  return (
    <div className="bg-white p-6 rounded-lg shadow">
      <h3 className="text-lg font-semibold text-gray-900 mb-4">{title}</h3>
      <div className="space-y-3">
        {entries.map(([key, count]) => (
          <div key={key} className="flex items-center gap-3">
            {renderKey(key)}
            <div className="flex-1 bg-gray-200 rounded-full h-4">
              <div
                className="bg-blue-600 h-4 rounded-full"
                style={{ width: `${(count / max) * 100}%` }}
              />
            </div>
            <span className="text-sm font-medium w-12 text-right">{count}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
