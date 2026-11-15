export const SESSION_BODY_LIMIT = 8192;
export const SESSION_UPSTREAM_TIMEOUT_MS = 5000;
const UPSTREAM_BODY_LIMIT = 65536;
const COOKIE_NAME = "__Host-layerx-session";
const COOKIE_ATTRIBUTES = "Secure; HttpOnly; SameSite=Strict; Path=/";

type Origins = { apiOrigin: string; gatewayOrigin?: string };
type Document = Record<string, unknown>;

class SessionFailure extends Error {
  readonly status: number;

  constructor(status: number, reason: string) {
    super(reason);
    this.status = status;
  }
}

function document(value: unknown): Document {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new SessionFailure(400, "invalid_request");
  }
  return value as Document;
}

function response(status: number, body: unknown = null, cookie?: string): Response {
  const headers = new Headers({ "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" });
  if (cookie !== undefined) headers.set("Set-Cookie", cookie);
  if (body !== null) headers.set("Content-Type", "application/json");
  return new Response(body === null ? null : JSON.stringify(body), { status, headers });
}

function cookie(token: string, maxAge: number): string {
  return `${COOKIE_NAME}=${token}; ${COOKIE_ATTRIBUTES}; Max-Age=${maxAge}`;
}

function validToken(value: unknown): value is string {
  return typeof value === "string" && value.length > 0 && value.length <= 4096
    && /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(value);
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
        throw new SessionFailure(413, "body_too_large");
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

function acceptsOrigin(request: Request): boolean {
  const origin = request.headers.get("origin");
  if (origin === null) return false;
  try {
    const parsedOrigin = originUrl(origin, "/").origin;
    if (parsedOrigin === new URL(request.url).origin) return true;
    const host = request.headers.get("host");
    if (host === null || !/^(?:\[[0-9a-f:.]+\]|[^\s:/\\?#@,\[\]%]+)(?::[0-9]+)?$/i.test(host)) {
      return false;
    }
    return parsedOrigin === new URL(`https://${host}`).origin;
  } catch {
    return false;
  }
}

async function requestDocument(request: Request): Promise<Document> {
  if (!acceptsOrigin(request)) {
    throw new SessionFailure(403, "origin_refused");
  }
  if (!/^application\/json(?:\s*;\s*charset=utf-8)?$/i.test(request.headers.get("content-type") ?? "")) {
    throw new SessionFailure(415, "json_required");
  }
  const length = request.headers.get("content-length");
  if (length !== null && (!/^\d+$/.test(length) || Number(length) > SESSION_BODY_LIMIT)) {
    throw new SessionFailure(413, "body_too_large");
  }
  try {
    const body = await boundedBody(request.body, SESSION_BODY_LIMIT);
    return document(body === "" && request.method === "DELETE" ? {} : JSON.parse(body));
  } catch (error) {
    if (error instanceof SessionFailure) throw error;
    throw new SessionFailure(400, "invalid_request");
  }
}

function exactFields(body: Document, fields: string[]): void {
  if (Object.keys(body).length !== fields.length || fields.some((field) => !Object.hasOwn(body, field))) {
    throw new SessionFailure(400, "invalid_request");
  }
}

function hex(value: unknown, bytes: number): value is string {
  return typeof value === "string" && value.length === bytes * 2 && /^[0-9a-f]+$/.test(value);
}

function proofParameters(body: Document, registration: boolean): string[] {
  const signature = registration ? "registration_signature" : "session_signature";
  exactFields(body, registration ? ["signer_public_key", signature] : ["signer_public_key", "proof_time", signature]);
  if (!hex(body.signer_public_key, 32) || !hex(body[signature], 64)) {
    throw new SessionFailure(400, "invalid_proof");
  }
  if (registration) return [body.signer_public_key, body[signature]];
  const time = body.proof_time;
  if (typeof time !== "string" || !/^(0|[1-9][0-9]{0,19})$/.test(time) || BigInt(time) > 18446744073709551615n) {
    throw new SessionFailure(400, "invalid_proof");
  }
  return [body.signer_public_key, time, body[signature]];
}

function originUrl(origin: string, path: string): URL {
  const url = new URL(origin);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname !== "/") {
    throw new Error("invalid origin");
  }
  return new URL(path, url);
}

async function apiAccepts(token: string, apiOrigin: string): Promise<void> {
  try {
    const upstream = await fetch(originUrl(apiOrigin, "/v1/dashboard/overview"), {
      headers: { Authorization: `Bearer ${token}` },
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(SESSION_UPSTREAM_TIMEOUT_MS),
    });
    void upstream.body?.cancel().catch(() => undefined);
    if (upstream.status === 401) throw new SessionFailure(401, "session_required");
    if (upstream.status !== 200) throw new SessionFailure(502, "dashboard_refused");
  } catch (error) {
    if (error instanceof SessionFailure) throw error;
    throw new SessionFailure(502, "dashboard_unavailable");
  }
}

function gatewayReason(error: Document): string {
  const data = document(error.data);
  const nested = data.error && typeof data.error === "object" ? document(data.error) : undefined;
  const reason = data.code ?? nested?.code;
  if (typeof reason !== "string" || !/^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$/.test(reason) || reason.length > 96) {
    throw new SessionFailure(502, "gateway_invalid_response");
  }
  return reason;
}

async function gatewayCall(origin: string, registration: boolean, params: string[]): Promise<Document> {
  try {
    const upstream = await fetch(originUrl(origin, "/"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: registration ? "px_register" : "px_openSession", params }),
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(SESSION_UPSTREAM_TIMEOUT_MS),
    });
    if (upstream.status !== 200 || !/^application\/json(?:\s*;|$)/i.test(upstream.headers.get("content-type") ?? "")) {
      void upstream.body?.cancel().catch(() => undefined);
      throw new SessionFailure(502, "gateway_invalid_response");
    }
    const answer = document(JSON.parse(await boundedBody(upstream.body, UPSTREAM_BODY_LIMIT)));
    if (answer.jsonrpc !== "2.0" || answer.id !== 1 || (Object.hasOwn(answer, "result") === Object.hasOwn(answer, "error"))) {
      throw new SessionFailure(502, "gateway_invalid_response");
    }
    if (answer.error !== undefined) {
      const error = document(answer.error);
      throw new SessionFailure(error.code === -32002 ? 401 : error.code === -32005 ? 429 : 502, gatewayReason(error));
    }
    return document(answer.result);
  } catch (error) {
    if (error instanceof SessionFailure && error.status !== 400 && error.status !== 413) throw error;
    throw new SessionFailure(502, "gateway_unavailable");
  }
}

