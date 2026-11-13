import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { readFileSync } from "node:fs";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import {
  AgentHttpTransport, ProductionClient, decodeRawEip1559, encodeUnsignedEip1559,
  keccak256, recoverEip1559Sender, verifySequencerReceipt, verifyCommittedReceipt,
  cometHeaderHash, cometValidatorSetHash, cometCommitHash,
} from "@sidiora/layerx-sdk";
import {
  DurableFulfillmentRepository, ReceiptPayloadAuthority, SellerMiddleware,
  decodePaymentExactLog, encodePaymentPayloadHeader, encodeSettlementHeader,
  encodePaymentRequiredHeader, verifyPaymentReceipt, encodeLedgerDraw,
  decodePaymentDrawnLog, LAYERX_LEDGER_ADDRESS, PAYMENT_RESPONSE_HEADER,
  encodePaymentCommitmentEvidence, decodePaymentCommitmentEvidence,
  verifyPaymentCommitment, verifyPaymentCommitmentEvidence,
} from "../dist/index.js";
import { recordedCommittedReceipt } from "./committed-fixture.mjs";
import { GrantPaymentAuthority } from "../dist/grant.js";
import { PaymentRpc, HostedGatewayRpc, verifyRpcPayment } from "../dist/rpc.js";

// Load the actual neighboring consumers with Node's built-in TypeScript transform.
// The declared gate needs no build of a neighboring package or subprocess toolchain.
const consumers = [new URL("../../buyer/src/index.ts", import.meta.url).href,
  new URL("../../merchant/src/index.ts", import.meta.url).href];
const hooks = registerHooks({ load(url, context, next) {
  if (!consumers.includes(url)) return next(url, context);
  return { format: "module", shortCircuit: true,
    source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
} });
const { BuyerMiddleware } = await import(consumers[0]);
const { MerchantMiddleware } = await import(consumers[1]);
hooks.deregister();

const bytes = (value) => Buffer.from(value.replace(/^0x/u, ""), "hex");
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const sha = (...values) => createHash("sha256").update(Buffer.concat(values)).digest();
const word = (value) => Buffer.alloc(32, value);
const fixtures = new URL("../../../../../sdk/platform/conformance/fixtures/", import.meta.url);
const nodeVectors = JSON.parse(readFileSync(new URL("sequencer-receipt-v1.json", fixtures), "utf8"));
const exact = nodeVectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
const committed = JSON.parse(readFileSync(new URL("vector-v4-committed.json", fixtures), "utf8")).positive;
const pinFor = (vector) => ({ keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight) });
const proofFor = async (vector) => {
  const verified = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [pinFor(vector)]);
  return { format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
    rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt,
    receiptDigest: hex(verified.receiptDigest),
    receiptKey: { keyId: vector.pinnedKey.keyId, publicKey: vector.pinnedKey.publicKey,
      firstHeight: `0x${BigInt(vector.pinnedKey.firstHeight).toString(16)}`,
      lastHeight: `0x${BigInt(vector.pinnedKey.lastHeight).toString(16)}` } };
};
const exactLog = exact.receiptJson.logs.find((log) => log.topics[0] ===
  hex(keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"))));
const payment = decodePaymentExactLog({ address: exactLog.address, data: bytes(exactLog.data), topics: exactLog.topics.map(bytes) });
const requirements = { scheme: "exact", network: "eip155:125", amount: payment.amount.toString(),
  asset: hex(payment.assetId), payTo: hex(payment.to), maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", payer: hex(payment.from).slice(2), purposeHash: hex(payment.paymentReference).slice(2) } } };
const proof = await proofFor(exact);
const pin = pinFor(exact);
const resource = { url: "https://merchant.example/paid" };
const header = (value = proof, accepted = requirements, description = "first") => encodePaymentPayloadHeader({
  x402Version: 2, accepted, payload: value, resource: { ...resource, description },
});
const failure = (error) => error.code === "verification-failure";

async function temporary(t) {
  const directory = await mkdtemp(join(tmpdir(), "seller-22-31-434-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  return directory;
}
function seller(directory, keys = [pin], accepted = requirements, authority = new ReceiptPayloadAuthority(keys)) {
  return new SellerMiddleware({ paymentRequired: { x402Version: 2, resource, accepts: [accepted] },
    receiptKeys: keys, authority, fulfillments: new DurableFulfillmentRepository(directory) });
}
async function loopback(t, handler) {
  const requests = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = []; for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push({ url: request.url, headers: request.headers, body });
      const answer = await handler(body, request);
      response.writeHead(answer.status ?? 200, answer.headers ?? { "content-type": "application/json" });
      response.end(answer.text ?? JSON.stringify({ jsonrpc: "2.0", id: body.id, ...answer }));
    } catch (error) { response.writeHead(500).end(String(error)); }
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); }));
  return { endpoint: `http://127.0.0.1:${server.address().port}/`, requests };
}

