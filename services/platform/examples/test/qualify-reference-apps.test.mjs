import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { programReceiptOutcome } from "../marketplace/receipt-outcome.mjs";
import { qualifyReferenceAppOutcomes } from "../qualify-reference-apps.mjs";

async function qualifierRefusesSequencerSignedLabel() {
  const directory = await mkdtemp(join(tmpdir(), "reference-outcomes-"));
  const path = join(directory, "outcomes.json");
  try {
    const resource = await readFile(new URL("../paid-api/resource.json", import.meta.url), "utf8");
    const vectors = JSON.parse(await readFile(new URL(
      "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
    ), "utf8"));
    assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
    const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
    const marketplace = await Promise.all(["deploy", "list", "buy"].map(async (action, index) => {
      const vector = vectors.cases[index];
      const verified = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [{
        keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
        first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
      }]);
      return {
        application: "marketplace", environment: "emulator", action,
        ...programReceiptOutcome(vector.receiptJson, verified, vector.expected.rung),
      };
    }));
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
        resultCode: 0,
      },
      ...marketplace,
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

    for (let index = 2; index <= 5; index += 1) {
      for (const resultCode of [undefined, 1, "0", null]) {
        const invalidCode = structuredClone(outcomes);
        invalidCode[index].resultCode = resultCode;
        await writeFile(path, JSON.stringify(invalidCode));
        await assert.rejects(qualifyReferenceAppOutcomes(path, "emulator"), {
          code: "ERR_ASSERTION", message: /result code must indicate success/u,
        });
      }
      const legacyStatus = structuredClone(outcomes);
      delete legacyStatus[index].resultCode;
      legacyStatus[index].status = 1;
      await writeFile(path, JSON.stringify(legacyStatus));
      await assert.rejects(qualifyReferenceAppOutcomes(path, "emulator"), {
        code: "ERR_ASSERTION", message: /result code must indicate success/u,
      });
    }

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
