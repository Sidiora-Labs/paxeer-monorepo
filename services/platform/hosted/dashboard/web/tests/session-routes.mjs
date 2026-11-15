import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { test } from "node:test";
import { handleSessionRequest, SESSION_BODY_LIMIT, SESSION_UPSTREAM_TIMEOUT_MS } from "../app/session-server.ts";

const browserOrigin = "https://dashboard.example";
const token = "session_fixture_opaque";
const csrf = "csrf_fixture_opaque";
const key = "ab".repeat(32);
const signature = "cd".repeat(64);
const registration = { signer_public_key: key, registration_signature: signature };
const proof = { signer_public_key: key, proof_time: "1700000000", session_signature: signature };

function request(body, { path = "/session", method = "POST", origin = browserOrigin, contentType = "application/json", raw, headers = {} } = {}) {
  return new Request(`${browserOrigin}${path}`, {
    method,
    headers: { ...(origin === null ? {} : { Origin: origin }), "Content-Type": contentType, ...headers },
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
      const body = Buffer.concat(chunks).toString();
      const call = { method: incoming.method, path: incoming.url, headers: incoming.headers, body };
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

function rpcResult(result) {
  return { body: { jsonrpc: "2.0", id: 1, result } };
}

function assertCookie(response, value, maxAge) {
  const parts = response.headers.get("set-cookie")?.split(/;\s*/);
  assert.ok(parts, "session cookie is present");
  assert.equal(parts[0], `__Host-layerx-session=${value}`);
  assert.deepEqual(new Set(parts.slice(1)), new Set(["Secure", "HttpOnly", "SameSite=Strict", "Path=/", `Max-Age=${maxAge}`]));
  assert.equal(response.headers.get("cache-control"), "no-store");
}

async function assertPrivateResponse(response) {
  const body = await response.text();
  assert.ok(!body.includes(token));
  assert.ok(!body.includes(csrf));
  for (const [name, value] of response.headers) {
    if (name !== "set-cookie") {
      assert.ok(!value.includes(token));
      assert.ok(!value.includes(csrf));
    }
  }
  return body;
}

async function accepted_session_proof_sets_only_the_host_cookie(t) {
  const expires = Math.floor(Date.now() / 1000) + 3600;
  const gateway = await peer(t, (call) => {
    assert.equal(call.path, "/");
    assert.equal(call.method, "POST");
    assert.equal(call.headers["content-type"], "application/json");
    assert.equal(call.headers.authorization, undefined);
    assert.deepEqual(JSON.parse(call.body), { jsonrpc: "2.0", id: 1, method: "px_openSession", params: [key, proof.proof_time, signature] });
    return rpcResult({ tenant: "beta", sub: "beta.developer", token, csrf_token: csrf, expires_at: expires });
  });
  const before = Math.floor(Date.now() / 1000);
  const result = await handleSessionRequest(request(proof), { gatewayOrigin: gateway.origin, apiOrigin: gateway.origin });
  const after = Math.floor(Date.now() / 1000);
  assert.equal(result.status, 204);
  const age = Number(result.headers.get("set-cookie").match(/Max-Age=(\d+)/)[1]);
  assert.ok(age >= expires - after && age <= expires - before);
  assertCookie(result, token, age);
  assert.equal(await assertPrivateResponse(result), "");
  assert.equal(gateway.calls.length, 1);
}

test(accepted_session_proof_sets_only_the_host_cookie);

async function accepted_token_uses_the_api_and_lasts_twelve_hours(t) {
  const api = await peer(t, (call) => {
    assert.equal(call.method, "GET");
    assert.equal(call.path, "/v1/dashboard/overview");
    assert.equal(call.headers.authorization, `Bearer ${token}`);
    assert.equal(call.headers.cookie, undefined);
    return { body: { principal: "developer" } };
  });
  const result = await handleSessionRequest(request({ token }), { apiOrigin: api.origin });
  assert.equal(result.status, 204);
  assertCookie(result, token, 43200);
  assert.equal(await assertPrivateResponse(result), "");
  assert.equal(api.calls.length, 1);
}

test(accepted_token_uses_the_api_and_lasts_twelve_hours);

async function refused_token_does_not_set_a_cookie_or_return_upstream_details(t) {
  const api = await peer(t, () => ({ status: 401, body: { token, error: { message: token } } }));
  const result = await handleSessionRequest(request({ token }), { apiOrigin: api.origin });
  assert.equal(result.status, 401);
  assert.equal(result.headers.get("set-cookie"), null);
  assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { reason: "session_required" });
}

test(refused_token_does_not_set_a_cookie_or_return_upstream_details);

async function registration_returns_only_the_confirmed_public_principal(t) {
  const gateway = await peer(t, (call) => {
    assert.deepEqual(JSON.parse(call.body), { jsonrpc: "2.0", id: 1, method: "px_register", params: [key, signature] });
    return rpcResult({ tenant: "beta", sub: "beta.developer", allowed_signer_public_keys: [key], token, csrf_token: csrf, extra: token });
  });
  const result = await handleSessionRequest(request(registration, { path: "/session/registration" }), { apiOrigin: gateway.origin, gatewayOrigin: gateway.origin });
  assert.equal(result.status, 200);
  assert.equal(result.headers.get("set-cookie"), null);
  assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { tenant: "beta", sub: "beta.developer", allowed_signer_public_keys: [key] });
}

test(registration_returns_only_the_confirmed_public_principal);

async function gateway_refusals_preserve_only_the_reason_and_map_the_status(t) {
  let current;
  const gateway = await peer(t, () => ({ body: { jsonrpc: "2.0", id: 1, error: current } }));
  const cases = [
    [-32002, "session_proof_invalid", 401],
    [-32002, "session_proof_replay", 401],
    [-32002, "session_time_out_of_range", 401],
    [-32005, "session_rate_limit", 429],
    [-32001, "identity_unavailable", 502],
    [-32603, "identity_session_invalid", 502],
    [-32602, "session_params_invalid", 502],
    [-32010, "kernel_unavailable", 502],
  ];
  for (const [code, reason, status] of cases) {
    current = { code, message: token, data: { code: reason, token, csrf_token: csrf, extra: token } };
    for (const [body, path] of [[proof, "/session"], [registration, "/session/registration"]]) {
      const result = await handleSessionRequest(request(body, { path }), { apiOrigin: gateway.origin, gatewayOrigin: gateway.origin });
      assert.equal(result.status, status);
      assert.equal(result.headers.get("set-cookie"), null);
      assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { reason });
    }
  }
  current = { code: -32002, message: token, data: { error: { code: "principal_refused", message: token } } };
  const nested = await handleSessionRequest(request(proof), { apiOrigin: gateway.origin, gatewayOrigin: gateway.origin });
  assert.equal(nested.status, 401);
  assert.deepEqual(JSON.parse(await assertPrivateResponse(nested)), { reason: "principal_refused" });
}

