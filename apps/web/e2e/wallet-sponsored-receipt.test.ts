import assert from "node:assert/strict";
import test from "node:test";
import type { SidioraGasQuote } from "../src/api/gas-station.ts";
import { wallet, HASH, SECOND_RECEIPT, expectedReceipt, withSubmission, withSponsoredSends } from "./repair-22-33-338-sponsored.ts";

async function completed_sponsored_submission_does_not_answer_a_new_call(): Promise<void> {
  for (const identical of [false, true]) {
    for (const completion of ["retained", "resumed", "immediate"] as const) {
      await withSponsoredSends(async (session, quotes) => {
        const firstCall = session.quoted.batch.calls[0]!;
        const contextKey = `layerx:private-sponsored:v1:${session.quoted.batch.account}`;
        const pendingKey = `paxeer:sponsored:pending:v1:125:${session.quoted.batch.account}`;
        if (completion === "immediate") {
          session.status = "confirmed";
          session.receiptStatus = "0x1";
        }
        const first = await wallet.sendWalletPrecompileCall(session.quoted.batch.account, () => firstCall);
        assert.equal(first.outcome, "sent");
        assert.ok("transactionHash" in first);
        assert.equal(first.transactionHash, HASH);
        assert.equal(session.batches.length, 1);
        assert.equal(session.storage.getItem(contextKey) === null, completion === "immediate");
        session.status = "confirmed";
        session.receiptStatus = "0x1";
        if (completion === "resumed") {
          const batch = session.batches[0]!;
          const raw = session.storage.getItem(`paxeer:sponsored:v1:125:${batch.account}:${batch.quote.sponsor}:${batch.quote.quoteNonce}`);
          assert.ok(raw);
          const retained = JSON.parse(raw) as { relayer_signature: string };
          const snapshot: SidioraGasQuote = {
            ...session.quoted, batch: { ...session.quoted.batch, quote: { ...session.quoted.batch.quote, quoteNonce: BigInt(batch.quote.quoteNonce) } },
            relayerSignature: retained.relayer_signature,
          };
          const review = wallet.sidioraQuotePresentation(snapshot);
          assert.ok(review);
          const resumed = await wallet.sendWalletSponsoredBatch(snapshot, review.identity);
          assert.equal(resumed.outcome, "sent");
          assert.equal(session.storage.getItem(contextKey), null);
          assert.equal(session.storage.getItem(pendingKey), "null");
        }
        session.nonce = 1n;
        const nextCall = identical ? firstCall : { ...firstCall, data: "0x5678" };
        const next = await wallet.sendWalletPrecompileCall(session.quoted.batch.account, () => nextCall);
        assert.equal(next.outcome, "sent");
        assert.ok("transactionHash" in next);
        assert.equal(next.transactionHash, SECOND_RECEIPT.txHash);
        assert.notEqual(next.transactionHash, first.transactionHash);
        assert.equal(session.batches.length, 2);
        assert.equal(quotes.length, 2);
        assert.deepEqual(session.batches[1]!.calls, [{ ...nextCall, value: nextCall.value.toString() }]);
        assert.equal(session.batches[1]!.nonce, "1");
        assert.equal(session.storage.getItem(contextKey), null);
        assert.equal(session.storage.getItem(pendingKey), "null");
      });
    }
  }
}

async function stale_sponsored_context_does_not_block_sends(): Promise<void> {
  await withSponsoredSends(async (session, quotes) => {
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "sent", transactionHash: HASH });
    const contextKey = `layerx:private-sponsored:v1:${session.quoted.batch.account}`;
    assert.ok(session.storage.getItem(contextKey));
    session.status = "confirmed";
    session.receiptStatus = "0x1";
    const { windowWalletProvider } = await import("../src/api/provider.ts");
    const provider = windowWalletProvider();
    assert.ok(provider);
    const module = wallet.sponsoredWalletModule(session.quoted, provider, session.storage);
    assert.equal((await module.resume(session.quoted.batch, session.quoted.relayerSignature)).status, "confirmed");
    assert.equal(await module.pending(session.quoted.batch.account), null);
    assert.ok(session.storage.getItem(contextKey));
    session.nonce = 1n;
    const next = await wallet.sendWalletPrecompileCall(session.quoted.batch.account, () => ({ ...session.quoted.batch.calls[0]!, data: "0x5678" }));
    assert.equal(next.outcome, "sent");
    assert.ok("transactionHash" in next);
    assert.equal(next.transactionHash, SECOND_RECEIPT.txHash);
    assert.equal(session.batches.length, 2);
    assert.equal(quotes.length, 1);
    assert.equal(session.storage.getItem(contextKey), null);
    assert.equal(await module.pending(session.quoted.batch.account), null);
  });
}

