export const KEY_BODY_LIMIT = 8192;
export const KEY_UPSTREAM_TIMEOUT_MS = 5000;

type JsonObject = Record<string, unknown>;
type KeyOrigins = { gatewayOrigin?: string };

class KeyFailure extends Error {
  readonly status: number;

  constructor(status: number, reason: string) {
    super(reason);
    this.status = status;
  }
}

function jsonObject(value: unknown): JsonObject {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new KeyFailure(400, "invalid_request");
  }
  return value as JsonObject;
}

function response(status: number, body: JsonObject): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Cache-Control": "no-store",
      "Content-Type": "application/json",
      "X-Content-Type-Options": "nosniff",
    },
  });
}

function reasonResponse(status: number, reason: string): Response {
  return response(status, { reason });
}

function originUrl(origin: string, path: string): URL {
  const url = new URL(origin);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname !== "/") {
    throw new Error("invalid origin");
  }
  return new URL(path, url);
}

function acceptsOrigin(request: Request): boolean {
  const origin = request.headers.get("origin");
  if (origin === null) return false;
  try {
    const parsedOrigin = originUrl(origin, "/").origin;
    if (parsedOrigin === new URL(request.url).origin) return true;
    const host = request.headers.get("host");
    if (host === null || !/^(?:\[[0-9a-f:.]+\]|[^\s:/\\?#@,\[\]%]+)(?::[0-9]+)?$/i.test(host)) return false;
    return parsedOrigin === new URL(`https://${host}`).origin;
  } catch {
    return false;
  }
}

async function boundedBody(stream: ReadableStream<Uint8Array> | null, limit: number): Promise<string> {
  if (!stream) return "";
  const reader = stream.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > limit) {
        void reader.cancel().catch(() => undefined);
        throw new KeyFailure(413, "body_too_large");
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
}

function validToken(value: unknown): value is string {
  return typeof value === "string" && value.length > 0 && value.length <= 4096
    && /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(value);
}

function sessionCookie(request: Request): string | undefined {
  const header = request.headers.get("cookie");
  if (!header) return undefined;
  for (const part of header.split(";")) {
    const separator = part.indexOf("=");
    if (separator < 0) continue;
    const name = part.slice(0, separator).trim();
    const value = part.slice(separator + 1).trim();
    if (name === "__Host-layerx-session" && validToken(value)) return value;
  }
  return undefined;
}

async function requestBody(request: Request): Promise<JsonObject> {
  if (!acceptsOrigin(request)) throw new KeyFailure(403, "origin_refused");
  if (!/^application\/json(?:\s*;\s*charset=utf-8)?$/i.test(request.headers.get("content-type") ?? "")) {
    throw new KeyFailure(415, "json_required");
  }
  const length = request.headers.get("content-length");
  if (length !== null && (!/^\d+$/.test(length) || Number(length) > KEY_BODY_LIMIT)) {
    throw new KeyFailure(413, "body_too_large");
  }
  try {
    const text = await boundedBody(request.body, KEY_BODY_LIMIT);
    return jsonObject(text === "" && request.method === "DELETE" ? {} : JSON.parse(text));
  } catch (error) {
    if (error instanceof KeyFailure) throw error;
    throw new KeyFailure(400, "invalid_request");
  }
}

function validIdentifier(value: unknown, maximum: number): value is string {
  return typeof value === "string" && value.length > 0 && value.length <= maximum && /^[A-Za-z0-9_-]+$/.test(value);
}

function keyId(pathname: string): string | undefined {
  const match = pathname.match(/^\/keys\/([^/]+)$/);
  return match && validIdentifier(match[1], 64) ? match[1] : undefined;
}

function idempotencyKey(request: Request): string {
  const value = request.headers.get("idempotency-key");
  if (!validIdentifier(value, 128)) throw new KeyFailure(400, "idempotency_key_required");
  return value;
}

function gatewayReason(value: unknown): string {
  const object = jsonObject(value);
  const nested = object.error && typeof object.error === "object" && !Array.isArray(object.error)
    ? object.error as JsonObject
    : undefined;
  const reason = object.code ?? nested?.code;
  if (!validIdentifier(reason, 96) || !/^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$/.test(reason)) {
    throw new KeyFailure(502, "gateway_invalid_response");
  }
  return reason;
}

async function gatewayCall(
  origin: string,
  session: string,
  method: "POST" | "DELETE",
  path: string,
  body: JsonObject,
  idempotency?: string,
): Promise<{ status: number; body: JsonObject }> {
  const headers: Record<string, string> = { Authorization: `Bearer ${session}` };
  if (method === "POST") headers["Content-Type"] = "application/json";
  if (idempotency) headers["Idempotency-Key"] = idempotency;
  let upstream: Response;
  try {
    upstream = await fetch(originUrl(origin, path), {
      method,
      headers,
      body: method === "POST" ? JSON.stringify(body) : undefined,
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(KEY_UPSTREAM_TIMEOUT_MS),
    });
  } catch {
    throw new KeyFailure(502, "gateway_unavailable");
  }
  let value: JsonObject;
  try {
    if (!/^application\/json(?:\s*;|$)/i.test(upstream.headers.get("content-type") ?? "")) {
      throw new Error("invalid content type");
    }
    value = jsonObject(JSON.parse(await boundedBody(upstream.body, 65536)));
  } catch (error) {
    if (error instanceof KeyFailure) throw error;
    throw new KeyFailure(502, "gateway_invalid_response");
  }
  if (!upstream.ok) {
    const status = upstream.status === 429
      ? 429
      : upstream.status >= 400 && upstream.status <= 599
        ? upstream.status
        : 502;
    throw new KeyFailure(status, gatewayReason(value));
  }
  return { status: upstream.status, body: value };
}

export async function handleKeysRequest(request: Request, origins: KeyOrigins): Promise<Response> {
  try {
    const body = await requestBody(request);
    const session = sessionCookie(request);
    if (!session) throw new KeyFailure(401, "session_required");
    if (!origins.gatewayOrigin) throw new KeyFailure(503, "gateway_not_configured");
    const pathname = new URL(request.url).pathname;
    if (request.method === "POST" && pathname === "/keys") {
      const idempotency = idempotencyKey(request);
      const upstream = await gatewayCall(origins.gatewayOrigin, session, "POST", "/v1/keys", body, idempotency);
      return response(upstream.status, upstream.body);
    }
    const rotate = pathname.match(/^\/keys\/([^/]+)\/rotate$/);
    if (request.method === "POST" && rotate) {
      const id = rotate[1];
      if (!validIdentifier(id, 64)) throw new KeyFailure(404, "not_found");
      const idempotency = idempotencyKey(request);
      const upstream = await gatewayCall(origins.gatewayOrigin, session, "POST", `/v1/keys/${id}/rotate`, body, idempotency);
      return response(upstream.status, upstream.body);
    }
    if (request.method === "DELETE") {
      const id = keyId(pathname);
      if (!id) throw new KeyFailure(404, "not_found");
      const upstream = await gatewayCall(origins.gatewayOrigin, session, "DELETE", `/v1/keys/${id}`, {});
      return response(upstream.status, upstream.body);
    }
    throw new KeyFailure(405, "method_refused");
  } catch (error) {
    const failure = error instanceof KeyFailure ? error : new KeyFailure(400, "invalid_request");
    return reasonResponse(failure.status, failure.message);
  }
}
