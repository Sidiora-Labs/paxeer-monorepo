import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { sha256 } from "@noble/hashes/sha2.js";
import {
  encodeNativeProgramCall,
  decodeNativeProgramCall,
} from "../src/index.js";
import { verifySequencerReceipt } from "../src/verifier.js";
import { gasReceiptDenominations } from "../src/gas-station.js";
import { PROGRAMS_PRECOMPILE } from "../src/ai_market.js";

function assert(condition: boolean, message: string): asserts condition {
  if (!condition) throw new Error(message);
}

function hexBytes(value: string): Uint8Array {
  assert(value.length % 2 === 0, "odd hex length");
  const bytes = new Uint8Array(value.length / 2);
  for (let index = 0; index < bytes.length; index += 1) {
    const parsed = Number.parseInt(value.slice(2 * index, 2 * index + 2), 16);
    assert(Number.isInteger(parsed), "invalid hex byte");
    bytes[index] = parsed;
  }
  return bytes;
}

function hexEqual(
  actual: Uint8Array,
  expectedHex: string,
  field: string,
): void {
  const expected = hexBytes(expectedHex);
  assert(actual.length === expected.length, `${field} length diverged`);
  for (let index = 0; index < actual.length; index += 1) {
    assert(
      actual[index] === expected[index],
      `${field} diverged at byte ${index}`,
    );
  }
}

const native = encodeNativeProgramCall({
  programId: new Uint8Array(32).fill(0x11),
  guestAbi: 1,
  entrypoint: "layerx_call",
  calldata: new Uint8Array(),
  capabilities: new Uint8Array([0, 0]),
  accessDeclaration: new TextEncoder().encode(
    "LayerX/programs/access-declaration/v1\0\0",
  ),
  responseCapacity: 16,
  resources: [
    1_000_000n,
    16_777_216n,
    1_048_576n,
    1_048_576n,
    64n,
    1_048_576n,
    4096n,
  ],
});
hexEqual(
  native.slice(32, 50),
  "0001000b0000000000020000002700000010",
  "native header",
);
assert(
  decodeNativeProgramCall(native).entrypoint === "layerx_call",
  "native decode",
);
for (let length = 0; length < native.length; length++) {
  let rejected = false;
  try {
    decodeNativeProgramCall(native.slice(0, length));
  } catch {
    rejected = true;
  }
  assert(rejected, "truncated native call accepted");
}

const nativeFixture = JSON.parse(
  readFileSync(
    fileURLToPath(
      new URL(
        "../../../../../../sdk/platform/conformance/fixtures/native-program-call-v3.json",
        import.meta.url,
      ),
    ),
    "utf8",
  ),
) as {
  payload_hex: string;
  signed_activity_hex: string;
  activity_id_hex: string;
};
const { NativeProgramRequest } = await import("../src/programs.js");
const { decodeSignedProgramCall } = await import("../src/program-wire.js");
const nativeModel = decodeNativeProgramCall(
  hexBytes(nativeFixture.payload_hex),
);
const nativeRequest = new NativeProgramRequest(
  nativeModel,
  1000n,
  hexBytes(nativeFixture.signed_activity_hex),
);
assert(
  (await decodeSignedProgramCall(nativeRequest)).activityId ===
    nativeFixture.activity_id_hex,
  "native signed binding",
);
const changedProgramId = nativeModel.programId.slice();
const nonzeroIndex = changedProgramId.findIndex((value) => value !== 0);
changedProgramId[nonzeroIndex] = changedProgramId[nonzeroIndex] === 1 ? 2 : 1;
const mutateBytes = (value: Uint8Array): Uint8Array => {
  const changed = value.length ? value.slice() : new Uint8Array(1);
  changed[0] = (changed[0] ?? 0) ^ 1;
  return changed;
};
const mismatches: {
  [K in keyof typeof nativeModel]: [K, (typeof nativeModel)[K]];
}[keyof typeof nativeModel][] = [
  ["programId", changedProgramId],
  ["guestAbi", nativeModel.guestAbi === 1 ? 2 : 1],
  [
    "entrypoint",
    (nativeModel.entrypoint[0] === "a" ? "b" : "a") +
      nativeModel.entrypoint.slice(1),
  ],
  ["calldata", mutateBytes(nativeModel.calldata)],
  ["capabilities", mutateBytes(nativeModel.capabilities)],
  ["accessDeclaration", mutateBytes(nativeModel.accessDeclaration)],
  ["responseCapacity", (nativeModel.responseCapacity + 1) % 1_048_577],
  [
    "resources",
    [
      nativeModel.resources[0] ^ 1n,
      nativeModel.resources[1],
      nativeModel.resources[2],
      nativeModel.resources[3],
      nativeModel.resources[4],
      nativeModel.resources[5],
      nativeModel.resources[6],
    ],
  ],
];
for (const [field, value] of mismatches) {
  const original = nativeModel[field];
  assert(
    value instanceof Uint8Array && original instanceof Uint8Array
      ? value.length !== original.length ||
          value.some((byte, index) => byte !== original[index])
      : Array.isArray(value) && Array.isArray(original)
        ? value.some((item, index) => item !== original[index])
        : value !== original,
    `native ${field} mutation is unchanged`,
  );
  const changed = { ...nativeModel, [field]: value };
  encodeNativeProgramCall(changed);
  let rejected = false;
  try {
    await decodeSignedProgramCall(
      new NativeProgramRequest(changed, 1000n, nativeRequest.signedActivity),
    );
  } catch {
    rejected = true;
  }
  assert(rejected, "mismatched native field accepted");
}
let rejectedNativeFee = false;
try {
  await decodeSignedProgramCall(
    new NativeProgramRequest(
      nativeModel,
      nativeRequest.budget.feeLimit - 1n,
      nativeRequest.signedActivity,
    ),
  );
} catch {
  rejectedNativeFee = true;
}
assert(rejectedNativeFee, "native envelope fee mismatch accepted");

