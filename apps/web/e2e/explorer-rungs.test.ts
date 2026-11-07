import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import type { VerificationRung } from "../src/kit/model.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const {
  decodeAccountActivity,
  decodeBatch,
  decodeCheckpoint,
  decodeReceipt,
  decodeVerificationReport,
} = await import("../src/explorer/model.ts");

const IDENTIFIER = "11".repeat(32);
const RECORDS = [
  {
    decode: decodeReceipt,
    value: {
      receipt_id: IDENTIFIER,
      batch_number: "7",
      ordinal: "0",
      canonical_bytes: "AQID",
    },
  },
  {
    decode: decodeCheckpoint,
    value: {
      checkpoint_id: IDENTIFIER,
      batch_number: "7",
      first_sequence: "1",
      last_sequence: "2",
      achieved_signatures: "3",
      required_signatures: "3",
    },
  },
  {
    decode: decodeBatch,
    value: {
      batch_number: "7",
      total_availability_bytes: "3",
      activity_count: "1",
      receipt_count: "1",
      event_count: "0",
    },
  },
  {
    decode: decodeAccountActivity,
    value: {
      receipt_id: IDENTIFIER,
      receipt_digest: IDENTIFIER,
      batch_number: "7",
      global_sequence: "1",
      activity_id: IDENTIFIER,
      operation: "1",
      result_code: "0",
      asset: IDENTIFIER,
      amount: "10",
      from: IDENTIFIER,
      to: "22".repeat(32),
    },
  },
] as const;

const REPORT = {
  kind: "receipt",
  receipt_digest: IDENTIFIER,
  freshness: {
    observed_chain_sequence: "2",
    observed_sealed_batch: "7",
    observed_finalised_checkpoint: IDENTIFIER,
    batches_behind: "0",
    current: true,
  },
} as const;

function explorer_levels_collapse_to_three_rungs() {
  const levels: readonly (readonly [string, VerificationRung])[] = [
    ["unverified", "unverified"],
    ["sequencer-signed", "receipt-verified"],
    ["batch-included", "checkpoint-finalised"],
    ["state-proven", "checkpoint-finalised"],
    ["checkpoint-finalised", "checkpoint-finalised"],
  ];
  for (const [level, expected] of levels) {
    for (const { decode, value } of RECORDS) {
      assert.equal(
        decode({ ...value, verification_level: level }).verificationLevel,
        expected,
        `${decode.name}: ${level}`,
      );
    }
    assert.equal(
      decodeVerificationReport({ ...REPORT, achieved_level: level }).achievedLevel,
      expected,
      `decodeVerificationReport: ${level}`,
    );
  }
}

function explorer_settlement_anchored_is_refused() {
  for (const level of ["settlement-anchored", "receipt-verified", "unknown", "SEQUENCER-SIGNED", " sequencer-signed", "", null, undefined, 1]) {
    for (const { decode, value } of RECORDS) {
      assert.throws(
        () => decode({ ...value, verification_level: level }),
        { name: "TypeError", message: /\.verification_level /u },
        `${decode.name}: ${String(level)}`,
      );
    }
    assert.throws(
      () => decodeVerificationReport({ ...REPORT, achieved_level: level }),
      { name: "TypeError", message: /\.achieved_level /u },
      `decodeVerificationReport: ${String(level)}`,
    );
  }
}

test(explorer_levels_collapse_to_three_rungs);
test(explorer_settlement_anchored_is_refused);