async function nodeRootRejected(t) {
  const node = await loopback(t, (body) => ({ result: body.method === "eth_sendRawTransaction"
    ? exact.receiptJson.txHash : exact.receiptJson }));
  const gateway = await loopback(t, () => ({ result: { account: "agent:merchant:main" } }));
  const rpc = new PaymentRpc(node.endpoint, { authorization: "Bearer local-transcript" });
  assert.equal(await rpc.send(exact.rawTransaction), exact.receiptJson.txHash);
  const receipt = await rpc.receipt(exact.receiptJson.txHash);
  assert.equal((await verifyRpcPayment(receipt, exact.rawTransaction,
    exact.receiptJson.from.slice(2).padStart(64, "0"), requirements, [pin])).kind, "verified");
  const hosted = new HostedGatewayRpc(gateway.endpoint, { authorization: "Bearer hosted-transcript" });
  assert.deepEqual(await hosted.call("px_resolveAccount", ["merchant"]), { account: "agent:merchant:main" });
  assert.deepEqual(node.requests.map((request) => request.url), ["/", "/"]);
  assert.deepEqual(gateway.requests.map((request) => request.url), ["/"]);
  assert.equal(node.requests[0].headers.authorization, "Bearer local-transcript");
  assert.equal(gateway.requests[0].headers.authorization, "Bearer hosted-transcript");
  assert.throws(() => rpc.call("px_resolveAccount", []), /invalid-node-method/u);
  assert.throws(() => hosted.call("eth_sendRawTransaction", []), /invalid-gateway-method/u);
  for (const endpoint of ["https://user:secret@example.com/", "https://example.com/rpc", "https://example.com/?q=1",
    "http://example.com/", "https://example.com/#fragment"]) assert.throws(() => new PaymentRpc(endpoint), /invalid-rpc-endpoint/u);
  const redirect = await loopback(t, () => ({ status: 307, headers: { location: gateway.endpoint } }));
  await assert.rejects(new PaymentRpc(redirect.endpoint).call("eth_chainId", []));
  const oversized = await loopback(t, () => ({ text: "x".repeat(8_388_609) }));
  await assert.rejects(new PaymentRpc(oversized.endpoint).call("eth_chainId", []), /rpc-body-too-large/u);
  const malformed = await loopback(t, () => ({ text: '{"jsonrpc":"2.0","id":999,"result":1}' }));
  await assert.rejects(new PaymentRpc(malformed.endpoint).call("eth_chainId", []), /invalid-rpc-envelope/u);
}

function uint(value, width) { return Buffer.from(value.toString(16).padStart(width * 2, "0"), "hex"); }
function integer(value) { const text = value.toString(16); return value === 0n ? Buffer.alloc(0) : Buffer.from(text.padStart(text.length + text.length % 2, "0"), "hex"); }
function rlp(value) {
  const list = Array.isArray(value); const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0] < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length)); return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}
