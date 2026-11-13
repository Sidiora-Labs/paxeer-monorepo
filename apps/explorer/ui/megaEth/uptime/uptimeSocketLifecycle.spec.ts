// @vitest-environment jsdom

import { act, renderHook, waitFor } from '@testing-library/react';
import { createHook } from 'node:async_hooks';
import type { AddressInfo } from 'node:net';
import { setTimeout as delay } from 'node:timers/promises';
import { WebSocket, WebSocketServer } from 'ws';

import type { UptimeHistoryFull, UptimeHistoryItem, UptimeRealTimeData, UptimeSocketData } from 'types/api/megaEth';

import { afterAll, beforeAll, expect, test } from 'vitest';

import type useUptimeSocketDataHook from './useUptimeSocketData';

const HORIZONS: Record<keyof UptimeHistoryFull, number> = {
  historical_gas_per_second_3h: 3 * 3600,
  historical_gas_per_second_24h: 24 * 3600,
  historical_gas_per_second_7d: 7 * 86400,
  historical_tps_3h: 3 * 3600,
  historical_tps_24h: 24 * 3600,
  historical_tps_7d: 7 * 86400,
  historical_mini_block_interval_3h: 3 * 3600,
  historical_mini_block_interval_24h: 24 * 3600,
  historical_mini_block_interval_7d: 7 * 86400,
};
const KEYS = Object.keys(HORIZONS) as Array<keyof UptimeHistoryFull>;
let server: WebSocketServer;
let useUptimeSocketData: typeof useUptimeSocketDataHook;
let originalEnvs: Window['__envs'];

beforeAll(async() => {
  server = new WebSocketServer({ host: '127.0.0.1', port: 0 });
  await new Promise<void>((resolve) => server.once('listening', resolve));
  originalEnvs = window.__envs;
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_METRICS: `ws://127.0.0.1:${ (server.address() as AddressInfo).port }`,
  };
  useUptimeSocketData = (await import('./useUptimeSocketData')).default;
});

