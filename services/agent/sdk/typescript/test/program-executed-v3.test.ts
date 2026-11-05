import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { parseProgramExecutionDocument } from "../src/programs.js";
import { encodeSequencerReceipt } from "../src/sequencer-receipt-codec.js";
import { SequencerReceiptVerificationError, verifySequencerReceipt } from "../src/verifier.js";

function program_execution_document_checks_module_version(): void {
  const document = JSON.parse(readFileSync(new URL(
    "../../../../../../tests/fixtures/programs/account-bound-call/execution.json",
    import.meta.url,
  ), "utf8")) as Readonly<Record<string, unknown>>;
  const parsed = parseProgramExecutionDocument(document);
  assert.equal(parsed.module_version, 4);
  assert.deepEqual(parsed, document);
  for (const module_version of [0, 5, true]) {
    assert.throws(() => parseProgramExecutionDocument({ ...document, module_version }), TypeError);
  }
}

test(program_execution_document_checks_module_version);

async function program_execution_receipt_uses_producer_encoding(): Promise<void> {
  const corpus: {
    readonly provenance: { readonly generator: string };
    readonly cases: readonly {
      readonly id: string;
      readonly signedReceipt: string;
      readonly rawTransaction: string;
      readonly receiptDigest: string;
      readonly pinnedKey: {
        readonly keyId: string;
        readonly publicKey: string;
        readonly firstHeight: number;
        readonly lastHeight: number;
      };
    }[];
  } = JSON.parse(readFileSync(new URL(
    "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json",
    import.meta.url,
  ), "utf8"));
  assert.equal(corpus.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
  const vector = corpus.cases.find((entry) => entry.id === "V1-layerxprograms-call");
  assert.ok(vector, "the producer corpus must include a Programs call");
  const bytes = (value: string): Uint8Array => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
  const signed = bytes(vector.signedReceipt);
  const transaction = bytes(vector.rawTransaction);
  const key = {
    keyId: bytes(vector.pinnedKey.keyId),
    publicKey: bytes(vector.pinnedKey.publicKey),
    first_height: BigInt(vector.pinnedKey.firstHeight),
    last_height: BigInt(vector.pinnedKey.lastHeight),
  };
  const verified = await verifySequencerReceipt(signed, transaction, [key]);
  assert.equal(verified.level, "sequencer-signed");
  assert.equal(verified.receipt.moduleId, 12);
  assert.equal(verified.receipt.moduleVersion, 1);
  assert.equal(verified.receipt.operation, 2);
  assert.deepEqual(verified.receiptDigest, bytes(vector.receiptDigest));
  const regenerated = encodeSequencerReceipt(verified.receipt);
  assert.deepEqual(regenerated, signed);
  assert.deepEqual(await verifySequencerReceipt(regenerated, transaction, [key]), verified);
  const corrupted = regenerated.slice();
  corrupted[corrupted.length - 1]! ^= 1;
  await assert.rejects(verifySequencerReceipt(corrupted, transaction, [key]),
    (error: unknown) => error instanceof SequencerReceiptVerificationError && error.check === "bad_signature");
}

test(program_execution_receipt_uses_producer_encoding);