export async function handleSessionRequest(request: Request, origins: Origins): Promise<Response> {
  try {
    const body = await requestDocument(request);
    const path = new URL(request.url).pathname;
    if (path === "/session" && request.method === "DELETE") {
      exactFields(body, []);
      return response(204, null, cookie("", 0));
    }
    if (request.method !== "POST" || !["/session", "/session/registration"].includes(path)) {
      throw new SessionFailure(405, "method_refused");
    }
    if (path === "/session" && Object.hasOwn(body, "token")) {
      exactFields(body, ["token"]);
      if (!validToken(body.token)) throw new SessionFailure(400, "invalid_token");
      await apiAccepts(body.token, origins.apiOrigin);
      return response(204, null, cookie(body.token, 12 * 60 * 60));
    }
    const registration = path === "/session/registration";
    const params = proofParameters(body, registration);
    if (!origins.gatewayOrigin) throw new SessionFailure(503, "sign_in_not_configured");
    const result = await gatewayCall(origins.gatewayOrigin, registration, params);
    if (registration) {
      if (typeof result.tenant !== "string" || !/^[a-z0-9._-]{1,95}$/.test(result.tenant)
        || typeof result.sub !== "string" || !/^[a-z0-9._-]{1,128}$/.test(result.sub)
        || !Array.isArray(result.allowed_signer_public_keys) || result.allowed_signer_public_keys.length !== 1
        || result.allowed_signer_public_keys[0] !== params[0]) {
        throw new SessionFailure(502, "gateway_invalid_response");
      }
      return response(200, { tenant: result.tenant, sub: result.sub, allowed_signer_public_keys: [params[0]] });
    }
    const expires = result.expires_at;
    const now = Math.floor(Date.now() / 1000);
    if (!validToken(result.token) || typeof expires !== "number" || !Number.isSafeInteger(expires) || expires <= now) {
      throw new SessionFailure(502, "gateway_invalid_response");
    }
    return response(204, null, cookie(result.token, expires - now));
  } catch (error) {
    const failure = error instanceof SessionFailure ? error : new SessionFailure(400, "invalid_request");
    return response(failure.status, { reason: failure.message });
  }
}