function signTransaction(transaction) {
  const unsigned = encodeUnsignedEip1559(transaction);
  const signature = secp256k1.sign(keccak256(unsigned), word(1), { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  return Buffer.concat([Buffer.of(2), rlp([transaction.chainId, transaction.nonce, transaction.maxPriorityFeePerGas,
    transaction.maxFeePerGas, transaction.gasLimit].map(integer).concat([transaction.to, integer(transaction.value), transaction.data,
      [], integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s)]))]);
}
const grantDraw = { recipient: payment.to, purposeHash: payment.paymentReference, amount: payment.amount,
  grantId: word(0x31), sequence: 1n, idempotencyKey: word(0x32), contextHash: word(0x33) };
function grantTransaction(recipient = payment.to) {
  return hex(signTransaction({ ...decodeRawEip1559(bytes(exact.rawTransaction)), accessList: [],
    data: encodeLedgerDraw({ ...grantDraw, recipient }) }));
}
function grantRequest(rawTransaction, scheme = "metered") {
  const accepted = { ...requirements, scheme, extra: { layerx: { ...requirements.extra.layerx,
    ...(scheme === "subscription" ? { windowSeconds: "60" } : {}) } } };
  return { principal: "owner", requestDigest: "aa".repeat(32), idempotencyKey: "bb".repeat(32), requirements: accepted,
    payload: { x402Version: 2, accepted, payload: { rawTransaction, idempotencyKey: hex(grantDraw.idempotencyKey).slice(2) } } };
}

async function sellerDrawRecipient() {
  assert.notDeepEqual(payment.from, payment.to);
  const grants = new Map();
  let resolutions = 0; let executions = 0;
  const authority = new GrantPaymentAuthority({
    async resolveGrant(id) { resolutions++; return grants.get(id); },
    async execute() { executions++; throw new Error("no authorized grant exists"); },
  }, new ReceiptPayloadAuthority([pin]), [pin]);
  for (const scheme of ["metered", "subscription"]) {
    await assert.rejects(authority.settle(grantRequest(grantTransaction(payment.from), scheme)),
      (error) => error.code === "invalid-payment-payload");
  }
  assert.equal(resolutions, 0); assert.equal(executions, 0);
  const calldata = decodeRawEip1559(bytes(grantTransaction())).data;
  assert.deepEqual(Buffer.from(calldata.subarray(36, 68)), Buffer.from(payment.to));
  const drawn = decodePaymentDrawnLog({ address: LAYERX_LEDGER_ADDRESS,
    topics: [keccak256(new TextEncoder().encode("PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)")),
      grantDraw.grantId, payment.from, payment.to],
    data: Buffer.concat([Buffer.from(payment.assetId), uint(payment.amount, 32), grantDraw.idempotencyKey, grantDraw.contextHash]) });
  assert.deepEqual(Buffer.from(drawn.to), Buffer.from(calldata.subarray(36, 68)));
  assert.notDeepEqual(drawn.from, drawn.to);
}

async function drawProducerUnavailable() {
  const grants = new Map();
  let executions = 0;
  const authority = new GrantPaymentAuthority({
    async resolveGrant(id) { return grants.get(id); },
    async execute() { executions++; throw new Error("grant lookup must precede execution"); },
  }, new ReceiptPayloadAuthority([pin]), [pin]);
  for (const scheme of ["metered", "subscription"]) {
    const input = grantRequest(grantTransaction(), scheme);
    assert.deepEqual(await authority.settle(input), { kind: "refused", reason: "grant_unavailable" });
    grants.set(hex(grantDraw.grantId), { source: hex(payment.from), asset: hex(payment.assetId), recipient: hex(payment.to),
      executor: recoverEip1559Sender(bytes(input.payload.payload.rawTransaction)), active: false });
    assert.deepEqual(await authority.settle(input), { kind: "refused", reason: "grant_unavailable" });
    grants.set(hex(grantDraw.grantId), { source: hex(payment.to), asset: hex(payment.assetId), recipient: hex(payment.to),
      executor: recoverEip1559Sender(bytes(input.payload.payload.rawTransaction)), active: true });
    await assert.rejects(authority.settle(input), failure);
    grants.clear();
  }
  assert.equal(executions, 0);
}

async function sellerGrantFinalVerifier(t) {
  const directory = await temporary(t);
  assert.equal((await verifyPaymentReceipt(proof, requirements, [pin])).payment.amount, payment.amount);
  for (const scheme of ["metered", "subscription"]) {
    const accepted = grantRequest(grantTransaction(), scheme).requirements;
    // An exact node event must never satisfy an offer that requires PaymentDrawn.
    await assert.rejects(verifyPaymentReceipt(proof, accepted, [pin]), failure);
    let releases = 0;
    await assert.rejects(seller(directory, [pin], accepted).handle("owner", header(proof, accepted), async () => { releases++; return "product"; }), failure);
    assert.equal(releases, 0);
  }
}

// An adversarial, cryptographically authentic foreign-chain witness is derived
// from the recorded node envelope. It is not a claim of a live ledger debit.
async function foreignChainWitness() {
  const pair = generateKeyPairSync("ed25519");
  const publicKey = pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32);
  const rawTransaction = signTransaction({ ...decodeRawEip1559(bytes(proof.rawTransaction)), chainId: 1n, accessList: [] });
  const original = bytes(proof.signedReceipt);
  const prefix = Buffer.from(original.subarray(0, -69));
  prefix.set(keccak256(rawTransaction), 10);
  // Locate the fixed receipt tail using the canonical layout, after the effects.
  let offset = 6;
  const bounded = () => { const size = prefix.readUInt32BE(offset); const start = offset + 4; offset = start + size; return start; };
  bounded(); offset += 8; const previous = bounded(); const resulting = bounded(); bounded(); offset += 4;
  const effects = prefix.readUInt32BE(offset); offset += 4;
  for (let i = 0; i < effects; i++) { offset += 8; bounded(); bounded(); }
  offset += 16; bounded(); offset += 11; bounded(); offset += 16;
  const sender = bounded(); prefix.set(Buffer.concat([Buffer.alloc(12), bytes(recoverEip1559Sender(rawTransaction))]), sender);
  offset += 16 + 16 + 8; bounded(); offset += 16 + 16; bounded(); bounded(); const context = bounded();
  const keyId = word(0x7a); prefix.set(keyId, context);
  const sequence = prefix.readBigUInt64BE(42);
  prefix.set(sha(Buffer.from("PAXEER/v1/sequence\0"), prefix.subarray(previous, previous + 32), uint(sequence, 8), keccak256(rawTransaction)), resulting);
  const signature = sign(null, sha(Buffer.from("LXP/v1/receipt\0"), prefix, Buffer.of(0)), pair.privateKey);
  const signedReceipt = Buffer.concat([prefix, Buffer.of(1), uint(64n, 4), signature]);
  const receiptKey = { ...proof.receiptKey, keyId: hex(keyId), publicKey: hex(publicKey) };
  const foreignPin = { ...pin, keyId, publicKey };
  const verified = await verifySequencerReceipt(signedReceipt, rawTransaction, [foreignPin]);
  return { proof: { ...proof, rawTransaction: hex(rawTransaction), signedReceipt: hex(signedReceipt),
    receiptDigest: hex(verified.receiptDigest), receiptKey },
    pin: foreignPin };
}

