import type { TaiwanLevel } from '../types/api';

export function LevelBadge({ level }: { level: TaiwanLevel }) {
  const config: Record<string, { label: string; color: string }> = {
    T5: { label: 'T5', color: 'bg-emerald-100 text-emerald-800' },
    T4: { label: 'T4', color: 'bg-green-100 text-green-800' },
    T3: { label: 'T3', color: 'bg-teal-100 text-teal-800' },
    T2: { label: 'T2', color: 'bg-blue-100 text-blue-800' },
    T1: { label: 'T1', color: 'bg-cyan-100 text-cyan-800' },
    T0: { label: 'T0', color: 'bg-gray-100 text-gray-800' },
  };

  const cfg = config[level] || { label: level, color: 'bg-gray-100 text-gray-800' };

  return (
    <span className={`px-2 py-1 rounded text-xs font-medium ${cfg.color}`}>
      {cfg.label}
    </span>
  );
}
