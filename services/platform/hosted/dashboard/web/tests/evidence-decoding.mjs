import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import { decodeDeliveries, decodeOverview, decodeReceipt, decodeRequests } from "../app/data.ts";

async function rust_overview_decodes_without_inventing_evidence() {
  const serialized = await readFile(new URL("./fixtures/rust-overview.json", import.meta.url), "utf8");
  const overview = decodeOverview(JSON.parse(serialized));
  assert.deepEqual(overview, {
    principal: "developer-fixture",
    usage: { keys: 1, live_keys: 1, requests_allowed: 10, requests_used: 1, requests_remaining: 9, utilisation_per_mille: 100 },
    keys: [{ key_id: "key-fixture", disabled: false, requests_per_window: 10, window_seconds: 60, used_in_window: 1, remaining_in_window: 9 }],
    recent_requests: [{ at: 1, operation_digest: "operation-fixture", outcome: "completed", verification: "unverified" }],
    endpoints: [],
    dead_letters: [],
    payments: [{
      event: "payment-fixture", subject: "subject-fixture", amount: "10", asset: undefined,
      verification: "unverified", settlement_verification: "unverified", receipt_digest: undefined, settled: false,
      facts: [{ name: "state", value: "pending", verification: "unverified", receipt_digest: undefined }],
    }],
  });
}

test(rust_overview_decodes_without_inventing_evidence);

assert.throws(() => decodeRequests([{ at: 1, operation_digest: "operation", outcome: "completed", verification: "paxeer-finalised" }]), /request evidence/);
assert.throws(() => decodeDeliveries([{ delivery: "delivery", endpoint: "endpoint", event: "event", state: { state: "delivered", status: 500 }, verification: "unverified", receipt_digest: null }]), /accepting status/);
assert.throws(() => decodeReceipt({ activity_id: "a".repeat(64), event: "event", receipt_digest: "b".repeat(64), verification: "unverified", settled: true }), /not verified/);
assert.throws(() => decodeReceipt({ activity_id: "a".repeat(64), event: "event", receipt_digest: "b".repeat(64), verification: "invented", settled: true }), /Unknown protocol verification/);
console.log("dashboard evidence refusals: 4 passed");