class OrderStore {
  records = new Map(); releases = 0;
  async open(request) {
    if (!this.records.has(request.checkoutKey)) this.records.set(request.checkoutKey,
      { orderId: request.checkoutKey, ...request, state: "awaiting-payment" });
    return structuredClone(this.records.get(request.checkoutKey));
  }
  async get(id) { return structuredClone(this.records.get(id)); }
  async releaseResource(id) { this.releases++; return this.get(id); }
  async markPaid(id, requestDigest, receiptDigest, transaction) {
    const order = await this.get(id); assert.equal(order.requestDigest, requestDigest);
    const paid = { ...order, state: "paid-verified", receiptDigest, transaction }; this.records.set(id, paid); return structuredClone(paid);
  }
  async markRefused(id, requestDigest) {
    const order = await this.get(id); assert.equal(order.requestDigest, requestDigest);
    const refused = { ...order, state: "refused" }; this.records.set(id, refused); return structuredClone(refused);
  }
}

async function sellerNetworkUnbound(t) {
  const foreign = await foreignChainWitness();
  const directory = await temporary(t);
  const decoded = await verifySequencerReceipt(bytes(foreign.proof.signedReceipt), bytes(foreign.proof.rawTransaction), [foreign.pin]);
  assert.equal(decoded.receipt.resultCode, 0);
  assert.equal((await verifyPaymentReceipt(foreign.proof, { ...requirements, network: "eip155:1" }, [foreign.pin])).payment.amount, payment.amount);
  await assert.rejects(verifyPaymentReceipt(foreign.proof, requirements, [foreign.pin]), failure);
  let releases = 0;
  await assert.rejects(seller(directory, [foreign.pin]).handle("owner", header(foreign.proof), async () => { releases++; return "product"; }), failure);
  assert.equal(releases, 0);
  const buyer = new BuyerMiddleware({ client: new ProductionClient(new AgentHttpTransport({ endpoint: "http://127.0.0.1/" })),
    source: "acct:buyer", supported: [{ scheme: "exact", network: requirements.network }], receiptKeys: [foreign.pin] });
  const transaction = hex(decoded.receipt.activityId);
  const response = encodeSettlementHeader({ success: true, payer: hex(decoded.receipt.from).slice(2), transaction,
    network: requirements.network, amount: requirements.amount, extensions: { layerx: foreign.proof } });
  await assert.rejects(buyer.captureSettlement(response, { ...foreign.proof, txHash: transaction,
    offer: { accepted: requirements }, receiptDigest: foreign.proof.receiptDigest }), failure);
  const orders = new OrderStore();
  const catalog = new Map([["product", { ...requirements, sku: "product", title: "Product", unitAmount: requirements.amount }]]);
  const merchant = new MerchantMiddleware({ catalog: { get: async (sku) => catalog.get(sku) }, orders,
    resourceUrl: () => resource.url,
    sellers: { create: (paymentRequired) => new SellerMiddleware({ paymentRequired, receiptKeys: [foreign.pin],
      authority: new ReceiptPayloadAuthority([foreign.pin]), fulfillments: new DurableFulfillmentRepository(join(directory, "merchant")) }) } });
  await assert.rejects(merchant.checkout("owner", "order-one", [{ sku: "product", quantity: 1 }], header(foreign.proof)), failure);
  assert.equal(orders.releases, 0);
  const rpcResult = { ...exact.receiptJson, txHash: transaction, receipt: foreign.proof.signedReceipt,
    keyId: foreign.proof.receiptKey.keyId };
  await assert.rejects(verifyRpcPayment(rpcResult, foreign.proof.rawTransaction, hex(decoded.receipt.from), requirements, [foreign.pin]), failure);
}

