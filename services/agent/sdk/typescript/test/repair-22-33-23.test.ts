import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { createServer as createTlsServer } from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

import {
  AGENT_ENVELOPE_OPERATION_NAMES,
  AGENT_ENVELOPE_PATH,
  AgentEnvelopeTransport,
  AgentSessionCredential,
  decodeTenantReadiness,
  LayerXKeyCredential,
} from "../src/agent-http.js";
import {
  abiSelector,
  exchangePlaceOrderCall,
  LAYERX_EXCHANGE_PRECOMPILE,
  PrecompileAbiError,
  precompileWriteCall,
} from "../src/exchange.js";
import { historyCursor, decodeUnifiedHistoryPage } from "../src/history.js";
import { PlatformSdkError, SecretBytes } from "../src/production.js";
import { PxClient } from "../src/px.js";
import { JsonRpcError } from "../src/rpc.js";
import { runAgentOperationEnvelopeProbe } from "./agent-operation-envelope.test.js";
import { gas_nonce_root_route_uses_pending } from "./support/gas-root-route.js";
import { exchangeOrderAbi } from "./support/exchange-abi.js";

// These imports execute the repaired tests, including their original refusals.
// px.test.ts also invokes a Python parity gate; the focused route proof below
// uses its committed gateway recording without invoking another toolchain.
import "./binding-preflight.test.js";
import "./names.test.js";
import "./precompile-calls.test.js";
import "./program-lifecycle.test.js";

const root = new URL("../../../../../../", import.meta.url);
const source = (path: string): string => readFileSync(new URL(path, root), "utf8");
const fixture = <T>(path: string): T => JSON.parse(source(path)) as T;
const goldens = "services/agent/schema/agent-api/golden/";

async function listen(server: Server, protocol = "http"): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert(address !== null && typeof address !== "string");
  return `${protocol}://127.0.0.1:${address.port}`;
}

async function close(server: Server): Promise<void> {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}

interface EnvelopeRecording {
  readonly request_id: string;
  readonly value?: unknown;
  readonly verification_status?: unknown;
}