afterAll(async() => {
  window.__envs = originalEnvs;
  server.clients.forEach((client) => client.terminate());
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

function nextConnection(): Promise<WebSocket> {
  return new Promise((resolve) => server.once('connection', resolve));
}

function historyWith(series: (key: keyof UptimeHistoryFull) => Array<UptimeHistoryItem>): UptimeHistoryFull {
  return {
    historical_gas_per_second_3h: series('historical_gas_per_second_3h'),
    historical_gas_per_second_7d: series('historical_gas_per_second_7d'),
    historical_gas_per_second_24h: series('historical_gas_per_second_24h'),
    historical_tps_3h: series('historical_tps_3h'),
    historical_tps_7d: series('historical_tps_7d'),
    historical_tps_24h: series('historical_tps_24h'),
    historical_mini_block_interval_3h: series('historical_mini_block_interval_3h'),
    historical_mini_block_interval_7d: series('historical_mini_block_interval_7d'),
    historical_mini_block_interval_24h: series('historical_mini_block_interval_24h'),
  };
}

function publish(socket: WebSocket, message: UptimeSocketData) {
  socket.send(JSON.stringify(message));
}

function realtimeWithValue(value: number): UptimeRealTimeData {
  return {
    end_block: value,
    instant_tps: value,
    instant_mgas_per_second: value,
    instant_mini_block_interval: value,
    latest_mini_block_id: value,
    updated_at: new Date().toISOString(),
  };
}

test('uptimeHistoryStaysBoundedAcrossDeltasAndReconnects', async function uptimeHistoryStaysBoundedAcrossDeltasAndReconnects() {
  const connected = nextConnection();
  const hook = renderHook(() => useUptimeSocketData());
  let socket = await connected;
  try {
    await waitFor(() => expect(hook.result.current.status).toBe('connected'));
    const now = Date.now() / 1000;
    const initial = historyWith((key) => {
      const horizon = HORIZONS[key];
      const points = Array.from({ length: 6500 }, (_, index) => ({
        timestamp: now - horizon + 5 + index * (horizon - 405) / 6499,
        value: index,
      }));
      return [ { timestamp: now - horizon - 10, value: -1 }, ...points, points[100] ];
    });
    await act(async() => publish(socket, { type: 'history_full', data: initial }));
    await waitFor(() => expect(hook.result.current.historyData).not.toBeNull());
    for (const key of KEYS) {
      const points = hook.result.current.historyData![key];
      expect(points.length).toBeLessThanOrEqual(2048);
      expect(points[0].timestamp).toBeGreaterThan(now - HORIZONS[key]);
      expect(points[0].timestamp).toBeLessThan(now - HORIZONS[key] + HORIZONS[key] / 2047 + 5);
      expect(points.at(-1)).toEqual(initial[key].at(-2));
      expect(new Set(points.map((point) => point.timestamp)).size).toBe(points.length);
    }

    for (let index = 0; index < 160; index++) {
      const point = { timestamp: now - 160 + index, value: 10_000 + index };
      const data = Object.fromEntries(KEYS.map((key) => [ key, point ]));
      await act(async() => {
        publish(socket, { type: 'history_delta', data });
        publish(socket, { type: 'history_delta', data });
      });
    }
    await waitFor(() => expect(hook.result.current.historyData?.historical_tps_3h.at(-1)?.value).toBe(10_159));
    for (const key of KEYS) {
      const points = hook.result.current.historyData![key];
      expect(points.length).toBeLessThanOrEqual(2048);
      expect(points.at(-1)).toEqual({ timestamp: now - 1, value: 10_159 });
      expect(points[0].timestamp).toBeLessThan(now - HORIZONS[key] + 2 * HORIZONS[key] / 2047 + 5);
      expect(new Set(points.map((point) => point.timestamp)).size).toBe(points.length);
    }

    socket.close();
    await waitFor(() => expect(hook.result.current.status).toBe('disconnected'));
    const reconnected = nextConnection();
    act(() => hook.result.current.onReconnect());
    socket = await reconnected;
    await waitFor(() => expect(hook.result.current.status).toBe('connected'));
    const snapshotTime = Date.now() / 1000;
    const snapshot = historyWith((key) => [
      { timestamp: snapshotTime - HORIZONS[key] + 1, value: 1 },
      { timestamp: snapshotTime - 2, value: 2 },
      { timestamp: snapshotTime - 2, value: 3 },
    ]);
    await act(async() => publish(socket, { type: 'history_full', data: snapshot }));
    await waitFor(() => expect(hook.result.current.historyData?.historical_tps_3h.at(-1)?.value).toBe(3));
    for (const key of KEYS) {
      expect(hook.result.current.historyData![key]).toEqual([ snapshot[key][0], snapshot[key][2] ]);
    }
    await delay(1100);
    await act(async() => publish(socket, {
      type: 'history_delta',
      data: { historical_tps_3h: { timestamp: snapshotTime, value: 4 } },
    }));
    await waitFor(() => expect(hook.result.current.historyData?.historical_tps_3h.at(-1)?.value).toBe(4));
    for (const key of KEYS) {
      expect(hook.result.current.historyData![key][0]).toEqual(snapshot[key][2]);
    }
  } finally {
    hook.unmount();
    socket.close();
  }
}, 30_000);

test('uptimeReconnectReleasesSocketsAndHeartbeats', async function uptimeReconnectReleasesSocketsAndHeartbeats() {
  const heartbeatTimers = new Set<number>();
  const timerLifecycle = createHook({
    init(id, type, _trigger, resource) {
      if (type === 'Timeout' && Reflect.get(resource, '_idleTimeout') === 10_000) {
        heartbeatTimers.add(id);
      }
    },
    destroy(id) {
      heartbeatTimers.delete(id);
    },
  });
  timerLifecycle.enable();
  const peers: Array<WebSocket> = [];
  let unmount: (() => void) | undefined;
  try {
    const connected = nextConnection();
    const hook = renderHook(() => useUptimeSocketData());
    unmount = hook.unmount;
    peers.push(await connected);
    await waitFor(() => expect(hook.result.current.status).toBe('connected'));
    await waitFor(() => expect(heartbeatTimers.size).toBe(1));

    for (let index = 0; index < 4; index++) {
      const previous = peers.at(-1)!;
      const connectedAgain = nextConnection();
      act(() => {
        publish(previous, {
          type: 'realtime_metrics',
          timestamp: Date.now(),
          realtime: realtimeWithValue(-1),
        });
        hook.result.current.onReconnect();
      });
      const current = await connectedAgain;
      peers.push(current);
      await waitFor(() => expect(hook.result.current.status).toBe('connected'));
      await waitFor(() => expect(previous.readyState).toBe(WebSocket.CLOSED));
      await waitFor(() => expect(heartbeatTimers.size).toBe(1));
      expect(hook.result.current.realtimeData).toBeNull();
      expect(peers.filter((peer) => peer.readyState === WebSocket.OPEN)).toHaveLength(1);
      if (index % 2 === 0) {
        current.close();
        await waitFor(() => expect(hook.result.current.status).toBe('disconnected'));
        await waitFor(() => expect(heartbeatTimers.size).toBe(0));
        const reopened = nextConnection();
        act(() => hook.result.current.onReconnect());
        peers.push(await reopened);
        await waitFor(() => expect(hook.result.current.status).toBe('connected'));
        await waitFor(() => expect(heartbeatTimers.size).toBe(1));
      }
    }
    const active = peers.at(-1)!;
    const metrics = realtimeWithValue(42);
    await act(async() => publish(active, {
      type: 'realtime_metrics',
      timestamp: Date.now(),
      realtime: metrics,
    }));
    await waitFor(() => expect(hook.result.current.realtimeData).toEqual(metrics));
    const pings: Array<string> = [];
    active.on('message', (data) => pings.push(data.toString()));
    await waitFor(() => expect(pings).toEqual([ 'ping' ]), { timeout: 11_000 });
    hook.unmount();
    await waitFor(() => expect(active.readyState).toBe(WebSocket.CLOSED));
    await waitFor(() => expect(heartbeatTimers.size).toBe(0));
    expect(peers.every((peer) => peer.readyState === WebSocket.CLOSED)).toBe(true);

    const remounted = nextConnection();
    const fresh = renderHook(() => useUptimeSocketData());
    unmount = fresh.unmount;
    peers.push(await remounted);
    await waitFor(() => expect(fresh.result.current.status).toBe('connected'));
    await waitFor(() => expect(heartbeatTimers.size).toBe(1));
    expect(fresh.result.current.historyData).toBeNull();
    const recoveredHistory = historyWith(() => [ { timestamp: Date.now() / 1000 - 1, value: 42 } ]);
    await act(async() => publish(peers.at(-1)!, { type: 'history_full', data: recoveredHistory }));
    await waitFor(() => expect(fresh.result.current.historyData).toEqual(recoveredHistory));
    fresh.unmount();
    await waitFor(() => expect(peers.at(-1)?.readyState).toBe(WebSocket.CLOSED));
    await waitFor(() => expect(heartbeatTimers.size).toBe(0));
  } finally {
    unmount?.();
    peers.forEach((peer) => peer.terminate());
    timerLifecycle.disable();
  }
}, 30_000);