async function sponsored_batch_reports_receipt(): Promise<void> {
  for (const status of ["0x1", "0x0", null] as const) {
    await withSubmission(async session => {
      session.receiptStatus = status;
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), status === null
        ? { outcome: "sent", transactionHash: session.hash }
        : { outcome: status === "0x1" ? "sent" : "reverted", transactionHash: session.hash, receipt: expectedReceipt(status, session.hash) });
      assert.deepEqual(session.receiptHashes, [[session.hash]]);
      assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
      assert.equal(session.paths.filter(path => path === "/v1/wallet/sign-digest").length, 2);
    }, status === "0x0");
  }
  await withSubmission(async session => {
    session.receiptHttpStatus = 503;
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    const result = await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    assert.deepEqual(result, { outcome: "unconfirmed", transactionHash: HASH });
    assert.deepEqual(session.receiptHashes, [[session.hash]]);
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
  await withSubmission(async session => {
    session.submitHttpStatus = 403;
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "rejected" });
    assert.deepEqual(session.receiptHashes, []);
  });
}

async function resumed_submission_reports_receipt(): Promise<void> {
  for (const status of ["confirmed", "reverted", "pending", "cancelled"] as const) {
    await withSubmission(async session => {
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "sent", transactionHash: session.hash });
      await session.restart();
      session.receiptHashes.length = 0;
      session.paths.length = 0;
      session.status = status;
      session.receiptStatus = status === "reverted" ? "0x0" : status === "confirmed" ? "0x1" : null;
      const result = status === "pending"
        ? await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account, session.quoted.batch.calls[0])
        : await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
      assert.deepEqual(result, status === "cancelled" ? { outcome: "rejected" }
        : session.receiptStatus === null ? { outcome: "sent", transactionHash: session.hash }
          : { outcome: status === "reverted" ? "reverted" : "sent", transactionHash: session.hash, receipt: expectedReceipt(session.receiptStatus, session.hash) });
      assert.deepEqual(session.receiptHashes, status === "cancelled" ? [] : [[session.hash]]);
      assert.ok(!session.paths.includes("/v1/wallet/sponsored/submit"));
      assert.ok(!session.paths.includes("/v1/wallet/sign-digest"));
      assert.equal(session.storage.getItem(`layerx:private-sponsored:v1:${session.quoted.batch.account}`) === null, status !== "pending");
    }, status === "reverted");
  }
  for (const status of ["pending", "confirmed"] as const) {
    await withSubmission(async session => {
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
      session.receiptHashes.length = 0;
      session.status = status;
      session.receiptStatus = status === "confirmed" ? "0x1" : null;
      const foreignCall = { to: `0x${"99".repeat(20)}`, value: 0n, data: "0x5678" };
      assert.deepEqual(await wallet.resumeWalletSponsoredSubmission(session.quoted.batch.account, foreignCall),
        status === "pending" ? { outcome: "unavailable" } : undefined);
      assert.deepEqual(session.receiptHashes, []);
    });
  }
  await withSubmission(async session => {
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    session.status = "confirmed";
    session.receiptStatus = "0x1";
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), {
      outcome: "sent", transactionHash: session.hash, receipt: expectedReceipt("0x1"),
    });
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
  for (const status of ["pending", "confirmed", "reverted"] as const) {
    await withSubmission(async session => {
      const review = wallet.sidioraQuotePresentation(session.quoted);
      assert.ok(review);
      await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
      session.status = status;
      session.receiptStatus = status === "reverted" ? "0x0" : "0x1";
      session.receiptHttpStatus = 503;
      assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "unconfirmed", transactionHash: session.hash });
      assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
    }, status === "reverted");
  }
  await withSubmission(async session => {
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    session.receiptHttpStatus = 503;
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "unconfirmed", transactionHash: session.hash });
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
  await withSubmission(async session => {
    const review = wallet.sidioraQuotePresentation(session.quoted);
    assert.ok(review);
    await wallet.sendWalletSponsoredBatch(session.quoted, review.identity);
    session.status = "reverted";
    assert.deepEqual(await wallet.sendWalletSponsoredBatch(session.quoted, review.identity), { outcome: "unconfirmed", transactionHash: session.hash });
    assert.equal(session.paths.filter(path => path === "/v1/wallet/sponsored/submit").length, 1);
  });
}


test(sponsored_batch_reports_receipt);
test(resumed_submission_reports_receipt);
test(completed_sponsored_submission_does_not_answer_a_new_call);
test(stale_sponsored_context_does_not_block_sends);