async function paymentDenom(): Promise<void> {
  const vectors: {
    feeDenom: string;
    cases: {
      id: string;
      rawTransaction: string;
      signedReceipt: string;
      receiptJson: Record<string, unknown>;
      pinnedKey: {
        keyId: string;
        publicKey: string;
        firstHeight: number;
        lastHeight: number;
      };
      expected: { verifier: string; rung?: string };
    }[];
  } = JSON.parse(
    readFileSync(
      new URL(
        "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json",
        import.meta.url,
      ),
      "utf8",
    ),
  );
  for (const entry of vectors.cases) {
    assert(
      entry.receiptJson.paymentDenom === vectors.feeDenom,
      `${entry.id}: recorded payment denomination differs from the base denomination`,
    );
    const denominations = gasReceiptDenominations(entry.receiptJson);
    assert(
      denominations.paymentDenom === vectors.feeDenom &&
        denominations.feeDenom === entry.receiptJson.feeDenom,
      `${entry.id}: recorded receipt view refused or denominations changed`,
    );
  }
  const accepted = vectors.cases.filter(
    (entry) => entry.expected.rung === "receipted",
  );
  assert(accepted.length > 0, "missing producer receipt cases");
  for (const entry of accepted) {
    const payment = entry.receiptJson.paymentDenom;
    const fee = entry.receiptJson.feeDenom;
    assert(
      typeof payment === "string" && typeof fee === "string",
      "missing receipt denominations",
    );
    const key = entry.pinnedKey;
    const signed = hexBytes(entry.rawTransaction.slice(2));
    const verified = await verifySequencerReceipt(
      hexBytes(entry.signedReceipt.slice(2)),
      signed,
      [
        {
          keyId: hexBytes(key.keyId.slice(2)),
          publicKey: hexBytes(key.publicKey.slice(2)),
          first_height: BigInt(key.firstHeight),
          last_height: BigInt(key.lastHeight),
        },
      ],
    );
    const digest = sha256(new TextEncoder().encode(payment));
    assert(
      verified.receipt.asset.every((byte, index) => byte === digest[index]),
      `${entry.id}: payment asset mismatch`,
    );
    const summary = verified.receipt.effects[0]?.body;
    assert(
      summary !== undefined && summary.length === 109 + summary[108]!,
      "invalid fee summary",
    );
    assert(
      new TextDecoder("utf-8", { fatal: true }).decode(summary.slice(109)) ===
        fee,
      "fee denomination changed",
    );
    if (entry.receiptJson.to === PROGRAMS_PRECOMPILE) {
      assert(
        gasReceiptDenominations(entry.receiptJson).paymentDenom === payment,
        "producer Programs receipt refused",
      );
      const missing = { ...entry.receiptJson };
      delete missing.paymentDenom;
      let rejected = false;
      try {
        gasReceiptDenominations(missing);
      } catch (error) {
        rejected =
          error instanceof Error && error.message === "invalid_response" &&
          "field" in error && error.field === "paymentDenom";
      }
      assert(rejected, "receipt without paymentDenom accepted");
    }
  }
  const recorded = accepted[0]!.receiptJson;
  const secondDenom = "ufee";
  assert(
    secondDenom !== recorded.feeDenom,
    "derived denominations must differ",
  );
  const receipt = recorded.receipt;
  const feeDenom = recorded.feeDenom;
  assert(typeof receipt === "string", "missing recorded receipt bytes");
  assert(typeof feeDenom === "string", "missing recorded fee denomination");
  const feeDigest = Buffer.from(
    sha256(new TextEncoder().encode(feeDenom)),
  ).toString("hex");
  const paymentDigest = Buffer.from(
    sha256(new TextEncoder().encode(secondDenom)),
  ).toString("hex");
  const assetOffset = receipt.indexOf(feeDigest);
  assert(
    assetOffset >= 2 && receipt.lastIndexOf(feeDigest) === assetOffset,
    "recorded receipt must contain exactly one payment asset digest",
  );
  const derived = {
    ...recorded,
    paymentDenom: secondDenom,
    receipt:
      receipt.slice(0, assetOffset) +
      paymentDigest +
      receipt.slice(assetOffset + feeDigest.length),
  };
  const denominations = gasReceiptDenominations(derived);
  assert(
    denominations.paymentDenom === secondDenom &&
      denominations.feeDenom === recorded.feeDenom,
    "distinct payment asset refused or fee denomination changed",
  );
  const missing: Record<string, unknown> = { ...derived };
  delete missing.paymentDenom;
  for (const invalid of [{ ...derived, receipt }, missing]) {
    let rejected = false;
    try {
      gasReceiptDenominations(invalid);
    } catch (error) {
      rejected =
        error instanceof Error &&
        error.message === "invalid_response" &&
        "field" in error &&
        error.field === "paymentDenom";
    }
    assert(rejected, "fee asset or missing paymentDenom accepted");
  }
}

test(paymentDenom);
