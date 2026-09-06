import type { QualityGrade } from '../types/api';

export function GradeBadge({ grade }: { grade: QualityGrade }) {
  const config: Record<string, { label: string; color: string }> = {
    A: { label: 'A', color: 'bg-emerald-100 text-emerald-800' },
    B: { label: 'B', color: 'bg-green-100 text-green-800' },
    C: { label: 'C', color: 'bg-yellow-100 text-yellow-800' },
    D: { label: 'D', color: 'bg-orange-100 text-orange-800' },
    F: { label: 'F', color: 'bg-red-100 text-red-800' },
  };

  const cfg = config[grade] || { label: grade, color: 'bg-gray-100 text-gray-800' };

  return (
    <span className={`px-2 py-1 rounded text-xs font-medium ${cfg.color}`}>
      {cfg.label}
    </span>
  );
}
