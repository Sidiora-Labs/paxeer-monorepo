import { env } from '../env.js';

export class StationAdapterError extends Error {
  constructor(
    readonly status: number,
    readonly code: string
  ) {
    super(code);
  }
}

export class GasStationClient {
  constructor(private readonly configured: string | undefined = env.WALLET_GAS_STATION_URL) {}

  requireConfiguredUrl(): string {
    if (!this.configured) throw new StationAdapterError(503, 'gas_station_unconfigured');
    return this.configured;
  }

  quote(body: unknown): Promise<Record<string, unknown>> {
    return this.request('quote', body);
  }

  submit(body: unknown): Promise<Record<string, unknown>> {
    return this.request('submit', body);
  }

  status(body: unknown): Promise<Record<string, unknown>> {
    return this.request('status', body);
  }

  private async request(
    path: 'quote' | 'submit' | 'status',
    body: unknown
  ): Promise<Record<string, unknown>> {
    const configured = this.requireConfiguredUrl();
    let url: URL;
    try {
      url = new URL(configured);
    } catch {
      throw new StationAdapterError(503, 'gas_station_configuration');
    }
    if (
      !['https:', 'http:'].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash
    )
      throw new StationAdapterError(503, 'gas_station_configuration');
    url.pathname = `${url.pathname.replace(/\/(quote|submit|status)\/?$/, '').replace(/\/$/, '')}/${path}`;
    let response: Response;
    try {
      response = await fetch(url, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(body),
        redirect: 'error',
        signal: AbortSignal.timeout(15_000),
      });
    } catch {
      throw new StationAdapterError(503, 'gas_station_unavailable');
    }
    const reader = response.body?.getReader();
    if (!reader) throw new StationAdapterError(502, 'gas_station_invalid_response');
    let text = '';
    let length = 0;
    const decoder = new TextDecoder('utf-8', { fatal: true });
    try {
      for (;;) {
        const chunk = await reader.read();
        if (chunk.done) break;
        length += chunk.value.length;
        if (length > 65_536) {
          await reader.cancel();
          throw new StationAdapterError(502, 'gas_station_invalid_response');
        }
        text += decoder.decode(chunk.value, { stream: true });
      }
      text += decoder.decode();
    } finally {
      reader.releaseLock();
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(text);
    } catch {
      throw new StationAdapterError(502, 'gas_station_invalid_response');
    }
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed))
      throw new StationAdapterError(502, 'gas_station_invalid_response');
    if (!response.ok)
      throw new StationAdapterError(
        response.status >= 400 && response.status < 500 ? response.status : 503,
        typeof (parsed as Record<string, unknown>).error === 'string'
          ? String((parsed as Record<string, unknown>).error)
          : 'gas_station_refused'
      );
    return parsed as Record<string, unknown>;
  }
}