test(gateway_refusals_preserve_only_the_reason_and_map_the_status);

async function request_guards_refuse_before_contacting_either_origin(t) {
  const upstream = await peer(t, () => { throw new Error("Request guard contacts an origin"); });
  const origins = { apiOrigin: upstream.origin, gatewayOrigin: upstream.origin };
  const cases = [
    [{ origin: null }, 403, "origin_refused"],
    [{ origin: "https://other.example" }, 403, "origin_refused"],
    [{ contentType: "text/plain" }, 415, "json_required"],
    [{ raw: " ".repeat(SESSION_BODY_LIMIT + 1) }, 413, "body_too_large"],
    [{ headers: { "Content-Length": String(SESSION_BODY_LIMIT + 1) } }, 413, "body_too_large"],
    [{ raw: JSON.stringify({ padding: "é".repeat(SESSION_BODY_LIMIT / 2) }) }, 413, "body_too_large"],
    [{ raw: "invalid json" }, 400, "invalid_request"],
  ];
  for (const [path, method, body] of [["/session", "POST", proof], ["/session/registration", "POST", registration], ["/session", "DELETE", {}]]) {
    for (const [options, status, reason] of cases) {
      const result = await handleSessionRequest(request(body, { path, method, ...options }), origins);
      assert.equal(result.status, status);
      assert.equal(result.headers.get("set-cookie"), null);
      assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { reason });
    }
  }
  for (const body of [{ ...proof, token }, { ...proof, proof_time: "01" }, { ...proof, signer_public_key: key.toUpperCase() }, { token: `${token};other=value` }]) {
    const result = await handleSessionRequest(request(body), origins);
    assert.equal(result.status, 400);
    assert.equal(result.headers.get("set-cookie"), null);
    await assertPrivateResponse(result);
  }
  assert.equal(upstream.calls.length, 0);
}