async function receiptReusePrincipal(t) {
  const directory = await temporary(t);
  const first = seller(directory); const second = seller(directory);
  let releases = 0;
  const release = async (key) => { releases++; await new Promise((resolve) => setImmediate(resolve)); return { key, product: "one" }; };
  const results = await Promise.allSettled([
    first.handle("principal-a", header(proof, requirements, "one"), release),
    second.handle("principal-b", header(proof, requirements, "two"), release),
    second.handle("principal-a", header(proof, requirements, "one"), release),
  ]);
  assert.equal(releases, 1);
  const released = results.filter((result) => result.status === "fulfilled" && result.value.kind === "released");
  assert.ok(released.length >= 1);
  const winningIndex = results.findIndex((result) => result.status === "fulfilled" && result.value.kind === "released");
  const owner = winningIndex === 1 ? "principal-b" : "principal-a";
  const description = winningIndex === 1 ? "two" : "one";
  const original = header(proof, requirements, description);
  const restarted = seller(directory);
  const recovered = await restarted.handle(owner, original, release);
  assert.equal(recovered.kind, "released");
  const conflict = await restarted.handle(owner === "principal-a" ? "principal-b" : "principal-a", original, release);
  assert.equal(conflict.kind, "refused"); assert.equal(conflict.status, 409); assert.equal(conflict.settlement.errorReason, "receipt_reuse");
  const changed = await restarted.handle(owner, header(proof, requirements, "another-payload"), release);
  assert.equal(changed.status, 409);
  assert.equal(releases, 1);
  assert.equal((await readdir(directory)).length, 1);
  const redeliveries = await Promise.all(Array.from({ length: 8 }, () => restarted.handle(owner, original, release)));
  assert.ok(redeliveries.every((result) => result.kind === "released")); assert.equal(releases, 1);
}

