export function finiteDelay(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null;
}

export interface RetryFields { retryable: boolean | null; retryAfterMs: number | null }

export function retryFields(body: unknown): RetryFields {
  if (typeof body !== 'object' || body === null || Array.isArray(body)) return { retryable: null, retryAfterMs: null };
  const row = body as { retryable?: unknown; retryAfterMs?: unknown };
  return { retryable: typeof row.retryable === 'boolean' ? row.retryable : null, retryAfterMs: finiteDelay(row.retryAfterMs) };
}

/**
 * Read `retryable` / `retryAfterMs` from a failed response's JSON error body,
 * for transports that do not go through the SDK. Consumes the body; a body
 * that is not a JSON object yields nulls.
 */
export async function readRetryFields(response: Response): Promise<RetryFields> {
  const body: unknown = await response.json().catch(() => null);
  return retryFields(body);
}
