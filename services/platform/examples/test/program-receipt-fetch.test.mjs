import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { test } from "node:test";

const sdkSourceRoot = new URL("../../../agent/sdk/typescript/src/", import.meta.url);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@sidiora/layerx-sdk") {
      return { url: new URL("index.ts", sdkSourceRoot).href, shortCircuit: true };
    }
    if (context.parentURL?.startsWith(sdkSourceRoot.href)
      && /^\.\.?\//u.test(specifier) && specifier.endsWith(".js")) {
      return { url: new URL(specifier.slice(0, -3) + ".ts", context.parentURL).href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(sdkSourceRoot.href) && url.endsWith(".ts")) {
      return {
        format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }),
      };
    }
    return nextLoad(url, context);
  },
});
const { keccak256, verifySequencerReceipt } = await import("@sidiora/layerx-sdk");
const {
  RAW_TRANSACTION_RESPONSE_BYTES,
  RAW_TRANSACTION_TIMEOUT_MS,
  verifyCalledReceipt,
} = await import("../agent-program-call/verify-receipt.mjs");
const { LayerXApplicationStateError } = await import("../support/runtime.mjs");

const vectors = JSON.parse(await readFile(new URL(
  "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
const fixture = vectors.cases.find((entry) => entry.id === "V1-layerxprograms-call");
assert.ok(fixture, "missing producer-generated program call transcript");
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const txHash = hex(keccak256(bytes(fixture.rawTransaction)));
assert.equal(txHash, fixture.receiptJson.txHash);
const key = {
  keyId: bytes(fixture.pinnedKey.keyId),
  publicKey: bytes(fixture.pinnedKey.publicKey),
  first_height: BigInt(fixture.pinnedKey.firstHeight),
  last_height: BigInt(fixture.pinnedKey.lastHeight),
};

function stateError(state, message) {
  return (error) => {
    assert.ok(error instanceof LayerXApplicationStateError);
    assert.equal(error.state, state);
    assert.equal(error.message, message);
    return true;
  };
}

async function receiptServer(t, rawReply, receipt = fixture.receiptJson) {
  const requests = [];
  const responses = [];
  const errors = [];
  const server = createServer(async (request, response) => {
    responses.push({ closed: new Promise((accept) => response.once("close", accept)) });
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      assert.equal(request.headers.accept, "application/json");
      assert.equal(request.headers["content-type"], "application/json");
      let body = "";
      for await (const chunk of request) body += chunk;
      const message = JSON.parse(body);
      assert.deepEqual(message, {
        jsonrpc: "2.0", id: 1, method: requests.length === 0
          ? "eth_getRawTransactionByHash" : "pax_sequencerReceipt", params: [txHash],
      });
      requests.push(message);
      response.setHeader("content-type", "application/json");
      if (message.method === "eth_getRawTransactionByHash") rawReply(response);
      else response.end(JSON.stringify({ jsonrpc: "2.0", id: 1, result: receipt }));
    } catch (error) {
      errors.push(error);
      response.destroy();
    }
  });
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((accept, reject) => server.close((error) => error ? reject(error) : accept()));
    assert.deepEqual(errors, []);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return { rpcUrl: `http://127.0.0.1:${server.address().port}/`, requests, responses };
}

function rawResponse(result = fixture.rawTransaction) {
  return JSON.stringify({ jsonrpc: "2.0", id: 1, result });
}

async function program_raw_transaction_fetch_verifies_bounded_producer_transcript(t) {
  const body = rawResponse();
  const service = await receiptServer(t, (response) => {
    // A response exactly at the transport cap remains valid, including its envelope.
    const encoded = body + " ".repeat(RAW_TRANSACTION_RESPONSE_BYTES - Buffer.byteLength(body));
    response.write(encoded.slice(0, 31));
    response.end(encoded.slice(31));
  });
  const proof = await verifyCalledReceipt(service.rpcUrl, txHash.toUpperCase().replace("0X", "0x"), [key]);
  assert.deepEqual(proof, {
    format: "layerx-receipt-proof-v2",
    verificationLevel: "receipted",
    rawTransaction: fixture.rawTransaction,
    signedReceipt: fixture.signedReceipt,
    receiptDigest: hex(keccak256(bytes(fixture.signedReceipt))),
    receiptKey: {
      keyId: fixture.pinnedKey.keyId, publicKey: fixture.pinnedKey.publicKey,
      firstHeight: `0x${key.first_height.toString(16)}`, lastHeight: `0x${key.last_height.toString(16)}`,
    },
  });
  const verified = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [key]);
  assert.equal(hex(verified.receiptDigest), fixture.receiptDigest);
  assert.deepEqual(service.requests.map((request) => request.method), [
    "eth_getRawTransactionByHash", "pax_sequencerReceipt",
  ]);
}

async function program_raw_transaction_fetch_caps_streamed_bytes_before_decoding(t) {
  for (const mode of ["chunked", "content-length", "utf8"]) {
    const service = await receiptServer(t, (response) => {
      let encoded;
      if (mode === "utf8") {
        encoded = JSON.stringify({ jsonrpc: "2.0", id: 1, result: fixture.rawTransaction,
          padding: "é".repeat(Math.ceil(RAW_TRANSACTION_RESPONSE_BYTES / 2)) });
        assert.ok(encoded.length < RAW_TRANSACTION_RESPONSE_BYTES);
      } else {
        const body = rawResponse();
        encoded = body + " ".repeat(RAW_TRANSACTION_RESPONSE_BYTES + 1 - Buffer.byteLength(body));
      }
      assert.ok(Buffer.byteLength(encoded) > RAW_TRANSACTION_RESPONSE_BYTES);
      if (mode === "content-length") response.setHeader("content-length", Buffer.byteLength(encoded) + 1024);
      // Leave the body open: overflow must cancel immediately, without waiting for EOF or deadline.
      for (let offset = 0; offset < encoded.length; offset += 16384) response.write(encoded.slice(offset, offset + 16384));
    });
    const started = performance.now();
    await assert.rejects(verifyCalledReceipt(service.rpcUrl, txHash, [key]),
      stateError("unknown", "transaction_unreachable"));
    await service.responses[0].closed;
    assert.ok(performance.now() - started < 3000, `${mode} overflow waited for the deadline`);
    assert.equal(service.requests.length, 1, "overflow must never query a receipt");
  }
}

async function program_raw_transaction_fetch_deadline_covers_headers_body_and_drips(t) {
  assert.equal(RAW_TRANSACTION_TIMEOUT_MS, 10000);
  await Promise.all(["headers", "body", "drip"].map(async (mode) => {
    const service = await receiptServer(t, (response) => {
      if (mode === "headers") return;
      response.flushHeaders();
      response.write('{"jsonrpc":"2.0",');
      if (mode === "drip") {
        const interval = setInterval(() => response.write(" "), 100);
        response.once("close", () => clearInterval(interval));
      }
    });
    const started = performance.now();
    await assert.rejects(verifyCalledReceipt(service.rpcUrl, txHash, [key]),
      stateError("unknown", "transaction_unreachable"));
    await service.responses[0].closed;
    const elapsed = performance.now() - started;
    assert.ok(elapsed >= 9000 && elapsed < 13000, `${mode} deadline elapsed ${elapsed}ms`);
    assert.equal(service.requests.length, 1, "timeout must never query a receipt");
  }));
}

async function program_raw_transaction_fetch_preserves_hash_and_raw_bounds(t) {
  const cases = [
    ["0x00", "transaction_hash_mismatch"],
    ["0x0", "invalid_raw_transaction"],
    ["0xgg", "invalid_raw_transaction"],
    ["0x", "invalid_raw_transaction"],
    [`0x${"00".repeat(1024 * 1024)}`, "transaction_hash_mismatch"],
    [`0x${"00".repeat(1024 * 1024 + 1)}`, "invalid_raw_transaction"],
  ];
  for (const [raw, message] of cases) {
    const service = await receiptServer(t, (response) => response.end(rawResponse(raw)));
    await assert.rejects(verifyCalledReceipt(service.rpcUrl, txHash, [key]), stateError("refused", message));
    assert.equal(service.requests.length, 1);
  }
}

async function program_raw_transaction_fetch_preserves_pinned_key_height_and_signature_checks(t) {
  const corrupted = bytes(fixture.signedReceipt);
  corrupted[corrupted.length - 1] ^= 1;
  const cases = [
    [{ ...key, publicKey: new Uint8Array(32).fill(1) }, fixture.receiptJson],
    [{ ...key, keyId: new Uint8Array(32).fill(1) }, fixture.receiptJson],
    [{ ...key, first_height: BigInt(fixture.receiptJson.height) + 1n }, fixture.receiptJson],
    [{ ...key, last_height: BigInt(fixture.receiptJson.height) - 1n }, fixture.receiptJson],
    [key, { ...fixture.receiptJson, receipt: hex(corrupted) }],
  ];
  for (const [pin, receipt] of cases) {
    const service = await receiptServer(t, (response) => response.end(rawResponse()), receipt);
    await assert.rejects(verifyCalledReceipt(service.rpcUrl, txHash, [pin]),
      stateError("refused", "sequencer_receipt_verification_failed"));
    assert.equal(service.requests.length, 2);
  }
}

async function program_raw_transaction_fetch_preserves_response_states(t) {
  const cases = [
    [rawResponse(null), 200, "pending", "transaction_unavailable"],
    ["{", 200, "unknown", "transaction_unreachable"],
    [JSON.stringify({ jsonrpc: "2.0", id: 2, result: fixture.rawTransaction }), 200, "unknown", "invalid_transaction_response"],
    [JSON.stringify({ jsonrpc: "2.0", id: 1, error: { code: -32000, message: "refused" } }), 200, "refused", "transaction_refused"],
    [rawResponse(), 503, "unknown", "transaction_http_503"],
  ];
  for (const [body, status, state, message] of cases) {
    const service = await receiptServer(t, (response) => {
      response.statusCode = status;
      response.end(body);
    });
    await assert.rejects(verifyCalledReceipt(service.rpcUrl, txHash, [key]), stateError(state, message));
    assert.equal(service.requests.length, 1);
  }
}

test(program_raw_transaction_fetch_verifies_bounded_producer_transcript, { timeout: 5000 });
test(program_raw_transaction_fetch_caps_streamed_bytes_before_decoding, { timeout: 12000 });
test(program_raw_transaction_fetch_deadline_covers_headers_body_and_drips, { timeout: 15000 });
test(program_raw_transaction_fetch_preserves_hash_and_raw_bounds, { timeout: 5000 });
test(program_raw_transaction_fetch_preserves_pinned_key_height_and_signature_checks, { timeout: 5000 });
test(program_raw_transaction_fetch_preserves_response_states, { timeout: 5000 });
