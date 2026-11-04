import assert from "node:assert/strict";
import { Rung } from "@sidiora/layerx-sdk";

export async function readConnectedBalance(transport, { tenant, agent, account, asset }) {
  const expectedAsset = asset.replace(/^0x/u, "");
  const response = await transport.call({
    plane: "agent", operation: "read.balance",
    request: { tenant, agent, account, asset: expectedAsset, requested_verification_level: "committed" },
  });
  const value = response.value;
  assert.ok(value && typeof value === "object" && !Array.isArray(value), "object required");
  assert.equal(response.verification_status?.state, "achieved");
  assert.equal(response.verification_status?.rung, Rung.Committed);
  assert.equal(value.account, account);
  assert.equal(value.asset, expectedAsset);
  assert.ok(typeof value.amount === "string" && /^(0|[1-9][0-9]*)$/u.test(value.amount));
  return value.amount;
}

export async function verifyConnectedRecovery(transport, coordinates, settled, recover) {
  const before = await readConnectedBalance(transport, coordinates);
  const recovered = await recover();
  assert.equal(recovered.kind, "owner-budget");
  assert.equal(recovered.ownerState, "settled");
  assert.equal(recovered.preparationId, settled.preparationId);
  assert.equal(recovered.receiptDigest, settled.receiptDigest);
  assert.equal(await readConnectedBalance(transport, coordinates), before,
    "fresh agent process reopens durable preparation and receipt without a second debit");
  return { before, recovered };
}
