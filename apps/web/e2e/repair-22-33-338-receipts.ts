import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import type { SequencerReceiptAnswer } from "../../../sdk/wallet/src/types.ts";

interface Recording {
  readonly rawTransaction: string;
  readonly receiptJson: SequencerReceiptAnswer;
  readonly pinnedKey: { readonly keyId: string; readonly publicKey: string; readonly firstHeight: number; readonly lastHeight: number };
}
const vector = JSON.parse(readFileSync(new URL("../../../sdk/platform/conformance/fixtures/vector-v4-committed.json", import.meta.url), "utf8")) as { positive: Recording };
const recorded = JSON.parse(readFileSync(new URL("../../../daemon/committed-fixture/vectors/vector-v4-committed.json", import.meta.url), "utf8")) as {
  settlement: { raw_transaction: string; receipt: SequencerReceiptAnswer };
};
export const committedReceipt = recorded.settlement.receipt;
assert.equal(committedReceipt.txHash, vector.positive.receiptJson.txHash);
assert.equal(committedReceipt.keyId, vector.positive.pinnedKey.keyId);
assert.equal(committedReceipt.signingKey, vector.positive.pinnedKey.publicKey);
export const committedRecording: Recording = {
  rawTransaction: recorded.settlement.raw_transaction,
  receiptJson: committedReceipt,
  pinnedKey: vector.positive.pinnedKey,
};

export async function withNativeReceipt(
  run: (read: () => Promise<SequencerReceiptAnswer>, origin: string, requests: readonly string[]) => Promise<void>,
): Promise<void> {
  const requests: string[] = [];
  const failures: unknown[] = [];
  const server = createServer(async (request, response) => {
    try {
      assert.equal(request.url, "/");
      assert.equal(request.method, "POST");
      assert.equal(request.headers.authorization, undefined, "public node reads must not leak hosted credentials");
      let text = "";
      for await (const chunk of request) text += String(chunk);
      const rpc = JSON.parse(text) as { id: number; method: string; params: unknown[] };
      requests.push(rpc.method);
      let result: unknown;
      switch (rpc.method) {
        case "pax_sequencerReceipt":
          assert.deepEqual(rpc.params, [committedReceipt.txHash]);
          result = committedReceipt;
          break;
        case "px_getReceiptKeys":
          assert.deepEqual(rpc.params, []);
          result = { keys: [committedRecording.pinnedKey] };
          break;
        case "eth_getRawTransactionByHash":
          assert.deepEqual(rpc.params, [committedReceipt.txHash]);
          result = committedRecording.rawTransaction;
          break;
        default: throw new Error(`Unexpected native receipt method ${rpc.method}`);
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id, result }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500);
      response.end();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  try {
    const address = server.address();
    assert.ok(address !== null && typeof address !== "string");
    const origin = `http://127.0.0.1:${address.port}`;
    const { JsonRpcClient } = await import("../../../sdk/wallet/src/rpc.ts");
    const { authenticateSequencerReceipt } = await import("../../../sdk/wallet/src/receipt.ts");
    const rpc = new JsonRpcClient({ url: origin });
    await run(async () => {
      const candidate = await rpc.call("pax_sequencerReceipt", [committedReceipt.txHash]);
      const receipt = await authenticateSequencerReceipt((method, params) => rpc.call(method, params), candidate, committedReceipt.txHash);
      assert.equal(receipt.rung, "committed");
      assert.equal(receipt.txHash, committedReceipt.txHash);
      assert.equal(receipt.blockNumber, committedReceipt.blockNumber);
      assert.deepEqual(receipt.logs, committedReceipt.logs);
      return receipt;
    }, origin, requests);
    assert.deepEqual(failures, []);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close(error => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
  }
}