async function daemon_probe_uses_hosted_authentication_and_disclosure(): Promise<void> {
  const original = fixture<{
    readonly credential: { readonly tenant: string; readonly session_id: string; readonly token_id: string; readonly generation: string };
  }>(goldens + "agent-http-request.json");
  const credential = original.credential;
  const tlsRoot = "services/platform/hosted/node/tests/fixtures/repair_22_31_355/";
  const cert = source(tlsRoot + "tls-certificate.pem.txt");
  const key = source(tlsRoot + "tls-private-key.pem.txt");
  const directory = mkdtempSync(join(tmpdir(), "hosted-envelope-22-33-23-"));
  const failures: unknown[] = [];
  const seen: { readonly operation: string; readonly credential: unknown; readonly idempotency_key: unknown }[] = [];
  let recording = fixture<EnvelopeRecording>(goldens + "agent-http-success.json");
  const submitted = fixture<EnvelopeRecording>(goldens + "agent-http-success.json");
  let readinessReply: unknown;
  let malformedBody = '{"result":null}';
  const apiKey = `probe:lxp_live_${"22".repeat(32)}`;
  const server = createTlsServer({ cert, key }, async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, AGENT_ENVELOPE_PATH);
      assert.equal(request.headers["content-type"], "application/json");
      assert.equal(request.headers.accept, "application/json");
      assert.equal(request.headers["user-agent"], "layerx-typescript/0.1.0");
      assert.equal(request.headers.authorization, `LayerX-Key ${apiKey}`);
      assert.equal(request.headers["layerx-tenant"], undefined);
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as {
        readonly version: number; readonly request_id: string; readonly operation: string;
        readonly credential: typeof credential; readonly idempotency_key: unknown;
      };
      assert.equal(body.version, 1);
      assert.equal(Number(request.headers["content-length"]), Buffer.concat(chunks).length);
      assert.match(body.request_id, /^(?:0|[1-9][0-9]*)$/u);
      seen.push(body);
      const stale = body.credential.generation !== credential.generation;
      if (!stale) assert.deepEqual(body.credential, credential);
      const reply = stale ? fixture<EnvelopeRecording>(goldens + "agent-http-refusal.json")
        : body.operation === "tenant.readiness" ? { ...submitted, value: readinessReply } : recording;
      response.writeHead(stale ? 403 : 200, { "content-type": "application/json" });
      response.end(JSON.stringify({ ...reply, request_id: body.request_id }));
    } catch (error) {
      failures.push(error);
      response.destroy();
    }
  });
  const malformedRequests: string[] = [];
  const malformed = createTlsServer({ cert, key }, async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, AGENT_ENVELOPE_PATH);
      assert.equal(request.headers["content-type"], "application/json");
      assert.equal(request.headers.accept, "application/json");
      assert.equal(request.headers.authorization, `LayerX-Key ${apiKey}`);
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as {
        readonly operation: string; readonly idempotency_key: string | null; readonly credential: unknown;
      };
      assert.deepEqual(body.credential, credential);
      assert.equal(body.idempotency_key, body.operation === "read.account" ? null : "33".repeat(32));
      malformedRequests.push(body.operation);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(malformedBody);
    } catch (error) {
      failures.push(error);
      response.destroy();
    }
  });
  let endpoint: string | undefined;
  let malformedEndpoint: string | undefined;
  try {
    endpoint = await listen(server, "https");
    malformedEndpoint = await listen(malformed, "https");
    writeFileSync(join(directory, "ca.pem"), cert, { mode: 0o600 });
    writeFileSync(join(directory, "credential.json"), JSON.stringify(credential), { mode: 0o600 });
    writeFileSync(join(directory, "gateway-key"), apiKey + "\n", { mode: 0o600 });
    const read = { operation: "read.account", request: { account: "11".repeat(32), requested_verification_level: 0 } };
    for (const [index, filename] of ["agent-http-success.json", "agent-http-success-pending.json", "agent-http-success-absent.json"].entries()) {
      recording = fixture<EnvelopeRecording>(goldens + filename);
      const readiness = decodeTenantReadiness(index === 0
        ? { transport_ready: true, verified_reads_ready: true, writes_admitted: true, recovery_reason: null }
        : index === 1
          ? { transport_ready: true, verified_reads_ready: false, writes_admitted: false, recovery_reason: "verified_read_unavailable" }
          : { transport_ready: true, verified_reads_ready: true, writes_admitted: false, recovery_reason: "recovery_pending" });
      readinessReply = readiness;
      const responseDir = join(directory, `responses-${index}`);
      mkdirSync(responseDir);
      const input = {
        phase: "read", operations: AGENT_ENVELOPE_OPERATION_NAMES,
        endpoint: endpoint + AGENT_ENVELOPE_PATH, server_name: "127.0.0.1",
        ca_pem: join(directory, "ca.pem"), credential_file: join(directory, "credential.json"),
        gateway_api_key_file: join(directory, "gateway-key"), response_dir: responseDir,
        cases: ["read_hosted", "readiness_hosted", "read_decode_failure", "mutation_decode_unknown"],
        requests: {
          read, read_hosted: read,
          readiness_hosted: { operation: "tenant.readiness", request: {}, expected_readiness: readiness },
          read_decode_failure: { ...read, endpoint: malformedEndpoint + AGENT_ENVELOPE_PATH },
          mutation_decode_unknown: { operation: "session.close", request: {}, idempotency_key: "33".repeat(32), endpoint: malformedEndpoint + AGENT_ENVELOPE_PATH },
        },
      };
      const path = join(directory, "case.json");
      writeFileSync(path, JSON.stringify(input), { mode: 0o600 });
      await runAgentOperationEnvelopeProbe(path);
      const observed = JSON.parse(readFileSync(join(responseDir, "read_hosted.json"), "utf8")) as { status: number; body: EnvelopeRecording };
      assert.equal(observed.status, 200);
      assert.deepEqual(observed.body.value, recording.value);
      assert.deepEqual(observed.body.verification_status, recording.verification_status);
      const readinessObserved = JSON.parse(readFileSync(join(responseDir, "readiness_hosted.json"), "utf8")) as { status: number; body: EnvelopeRecording };
      assert.equal(readinessObserved.status, 200);
      assert.deepEqual(readinessObserved.body.value, readiness);
      for (const name of ["read_decode_failure", "mutation_decode_unknown"]) {
        const response = JSON.parse(readFileSync(join(responseDir, name + ".json"), "utf8")) as { status: number; body: unknown };
        assert.deepEqual(response, { status: 200, body: { result: null } });
      }
      writeFileSync(path, JSON.stringify({ ...input, daemon_endpoint: endpoint + "/rpc" }), { mode: 0o600 });
      const before = seen.length;
      await assert.rejects(runAgentOperationEnvelopeProbe(path), /retired probe field daemon_endpoint/u);
      assert.equal(seen.length, before);
    }
    assert.deepEqual(malformedRequests, Array.from({ length: 3 }, () => ["read.account", "session.close"]).flat());
    assert.deepEqual(seen.map(request => request.operation), Array.from({ length: 3 }, () => ["read.account", "tenant.readiness", "read.account"]).flat());
    assert(seen.every(request => request.idempotency_key === null));
    const transport = new AgentEnvelopeTransport({
      endpoint, trustedCa: cert,
      session: new AgentSessionCredential(credential.tenant, credential.session_id, new SecretBytes(new Uint8Array(Buffer.from(credential.token_id, "hex"))), credential.generation),
      gatewayCredential: new LayerXKeyCredential("probe", new SecretBytes(new Uint8Array(Buffer.from(`lxp_live_${"22".repeat(32)}`, "ascii")))),
    });
    malformedBody = '{"result":';
    const truncatedPath = join(directory, "truncated-case.json");
    writeFileSync(truncatedPath, JSON.stringify({
      phase: "read", operations: AGENT_ENVELOPE_OPERATION_NAMES,
      endpoint: endpoint + AGENT_ENVELOPE_PATH, server_name: "127.0.0.1",
      ca_pem: join(directory, "ca.pem"), credential_file: join(directory, "credential.json"),
      gateway_api_key_file: join(directory, "gateway-key"), response_dir: join(directory, "responses-0"),
      cases: ["read_transport_failure", "mutation_decode_unknown"],
      requests: {
        read: { request: {} },
        read_transport_failure: { operation: "read.account", request: {}, endpoint: malformedEndpoint + AGENT_ENVELOPE_PATH },
        mutation_decode_unknown: { operation: "session.close", request: {}, idempotency_key: "33".repeat(32), endpoint: malformedEndpoint + AGENT_ENVELOPE_PATH },
      },
    }), { mode: 0o600 });
    await runAgentOperationEnvelopeProbe(truncatedPath);
    assert.deepEqual(malformedRequests.slice(-2), ["read.account", "session.close"]);
    for (const name of ["read_transport_failure", "mutation_decode_unknown"]) {
      const response = JSON.parse(readFileSync(join(directory, "responses-0", name + ".json"), "utf8")) as { status: number; body: unknown };
      assert.deepEqual(response, { status: 200, body: '{"result":' });
    }
    // A generic success/absence must never masquerade as tenant readiness.
    readinessReply = recording.value;
    await assert.rejects(transport.tenantReadiness(), (error: unknown) => error instanceof PlatformSdkError && error.code === "decode-failure");
    readinessReply = decodeTenantReadiness({ transport_ready: true, verified_reads_ready: true, writes_admitted: true, recovery_reason: null });
    const diagnosticPath = join(directory, "diagnostic-case.json");
    writeFileSync(diagnosticPath, JSON.stringify({
      phase: "read", operations: AGENT_ENVELOPE_OPERATION_NAMES,
      endpoint: endpoint + AGENT_ENVELOPE_PATH, server_name: "127.0.0.1",
      ca_pem: join(directory, "ca.pem"), credential_file: join(directory, "credential.json"),
      gateway_api_key_file: join(directory, "gateway-key"), response_dir: join(directory, "responses-0"),
      cases: ["readiness_hosted"],
      requests: {
        read: { request: {} },
        readiness_hosted: { operation: "tenant.readiness", request: {},
          expected_readiness: { transport_ready: true, verified_reads_ready: false, writes_admitted: true, recovery_reason: null } },
      },
    }), { mode: 0o600 });
    await assert.rejects(runAgentOperationEnvelopeProbe(diagnosticPath), (error: unknown) => {
      assert(error instanceof Error);
      assert.match(error.message, /qualification failed: readiness_hosted: readiness_hosted: readiness verified_reads_ready differs/u);
      assert.equal(error.message.includes(apiKey), false);
      assert.equal(error.message.includes(credential.token_id), false);
      return true;
    });
    assert.deepEqual(failures, []);
  } finally {
    if (malformedEndpoint !== undefined) await close(malformed);
    if (endpoint !== undefined) await close(server);
    rmSync(directory, { recursive: true, force: true });
  }
}

