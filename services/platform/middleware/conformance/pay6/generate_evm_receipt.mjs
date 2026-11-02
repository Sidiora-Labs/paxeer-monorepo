import { createPrivateKey, createPublicKey, webcrypto } from "node:crypto";
import { writeFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";
import { recoverEip1559Sender } from "@sidiora/layerx-sdk";
import { ConformanceSequencer, buildSignedReceipt, fixedBytes, offerFixture } from "../src/receipts.ts";

export function pinnedReceiptKeys(fixture) {
  const key = fixture.receipt.receiptKey;
  return [{
    keyId: Uint8Array.from(Buffer.from(key.keyId.slice(2), "hex")),
    publicKey: Uint8Array.from(Buffer.from(key.publicKey.slice(2), "hex")),
    first_height: BigInt(key.firstHeight),
    last_height: BigInt(key.lastHeight),
  }];
}

export async function generateEvmReceiptFixture() {
  const privateKey = createPrivateKey({
    key: Buffer.concat([Buffer.from("302e020100300506032b657004220420", "hex"), Buffer.alloc(32, 7)]),
    format: "der",
    type: "pkcs8",
  });
  const publicKey = createPublicKey(privateKey).export({ format: "der", type: "spki" }).subarray(-32);
  const signingKey = await webcrypto.subtle.importKey(
    "pkcs8", privateKey.export({ format: "der", type: "pkcs8" }), "Ed25519", false, ["sign"],
  );
  const sequencer = new ConformanceSequencer(signingKey, Uint8Array.from(publicKey), fixedBytes(1));
  const facts = {
    asset: fixedBytes(0x11), payTo: fixedBytes(0x22), amount: 25000n,
    from: fixedBytes(0x33), batchId: fixedBytes(7),
    previousStateRoot: fixedBytes(0x55), resultingStateRoot: fixedBytes(0x66),
  };
  const { txHash, ...receipt } = await buildSignedReceipt(sequencer, facts);
  const offer = offerFixture(facts.payTo, facts.asset, facts.amount).requirements;
  const payer = recoverEip1559Sender(Buffer.from(receipt.rawTransaction.slice(2), "hex"));
  return `${JSON.stringify({
    receipt, offer, txHash,
    payer: `0x${payer.slice(2).toLowerCase().padStart(64, "0")}`,
    rpcResult: {
      txHash, rung: "receipted", status: "0x1", height: "0x7",
      keyId: receipt.receiptKey.keyId, receipt: receipt.signedReceipt,
    },
  }, null, 2)}\n`;
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  await writeFile(process.argv[2] ?? new URL("evm-receipt.json", import.meta.url), await generateEvmReceiptFixture());
}
