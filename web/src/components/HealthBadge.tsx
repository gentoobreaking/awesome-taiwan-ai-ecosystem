import type { HealthStatus } from '../types/api';

export function HealthBadge({ health }: { health: HealthStatus }) {
  const config: Record<string, { label: string; color: string }> = {
    HEALTHY: { label: 'Healthy', color: 'bg-green-100 text-green-800' },
    DEGRADED: { label: 'Degraded', color: 'bg-yellow-100 text-yellow-800' },
    UNHEALTHY: { label: 'Unhealthy', color: 'bg-red-100 text-red-800' },
    INVALID: { label: 'Invalid', color: 'bg-gray-100 text-gray-800' },
  };

  const cfg = config[health] || { label: health, color: 'bg-gray-100 text-gray-800' };

  return (
    <span className={`px-2 py-1 rounded text-xs font-medium ${cfg.color}`}>
      {cfg.label}
    </span>
  );
}