async function interruptedReleaseNeverDebitsAgain(t) {
  const directory = await temporary(t);
  let releases = 0;
  await assert.rejects(seller(directory).handle("principal", header(), async () => { releases++; throw new Error("interrupted-after-side-effect"); }), /interrupted/u);
  await assert.rejects(seller(directory).handle("principal", header(), async () => { releases++; return "second"; }),
    (error) => error.code === "fulfillment-outcome-unknown");
  assert.equal(releases, 1);
}

function timestamp(value) {
  const match = /^(.*?)(?:\.([0-9]{1,9}))?Z$/u.exec(value);
  return { seconds: BigInt(Date.parse(`${match[1]}Z`) / 1000), nanos: Number((match[2] ?? "").padEnd(9, "0")) };
}
const base64 = (value) => new Uint8Array(Buffer.from(value, "base64"));
function blockId(value) { return { hash: base64(value.Hash), partSetHeader: { total: value.PartSetTotal, hash: base64(value.PartSetHash) } }; }
function nodeHeader(value) {
  return { versionBlock: BigInt(value.VersionBlock), versionApp: BigInt(value.VersionApp), chainId: value.ChainID,
    height: BigInt(value.Height), time: timestamp(value.Time), lastBlockId: blockId(value.LastBlockID),
    ...Object.fromEntries(["LastCommitHash", "DataHash", "ValidatorsHash", "NextValidatorsHash", "ConsensusHash", "AppHash", "LastResultsHash", "EvidenceHash", "ProposerAddress"]
      .map((field) => [field[0].toLowerCase() + field.slice(1), base64(value[field])])) };
}
function nodeCommit(value) {
  return { height: BigInt(value.Height), round: value.Round, blockId: blockId(value.BlockID),
    signatures: value.Signatures.map((signature) => ({ blockIdFlag: signature.BlockIDFlag,
      validatorAddress: base64(signature.ValidatorAddress), timestamp: timestamp(signature.Timestamp), signature: base64(signature.Signature) })) };
}
function validatorSet(value) { return value.map((validator) => { const publicKey = new Uint8Array(validator.PublicKey);
  return { publicKey, address: new Uint8Array(sha(publicKey).subarray(0, 20)), votingPower: BigInt(validator.VotingPower) }; }); }
async function commitmentFixture() {
  return recordedCommittedReceipt();
}

