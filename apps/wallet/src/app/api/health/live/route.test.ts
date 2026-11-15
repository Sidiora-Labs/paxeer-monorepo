import { expect, test } from 'vitest';

async function health_live_route_returns_the_liveness_form() {
  const { GET } = await import('./route');
  const response = await GET();
  const body = await response.json();

  expect(response.status).toBe(200);
  expect(body).toEqual({
    status: 'ok',
    check: 'liveness',
    timestamp: expect.any(String),
  });
  expect(new Date(body.timestamp).toISOString()).toBe(body.timestamp);
  expect(response.headers.get('cache-control')).toBe('no-store');
}

test(health_live_route_returns_the_liveness_form, health_live_route_returns_the_liveness_form);