async function px_root_route_preserves_history_cursor_and_errors(): Promise<void> {
  const recorded = fixture<{ readonly result: Record<string, unknown> }>("apps/web/e2e/fixtures/px-get-unified-history.json").result;
  const account = "0x" + "ee".repeat(20);
  const expected = decodeUnifiedHistoryPage(recorded);
  const cursor = historyCursor(12n, 0, 0);
  const calls: { readonly method: string; readonly params: unknown }[] = [];
  const failures: unknown[] = [];
  let refusal = false;
  let malformed = false;
  const server = createServer(async (request, response) => {
    if (request.url !== "/") {
      request.resume();
      response.writeHead(404);
      response.end();
      return;
    }
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.headers["content-type"], "application/json");
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as {
        readonly jsonrpc: string; readonly id: string; readonly method: string; readonly params: unknown;
      };
      assert.equal(body.jsonrpc, "2.0");
      assert.equal(body.method, "px_getUnifiedHistory");
      calls.push(body);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, ...(refusal
        ? { error: { code: -32001, message: "Paxeer read unavailable", data: { code: "paxeer_unreachable" } } }
        : { result: malformed ? { ...recorded, next_cursor: "not-a-cursor" } : recorded }) }));
    } catch (error) {
      failures.push(error);
      response.destroy();
    }
  });
  const endpoint = await listen(server);
  try {
    for (const suffix of ["/rpc", "/rpc/ws"]) {
      const response = await fetch(endpoint + suffix, { method: "POST", body: "{}" });
      assert.equal(response.status, 404);
    }
    assert.equal(calls.length, 0);
    const client = new PxClient({ endpoint });
    assert.deepEqual(await client.getUnifiedHistory(account.toUpperCase(), { limit: 4 }), expected);
    assert.deepEqual(calls.at(-1), { jsonrpc: "2.0", id: "1", method: "px_getUnifiedHistory", params: [account, null, 4, null] });
    assert.deepEqual(await client.getUnifiedHistory(account, { cursor, limit: 4, kind: "native_transfer" }), expected);
    assert.deepEqual(calls.at(-1), { jsonrpc: "2.0", id: "2", method: "px_getUnifiedHistory", params: [account, cursor, 4, "native_transfer"] });
    malformed = true;
    await assert.rejects(client.getUnifiedHistory(account));
    malformed = false;
    refusal = true;
    await assert.rejects(client.getUnifiedHistory(account), (error: unknown) =>
      error instanceof JsonRpcError && error.code === -32001 && error.message === "Paxeer read unavailable" &&
      (error.data as { code: string }).code === "paxeer_unreachable");
    const before = calls.length;
    await assert.rejects(client.getUnifiedHistory(account, { cursor: "bad" }));
    await assert.rejects(client.getUnifiedHistory("not-an-account"));
    assert.equal(calls.length, before);
    assert.deepEqual(failures, []);
  } finally {
    await close(server);
  }
}

