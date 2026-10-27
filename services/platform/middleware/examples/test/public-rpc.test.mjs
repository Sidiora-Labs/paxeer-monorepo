import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { encodePaymentRequiredHeader } from "../../seller/dist/index.js";

const vectors = JSON.parse(
  await readFile(
    new URL(
      "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json",
      import.meta.url,
    ),
    "utf8",
  ),
);
const payment = vectors.cases.find(
  (entry) => entry.id === "V1-layerxledger-pay-exact",
);
assert.ok(payment);
const paymentLog = payment.receiptJson.logs[0];
const payer = payment.receiptJson.from.slice(2).padStart(64, "0");
const did = `did:layerx:${payer}`;
const offer = {
  scheme: "exact",
  network: "eip155:125",
  amount: BigInt(`0x${paymentLog.data.slice(66)}`).toString(),
  asset: paymentLog.data.slice(0, 66),
  payTo: paymentLog.topics[3],
  maxTimeoutSeconds: 120,
  extra: {
    layerx: {
      commitment: "receipted",
      payer: paymentLog.topics[2].slice(2),
      purposeHash: paymentLog.topics[1].slice(2),
    },
  },
};

async function exampleInputs(context, receiptKeys) {
  const directory = await mkdtemp(join(tmpdir(), "layerx-public-rpc-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const activity = join(directory, "transaction.hex");
  const required = join(directory, "offer.header");
  const keys = join(directory, "receipt-keys.json");
  await Promise.all([
    writeFile(activity, payment.rawTransaction + "\n"),
    writeFile(
      required,
      encodePaymentRequiredHeader({
        x402Version: 2,
        resource: { url: "https://seller.example/paid" },
        accepts: [offer],
      }),
    ),
    writeFile(keys, JSON.stringify(receiptKeys)),
  ]);
  return [
    "--activity",
    activity,
    "--offer",
    required,
    "--receipt-keys",
    keys,
    "--payer",
    payer,
  ];
}

async function paymentListener(context) {
  const requests = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push({ method: request.method, url: request.url, body });
      let result;
      switch (body.method) {
        case "px_resolveAccount":
          result = {
            evm_address: payment.receiptJson.from,
            pax_address: null,
            layerx_did: did,
            layerx_account: payer,
            bound: true,
          };
          break;
        case "eth_getTransactionCount":
          result = payment.receiptJson.nonce;
          break;
        case "eth_sendRawTransaction":
          result = payment.receiptJson.txHash;
          break;
        case "pax_sequencerReceipt":
          result = payment.receiptJson;
          break;
        default:
          throw new Error(`unexpected-rpc-method: ${body.method}`);
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result }));
    } catch (error) {
      response.writeHead(500);
      response.end(String(error));
    }
  });
  context.after(async () => {
    const closed = new Promise((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
    server.closeAllConnections();
    await closed;
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return { url: `http://127.0.0.1:${server.address().port}/rpc`, requests };
}

async function runExample(context, url, args) {
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([key]) => !key.startsWith("LAYERX_")),
  );
  const child = spawn(
    process.execPath,
    [
      fileURLToPath(new URL("../public-rpc.mjs", import.meta.url)),
      "--rpc",
      url,
      "--did",
      did,
      ...args,
    ],
    { env, stdio: ["ignore", "pipe", "pipe"], timeout: 10_000 },
  );
  context.after(() => {
    if (child.exitCode === null) child.kill();
  });
  let stdout = "";
  let stderr = "";
  child.stdout.setEncoding("utf8").on("data", (chunk) => {
    stdout += chunk;
  });
  child.stderr.setEncoding("utf8").on("data", (chunk) => {
    stderr += chunk;
  });
  const [code, signal] = await once(child, "close");
  return { code, signal, stdout, stderr };
}

async function nodeExampleVerifiesReceiptedPayment(context) {
  const args = await exampleInputs(context, [payment.pinnedKey]);
  const { url, requests } = await paymentListener(context);
  const result = await runExample(context, url, args);
  assert.equal(result.signal, null, result.stderr);
  assert.equal(result.code, 0, result.stderr);
  assert.equal(result.stderr, "");
  const output = result.stdout
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  assert.deepEqual(output, [
    { sequence: payment.receiptJson.nonce },
    { transaction: payment.receiptJson.txHash, commitment: "receipted" },
  ]);
  assert.match(output[1].transaction, /^0x[0-9a-f]{64}$/u);
  for (const request of requests) {
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/rpc");
  }
  assert.deepEqual(
    requests.map(({ body }) => body),
    [
      { jsonrpc: "2.0", id: 1, method: "px_resolveAccount", params: [did] },
      {
        jsonrpc: "2.0",
        id: 2,
        method: "eth_getTransactionCount",
        params: [payment.receiptJson.from, "pending"],
      },
      {
        jsonrpc: "2.0",
        id: 3,
        method: "eth_sendRawTransaction",
        params: [payment.rawTransaction],
      },
      {
        jsonrpc: "2.0",
        id: 4,
        method: "pax_sequencerReceipt",
        params: [payment.receiptJson.txHash],
      },
    ],
  );
}

async function nodeExampleRefusesReceiptKeyWithoutRange(context) {
  const { lastHeight, ...missingRange } = payment.pinnedKey;
  assert.ok(lastHeight !== undefined);
  const args = await exampleInputs(context, [payment.pinnedKey, missingRange]);
  const { url, requests } = await paymentListener(context);
  const result = await runExample(context, url, args);
  assert.equal(result.signal, null, result.stderr);
  assert.equal(result.code, 1, result.stderr);
  assert.match(result.stderr, /receipt-key-id-range-and-public-key-required/u);
  assert.ok(!result.stdout.includes('"transaction"'));
  assert.deepEqual(
    requests.map(({ body }) => body.method),
    ["px_resolveAccount", "eth_getTransactionCount"],
  );
}

test(nodeExampleVerifiesReceiptedPayment);
test(nodeExampleRefusesReceiptKeyWithoutRange);
