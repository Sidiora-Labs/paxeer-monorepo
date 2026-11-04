import type { JsonRpcCall, JsonRpcErrorObject, JsonRpcOutcome, JsonRpcParams } from './types.js';

export class JsonRpcError extends Error {
  readonly code: number;
  readonly data: unknown;

  constructor(error: JsonRpcErrorObject) {
    super(error.message);
    this.name = 'JsonRpcError';
    this.code = error.code;
    this.data = error.data;
  }
}

export class JsonRpcTransportError extends Error {
  readonly status: number | null;

  constructor(message: string, status: number | null = null) {
    super(message);
    this.name = 'JsonRpcTransportError';
    this.status = status;
  }
}

export class JsonRpcTimeoutError extends JsonRpcTransportError {
  constructor(readonly timeoutMs: number) {
    super(`JSON-RPC request timed out after ${timeoutMs}ms`);
    this.name = 'JsonRpcTimeoutError';
  }
}

export interface JsonRpcRequestOptions {
  signal?: AbortSignal;
}

export interface JsonRpcClientOptions {
  url: string;
  fetch?: typeof fetch;
  headers?: Record<string, string>;
  /** Deadline for both response headers and body (default 30 seconds). */
  timeoutMs?: number;
  /** Maximum decoded response bytes (default 4 MiB). */
  maxResponseBytes?: number;
}

type Envelope = { jsonrpc: '2.0'; id: number; method: string; params: JsonRpcParams };

export class JsonRpcClient {
  readonly url: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: Record<string, string>;
  private nextId = 1;
  readonly timeoutMs: number;
  readonly maxResponseBytes: number;

  constructor(options: JsonRpcClientOptions) {
    if (!options.url) throw new Error('JsonRpcClient: url required');
    this.timeoutMs = options.timeoutMs ?? 30_000;
    this.maxResponseBytes = options.maxResponseBytes ?? 4 * 1024 * 1024;
    if (!Number.isSafeInteger(this.timeoutMs) || this.timeoutMs < 1 || this.timeoutMs > 2147483647)
      throw new RangeError('timeoutMs must be between 1 and 2147483647');
    if (!Number.isSafeInteger(this.maxResponseBytes) || this.maxResponseBytes < 1)
      throw new RangeError('maxResponseBytes must be a positive safe integer');
    this.url = options.url;
    this.fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
    this.headers = { ...(options.headers ?? {}) };
  }

  async call<T>(
    method: string,
    params: JsonRpcParams = [],
    options: JsonRpcRequestOptions = {}
  ): Promise<T> {
    const envelope = this.envelope({ method, params });
    const payload = await this.post(envelope, options);
    const outcome = decodeResponse(payload, envelope.id);
    if (!outcome.ok) throw new JsonRpcError(outcome.error);
    return outcome.result as T;
  }

  async batch(
    calls: readonly JsonRpcCall[],
    options: JsonRpcRequestOptions = {}
  ): Promise<JsonRpcOutcome[]> {
    if (calls.length === 0) return [];
    const envelopes = calls.map((call) => this.envelope(call));
    const payload = await this.post(envelopes, options);
    if (!Array.isArray(payload)) {
      if (isRecord(payload) && isRecord(payload.error)) {
        throw new JsonRpcError(decodeError(payload.error));
      }
      throw new JsonRpcTransportError('batch response is not an array');
    }
    const byId = new Map<number, unknown>();
    for (const entry of payload) {
      if (!isRecord(entry) || typeof entry.id !== 'number') {
        throw new JsonRpcTransportError('batch response entry carries no numeric id');
      }
      if (byId.has(entry.id)) throw new JsonRpcTransportError('batch response repeats an id');
      byId.set(entry.id, entry);
    }
    return envelopes.map((envelope) => {
      const entry = byId.get(envelope.id);
      if (entry === undefined) {
        return {
          ok: false,
          error: { code: -32603, message: `no response for ${envelope.method}` },
        };
      }
      return decodeResponse(entry, envelope.id);
    });
  }

  private envelope(call: JsonRpcCall): Envelope {
    const id = this.nextId;
    this.nextId += 1;
    return { jsonrpc: '2.0', id, method: call.method, params: call.params };
  }

