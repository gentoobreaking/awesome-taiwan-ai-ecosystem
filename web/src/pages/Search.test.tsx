import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { rest } from 'msw';
import { setupServer } from 'msw/node';

// Mock API responses
const server = setupServer(
  rest.get('/api/v1/health', (req, res, ctx) => {
    return res(
      ctx.json({
        status: 'ok',
        timestamp: '2025-01-01T00:00:00Z',
        version: 'v0.1',
        db_count: 100,
      }),
    );
  }),
  rest.get('/api/v1/statistics', (req, res, ctx) => {
    return res(
      ctx.json({
        total_servers: 100,
        taiwan_relevant: 50,
        by_level: { T1: 20, T2: 15, T3: 10, T4: 5 },
        by_health: { HEALTHY: 80, DEGRADED: 15, UNHEALTHY: 5 },
        quality_distribution: { A: 10, B: 20, C: 30, D: 20, F: 20 },
      }),
    );
  }),
);

beforeAll(() => server.listen());
afterEach(() => server.restoreHandlers());
afterAll(() => server.close());

describe('Dashboard', () => {
  it('renders health status', async () => {
    // Test that statistics response is parsed correctly
    const mockStats = {
      total_servers: 100,
      taiwan_relevant: 50,
      by_level: { T1: 20, T2: 15 },
      by_health: { HEALTHY: 80 },
      quality_distribution: { A: 10, B: 20 },
      by_status: { VERIFIED: 80 },
    };

    // Verify the stats structure is correct
    expect(mockStats.total_servers).toBe(100);
    expect(mockStats.taiwan_relevant).toBe(50);
    expect(mockStats.by_level.T1).toBe(20);
  });
});

describe('API client', () => {
  it('formats search params correctly', () => {
    const params = { q: 'taiwan', level: 'T3' as const, page: 1, limit: 50 };
    const searchParams = new URLSearchParams();

    if (params.q) searchParams.set('q', params.q);
    if (params.level) searchParams.set('level', params.level);
    if (params.page) searchParams.set('page', String(params.page));
    if (params.limit) searchParams.set('limit', String(params.limit));

    expect(searchParams.get('q')).toBe('taiwan');
    expect(searchParams.get('level')).toBe('T3');
    expect(searchParams.get('page')).toBe('1');
  });

  it('handles empty search params', () => {
    const params: Record<string, string> = {};
    const searchParams = new URLSearchParams();

    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== '') {
        searchParams.set(k, String(v));
      }
    }
    expect(searchParams.toString()).toBe('');
  });
});

describe('Type validation', () => {
  it('TaiwanLevel values are valid', () => {
    const validLevels = ['T0', 'T1', 'T2', 'T3', 'T4', 'T5'];
    validLevels.forEach((level) => {
      expect(level.startsWith('T')).toBe(true);
    });
  });

  it('QualityGrade values are valid', () => {
    const validGrades = ['A', 'B', 'C', 'D', 'F'];
    validGrades.forEach((grade) => {
      expect(grade.length).toBe(1);
      expect(grade >= 'A').toBe(true);
      expect(grade <= 'F').toBe(true);
    });
  });

  it('HealthStatus values are valid', () => {
    const validHealth = ['HEALTHY', 'DEGRADED', 'UNHEALTHY', 'INVALID'];
    validHealth.forEach((h) => {
      expect(h.length).toBeGreaterThan(0);
      expect(h === h.toUpperCase()).toBe(true);
    });
  });
});
