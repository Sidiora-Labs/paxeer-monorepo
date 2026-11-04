import { useEffect, useState } from 'react';
import type { Runtime } from '../navigation.tsx';
import { validatePublicSnapshot } from './offline-cache.ts';
export function useResource<T>(
  runtime: Pick<Runtime, 'session' | 'cache'>,
  key: string,
  read: (signal: AbortSignal) => Promise<T>,
  privateData: boolean,
  onNetworkLost: () => void,
) {
  const [data, setData] = useState<T>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [offline, setOffline] = useState(false);
  const [storedAt, setStoredAt] = useState<number>();
  const [revision, reload] = useState(0);
  const account = privateData ? (runtime.session.account ?? null) : null;
  useEffect(() => {
    const controller = new AbortController();
    const generation = runtime.session.generation;
    let expiryTimer: ReturnType<typeof setTimeout> | undefined;
    setData(undefined);
    setError(undefined);
    setOffline(false);
    if (privateData && !runtime.session.token) {
      setError('Sign in to view this account');
      return () => controller.abort();
    }
    setLoading(true);
    const authAbort = () => controller.abort();
    runtime.session.signal.addEventListener('abort', authAbort, { once: true });
    void read(controller.signal)
      .then((value) => {
        if (
          controller.signal.aborted ||
          generation !== runtime.session.generation
        )
          return;
        setData(value);
        if (privateData) {
          const until = Math.min(
            Date.now() + 60000,
            Date.parse(runtime.session.current!.session.accessExpiresAt),
          );
          expiryTimer = setTimeout(
            () => {
              setData(undefined);
              setError('Private snapshot expired. Refresh authorization.');
            },
            Math.max(0, until - Date.now()),
          );
        }
        setStoredAt(Date.now());
        if (value !== undefined)
          runtime.cache.put(
            key,
            value,
            account,
            Date.now() + (privateData ? 60000 : 300000),
          );
      })
      .catch(async (failure: unknown) => {
        if (controller.signal.aborted) return;
        const status =
          failure && typeof failure === 'object' && 'status' in failure
            ? failure.status
            : undefined;
        if (status !== 401 && status !== 403) onNetworkLost();
        const cached =
          status === 401 || status === 403
            ? undefined
            : await runtime.cache.read<T>(key, account).catch(() => undefined);
        if (
          controller.signal.aborted ||
          generation !== runtime.session.generation
        )
          return;
        if (cached) {
          if (!privateData) {
            try {
              validatePublicSnapshot(key, cached.value);
            } catch {
              setError(
                'Stored public snapshot is invalid. Reconnect to refresh it.',
              );
              return;
            }
          }
          setData(cached.value);
          if (privateData) {
            const until = Math.min(
              cached.expiresAt,
              Date.parse(runtime.session.current!.session.accessExpiresAt),
            );
            expiryTimer = setTimeout(
              () => {
                setData(undefined);
                setError('Private snapshot expired. Refresh authorization.');
              },
              Math.max(0, until - Date.now()),
            );
          }
          setStoredAt(cached.storedAt);
          setOffline(true);
        }
        setError(
          status === 401 || status === 403
            ? 'Authorization expired. Sign in again.'
            : 'Service unavailable. Retry when connected.',
        );
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => {
      if (expiryTimer) clearTimeout(expiryTimer);
      controller.abort();
      runtime.session.signal.removeEventListener('abort', authAbort);
    };
  }, [runtime, key, revision, account, runtime.session.generation]);
  return {
    data,
    loading,
    error,
    offline,
    storedAt,
    reload: () => reload((n) => n + 1),
  };
}
export function offlineSnapshotLabel(storedAt: number | undefined) {
  return `Offline snapshot · ${storedAt ? new Date(storedAt).toLocaleString() : 'age unavailable'}`;
}
