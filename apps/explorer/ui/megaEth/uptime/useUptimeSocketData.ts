import React from 'react';

import type { UptimeHistoryFull, UptimeHistoryItem, UptimeRealTimeData, UptimeSocketData } from 'types/api/megaEth';

import config from 'configs/app';
import { DAY, HOUR, SECOND } from 'toolkit/utils/consts';

const megaEthFeature = config.features.megaEth;
const MAX_HISTORY_POINTS = 2048;
const HISTORY_HORIZONS: Record<keyof UptimeHistoryFull, number> = {
  historical_gas_per_second_3h: 3 * HOUR / SECOND,
  historical_gas_per_second_24h: 24 * HOUR / SECOND,
  historical_gas_per_second_7d: 7 * DAY / SECOND,
  historical_tps_3h: 3 * HOUR / SECOND,
  historical_tps_24h: 24 * HOUR / SECOND,
  historical_tps_7d: 7 * DAY / SECOND,
  historical_mini_block_interval_3h: 3 * HOUR / SECOND,
  historical_mini_block_interval_24h: 24 * HOUR / SECOND,
  historical_mini_block_interval_7d: 7 * DAY / SECOND,
};

function retainHistory(points: Array<UptimeHistoryItem>, horizon: number, now: number) {
  const unique = new Map<number, UptimeHistoryItem>();
  for (const point of points) {
    if (point.timestamp > now - horizon && point.timestamp <= now) {
      unique.set(point.timestamp, point);
    }
  }
  const sorted = Array.from(unique.values()).sort((a, b) => a.timestamp - b.timestamp);
  if (sorted.length <= MAX_HISTORY_POINTS) {
    return sorted;
  }

  const buckets = new Map<number, UptimeHistoryItem>();
  const bucketWidth = horizon / (MAX_HISTORY_POINTS - 1);
  for (const point of sorted) {
    buckets.set(Math.floor(point.timestamp / bucketWidth), point);
  }
  return Array.from(buckets.values());
}

function mergeHistory(history: UptimeHistoryFull, delta?: Extract<UptimeSocketData, { type: 'history_delta' }>['data']): UptimeHistoryFull {
  const next = { ...history };
  const now = Date.now() / SECOND;
  for (const key of Object.keys(HISTORY_HORIZONS) as Array<keyof UptimeHistoryFull>) {
    const point = delta?.[key];
    next[key] = retainHistory(point ? [ ...history[key], point ] : history[key], HISTORY_HORIZONS[key], now);
  }
  return next;
}

export type Status = 'initial' | 'connected' | 'disconnected' | 'error';

export default function useUptimeSocketData() {
  const websocketRef = React.useRef<WebSocket | null>(null);
  const heartbeatRef = React.useRef<number | undefined>(undefined);
  const [ status, setStatus ] = React.useState<Status>('initial');
  const [ realtimeData, setRealtimeData ] = React.useState<UptimeRealTimeData | null>(null);
  const [ historyData, setHistoryData ] = React.useState<UptimeHistoryFull | null>(null);

  const disconnect = React.useCallback(() => {
    const socket = websocketRef.current;
    websocketRef.current = null;
    window.clearInterval(heartbeatRef.current);
    heartbeatRef.current = undefined;
    if (socket) {
      socket.onopen = null;
      socket.onmessage = null;
      socket.onerror = null;
      socket.onclose = null;
      socket.close(4000, 'Connection released');
    }
  }, []);

  const connect = React.useCallback(() => {
    disconnect();
    if (!megaEthFeature.isEnabled || !megaEthFeature.socketUrl.metrics) {
      return;
    }

    const socket = new WebSocket(megaEthFeature.socketUrl.metrics);
    websocketRef.current = socket;

    socket.onmessage = (event) => {
      if (websocketRef.current !== socket) {
        return;
      }
      try {
        const data = JSON.parse(event.data) as UptimeSocketData;
        switch (data.type) {
          case 'realtime_metrics':
            setRealtimeData((prev) => websocketRef.current === socket ? data.realtime : prev);
            break;
          case 'history_full':
            setHistoryData((prev) => websocketRef.current === socket ? mergeHistory(data.data) : prev);
            break;
          case 'history_delta':
            setHistoryData((prev) => {
              if (websocketRef.current !== socket || !prev) {
                return prev;
              }
              return mergeHistory(prev, data.data);
            });
            break;
        }
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error(error);
      }
    };

    socket.onopen = () => {
      if (websocketRef.current !== socket) {
        return;
      }
      setStatus('connected');
      window.clearInterval(heartbeatRef.current);
      heartbeatRef.current = window.setInterval(() => {
        if (websocketRef.current === socket && socket.readyState === WebSocket.OPEN) {
          socket.send('ping');
        }
      }, 10 * SECOND);
    };
    socket.onerror = (error) => {
      if (websocketRef.current !== socket) {
        return;
      }
      // eslint-disable-next-line no-console
      console.error(error);
      setStatus('error');
    };
    socket.onclose = () => {
      if (websocketRef.current !== socket) {
        return;
      }
      disconnect();
      setStatus('disconnected');
    };
  }, [ disconnect ]);

  const onReconnect = React.useCallback(() => {
    connect();
  }, [ connect ]);

  React.useEffect(() => {
    connect();
    return disconnect;
  }, [ connect, disconnect ]);

  return { status, realtimeData, historyData, onReconnect };
}
