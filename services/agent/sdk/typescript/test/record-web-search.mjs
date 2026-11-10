import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { registerHooks } from "node:module";

// Capture with node --experimental-transform-types test/record-web-search.mjs.
registerHooks({ resolve(specifier, context, next) {
  if (specifier.startsWith(".") && specifier.endsWith(".js")) {
    const source = new URL(specifier.slice(0, -3) + ".ts", context.parentURL);
    if (existsSync(source)) return next(source.href, context);
  }
  return next(specifier, context);
} });
const { ed25519 } = await import("@noble/curves/ed25519.js");
const { encodeSequencerReceipt } = await import("../src/sequencer-receipt-codec.ts");
const { encodeSequencerLogEffect } = await import("../src/sequencer-log.ts");
const { verifySequencerReceipt } = await import("../src/verifier.ts");
const { decodeLedgerTransaction } = await import("../src/ledger-payment.ts");
const { keccak256, decodeRawEip1559, recoverEip1559Sender } = await import("../src/evm-transaction.ts");
const { signedDraw } = await import("./record-draw.ts");
const upstream = new URL("../../../../interop/crates/x-websearch/tests/fixtures/", import.meta.url);
const output = new URL("./fixtures/repair-22-33-24/", import.meta.url);
mkdirSync(output, { recursive: true });
const read = path => JSON.parse(readFileSync(new URL(path, upstream), "utf8"));
const hex = value => Buffer.from(value).toString("hex");
const bytes = value => new Uint8Array(Buffer.from(value.replace(/^0x/u, ""), "hex"));
const digest = value => createHash("sha256").update("LXP/v1/receipt\0").update(value).digest();
const buyer = read("gateway/buyer.json");
const keys = [{ keyId: bytes(buyer.sequencerId), publicKey: bytes(buyer.sequencerPublicKey),
  first_height: BigInt(buyer.firstBatchNumber), last_height: BigInt(buyer.lastBatchNumber) }];
const seed = createHash("sha256").update("x-websearch-test/key/sequencer").digest();
assert.deepEqual(ed25519.getPublicKey(seed), keys[0].publicKey);
function issue(receipt) {
  const hash = digest(encodeSequencerReceipt(receipt, false));
  return { signed: encodeSequencerReceipt({ ...receipt, sequencerSignature: ed25519.sign(hash, seed) }), hash };
}
const metered = read("gateway/metered.json"), replacements = new Map();
let template;
for (const submission of metered.endpoints.gateway["*"].filter(rule => rule.method === "eth_sendRawTransaction")) {
  const rule = metered.endpoints.gateway["*"].find(entry => entry.method === "pax_sequencerReceipt" && entry.params[0] === submission.result);
  assert(rule);
  const result = rule.result;
  const verified = await verifySequencerReceipt(bytes(result.receipt), bytes(submission.params[0]), keys);
  template ??= verified.receipt;
  const data = decodeLedgerTransaction(bytes(submission.params[0])).data;
  const logs = result.logs.map(log => ({ ...log, data: log.data.slice(0, -64) + hex(data.subarray(196, 228)) }));
  const effects = [verified.receipt.effects[0], ...logs.map((log, i) => ({ ...verified.receipt.effects[i + 1],
    body: encodeSequencerLogEffect(bytes(log.address), log.topics.map(bytes), bytes(log.data)) }))];
  const { signed, hash } = issue({ ...verified.receipt, effects });
  replacements.set(result.receipt.slice(2), hex(signed));
  replacements.set(Buffer.from(bytes(result.receipt)).toString("base64"), Buffer.from(signed).toString("base64"));
  replacements.set(hex(verified.receiptDigest), hex(hash));
  for (let i = 0; i < logs.length; i++) replacements.set(result.logs[i].data, logs[i].data);
}
function replace(value) {
  if (typeof value === "string") {
    if (replacements.has(value)) return replacements.get(value);
    if (value.startsWith("0x") && replacements.has(value.slice(2))) return "0x" + replacements.get(value.slice(2));
    return value;
  }
  if (Array.isArray(value)) return value.map(replace);
  if (value !== null && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, replace(item)]));
  return value;
}
const write = (path, value) => writeFileSync(new URL(path, output), JSON.stringify(value, null, 2) + "\n");
write("gateway-metered.json", replace(metered));
const exchange = read("client-exchange.json");
write("client-exchange.json", { ...exchange,
  exchange: exchange.exchange.map((step, index) => index < 2 ? replace(step) : step) });
const id = n => n.toString(16).padStart(2, "0").repeat(32);
const draw = { grantId: id(1), recipient: id(3), amount: 40n, idempotencyKey: id(11), sequence: 7n,
  purposeHash: id(5), contextHash: id(12), chainId: 125n };
const raw = signedDraw(4n, draw), transaction = decodeRawEip1559(raw);
const topic = keccak256(new TextEncoder().encode("PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)"));
const log = { address: "0x000000000000000000000000000000000000101a",
  topics: ["0x" + hex(topic), "0x" + draw.grantId, "0x" + id(2), "0x" + draw.recipient],
  data: "0x" + id(4) + draw.amount.toString(16).padStart(64, "0") + draw.idempotencyKey + draw.contextHash };
const { signed } = issue({ ...template, activityId: keccak256(raw),
  from: bytes("00".repeat(12) + recoverEip1559Sender(raw).slice(2)), fromSequence: transaction.nonce,
  effects: [template.effects[0], { ...template.effects[1], body: encodeSequencerLogEffect(bytes(log.address), log.topics.map(bytes), bytes(log.data)) }] });
write("draw-recovery.json", { rawTransaction: "0x" + hex(raw), calldata: hex(transaction.data),
  txHash: "0x" + hex(keccak256(raw)), receipt: "0x" + hex(signed), logs: [log] });
const sources = {};
for (const name of ["sequencer-receipt-codec.ts", "sequencer-log.ts", "ledger-payment.ts", "evm-transaction.ts"]) {
  sources[`services/agent/sdk/typescript/src/${name}`] = createHash("sha256").update(readFileSync(new URL(`../src/${name}`, import.meta.url))).digest("hex");
}
for (const name of ["client-exchange.json", "gateway/exact.json"]) {
  sources[`services/interop/crates/x-websearch/tests/fixtures/${name}`] = createHash("sha256").update(readFileSync(new URL(name, upstream))).digest("hex");
}
write("provenance.json", { base: "8d343183c0e4380323166b7b998f97f41221d046",
  command: "node --experimental-transform-types test/record-web-search.mjs", sources,
  recording: "Canonical SDK codec regeneration authorized by attempt 5; original exact evidence and metered raw transactions are unchanged. Metered log context comes from signed calldata. Recovery uses the shared type-2 draw signer and production receipt codec under the deterministic seller fixture pin." });
