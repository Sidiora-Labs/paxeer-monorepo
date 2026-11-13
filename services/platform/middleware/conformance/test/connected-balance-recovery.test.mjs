import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { AgentEnvelopeTransport, AgentSessionCredential, LayerXKeyCredential, PlatformSdkError, Rung, SecretBytes } from "@sidiora/layerx-sdk";
import { readConnectedBalance, verifyConnectedRecovery } from "../connected-balance.mjs";

// The peer fixture follows read.balance's BalanceValue and VerificationStatus
// contracts. Only the peer is fixture data; the envelope codec, HTTP transport
// and connected recovery consumer are production modules.
const coordinates = {
  tenant: "connected-recovery", agent: "connected-agent",
  account: "01".repeat(32), asset: `0x${"02".repeat(32)}`,
};
const balanceValue = {
  account: coordinates.account, asset: coordinates.asset.slice(2),
  amount: "12345678901234567890", canonical_state: "0000000000000000ab54a98ceb1f0ad2", height: "140",
};
const settled = {
  kind: "owner-budget", ownerState: "settled",
  preparationId: "03".repeat(32), receiptDigest: "04".repeat(32),
};

async function balancePeer(t) {
  const directory = await mkdtemp(join(tmpdir(), "connected-balance-"));
  const path = join(directory, "peer.sqlite");
  let store = new DatabaseSync(path);
  store.exec("CREATE TABLE fixture (id INTEGER PRIMARY KEY, value TEXT NOT NULL, status TEXT NOT NULL, settlement TEXT NOT NULL)");
  store.prepare("INSERT INTO fixture VALUES (1, ?, ?, ?)").run(
    JSON.stringify(balanceValue), JSON.stringify({ state: "achieved", rung: "committed" }), JSON.stringify(settled),
  );
  const token = new SecretBytes(Buffer.alloc(32, 5));
  const session = new AgentSessionCredential(coordinates.tenant, "06".repeat(32), token, "0");
  const gatewaySecret = new SecretBytes(new TextEncoder().encode(`lxp_live_${"07".repeat(32)}`));
  const gatewayCredential = new LayerXKeyCredential("connected-balance", gatewaySecret);
  const calls = [];
  const failures = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const envelope = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/v1/agent/rpc");
      assert.equal(envelope.version, 1);
      assert.equal(envelope.operation, "read.balance");
      assert.deepEqual(envelope.credential, session.encode());
      assert.equal(envelope.idempotency_key, null);
      assert.deepEqual(envelope.request, {
        tenant: coordinates.tenant, agent: coordinates.agent,
        account: coordinates.account, asset: balanceValue.asset, requested_verification_level: "committed",
      });
      calls.push(envelope);
      const row = store.prepare("SELECT value, status FROM fixture WHERE id = 1").get();
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({
        request_id: envelope.request_id, value: JSON.parse(row.value), verification_status: JSON.parse(row.status),
      }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500, { "content-type": "application/json" });
      response.end("{}");
    }
  });
  t.after(async () => {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    store.close();
    token.destroy();
    gatewaySecret.destroy();
    await rm(directory, { recursive: true, force: true });
    assert.deepEqual(failures, []);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const transport = new AgentEnvelopeTransport({ endpoint: `http://127.0.0.1:${server.address().port}`,
    session, surface: "gateway", gatewayCredential, timeoutMs: 1000 });
  return {
    transport, calls,
    value(value) { store.prepare("UPDATE fixture SET value = ? WHERE id = 1").run(JSON.stringify(value)); },
    status(status) { store.prepare("UPDATE fixture SET status = ? WHERE id = 1").run(JSON.stringify(status)); },
    reopen() {
      store.close();
      store = new DatabaseSync(path);
      return JSON.parse(store.prepare("SELECT settlement FROM fixture WHERE id = 1").get().settlement);
    },
  };
}

async function connected_verification_rung_uses_real_balance_during_recovery(t) {
  const peer = await balancePeer(t);
  const decoded = await peer.transport.call({
    plane: "agent", operation: "read.balance",
    request: { ...coordinates, asset: balanceValue.asset, requested_verification_level: "committed" },
  });
  assert.deepEqual(decoded.value, balanceValue);
  assert.deepEqual(decoded.verification_status, { state: "achieved", rung: Rung.Committed });
  let reopens = 0;
  const recovered = await verifyConnectedRecovery(peer.transport, coordinates, settled, async () => {
    reopens += 1;
    return peer.reopen();
  });
  assert.equal(reopens, 1);
  assert.equal(recovered.before, balanceValue.amount);
  assert.deepEqual(recovered.recovered, settled);
  assert.equal(peer.calls.length, 3);
  assert.equal(new Set(peer.calls.map((call) => call.request_id)).size, 3);
}

async function connected_balance_requires_achieved_committed_evidence(t) {
  const peer = await balancePeer(t);
  for (const status of [
    { state: "achieved", rung: "submitted" },
    { state: "achieved", rung: "receipted" },
    { state: "pending", requested: "committed", achieved: "receipted" },
    { state: "absent", requested: "committed" },
    { state: "unverified", requested: "committed", achieved: "submitted", reason: "read.no_evidence" },
  ]) {
    peer.status(status);
    await assert.rejects(readConnectedBalance(peer.transport, coordinates), { code: "ERR_ASSERTION" });
  }
  for (const status of [
    { state: "achieved", level: "CheckpointFinalised" },
    { state: "achieved", level: "SettlementAnchored" },
    { state: "achieved", rung: "Committed" },
    { state: "achieved", rung: Rung.Committed },
    { state: "achieved", rung: "committed", level: "SettlementAnchored" },
    { state: "achieved" },
  ]) {
    peer.status(status);
    await assert.rejects(readConnectedBalance(peer.transport, coordinates),
      (error) => error instanceof PlatformSdkError && error.code === "verification-failure" && error.retry === "never");
  }
  peer.status({ state: "achieved", rung: "committed" });
  assert.equal(await readConnectedBalance(peer.transport, coordinates), balanceValue.amount);
}

async function connected_balance_preserves_identity_and_canonical_amount(t) {
  const peer = await balancePeer(t);
  for (const value of [
    null, [], "balance",
    { ...balanceValue, account: "ff".repeat(32) },
    { ...balanceValue, asset: "ff".repeat(32) },
    ...[undefined, 25, "", "01", "-1", "1.0", "1e3", " 1", "+1"].map((amount) => ({ ...balanceValue, amount })),
  ]) {
    peer.value(value);
    await assert.rejects(readConnectedBalance(peer.transport, coordinates), { code: "ERR_ASSERTION" });
  }
  for (const amount of ["0", "340282366920938463463374607431768211455"]) {
    peer.value({ ...balanceValue, amount });
    assert.equal(await readConnectedBalance(peer.transport, coordinates), amount);
    assert.equal(await readConnectedBalance(peer.transport, { ...coordinates, asset: balanceValue.asset }), amount);
  }
}

async function connected_recovery_refuses_second_debit_or_changed_replay(t) {
  const peer = await balancePeer(t);
  await assert.rejects(verifyConnectedRecovery(peer.transport, coordinates, settled, async () => {
    const recovered = peer.reopen();
    peer.value({ ...balanceValue, amount: (BigInt(balanceValue.amount) - 1n).toString() });
    return recovered;
  }), /without a second debit/u);
  peer.value(balanceValue);
  for (const changed of [
    { kind: "verified" }, { ownerState: "pending" },
    { preparationId: "ff".repeat(32) }, { receiptDigest: "ff".repeat(32) },
  ]) {
    await assert.rejects(verifyConnectedRecovery(peer.transport, coordinates, settled, async () => ({
      ...peer.reopen(), ...changed,
    })), { code: "ERR_ASSERTION" });
  }
  peer.status({ state: "achieved", rung: "receipted" });
  let recoveredWithoutCommit = false;
  await assert.rejects(verifyConnectedRecovery(peer.transport, coordinates, settled, async () => {
    recoveredWithoutCommit = true;
    return peer.reopen();
  }), { code: "ERR_ASSERTION" });
  assert.equal(recoveredWithoutCommit, false);
}

test(connected_verification_rung_uses_real_balance_during_recovery);
test(connected_balance_requires_achieved_committed_evidence);
test(connected_balance_preserves_identity_and_canonical_amount);
test(connected_recovery_refuses_second_debit_or_changed_replay);