function root_route_contracts_match_the_actual_producers(): void {
  assert.match(source("rpc/rpcstack.go"), /if path == "" \{\s*return r.URL.Path == "\/"/u);
  assert.match(source("services/platform/hosted/gateway/src/main.rs"), /request.path == "\/" && request.method == "POST"/u);
}

function exchange_account_bearing_dispatch_is_required(): void {
  const { entry, signature } = exchangeOrderAbi(true);
  const legacy = exchangeOrderAbi(false);
  assert.deepEqual(entry.inputs?.map(input => [input.name, input.type]), [
    ["market", "bytes32"], ["accountId", "bytes32"], ["side", "uint8"],
    ["price", "uint256"], ["qty", "uint256"], ["tif", "uint8"],
  ]);
  assert.deepEqual(legacy.entry.inputs, entry.inputs!.filter(input => input.name !== "accountId"));
  const recorded = fixture<{
    readonly producer: string; readonly signature: string;
    readonly marketId: string; readonly accountId: string; readonly side: number;
    readonly price: string; readonly quantity: string; readonly timeInForce: number;
    readonly to: string; readonly data: string; readonly value: string;
  }>("precompiles/layerxexchange/testdata/repair_459_typescript.json");
  assert.equal(recorded.producer, "services/agent/sdk/typescript/src/exchange.ts#exchangePlaceOrderCall");
  assert.equal(recorded.signature, signature);
  const order = { ...recorded, price: BigInt(recorded.price), quantity: BigInt(recorded.quantity) };
  const call = exchangePlaceOrderCall(order);
  assert.deepEqual(call, { to: recorded.to, data: recorded.data, value: BigInt(recorded.value) });
  assert.equal(call.to, LAYERX_EXCHANGE_PRECOMPILE);
  assert.equal(call.value, 0n);
  assert.equal(call.data.slice(0, 10), abiSelector(signature));
  assert.equal(call.data.length, 10 + entry.inputs!.length * 64);
  const word = (value: bigint | number): string => BigInt(value).toString(16).padStart(64, "0");
  const words = [order.marketId.slice(2), order.accountId.slice(2), word(order.side),
    word(order.price), word(order.quantity), word(order.timeInForce)];
  assert.deepEqual(call.data.slice(10).match(/.{64}/gu), words);

  const values = [order.marketId, order.accountId, BigInt(order.side), order.price,
    order.quantity, BigInt(order.timeInForce)];
  const accountWrite = { types: entry.inputs!.map(input => input.type), payable: false };
  assert.deepEqual(precompileWriteCall(call.to, signature, accountWrite, values), call);
  const legacyWrite = { types: legacy.entry.inputs!.map(input => input.type), payable: false };
  const legacyValues = values.filter((_, index) => index !== 1);
  const legacyCall = precompileWriteCall(call.to, legacy.signature, legacyWrite, legacyValues);
  assert.equal(legacyCall.to, call.to);
  assert.equal(legacyCall.value, 0n);
  assert.equal(legacyCall.data.slice(0, 10), abiSelector(legacy.signature));
  assert.notEqual(legacyCall.data.slice(0, 10), call.data.slice(0, 10));
  assert.equal(legacyCall.data.length, 10 + 5 * 64);
  assert.deepEqual(legacyCall.data.slice(10).match(/.{64}/gu), words.filter((_, index) => index !== 1));
  assert.throws(() => precompileWriteCall(call.to, signature, accountWrite, legacyValues), PrecompileAbiError);
  assert.throws(() => precompileWriteCall(call.to, legacy.signature, legacyWrite, values), PrecompileAbiError);
}

test(daemon_probe_uses_hosted_authentication_and_disclosure);
test(px_root_route_preserves_history_cursor_and_errors);
test(root_route_contracts_match_the_actual_producers);
test(exchange_account_bearing_dispatch_is_required);
test(gas_nonce_root_route_uses_pending);