async function commitmentInclusionUnverified() {
  const { verified, evidence, resolver } = await commitmentFixture();
  assert.equal(hex(await cometHeaderHash(evidence.blockHeader)), hex(evidence.commit.blockId.hash));
  assert.equal(hex(await cometValidatorSetHash(evidence.validatorSet)), hex(evidence.blockHeader.validatorsHash));
  await verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", resolver);
  await verifyPaymentCommitment(verified, "eip155:125", "committed", resolver);
  const portable = JSON.parse(JSON.stringify(encodePaymentCommitmentEvidence(evidence)));
  const recovered = decodePaymentCommitmentEvidence(portable);
  assert.deepEqual(recovered, evidence);
  await verifyPaymentCommitmentEvidence(verified, "committed", recovered, "eip155:125", resolver);
  for (const mutate of [
    (proof) => { proof.receiptInclusion.batchProof.aunts[0][0] ^= 1; },
    (proof) => { proof.receiptInclusion.batchProof.index = 1n; },
    (proof) => { proof.receiptInclusion.activityIds[0] = word(0x99); },
    (proof) => { proof.receiptInclusion.batchTransaction[40] ^= 1; },
    (proof) => { proof.blockHeader.dataHash[0] ^= 1; },
    (proof) => { proof.receiptInclusion.receiptDigest[0] ^= 1; },
  ]) {
    const invalid = structuredClone(evidence); mutate(invalid);
    await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", invalid, "eip155:125", resolver), failure);
  }
  // The execution commit has an authentic quorum but cannot authenticate the successor results.
  const unrelated = structuredClone(evidence);
  unrelated.blockHeader = { ...nodeHeader(committed.blockHeader), blockId: blockId(committed.commit.BlockID) };
  unrelated.commit = nodeCommit(committed.commit);
  unrelated.validatorSet = validatorSet(committed.validatorSet);
  unrelated.landingBlockHeight = unrelated.blockHeader.height;
  const nextResolver = { ...resolver, async resolveValidators(network, height) {
    assert.equal(network, "eip155:125"); assert.equal(height, unrelated.landingBlockHeight);
    return { chainId: unrelated.blockHeader.chainId, validatorsHash: unrelated.blockHeader.validatorsHash };
  } };
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", unrelated, "eip155:125", nextResolver), failure);
  // Metadata-only evidence, previously accepted, can no longer upgrade a receipt.
  await assert.rejects(verifyCommittedReceipt(verified, { chainId: evidence.blockHeader.chainId,
    landingHeight: evidence.landingBlockHeight, blockId: evidence.commit.blockId, commit: evidence.commit, validators: evidence.validatorSet }), failure);
}

function varint(value) {
  let remaining = BigInt.asUintN(64, value); const result = [];
  while (remaining >= 128n) { result.push(Number(remaining & 127n) | 128); remaining >>= 7n; }
  return Buffer.from([...result, Number(remaining)]);
}
function protoBytes(field, value) { return Buffer.concat([Buffer.of(field * 8 + 2), varint(BigInt(value.length)), value]); }
function fixed64(field, value) {
  if (value === 0n) return Buffer.alloc(0);
  const result = Buffer.alloc(9); result[0] = field * 8 + 1; result.writeBigInt64LE(value, 1); return result;
}
function commitVote(evidence, time) {
  const { blockId, height, round } = evidence.commit;
  const parts = Buffer.concat([Buffer.of(8), varint(BigInt(blockId.partSetHeader.total)), protoBytes(2, blockId.partSetHeader.hash)]);
  const block = Buffer.concat([protoBytes(1, blockId.hash), protoBytes(2, parts)]);
  const timestamp = Buffer.concat([Buffer.of(8), varint(time.seconds), Buffer.of(16), varint(BigInt(time.nanos))]);
  const vote = Buffer.concat([Buffer.of(8, 2), fixed64(2, height), fixed64(3, BigInt(round)), protoBytes(4, block),
    protoBytes(5, timestamp), protoBytes(6, Buffer.from(evidence.blockHeader.chainId))]);
  return Buffer.concat([varint(BigInt(vote.length)), vote]);
}

