'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { isValidLabel, type PriceResult } from '@/lib/pns';
import type { PnsController } from './pnsRegistration';

export interface PnsSearchResult {
  generation: number;
  label: string;
  duration: number;
  available: boolean;
  price: PriceResult;
}

export function usePnsSearch(client: PnsController, query: string, duration: number) {
  const label = query
    .toLowerCase()
    .replace(/\.pax$/, '')
    .trim();
  const identity = `${label}:${duration}`;
  const generation = useRef(0);
  const current = useRef({ identity, client });
  if (current.current.identity !== identity || current.current.client !== client) {
    generation.current++;
    current.current = { identity, client };
  }
  const [result, setResult] = useState<PnsSearchResult | null>(null);
  const resultRef = useRef<PnsSearchResult | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const invalidate = useCallback(() => {
    generation.current++;
    resultRef.current = null;
    setResult(null);
  }, []);

  useEffect(() => {
    const request = ++generation.current;
    resultRef.current = null;
    setResult(null);
    setError('');
    setPending(isValidLabel(label));
    if (!isValidLabel(label)) return;
    const timer = setTimeout(async () => {
      try {
        const response = await client.search(label, duration);
        if (generation.current !== request) return;
        const next = { ...response, label, duration, generation: request };
        resultRef.current = next;
        setResult(next);
      } catch (reason) {
        if (generation.current === request)
          setError(reason instanceof Error ? reason.message : 'PNS search failed');
      } finally {
        if (generation.current === request) setPending(false);
      }
    }, 500);
    return () => {
      clearTimeout(timer);
      generation.current++;
      resultRef.current = null;
    };
  }, [client, label, duration, invalidate]);

  const validated = useCallback(() => {
    const value = resultRef.current;
    return value &&
      value.generation === generation.current &&
      `${value.label}:${value.duration}` === current.current.identity
      ? value
      : null;
  }, []);
  const value =
    result &&
    result.generation === generation.current &&
    result.label === label &&
    result.duration === duration
      ? result
      : null;
  return { result: value, searching: pending && isValidLabel(label), error, validated, invalidate };
}
