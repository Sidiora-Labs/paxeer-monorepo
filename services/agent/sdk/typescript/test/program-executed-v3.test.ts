import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseProgramExecutionDocument } from "../src/programs.js";

const fixture = JSON.parse(readFileSync(new URL("../../../../../../sdk/platform/conformance/fixtures/receipt-programs-executed-v3.json", import.meta.url), "utf8")) as {
  canonical_receipt_hex: string; program_id_hex: string; receipt_digest_hex: string; terminal_payload_hex: string; call_graph_hex: string;
  execution_document: Readonly<Record<string, unknown>>;
  authorized_batch: { batch_id_hex: string; asset_hex: string; previous_state_root_hex: string; resulting_state_root_hex: string; sequencer_public_key_hex: string };
};
const document = fixture.execution_document;
const parsed = parseProgramExecutionDocument(document);
assert.equal(parsed.module_version, 4);
for (const module_version of [0, 5, true]) assert.throws(() => parseProgramExecutionDocument({ ...document, module_version }));
