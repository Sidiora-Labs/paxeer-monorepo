import { InvalidParamsError } from './errors.js';

export interface GatewayClientOptions {
  apiUrl: string;
  fetch?: typeof fetch;
}

/** A gateway refusal, including recovery instructions in the original body. */
export class GatewayClientError extends Error {
  readonly code: string;

  constructor(
    public readonly status: number,
    public readonly body: unknown
  ) {
    const record = asRecord(body);
    const nested = asRecord(record.error);
    const code =
      typeof record.error === 'string'
        ? record.error
        : typeof nested.code === 'string'
          ? nested.code
          : typeof record.code === 'string'
            ? record.code
            : `HTTP_${status}`;
    const message =
      typeof record.message === 'string'
        ? record.message
        : typeof nested.message === 'string'
          ? nested.message
          : typeof record.detail === 'string'
            ? record.detail
            : code;
    super(message);
    this.name = 'GatewayClientError';
    this.code = code;
  }
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

export function gatewayBearer(token: string): Record<string, string> {
  if (!token.trim()) throw new InvalidParamsError('token', 'a bearer token is required');
  return { authorization: `Bearer ${token}` };
}

export class GatewayHttpClient {
  protected readonly apiUrl: string;
  private readonly fetchImpl: typeof fetch;

  constructor(options: GatewayClientOptions) {
    if (!options.apiUrl) throw new InvalidParamsError('apiUrl', 'the gateway URL is required');
    this.apiUrl = options.apiUrl.replace(/\/+$/, '');
    this.fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
  }

  protected async request<T>(
    method: string,
    path: string,
    headers: Record<string, string>,
    body?: string
  ): Promise<T> {
    const response = await this.fetchImpl(`${this.apiUrl}${path}`, {
      method,
      headers: {
        ...headers,
        ...(body === undefined ? {} : { 'content-type': 'application/json' }),
      },
      ...(body === undefined ? {} : { body }),
    });
    let payload: unknown;
    try {
      payload = await response.json();
    } catch {
      throw new GatewayClientError(
        response.status,
        response.ok ? { error: 'invalid_gateway_response' } : null
      );
    }
    if (!response.ok) {
      throw new GatewayClientError(response.status, payload);
    }
    return payload as T;
  }
}
