import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import test from "node:test";

const sdk = new URL("../../../agent/sdk/typescript/src/", import.meta.url);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@sidiora/layerx-sdk") {
      return { url: new URL("index.ts", sdk).href, shortCircuit: true };
    }
    if (context.parentURL?.startsWith(sdk.href) && specifier.startsWith(".") && specifier.endsWith(".js")) {
      return { url: new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(sdk.href) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});

const { LayerXApplicationStateError, SequencerReceiptClient } = await import("../support/runtime.mjs");
const { verifySequencerReceipt } = await import("@sidiora/layerx-sdk");
const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");
const bytes = (hex) => Uint8Array.from(Buffer.from(hex.slice(2), "hex"));

function examplesAuthorityDocMatchesApplicationProfiles() {
  const guide = read("../README.md");
  const applications = [
    ["buyer-agent", "Buyer agent", "LAYERX_TESTNET_RECEIPT_KEYS_JSON"],
    ["paid-api", "Paid API", "LAYERX_RECEIPT_KEYS_JSON"],
    ["merchant-shop", "Merchant shop", "LAYERX_TESTNET_RECEIPT_KEYS_JSON"],
    ["marketplace", "Programs marketplace", "LAYERX_TESTNET_RECEIPT_KEYS_JSON"],
  ];
  for (const [directory, label, keyVariable] of applications) {
    const { beta } = JSON.parse(read(`../${directory}/layerx.example.json`)).environments;
    assert.equal(new URL(beta.rpcUrl).pathname, "/");
    assert.equal(beta.receiptKeysEnvironment, keyVariable);
    assert.equal(beta.network, "eip155:125");
    assert.ok(guide.includes(`| ${label} | \`${beta.rpcUrl}\` | \`${keyVariable}\` |`));
  }
  assert.ok(guide.includes("## Historical emulator workflow"));
  assert.ok(guide.includes("export PAXEER_X_REFERENCE_APPS_ENVIRONMENT=beta"));
  for (const state of ["pending", "unknown", "refused"]) assert.ok(guide.includes(`| \`${state}\` |`));
}

async function examplesAuthorityDocProvesRootRpcAndReceiptStates(t) {
  const vectors = JSON.parse(read("../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"));
  assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
  const vector = vectors.cases.find(({ id }) => id === "V1-layerxledger-pay-exact");
  assert.ok(vector, "real node-produced payment receipt fixture required");
  const pin = {
    keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
    first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
  };
  const observed = [];
  let result = vector.receiptJson;
  let rpcError;
  let status = 200;
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    observed.push({ path: request.url, method: request.method, message });
    response.writeHead(status, { "content-type": "application/json" });
    response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id,
      ...(rpcError === undefined ? { result } : { error: rpcError }) }));
  });
  const close = async () => {
    if (!server.listening) return;
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  };
  t.after(close);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const client = new SequencerReceiptClient(`http://127.0.0.1:${server.address().port}/`, [pin]);
  const proof = await client.resolve(vector.rawTransaction);
  assert.equal(proof.format, "layerx-receipt-proof-v2");
  assert.equal(proof.verificationLevel, "receipted");
  assert.equal(proof.signedReceipt, vector.signedReceipt);
  assert.equal(proof.receiptDigest, vector.receiptDigest);
  const verified = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [pin]);
  assert.equal(verified.height, BigInt(vector.receiptJson.height));
  assert.deepEqual(observed, [{ path: "/", method: "POST", message: {
    jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [vector.receiptJson.txHash],
  } }]);
  result = { ...vector.receiptJson, rung: "committed" };
  assert.equal((await client.resolve(vector.rawTransaction)).verificationLevel, "receipted");
  const stateIs = (state) => (error) => {
    assert.ok(error instanceof LayerXApplicationStateError);
    assert.equal(error.state, state);
    return true;
  };
  result = { ...vector.receiptJson, rung: "submitted" };
  await assert.rejects(client.resolve(vector.rawTransaction), stateIs("pending"));
  result = { ...vector.receiptJson, rung: "unrecognized" };
  await assert.rejects(client.resolve(vector.rawTransaction), stateIs("unknown"));
  result = { ...vector.receiptJson, txHash: `0x${"00".repeat(32)}` };
  await assert.rejects(client.resolve(vector.rawTransaction), stateIs("unknown"));
  result = vector.receiptJson;
  status = 503;
  await assert.rejects(client.resolve(vector.rawTransaction), stateIs("unknown"));
  status = 200;
  rpcError = { code: -32000, message: "receipt refused" };
  await assert.rejects(client.resolve(vector.rawTransaction), stateIs("refused"));
  rpcError = undefined;
  const wrongPin = { ...pin, publicKey: new Uint8Array(32) };
  await assert.rejects(new SequencerReceiptClient(client.rpcUrl, [wrongPin]).resolve(vector.rawTransaction), stateIs("refused"));
  assert.ok(observed.every(({ path, method, message }) => path === "/" && method === "POST"
    && message.method === "pax_sequencerReceipt" && message.params[0] === vector.receiptJson.txHash));
  await close();
  await assert.rejects(client.resolve(vector.rawTransaction), stateIs("unknown"));
}

test(examplesAuthorityDocMatchesApplicationProfiles);
test(examplesAuthorityDocProvesRootRpcAndReceiptStates);
