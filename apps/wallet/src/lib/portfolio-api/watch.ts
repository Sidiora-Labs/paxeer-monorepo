/**
 * v2 Watch — register an address for real-time balance tracking.
 *
 * Endpoints:
 *   POST   /api/v2/watch              — start watching
 *   GET    /api/v2/watch/{address}    — fetch watch status
 *   DELETE /api/v2/watch/{address}    — stop watching
 */

import { portfolioApi } from './client';
import type { ApiV2WatchRequest, ApiV2WatchResponse, ApiV2WatchStatusResponse } from './types';

export const watchAddress = (body: ApiV2WatchRequest, signal?: AbortSignal, accessToken?: string) =>
  portfolioApi.post<ApiV2WatchResponse>(
    '/api/v2/watch',
    { address: body.address },
    { signal, accessToken }
  );

export const getWatchStatus = (address: string, signal?: AbortSignal, accessToken?: string) =>
  portfolioApi.get<ApiV2WatchStatusResponse>(
    `/api/v2/watch/${encodeURIComponent(address)}`,
    undefined,
    { signal, accessToken }
  );

export const unwatchAddress = (address: string, signal?: AbortSignal, accessToken?: string) =>
  portfolioApi.delete<{ status?: string; [key: string]: unknown }>(
    `/api/v2/watch/${encodeURIComponent(address)}`,
    { signal, accessToken }
  );