  private async post(
    body: Envelope | Envelope[],
    { signal }: JsonRpcRequestOptions
  ): Promise<unknown> {
    if (signal?.aborted) throw new DOMException('JSON-RPC request aborted', 'AbortError');
    const controller = new AbortController();
    const onAbort = () =>
      controller.abort(new DOMException('JSON-RPC request aborted', 'AbortError'));
    signal?.addEventListener('abort', onAbort, { once: true });
    const timer = setTimeout(
      () => controller.abort(new JsonRpcTimeoutError(this.timeoutMs)),
      this.timeoutMs
    );
    let rejectAbort!: () => void;
    const aborted = new Promise<never>((_resolve, reject) => {
      rejectAbort = () => reject(controller.signal.reason);
      controller.signal.addEventListener('abort', rejectAbort, { once: true });
    });
    let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
    let status: number | null = null;
    try {
      const response = await Promise.race([
        this.fetchImpl(this.url, {
          method: 'POST',
          headers: {
            'content-type': 'application/json',
            accept: 'application/json',
            ...this.headers,
          },
          body: JSON.stringify(body),
          signal: controller.signal,
        }),
        aborted,
      ]);
      status = response.status;
      const length = response.headers.get('content-length');
      if (length !== null && /^\d+$/.test(length) && BigInt(length) > BigInt(this.maxResponseBytes))
        throw new JsonRpcTransportError('response exceeds maxResponseBytes', status);
      reader = response.body?.getReader();
      const decoder = new TextDecoder();
      let bytes = 0;
      let text = '';
      if (reader) {
        for (;;) {
          const { value, done } = await Promise.race([reader.read(), aborted]);
          if (done) break;
          bytes += value.byteLength;
          if (bytes > this.maxResponseBytes)
            throw new JsonRpcTransportError('response exceeds maxResponseBytes', status);
          text += decoder.decode(value, { stream: true });
        }
      }
      text += decoder.decode();
      let payload: unknown;
      try {
        payload = (
          Array.isArray(body)
            ? body.some((entry) => entry.method === 'px_getReceiptKeys')
            : body.method === 'px_getReceiptKeys'
        )
          ? parseReceiptKeyResponse(text)
          : JSON.parse(text);
      } catch {
        throw new JsonRpcTransportError(`response is not JSON (HTTP ${status})`, status);
      }
      if (!response.ok && !(isRecord(payload) || Array.isArray(payload)))
        throw new JsonRpcTransportError(`HTTP ${status}`, status);
      return payload;
    } catch (err) {
      if (controller.signal.aborted) throw controller.signal.reason;
      if (err instanceof JsonRpcTransportError) throw err;
      throw new JsonRpcTransportError(
        err instanceof Error ? err.message : 'request failed',
        status
      );
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
      controller.signal.removeEventListener('abort', rejectAbort);
      if (reader) void reader.cancel().catch(() => undefined);
      controller.abort();
    }
  }
}

export function decodeResponse(value: unknown, id: number): JsonRpcOutcome {
  if (!isRecord(value) || value.jsonrpc !== '2.0' || value.id !== id) {
    throw new JsonRpcTransportError('response does not answer the request id');
  }
  const hasResult = 'result' in value;
  const hasError = 'error' in value;
  if (hasResult === hasError)
    throw new JsonRpcTransportError('response carries neither or both of result and error');
  if (hasError) return { ok: false, error: decodeError(value.error) };
  return { ok: true, result: value.result };
}

function decodeError(value: unknown): JsonRpcErrorObject {
  if (
    !isRecord(value) ||
    typeof value.code !== 'number' ||
    !Number.isInteger(value.code) ||
    typeof value.message !== 'string'
  ) {
    throw new JsonRpcTransportError('malformed JSON-RPC error');
  }
  return 'data' in value
    ? { code: value.code, message: value.message, data: value.data }
    : { code: value.code, message: value.message };
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** Preserve uint64 receipt-key height bounds before JSON number rounding. */
export function parseReceiptKeyResponse(text: string): unknown {
  const lossless = text.replace(
    /"(?:\\.|[^"\\])*"|(-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)/g,
    (token, number: string | undefined) => {
      if (
        number !== undefined &&
        /^-?(?:0|[1-9][0-9]*)$/.test(number) &&
        !Number.isSafeInteger(Number(number))
      )
        return JSON.stringify(number);
      return token;
    }
  );
  return JSON.parse(lossless) as unknown;
}
