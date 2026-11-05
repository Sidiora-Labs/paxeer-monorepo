import { test } from 'vitest';
import { assertWalletSendEvidence } from './support/wallet-send-evidence.js';

async function walletSendReturnsSequencerReceipt(): Promise<void> {
  await assertWalletSendEvidence('receipted');
}

async function walletSendReportsSubmittedWhenReceiptPending(): Promise<void> {
  await assertWalletSendEvidence(null);
}

async function walletSendPreservesSubmittedRung(): Promise<void> {
  await assertWalletSendEvidence('submitted');
}

async function walletSendPreservesCommittedRung(): Promise<void> {
  await assertWalletSendEvidence('committed');
}

test(walletSendReturnsSequencerReceipt, walletSendReturnsSequencerReceipt);
test(
  walletSendReportsSubmittedWhenReceiptPending,
  walletSendReportsSubmittedWhenReceiptPending
);
test(walletSendPreservesSubmittedRung, walletSendPreservesSubmittedRung);
test(walletSendPreservesCommittedRung, walletSendPreservesCommittedRung);
