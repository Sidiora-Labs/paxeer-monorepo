import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { test } from "node:test";
import { handleKeysRequest, KEY_BODY_LIMIT, KEY_UPSTREAM_TIMEOUT_MS } from "../app/keys-server.ts";

// These cases follow manage_keys, issue_key, list_keys and rotate_key in the
// gateway source. The stand-in returns the same key fields as issue_key and
// rotate_key, and the same error code field as response.
const browserOrigin = "https://dashboard.example";
const session = "session_fixture_opaque";
const keyId = "key_01_ABC";
const issueBody = {
  signer_public_key: "ab".repeat(32),
  scopes: ["receipt:read", "state:read"],
  quota_requests: 1000,
  quota_window_seconds: 3600,
};

function request(body, { path = "/keys", method = "POST", origin = browserOrigin, cookie = session, contentType = "application/json", headers = {}, raw } = {}) {
  return new Request(`${browserOrigin}${path}`, {
    method,
    headers: {
      Origin: origin,
      "Content-Type": contentType,
      ...(cookie === undefined ? {} : { Cookie: `__Host-layerx-session=${cookie}` }),
      ...headers,
    },
    body: raw ?? JSON.stringify(body),
  });
}

async function peer(t, handler) {
  const calls = [];
  const errors = [];
  const server = createServer(async (incoming, outgoing) => {
    try {
      const chunks = [];
      for await (const chunk of incoming) chunks.push(chunk);
      const call = {
        method: incoming.method,
        path: incoming.url,
        headers: incoming.headers,
        body: Buffer.concat(chunks).toString(),
      };
      calls.push(call);
      const reply = await handler(call);
      if (reply === undefined) return;
      outgoing.writeHead(reply.status ?? 200, { "Content-Type": "application/json", ...reply.headers });
      outgoing.end(typeof reply.body === "string" ? reply.body : JSON.stringify(reply.body));
    } catch (error) {
      errors.push(error);
      outgoing.writeHead(500);
      outgoing.end();
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    assert.deepEqual(errors, []);
  });
  return { origin: `http://127.0.0.1:${server.address().port}`, calls };
}

function keyResult(secret = "lxp_live_" + "cd".repeat(32)) {
  return {
    ok: true,
    key: {
      id: keyId,
      secret,
      authorization_scheme: "LayerX-Key",
      signer_public_key: issueBody.signer_public_key,
      scopes: issueBody.scopes,
      quota_requests: issueBody.quota_requests,
      quota_window_seconds: issueBody.quota_window_seconds,
    },
  };
}

function errorResult(code) {
  return { ok: false, error: { code } };
}

async function issue_passes_session_idempotency_and_body_once(t) {
  const gateway = await peer(t, (call) => {
    assert.equal(call.method, "POST");
    assert.equal(call.path, "/v1/keys");
    assert.equal(call.headers.authorization, `Bearer ${session}`);
    assert.equal(call.headers["idempotency-key"], "issue_01");
    assert.deepEqual(JSON.parse(call.body), issueBody);
    return { status: 201, body: keyResult() };
  });
  const result = await handleKeysRequest(request(issueBody, { headers: { "Idempotency-Key": "issue_01" } }), { gatewayOrigin: gateway.origin });
  assert.equal(result.status, 201);
  assert.equal(result.headers.get("cache-control"), "no-store");
  assert.deepEqual(await result.json(), keyResult());
  assert.equal(gateway.calls.length, 1);
}

test(issue_passes_session_idempotency_and_body_once);

async function request_prologue_refuses_before_gateway_call(t) {
  const gateway = await peer(t, () => { throw new Error("gateway must not be called"); });
  const cases = [
    request(issueBody, { cookie: "", headers: { "Idempotency-Key": "issue_01" } }),
    request(issueBody, { origin: "https://other.example", headers: { "Idempotency-Key": "issue_01" } }),
    request(issueBody, { contentType: "text/plain", headers: { "Idempotency-Key": "issue_01" } }),
    request({}, { raw: "x".repeat(KEY_BODY_LIMIT + 1), headers: { "Idempotency-Key": "issue_01" } }),
    request(issueBody),
    request({}, { path: "/keys/not.valid/rotate", headers: { "Idempotency-Key": "rotate_01" } }),
  ];
  for (const [index, incoming] of cases.entries()) {
    const result = await handleKeysRequest(incoming, { gatewayOrigin: gateway.origin });
    assert.ok([400, 401, 403, 404, 413, 415].includes(result.status), `case ${index} status`);
  }
  assert.equal(gateway.calls.length, 0);
}

test(request_prologue_refuses_before_gateway_call);

async function gateway_reason_names_are_forwarded(t) {
  const reasons = [
    [401, "session_required"],
    [403, "signer_not_owned"],
    [400, "invalid_key_request"],
    [409, "idempotency_conflict"],
    [409, "key_limit_reached"],
    [503, "persistence_unavailable"],
  ];
  const gateway = await peer(t, (call) => {
    const [, code] = reasons[gateway.calls.length - 1];
    return { status: code === "persistence_unavailable" ? 503 : reasons[gateway.calls.length - 1][0], body: errorResult(code) };
  });
  for (const [index] of reasons.entries()) {
    const result = await handleKeysRequest(request(issueBody, { headers: { "Idempotency-Key": `reason_${index}` } }), { gatewayOrigin: gateway.origin });
    assert.equal(result.status, reasons[index][0]);
    assert.deepEqual(await result.json(), { reason: reasons[index][1] });
  }
}

test(gateway_reason_names_are_forwarded);

async function rotation_and_revocation_use_owned_key_paths(t) {
  const gateway = await peer(t, (call) => call.method === "POST"
    ? { status: 201, body: keyResult("lxp_live_" + "ef".repeat(32)) }
    : { status: 200, body: { ok: true, id: keyId, state: "revoked" } });
  const rotated = await handleKeysRequest(request({}, { path: `/keys/${keyId}/rotate`, headers: { "Idempotency-Key": "rotate_01" } }), { gatewayOrigin: gateway.origin });
  assert.equal(rotated.status, 201);
  const revoked = await handleKeysRequest(request({}, { path: `/keys/${keyId}`, method: "DELETE" }), { gatewayOrigin: gateway.origin });
  assert.equal(revoked.status, 200);
  assert.deepEqual(gateway.calls.map((call) => [call.method, call.path]), [
    ["POST", `/v1/keys/${keyId}/rotate`],
    ["DELETE", `/v1/keys/${keyId}`],
  ]);
}

test(rotation_and_revocation_use_owned_key_paths);

async function timeout_returns_gateway_unavailable(t) {
  const gateway = await peer(t, () => undefined);
  const started = Date.now();
  const result = await handleKeysRequest(request(issueBody, { headers: { "Idempotency-Key": "timeout_01" } }), { gatewayOrigin: gateway.origin });
  assert.equal(result.status, 502);
  assert.deepEqual(await result.json(), { reason: "gateway_unavailable" });
  assert.ok(Date.now() - started >= KEY_UPSTREAM_TIMEOUT_MS - 250);
}

test(timeout_returns_gateway_unavailable, { timeout: KEY_UPSTREAM_TIMEOUT_MS + 2000 });
