import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { qualifyReferenceAppOutcomes } from "../qualify-reference-apps.mjs";

async function qualifierRefusesSequencerSignedLabel() {
  const directory = await mkdtemp(join(tmpdir(), "reference-outcomes-"));
  const path = join(directory, "outcomes.json");
  try {
    const resource = await readFile(new URL("../paid-api/resource.json", import.meta.url), "utf8");
    const outcomes = [
      { application: "paid-api", environment: "emulator", path: "/paid", listening: 8080 },
      {
        application: "buyer-agent", environment: "emulator", state: "paid", status: 200,
        rung: "receipted", receiptDigest: "1".repeat(64), transactionHash: `0x${"a".repeat(64)}`,
        body: resource,
      },
      {
        application: "merchant-shop", environment: "emulator", state: "paid", orderId: "reference-order",
        rung: "committed", receiptDigest: "2".repeat(64), transactionHash: `0x${"b".repeat(64)}`,
      },
      ...["deploy", "list", "buy"].map((action, index) => ({
        application: "marketplace", environment: "emulator", action, state: "completed", resultCode: 0,
        rung: index === 1 ? "committed" : "receipted",
        receiptDigest: String(index + 3).repeat(64), transactionHash: `0x${String(index + 3).repeat(64)}`,
      })),
      {
        environment: "emulator", state: "completed",
        applications: ["buyer-agent", "paid-api", "merchant-shop", "marketplace"],
      },
    ];
    await writeFile(path, JSON.stringify(outcomes), { mode: 0o600 });
    const qualified = await qualifyReferenceAppOutcomes(path, "emulator");
    assert.deepEqual(qualified.buyer, outcomes[1]);
    assert.deepEqual(qualified.merchant, outcomes[2]);
    assert.deepEqual(qualified.marketplace, outcomes.slice(3, 6));

    const legacy = structuredClone(outcomes);
    delete legacy[1].rung;
    legacy[1].verification = "sequencer-signed";
    await writeFile(path, JSON.stringify(legacy));
    await assert.rejects(qualifyReferenceAppOutcomes(path, "emulator"), {
      code: "ERR_ASSERTION",
      message: "buyer payment rung must be receipted or committed",
    });

    for (let index = 1; index <= 5; index += 1) {
      const invalidRung = structuredClone(outcomes);
      invalidRung[index].rung = "sequencer-signed";
      await writeFile(path, JSON.stringify(invalidRung));
      await assert.rejects(qualifyReferenceAppOutcomes(path, "emulator"), {
        code: "ERR_ASSERTION", message: /rung must be receipted or committed/u,
      });

      for (const transactionHash of [undefined, "a".repeat(64), `0x${"a".repeat(63)}`, `0x${"g".repeat(64)}`]) {
        const invalidHash = structuredClone(outcomes);
        invalidHash[index].transactionHash = transactionHash;
        await writeFile(path, JSON.stringify(invalidHash));
        await assert.rejects(qualifyReferenceAppOutcomes(path, "emulator"), {
          code: "ERR_ASSERTION", message: /invalid .*transaction hash/u,
        });
      }
    }

    const reusedTransaction = structuredClone(outcomes);
    reusedTransaction[2].transactionHash = `0x${"A".repeat(64)}`;
    await writeFile(path, JSON.stringify(reusedTransaction));
    await assert.rejects(qualifyReferenceAppOutcomes(path, "emulator"), {
      code: "ERR_ASSERTION", message: "distinct real economic operations reused a transaction\n\n4 !== 5\n",
      actual: 4, expected: 5, operator: "strictEqual",
    });

    const reusedReceipt = structuredClone(outcomes);
    reusedReceipt[5].receiptDigest = reusedReceipt[1].receiptDigest;
    await writeFile(path, JSON.stringify(reusedReceipt));
    await assert.rejects(qualifyReferenceAppOutcomes(path, "emulator"), {
      code: "ERR_ASSERTION", message: "distinct real economic operations reused a receipt\n\n4 !== 5\n",
      actual: 4, expected: 5, operator: "strictEqual",
    });
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

test(qualifierRefusesSequencerSignedLabel);
