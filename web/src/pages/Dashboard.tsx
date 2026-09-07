import { useEffect, useState } from 'react';
import { api } from '../utils/api';
import type { Statistics, HealthResponse, TaiwanLevel, HealthStatus, QualityGrade } from '../types/api';
import { LevelBadge } from '../components/LevelBadge';
import { GradeBadge } from '../components/GradeBadge';
import { HealthBadge } from '../components/HealthBadge';
import { renderMarkdown } from '../utils/markdown';

export default function Dashboard() {
  const [stats, setStats] = useState<Statistics | null>(null);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [mdFiles, setMdFiles] = useState<string[]>([]);
  const [selectedMd, setSelectedMd] = useState<string>('taiwan-ai-ecosystem.md');
  const [mdHtml, setMdHtml] = useState<string>('');
  const [mdLoading, setMdLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      api.statistics().then(setStats).catch(setError),
      api.health().then(setHealth).catch(setError),
      api.registryMarkdownIndex().then((d) => {
        setMdFiles(d.files);
        if (d.files.length > 0 && !d.files.includes(selectedMd)) {
          setSelectedMd(d.files[0]);
        }
      }).catch(() => setMdFiles([])),
    ]).finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!selectedMd) return;
    setMdLoading(true);
    api.registryMarkdown(selectedMd)
      .then((src) => setMdHtml(renderMarkdown(src)))
      .catch((e) => setMdHtml(`<p class="text-red-600">Failed to load: ${e.message}</p>`))
      .finally(() => setMdLoading(false));
  }, [selectedMd]);

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
            <StatCard label="Total Entities" value={stats.total_servers} />
            <StatCard label="Taiwan Relevant" value={stats.taiwan_relevant} />
            <StatCard
              label="Total Categories"
              value={Object.keys(stats.by_classification || {}).length || Object.keys(stats.by_level || {}).length}
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {stats.by_classification && Object.keys(stats.by_classification).length > 0 && (
              <StatBreakdown
                title="Primary Classification (spec §60)"
                data={stats.by_classification}
                renderKey={(k) => (
                  <span className="px-2 py-1 bg-blue-50 text-blue-700 rounded text-xs font-mono">
                    {k}
                  </span>
                )}
              />
            )}
            <StatBreakdown
              title="Taiwan Relevance Levels"
              data={stats.by_level || {}}
              renderKey={(k) => <LevelBadge level={k as TaiwanLevel} />}
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
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

      {selectedMd && (
        <div className="bg-white rounded-lg shadow">
          <div className="border-b border-gray-200 px-4 py-3 flex items-center gap-3 flex-wrap">
            <h2 className="text-lg font-semibold text-gray-900">Registry Views (spec §60)</h2>
            <select
              className="px-3 py-1 border border-gray-300 rounded text-sm"
              value={selectedMd}
              onChange={(e) => setSelectedMd(e.target.value)}
              disabled={mdFiles.length === 0}
            >
              {mdFiles.length === 0 ? (
                <option value={selectedMd}>{selectedMd}</option>
              ) : (
                mdFiles.map((f) => (
                  <option key={f} value={f}>{f}</option>
                ))
              )}
            </select>
            {mdLoading && <span className="text-xs text-gray-500">loading…</span>}
          </div>
          <article
            className="prose prose-sm max-w-none p-6 [&_a]:text-blue-600 [&_a]:underline [&_h1]:text-2xl [&_h1]:font-bold [&_h1]:mt-4 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:mt-4 [&_h3]:text-lg [&_h3]:font-semibold [&_h3]:mt-3 [&_ul]:list-disc [&_ul]:pl-6 [&_ol]:list-decimal [&_ol]:pl-6 [&_li]:my-1 [&_p]:my-2 [&_code]:bg-gray-100 [&_code]:px-1 [&_code]:rounded [&_strong]:font-semibold [&_pre]:bg-gray-50 [&_pre]:p-3 [&_pre]:overflow-x-auto"
            dangerouslySetInnerHTML={{ __html: mdHtml }}
          />
        </div>
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
  renderKey: (k: string) => React.ReactNode;
}) {
  const entries = Object.entries(data).sort(([, a], [, b]) => b - a);
  return (
    <div className="bg-white p-4 rounded-lg shadow">
      <h3 className="text-sm font-semibold text-gray-700 mb-3">{title}</h3>
      {entries.length === 0 ? (
        <div className="text-sm text-gray-400">No data</div>
      ) : (
        <ul className="space-y-2">
          {entries.map(([k, v]) => (
            <li key={k} className="flex items-center justify-between text-sm">
              {renderKey(k)}
              <span className="font-mono text-gray-700">{v}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
