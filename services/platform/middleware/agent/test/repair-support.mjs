import "./production-sources.mjs";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { AgentEnvelopeTransport, AgentSessionCredential, LayerXKeyCredential, SecretBytes, ProductionClient,
  decodeRawEip1559, eip1559SigningHash } from "@sidiora/layerx-sdk";

const bytes = (value) => Buffer.from(value.slice(2), "hex");
class AgentBudgetLedger {
  reservations = new Map();
  transitions = [];

  constructor(available = 100n) { this.available = available; }

  async reserve(request) {
    const key = `${request.tenant}:${request.idempotencyKey}`;
    const previous = this.reservations.get(key);
    if (previous !== undefined) {
      if (previous.requestDigest !== request.requestDigest || previous.amount !== request.amount || previous.asset !== request.asset) {
        return { kind: "conflict" };
      }
      return { kind: "reserved", reservation: previous };
    }
    if (BigInt(request.amount) > this.available) return { kind: "exhausted", available: this.available.toString() };
    this.available -= BigInt(request.amount);
    const reservation = Object.freeze({ state: "reserved", reservationId: key, requestDigest: request.requestDigest,
      amount: request.amount, asset: request.asset });
    this.reservations.set(key, reservation);
    return { kind: "reserved", reservation };
  }

  transition(transition, state) {
    const previous = this.reservations.get(transition.reservationId);
    assert.ok(previous, "transition requires a reservation");
    assert.equal(previous.requestDigest, transition.requestDigest);
    assert.equal(previous.amount, transition.amount);
    assert.equal(previous.asset, transition.asset);
    assert.ok(previous.state === "reserved" || previous.state === "held");
    const reservation = Object.freeze({ ...transition, state });
    this.reservations.set(transition.reservationId, reservation);
    this.transitions.push(reservation);
    return reservation;
  }

  async hold(transition) { return this.transition(transition, "held"); }
  async commit(transition) { return this.transition(transition, "committed"); }
  async release(transition) {
    const reservation = this.transition(transition, "released");
    this.available += BigInt(reservation.amount);
    return reservation;
  }
}

class BudgetStore extends AgentBudgetLedger {
  constructor(path, available = 100000n) {
    super(available);
    this.db = new DatabaseSync(path);
    this.db.exec("CREATE TABLE IF NOT EXISTS budget (id INTEGER PRIMARY KEY, snapshot TEXT NOT NULL)");
    const row = this.db.prepare("SELECT snapshot FROM budget WHERE id = 1").get();
    if (row) {
      const saved = JSON.parse(row.snapshot);
      this.available = BigInt(saved.available);
      this.reservations = new Map(saved.reservations);
      this.transitions = saved.transitions;
    }
  }
  save() {
    this.db.prepare("INSERT INTO budget VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET snapshot = excluded.snapshot")
      .run(JSON.stringify({ available: this.available.toString(), reservations: [...this.reservations], transitions: this.transitions }));
  }
  async reserve(request) { const result = await super.reserve(request); this.save(); return result; }
  transition(request, state) { const result = super.transition(request, state); this.save(); return result; }
  async release(request) { const result = await super.release(request); this.save(); return result; }
  close() { this.save(); this.db.close(); }
}

function temporaryDirectory(t) {
  const path = mkdtempSync(join(tmpdir(), "repair-22-33-1476-"));
  t.after(() => rmSync(path, { recursive: true, force: true }));
  return path;
}

async function loopback(t, handle) {
  const errors = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      const result = await handle(body, request, response);
      if (response.destroyed) return;
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify(result));
    } catch (error) {
      errors.push(error);
      response.writeHead(500, { "content-type": "application/json" });
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
    assert.deepEqual(errors, []);
  });
  return `http://127.0.0.1:${server.address().port}`;
}

function dispatchTranscript() {
  const source = readFileSync(new URL("../../../../agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs", import.meta.url), "utf8");
  const match = source.match(/const MIDDLEWARE_SUBMISSION_TRANSCRIPT: &str = r#"([\s\S]*?)"#;/u);
  assert.ok(match, "shared dispatch producer transcript must exist");
  return JSON.parse(match[1]);
}

function daemonFixture() {
  const transcript = dispatchTranscript();
  const raw = Buffer.from(transcript.raw_transaction, "hex");
  const tx = decodeRawEip1559(raw);
  const payload = "01";
  const input = {
    tenant: "repair-agent", networkId: "125", authorityHex: "01",
    signerPublicKey: Buffer.from(secp256k1.getPublicKey(new Uint8Array(32).fill(1))).toString("hex"),
    submitIdempotencyKey: "88".repeat(32), approvalCurrentSequence: "1",
    asset: "11".repeat(32), amount: "25000", recipient: "22".repeat(32),
    commitment: { network: "eip155:125", level: "receipted" },
    preparation: { activity_type: "65537", actor: "agent:repair", authority: "repair-authority",
      account_sequence: "1", not_before: "0", not_after: "1000", fee_limit: "1",
      idempotency_key: "44".repeat(32), payload,
      payload_hash: createHash("sha256").update("LXP/v1/payload-hash\0").update(Buffer.from(payload, "hex")).digest("hex") },
  };
  const prepared = { ...input.preparation, preparation_ref: transcript.submit.preparation_ref,
    chain_id: tx.chainId.toString(), nonce: tx.nonce.toString(), max_priority_fee_per_gas: tx.maxPriorityFeePerGas.toString(),
    max_fee_per_gas: tx.maxFeePerGas.toString(), gas_limit: tx.gasLimit.toString(), to: Buffer.from(tx.to).toString("hex"),
    data: Buffer.from(tx.data).toString("hex"), signing_hash: Buffer.from(eip1559SigningHash(tx)).toString("hex") };
  const receiptKeys = [{ keyId: bytes(transcript.receipt_key.keyId), publicKey: bytes(transcript.receipt_key.publicKey),
    first_height: BigInt(transcript.receipt_key.firstHeight), last_height: BigInt(transcript.receipt_key.lastHeight) }];
  const signer = { async sign(value) {
    const recovered = secp256k1.sign(Buffer.from(value.signing_hash, "hex"), new Uint8Array(32).fill(1), { prehash: false, format: "recovered" });
    return Buffer.concat([recovered.subarray(1), recovered.subarray(0, 1)]).toString("hex");
  } };
  return { transcript, raw, input, prepared, receiptKeys, signer };
}

function envelopeClient(endpoint) {
  const session = new AgentSessionCredential("repair-agent", "11".repeat(32), new SecretBytes(new Uint8Array(32).fill(1)), "0");
  const gatewayCredential = new LayerXKeyCredential("repair-proof",
    new SecretBytes(new TextEncoder().encode(`lxp_live_${"33".repeat(32)}`)));
  const transport = new AgentEnvelopeTransport({ endpoint, session, surface: "gateway", gatewayCredential });
  // ProductionClient consumes values; AgentEnvelopeTransport authenticates and decodes the envelope.
  return new ProductionClient({ async call(request) { return (await transport.call(request)).value; } });
}

function receiptsForDaemon() {
  const values = new Map();
  return {
    values,
    async resolve(digest) { assert.ok(values.has(digest)); return structuredClone(values.get(digest)); },
    async retainVerified(record) { this.record = structuredClone(record); },
  };
}


export { AgentBudgetLedger, BudgetStore, temporaryDirectory,
  loopback, dispatchTranscript, daemonFixture, envelopeClient, receiptsForDaemon };