test(request_guards_refuse_before_contacting_either_origin);

async function missing_gateway_refuses_proofs_and_keeps_token_sign_in_and_sign_out(t) {
  const api = await peer(t, () => ({ body: {} }));
  for (const [body, path] of [[proof, "/session"], [registration, "/session/registration"]]) {
    const result = await handleSessionRequest(request(body, { path }), { apiOrigin: api.origin });
    assert.equal(result.status, 503);
    assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { reason: "sign_in_not_configured" });
    assert.equal(result.headers.get("set-cookie"), null);
  }
  assert.equal(api.calls.length, 0);
  const accepted = await handleSessionRequest(request({ token }), { apiOrigin: api.origin });
  assert.equal(accepted.status, 204);
  assertCookie(accepted, token, 43200);
  assert.equal(await assertPrivateResponse(accepted), "");
  const deleted = await handleSessionRequest(request({}, { method: "DELETE" }), { apiOrigin: api.origin });
  assert.equal(deleted.status, 204);
  assertCookie(deleted, "", 0);
  assert.equal(await assertPrivateResponse(deleted), "");
  assert.equal(api.calls.length, 1);
}

test(missing_gateway_refuses_proofs_and_keeps_token_sign_in_and_sign_out);

async function upstream_failures_are_bounded_and_do_not_disclose_tokens(t) {
  const hanging = await peer(t, () => undefined);
  const start = Date.now();
  const results = await Promise.all([
    handleSessionRequest(request(proof), { gatewayOrigin: hanging.origin, apiOrigin: hanging.origin }),
    handleSessionRequest(request({ token }), { apiOrigin: hanging.origin }),
  ]);
  assert.ok(Date.now() - start < SESSION_UPSTREAM_TIMEOUT_MS + 2000);
  for (const [index, result] of results.entries()) {
    assert.equal(result.status, 502);
    assert.equal(result.headers.get("set-cookie"), null);
    assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { reason: index === 0 ? "gateway_unavailable" : "dashboard_unavailable" });
  }
  assert.equal(hanging.calls.length, 2);
  for (const origin of ["invalid", "http://user:password@localhost", "ftp://localhost"]) {
    const result = await handleSessionRequest(request({ token }), { apiOrigin: origin });
    assert.equal(result.status, 502);
    assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { reason: "dashboard_unavailable" });
  }
}

test(upstream_failures_are_bounded_and_do_not_disclose_tokens);

async function invalid_gateway_answers_never_set_a_session_cookie(t) {
  let reply;
  const gateway = await peer(t, () => reply);
  for (const body of [
    { jsonrpc: "2.0", id: 2, result: { token, expires_at: 9999999999 } },
    { jsonrpc: "2.0", id: 1, result: { token, expires_at: 1 } },
    { jsonrpc: "2.0", id: 1, result: { token: `${token};other=value`, expires_at: 9999999999 } },
    { jsonrpc: "2.0", id: 1, result: { token, expires_at: "9999999999" } },
    { jsonrpc: "2.0", id: 1, error: { code: -32002, data: { code: `invalid ${token}` }, message: token } },
    { jsonrpc: "2.0", id: 1, error: { code: -32002, data: { code: "session_proof_invalid" } }, result: { token } },
  ]) {
    reply = { body };
    const result = await handleSessionRequest(request(proof), { apiOrigin: gateway.origin, gatewayOrigin: gateway.origin });
    assert.equal(result.status, 502);
    assert.equal(result.headers.get("set-cookie"), null);
    await assertPrivateResponse(result);
  }
  reply = { body: { message: token }, status: 500 };
  const result = await handleSessionRequest(request(proof), { apiOrigin: gateway.origin, gatewayOrigin: gateway.origin });
  assert.equal(result.status, 502);
  assert.deepEqual(JSON.parse(await assertPrivateResponse(result)), { reason: "gateway_invalid_response" });
}

test(invalid_gateway_answers_never_set_a_session_cookie);