async function commitmentValidatorTrust() {
  const { verified, evidence, resolver } = await commitmentFixture();
  await verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", resolver);
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:1", resolver), failure);
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", undefined), failure);
  const foreign = { async resolveValidators() { return { chainId: "foreign-chain", validatorsHash: evidence.blockHeader.validatorsHash }; } };
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", foreign), failure);
  const substituted = structuredClone(evidence);
  const unsignedPair = generateKeyPairSync("ed25519");
  const unsignedPublicKey = new Uint8Array(unsignedPair.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
  substituted.validatorSet.push({ publicKey: unsignedPublicKey, address: new Uint8Array(sha(unsignedPublicKey).subarray(0, 20)), votingPower: 1n });
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", substituted, "eip155:125", resolver), failure);
  const renamed = structuredClone(evidence); renamed.blockHeader.chainId = "foreign-chain";
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", renamed, "eip155:125", resolver), failure);
  const selfSigned = structuredClone(evidence);
  const pair = generateKeyPairSync("ed25519");
  const publicKey = new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
  const address = new Uint8Array(sha(publicKey).subarray(0, 20));
  selfSigned.validatorSet = [{ publicKey, address, votingPower: 100n }];
  const inclusion = selfSigned.receiptInclusion;
  inclusion.executionValidators = structuredClone(selfSigned.validatorSet);
  inclusion.executionHeader.validatorsHash = await cometValidatorSetHash(selfSigned.validatorSet);
  inclusion.executionHeader.nextValidatorsHash = inclusion.executionHeader.validatorsHash.slice();
  inclusion.executionCommit.blockId.hash = await cometHeaderHash(inclusion.executionHeader);
  const executionTime = { seconds: 1n, nanos: 1 };
  inclusion.executionCommit.signatures = [{ blockIdFlag: 2, validatorAddress: address, timestamp: executionTime,
    signature: new Uint8Array(sign(null, commitVote({ commit: inclusion.executionCommit, blockHeader: inclusion.executionHeader }, executionTime), pair.privateKey)) }];
  selfSigned.blockHeader.lastBlockId = structuredClone(inclusion.executionCommit.blockId);
  selfSigned.blockHeader.lastCommitHash = cometCommitHash(inclusion.executionCommit);
  selfSigned.blockHeader.validatorsHash = await cometValidatorSetHash(selfSigned.validatorSet);
  selfSigned.commit.blockId.hash = await cometHeaderHash(selfSigned.blockHeader);
  selfSigned.blockHeader.blockId = structuredClone(selfSigned.commit.blockId);
  inclusion.successorHeader = structuredClone(selfSigned.blockHeader);
  const time = { seconds: 1n, nanos: 1 };
  selfSigned.commit.signatures = [{ blockIdFlag: 2, validatorAddress: address, timestamp: time,
    signature: new Uint8Array(sign(null, commitVote(selfSigned, time), pair.privateKey)) }];
  // The low-level verifier explicitly trusts its caller's validator argument.
  // A seller must independently refuse this otherwise valid, self-signed set.
  const lowLevel = await verifyCommittedReceipt(verified, { chainId: selfSigned.blockHeader.chainId,
    landingHeight: selfSigned.landingBlockHeight, blockId: selfSigned.commit.blockId, commit: selfSigned.commit,
    validators: selfSigned.validatorSet, header: selfSigned.blockHeader, inclusion: selfSigned.receiptInclusion });
  assert.equal(lowLevel.signedPower, 100n);
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", selfSigned, "eip155:125", resolver), failure);
  const changedPower = structuredClone(evidence); changedPower.validatorSet[0].votingPower += 1n;
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", changedPower, "eip155:125", resolver), failure);
}

test(nodeRootRejected);
test(sellerDrawRecipient);
test(drawProducerUnavailable);
test(sellerGrantFinalVerifier);
test(sellerNetworkUnbound);
test(receiptReusePrincipal);
test(interruptedReleaseNeverDebitsAgain);
test(commitmentInclusionUnverified);
test(commitmentValidatorTrust);
